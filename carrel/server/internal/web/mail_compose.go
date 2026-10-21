package web

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/fanout"
	cmail "carrel.dev/carrel/server/internal/mail"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/publish"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// S-26.2: a person's letter. It is a sheet over the screen, held in the
// session between requests and never on disk: Carrel keeps no copy of a
// letter (C-2). What goes as a file and what as a link is decided when a
// thing is attached, and shown then — a folder always goes as a link, and
// anything over the ceiling of one attachment, or of the letter, does too.
// Links are made when the letter goes, so a letter abandoned leaves none.

// mailDraft is the letter being written.
type mailDraft struct {
	To      []string
	Subject string
	Body    string
	Parts   []mailPart
	// Known are the people the suggestions offered, by address in lower
	// case: one chosen from them is drawn with the name and the book's
	// colour (Пб18; s-mail-1), one typed by hand as its address.
	Known map[string]knownRecipient
}

// knownRecipient is who an address is, as a suggestion said it.
type knownRecipient struct {
	Name, Colour string
}

// mailPart is one thing attached.
type mailPart struct {
	What    string // a key of attachKinds
	Kind    view.Kind
	Account string
	Col     string
	UID     string // one record
	Path    string // a file or folder under the collection root
	Folder  bool
	From    string // a calendar's days, 2006-01-02, both ends in it; empty: the whole calendar
	To      string
	Format  string // "md" or "ics" for a note
	Title   string
	// Detail tells this one from another after its title on the sheet's row
	// (Пб21): an event's day and hour, a contact's organisation, a folder's
	// count. The title alone names a link the letter makes.
	Detail string
	Name   string // the file name in the letter
	Media  string
	Size   int64
	AsLink bool
	// Terms are the publish sheet's form the person applied over the letter
	// to a link it will make (note 70 of Typecase): how long it lives and
	// what the visitor sees. Nil — the letter's own: thirty days, what the
	// sheet gives untouched.
	Terms url.Values
}

// attachKind is one line of the attach menu, in rail order.
type attachKind struct {
	What, Label, Segment string
	Kind                 view.Kind
}

var attachKinds = []attachKind{
	{"contact", "Contact", "contacts", view.KindContact},
	{"book", "Address book", "contacts", view.KindContact},
	{"event", "Event", "calendar", view.KindEvent},
	{"calendar", "Calendar", "calendar", view.KindEvent},
	{"task", "Task", "tasks", view.KindTask},
	{"note", "Note", "notes", view.KindNote},
	{"file", "File", filesSpec.segment, view.KindFile},
	{"folder", "Folder link", filesSpec.segment, view.KindFile},
}

func attachKindOf(what string) (attachKind, bool) {
	for _, k := range attachKinds {
		if k.What == what {
			return k, true
		}
	}
	return attachKind{}, false
}

func loadMailDraft(sess *session.Session) *mailDraft {
	d := &mailDraft{}
	if data := sess.Draft(session.DraftMail); data != nil {
		_ = gob.NewDecoder(bytes.NewReader(data)).Decode(d)
	}
	return d
}

func saveMailDraft(sess *session.Session, d *mailDraft) {
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(d) == nil {
		sess.SetDraft(session.DraftMail, buf.Bytes())
	}
}

// linkToken is one of the person's links whole, from the blob.
func (s *Server) linkToken(sess *session.Session, id string) string {
	key, _ := sess.DataKey()
	return s.users.LinkOf(sess.AccountID(), key, id)
}

// relayOn reports whether the instance's relay sends people's letters (S-26):
// the instance lets them and there is a relay. What depends on the relay
// itself — the Reply-To a letter carries — asks this.
func (s *Server) relayOn() bool {
	return s.mailPolicy().People && s.mailConfigured()
}

// canSendLetters reports whether this person can send a letter at all. Off,
// the sheet is not there — sharing a link by hand still is. Only the relay
// sends today; a mailbox that sends (carrel-mailbox.md M-7) is the second way,
// and every place that offers a letter asks this, not the relay.
func (s *Server) canSendLetters(sess *session.Session) bool {
	return s.relayOn()
}

func (s *Server) writeMailSheet(w http.ResponseWriter, r *http.Request, sheet any) {
	if render.WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

// letterUser is the person writing, as the store has them.
func (s *Server) letterUser(sess *session.Session) *store.User {
	snap := s.store.Snapshot()
	return snap.UserByID(sess.AccountID())
}

func (s *Server) getMailCompose(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if !s.canSendLetters(sess) {
		s.notFound(w, r)
		return
	}
	d := loadMailDraft(sess)
	note := ""
	if n, err := strconv.Atoi(r.URL.Query().Get("sent")); err == nil && n > 0 {
		note = fmt.Sprintf("The letter went to %s.", plural(n, "recipient"))
	}
	s.writeMailSheet(w, r, s.mailSheet(r.Context(), sess, d, strings.TrimSpace(r.URL.Query().Get("q")), note))
}

// postMailCompose keeps what was typed and then does the op: attach opens
// the attach sheet, remove drops a part, discard throws the letter away,
// send sends it. A link given out before is sent by a letter of its own
// («Send this link», mail_link.go): it never joins this one (the owner's
// decision of 21.10.2026).
func (s *Server) postMailCompose(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if !s.canSendLetters(sess) {
		s.notFound(w, r)
		return
	}
	d := loadMailDraft(sess)
	op := r.FormValue("op")
	if op == "discard" {
		saveMailDraft(sess, &mailDraft{})
		s.redirect(w, r, "/app/mail/compose")
		return
	}
	if r.Form.Has("to") || r.Form.Has("subject") || r.Form.Has("body") {
		d.To = splitRecipients(r.Form["to"])
		d.Subject = r.FormValue("subject")
		d.Body = r.FormValue("body")
	}
	switch op {
	case "attach":
		saveMailDraft(sess, d)
		s.redirect(w, r, "/app/mail/attach?what="+url.QueryEscape(r.FormValue("what")))
		return
	case "terms":
		// Apply of the publish sheet over the letter (s-mail-1): the terms
		// of a link the letter makes when it goes; nothing is made now.
		i, err := strconv.Atoi(r.FormValue("i"))
		if err != nil || i < 0 || i >= len(d.Parts) || !d.Parts[i].AsLink {
			s.notFound(w, r)
			return
		}
		terms := url.Values{}
		for k, v := range r.PostForm {
			if k != csrfFormField && k != "op" && k != "i" {
				terms[k] = v
			}
		}
		d.Parts[i].Terms = terms
		saveMailDraft(sess, d)
		s.redirect(w, r, "/app/mail/compose")
		return
	case "remove":
		if i, err := strconv.Atoi(r.FormValue("i")); err == nil && i >= 0 && i < len(d.Parts) {
			d.Parts = append(d.Parts[:i], d.Parts[i+1:]...)
			d.Parts = s.reckonLinks(d.Parts)
		}
	case "send":
		saveMailDraft(sess, d)
		n, err := s.sendLetter(r, sess, d)
		if err != nil {
			s.writeMailSheet(w, r, s.mailSheet(r.Context(), sess, d, "", err.Error()))
			return
		}
		saveMailDraft(sess, &mailDraft{})
		s.redirect(w, r, "/app/mail/compose?sent="+strconv.Itoa(n))
		return
	}
	saveMailDraft(sess, d)
	s.writeMailSheet(w, r, s.mailSheet(r.Context(), sess, d, "", ""))
}

// splitRecipients takes the field as typed — one address a value, or
// several with commas or semicolons — and keeps each once.
func splitRecipients(values []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range values {
		for _, a := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
			a = strings.TrimSpace(a)
			if a != "" && !seen[strings.ToLower(a)] {
				seen[strings.ToLower(a)] = true
				out = append(out, a)
			}
		}
	}
	return out
}

// publishedLinksMark stands for «Published links» in the limits until the
// sentence is cut into pieces.
const publishedLinksMark = "{published-links}"

// mailSheet is the letter as it will go: who it is from and where replies
// go, the recipients, what goes as a file and what as a link.
func (s *Server) mailSheet(ctx context.Context, sess *session.Session, d *mailDraft, q, note string) view.MailSheet {
	u := s.letterUser(sess)
	if u == nil {
		return view.MailSheet{}
	}
	ls := s.senderOf(u)
	sheet := view.MailSheet{
		Subject: text(d.Subject), Body: text(d.Body),
		From: text(ls.Address), FromName: text(ls.FromName), ReplyTo: text(ls.ReplyTo), ReplyNote: ls.replyNote(),
	}
	if q != "" {
		sheet.Suggest, sheet.NotAsked = s.suggestRecipients(ctx, sess, q)
		if len(sheet.Suggest) > 0 {
			if d.Known == nil {
				d.Known = map[string]knownRecipient{}
			}
			for _, r := range sheet.Suggest {
				d.Known[strings.ToLower(string(r.Address))] = knownRecipient{Name: string(r.Name), Colour: string(r.Colour)}
			}
			saveMailDraft(sess, d)
		}
	}
	for _, a := range d.To {
		r := view.Recipient{Address: text(a)}
		if k, ok := d.Known[strings.ToLower(a)]; ok {
			r.Name, r.Colour = text(k.Name), view.Colour(k.Colour)
		}
		sheet.To = append(sheet.To, r)
	}
	links := 0
	var why []string
	for i, p := range d.Parts {
		title := p.Title
		if p.Detail != "" {
			title += " · " + p.Detail
		}
		row := view.MailAttachRow{Kind: p.Kind, Title: text(title), AsLink: p.AsLink,
			Remove: view.Action{ID: "mail.attach.remove", Label: "Remove", Method: "POST", Enabled: true,
				Href: s.postQuery("/app/mail/compose", url.Values{"op": {"remove"}, "i": {strconv.Itoa(i)}})}}
		switch {
		case p.Folder:
			row.Format = "folder"
		case p.Name != "":
			// A file's format is in its name (s-mail-1: «смета-подрядчик.pdf ·
			// 4.2 MB»); a record says the format it goes in («.vcf · 3 KB»).
			if p.Kind != view.KindFile {
				row.Format = text(p.Name[strings.LastIndex(p.Name, ".")+1:])
			}
			row.Size = formatSize(p.Size)
		}
		if p.AsLink {
			links++
			why = append(why, s.linkReason(p))
		}
		// A link the letter makes says its terms, and «Change…» opens the
		// publish sheet over the letter with them (note 70 of Typecase).
		if p.AsLink {
			row.Terms = text(livesWords(partLives(p)))
			row.Change = &view.Action{ID: "mail.link.terms", Label: "Change…", Method: "GET", Enabled: true,
				Href: s.route("/app/publish?" + partPublishQuery(p, i).Encode())}
		}
		sheet.Attach = append(sheet.Attach, row)
	}
	sheet.Links = linksLine(why)

	left := s.users.LettersLeft(u.ID, s.mailPolicy().PerDay)
	limits := []string{
		fmt.Sprintf("%d of %d recipients. Everyone sees the whole list.", len(d.To), s.mailPolicy().Recipients),
		fmt.Sprintf("An attachment up to %s, the letter up to %s; a folder, and anything larger, goes as a link that lives 30 days and can be revoked in %s.",
			formatSize(s.mailPolicy().Attach), formatSize(s.mailPolicy().Letter), publishedLinksMark),
		fmt.Sprintf("%s left today.", plural(left, "letter")),
	}
	if note != "" {
		limits = append([]string{note}, limits...)
	}
	// «Published links» is bold (s-mail-1): the place the links are
	// revoked, named as the rail names it.
	before, after, _ := strings.Cut(strings.Join(limits, " "), publishedLinksMark)
	sheet.Limits = join(before, strongRun("Published links"), after)

	for _, k := range attachKinds {
		sheet.Actions = append(sheet.Actions, view.Action{ID: "mail.attach." + k.What, Label: text(k.Label), Method: "POST", Enabled: true,
			Href: s.postQuery("/app/mail/compose", url.Values{"op": {"attach"}, "what": {k.What}})})
	}
	send := view.Action{ID: "mail.send", Label: "Send", Method: "POST", Primary: true, Enabled: true,
		Href: s.postQuery("/app/mail/compose", url.Values{"op": {"send"}})}
	switch {
	case len(d.To) == 0:
		send.Enabled, send.Reason = false, "Add a recipient."
	case len(d.To) > s.mailPolicy().Recipients:
		send.Enabled, send.Reason = false, text(fmt.Sprintf("A letter goes to %d recipients at most.", s.mailPolicy().Recipients))
	case left == 0:
		send.Enabled, send.Reason = false, text(user.ErrLetterLimit.Error())
	default:
		if err := s.givesTooMuch(s.reckonLinks(slices.Clone(d.Parts))); err != nil {
			send.Enabled, send.Reason = false, text(err.Error())
		}
	}
	sheet.Actions = append(sheet.Actions, send,
		view.Action{ID: "mail.discard", Label: "Discard", Method: "POST", Enabled: true, Danger: true,
			Href: s.postQuery("/app/mail/compose", url.Values{"op": {"discard"}})})
	return sheet
}

// suggestRecipients asks the ticked address books for q, the same fanout as
// search (S-15), and offers every address of every card found — one row an
// address, labelled from the card. Books that did not answer are named: a
// silent list would read as "nobody by that name".
func (s *Server) suggestRecipients(ctx context.Context, sess *session.Session, q string) ([]view.Recipient, []view.Stamp) {
	blob := s.catalogOf(sess)
	var sources []fanout.Source
	for _, pick := range blob.Selected {
		conn, hasConn := blob.Connection(pick.AccountID)
		col, hasCol := blob.Collection(pick.AccountID, pick.Path)
		if pick.Kind != view.KindContact || !hasConn || !hasCol || col.Gone {
			continue
		}
		src := s.openSource(sess, conn.ID+":"+col.Path, conn, col)
		src.kinds = []view.Kind{view.KindContact}
		sources = append(sources, src)
	}
	if len(sources) == 0 {
		return nil, nil
	}
	opts := s.searchOptions(sess)
	opts.Finished = nil // a suggestion is not a search of the history
	id, err := user.NewTaskID()
	if err != nil {
		return nil, nil
	}
	task, err := fanout.Start(id, fanout.Query{Text: q}, sources, opts)
	if err != nil {
		return nil, nil
	}
	for task.Running() {
		changed := task.Changed()
		if !task.Running() {
			break
		}
		select {
		case <-changed:
		case <-ctx.Done():
			task.Cancel()
		}
	}
	snap := task.Snapshot()
	var out []view.Recipient
	seen := map[string]bool{}
	for _, it := range snap.Items {
		name := contactName(it.Object)
		for _, v := range it.Object.Props()["EMAIL"] {
			addr := strings.TrimSpace(v.Text)
			if addr == "" || seen[strings.ToLower(addr)] {
				continue
			}
			seen[strings.ToLower(addr)] = true
			// The label the card shows (S-11), not the raw TYPE: INTERNET and
			// the like are no label.
			label, _ := shownLabel("EMAIL", v.Params["TYPE"])
			r := view.Recipient{Address: text(addr), Name: text(name), Label: text(strings.ToLower(label))}
			// The book's mark before the name (s-mail-1, -3).
			if col, ok := blob.Collection(it.AccountID, it.Col); ok {
				r.Colour = catalogColour(col)
			}
			out = append(out, r)
		}
	}
	var notAsked []view.Stamp
	for _, st := range snap.Sources {
		if st.State == view.SourceDone || st.State == view.SourceEmpty {
			continue
		}
		account, path, _ := strings.Cut(st.ID, ":")
		origin, _ := sourceOrigin(blob, account, path)
		notAsked = append(notAsked, origin)
	}
	return out, notAsked
}

// givesTooMuch is what a letter must never give out more of than the person
// chose, said before anything is read or made — and on the sheet before Send
// is pressed (Б37; Пб39): Send is off with it as its reason.
func (s *Server) givesTooMuch(parts []mailPart) error {
	for _, p := range parts {
		if p.AsLink && p.From != "" {
			// A link is the calendar whole; the person chose some of its
			// days.
			return fmt.Errorf("«%s» does not fit the letter, and a link would open the whole calendar, not these days. Remove it, or attach fewer days.", p.Title)
		}
	}
	return nil
}

// reckonLinks decides again what goes as a link: a folder always; a file
// over the attachment ceiling; and, in order, whatever would take the letter
// over its own. Run on every change, so the sheet shows the letter that
// will go.
func (s *Server) reckonLinks(parts []mailPart) []mailPart {
	var total int64
	for i := range parts {
		p := &parts[i]
		if p.Folder {
			p.AsLink = true
			continue
		}
		p.AsLink = p.Size > s.mailPolicy().Attach || total+p.Size > s.mailPolicy().Letter
		if !p.AsLink {
			total += p.Size
		}
	}
	return parts
}

// partContent is what an attached thing is as a file: its exchange format,
// read now.
func (s *Server) partContent(ctx context.Context, sess *session.Session, p mailPart) ([]byte, error) {
	k, ok := attachKindOf(p.What)
	if !ok {
		return nil, errors.New("nothing to read")
	}
	if k.Kind == view.KindFile {
		fc, ok := s.fileCollection(sess, p.Account, p.Col)
		if !ok {
			return nil, errors.New("the collection is not there")
		}
		segs, ok := relSegs(p.Path)
		if !ok {
			return nil, errors.New("no such file")
		}
		path, err := fc.path(segs, false)
		if err != nil {
			return nil, err
		}
		body, _, err := fc.files.Get(ctx, path, nil)
		if err != nil {
			return nil, err
		}
		defer body.Close()
		return io.ReadAll(io.LimitReader(body, s.mailPolicy().Attach+1))
	}
	c, ok := s.collectionOf(sess, recordKinds[k.Segment], p.Account, p.Col)
	if !ok || c.col.Gone {
		return nil, errors.New("the collection is not there")
	}
	var buf bytes.Buffer
	switch p.What {
	case "book":
		objs, err := c.contactsAll(ctx)
		if err != nil {
			return nil, err
		}
		for _, o := range objs {
			if err := card.Encode(&buf, o); err != nil {
				return nil, err
			}
		}
	case "calendar":
		if p.From == "" && p.To == "" {
			objs, bad, err := c.calQuery(ctx, "VEVENT", nil, nil) // the whole calendar
			if err != nil {
				return nil, err
			}
			// An event Carrel could not read goes as the server holds it;
			// a calendar that would go without one does not go (unreadKept).
			raw, err := c.unreadKept(ctx, bad)
			if err != nil {
				return nil, err
			}
			writeCalendar(&buf, objs, raw)
			break
		}
		from, okFrom := parseDay(c.tf, p.From)
		to, okTo := parseDay(c.tf, p.To)
		if !okFrom || !okTo {
			return nil, errors.New("no range")
		}
		to = to.AddDate(0, 0, 1)
		objs, bad, err := c.calQuery(ctx, "VEVENT", &from, &to)
		if err != nil {
			return nil, err
		}
		raw, err := c.unreadKept(ctx, bad)
		if err != nil {
			return nil, err
		}
		writeCalendar(&buf, objs, raw)
	default:
		obj, err := c.find(ctx, p.UID)
		if err != nil {
			return nil, err
		}
		switch {
		case p.What == "contact":
			err = card.Encode(&buf, obj)
		case p.What == "note" && p.Format == "md":
			err = cal.NoteMarkdown(&buf, obj)
		default:
			writeCalendar(&buf, []*model.Object{obj}, nil)
		}
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// sendLetter checks the letter against the ceilings, reads what goes as
// files, makes the links, and hands it to the relay. It returns the number
// of recipients.
func (s *Server) sendLetter(r *http.Request, sess *session.Session, d *mailDraft) (int, error) {
	ctx := r.Context()
	u := s.letterUser(sess)
	if u == nil {
		return 0, errors.New("no account")
	}
	to, err := cmail.Addresses(d.To)
	switch {
	case err != nil:
		return 0, errors.New("A recipient is not an address: " + strings.TrimPrefix(err.Error(), "mail: "))
	case len(to) == 0:
		return 0, errors.New("Add a recipient.")
	case len(to) > s.mailPolicy().Recipients:
		return 0, fmt.Errorf("A letter goes to %d recipients at most.", s.mailPolicy().Recipients)
	}
	d.Parts = s.reckonLinks(d.Parts)
	if err := s.givesTooMuch(d.Parts); err != nil {
		return 0, err
	}

	var files []cmail.Attachment
	var links []string
	kinds := map[string]int{}
	var total int64
	for _, p := range d.Parts {
		if p.AsLink {
			continue
		}
		data, err := s.partContent(ctx, sess, p)
		if err != nil {
			return 0, fmt.Errorf("«%s» could not be read: %s. Remove it or try again.", p.Title, sentence(err.Error()))
		}
		if int64(len(data)) > s.mailPolicy().Attach || total+int64(len(data)) > s.mailPolicy().Letter {
			return 0, fmt.Errorf("«%s» has grown since it was attached and no longer fits the letter. Attach it again: it will go as a link.", p.Title)
		}
		total += int64(len(data))
		files = append(files, cmail.Attachment{Name: p.Name, Type: p.Media, Data: data})
		kinds[string(p.Kind)]++
	}

	if err := s.users.ReserveLetter(u.ID, s.mailPolicy().PerDay); err != nil {
		return 0, err
	}
	sent := false
	defer func() {
		if !sent {
			s.users.ReleaseLetter(u.ID)
		}
	}()
	// Links last, after everything that can refuse: a letter that does not
	// go leaves no link behind it, save the ones already asked for below.
	for _, p := range d.Parts {
		if !p.AsLink {
			continue
		}
		token, err := s.partLink(sess, p)
		if err != nil {
			return 0, fmt.Errorf("The link to «%s» could not be made. Remove it or try again.", p.Title)
		}
		links = append(links, p.Title+" — "+s.absURL(r, "/p/"+token))
		kinds[string(p.Kind)+" link"]++
	}

	if err := s.relayLetter(r, s.senderOf(u), to, d.Subject, d.Body, links, files); err != nil {
		return 0, err
	}
	sent = true
	var attached []string
	for _, k := range attachKinds {
		for _, suffix := range []string{"", " link"} {
			if n := kinds[string(k.Kind)+suffix]; n > 0 {
				attached = append(attached, fmt.Sprintf("%s%s ×%d", k.Kind, suffix, n))
				delete(kinds, string(k.Kind)+suffix)
			}
		}
	}
	s.users.LetterSent(u.ID, to, strings.Join(attached, ", "))
	return len(to), nil
}

// relayLetter writes a letter and hands it to the relay: what the person
// wrote, signed with their name, the links each on a line of its own, and —
// without an address to answer — a foot that says a reply reaches nobody.
func (s *Server) relayLetter(r *http.Request, ls letterSender, to []string, subject, typed string, links []string, files []cmail.Attachment) error {
	body := strings.TrimRight(strings.ReplaceAll(typed, "\r\n", "\n"), "\n") + "\n\n" + ls.Name + "\n"
	if len(links) > 0 {
		body += "\n" + strings.Join(links, "\n") + "\n"
	}
	if ls.ReplyTo == "" {
		body += "\n-- \nThis letter was sent from Carrel at " + s.publicOrigin(r) +
			". Do not reply to it: the sender's address is not confirmed, and a reply would reach nobody.\n"
	}
	data, err := cmail.Build(s.relay(), cmail.Message{
		To: to, ReplyTo: ls.ReplyTo, Sender: ls.Name, FromLogin: ls.FromLogin,
		Subject: subject, Text: body, Attachments: files,
	}, time.Now())
	if err != nil {
		return errors.New("The letter could not be written: " + strings.TrimPrefix(err.Error(), "mail: "))
	}
	sendCtx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if _, err := cmail.Send(sendCtx, s.relay(), to, data); err != nil {
		return errors.New("The relay did not take the letter: " + err.Error())
	}
	s.markSent()
	return nil
}

// partLink publishes an attached thing for thirty days (S-25.1).
func (s *Server) partLink(sess *session.Session, p mailPart) (string, error) {
	blob := s.catalogOf(sess)
	conn, ok := blob.Connection(p.Account)
	if !ok || conn.Feed {
		return "", errors.New("not the person's connection")
	}
	expires := time.Now().Add(publish.DefaultTTL)
	expiry := &expires
	target := user.PublishTarget{AccountID: p.Account, Kind: string(p.Kind), Col: p.Col, Title: p.Title}
	if p.Kind == view.KindFile {
		target.Path, target.Folder = p.Path, p.Folder
	} else {
		target.UID = p.UID // empty for a book or a calendar: the whole of it
	}
	if p.Terms != nil {
		// The terms applied over the letter (note 70 of Typecase).
		expiry = s.expiryOf(p.Terms, "30")
		publishChoices(p.Terms, p.Kind, p.UID, &target)
	} else {
		// Left as they are, the link gives what the publish sheet gives
		// untouched (Пб38).
		sheetDefaults(p.Kind, &target)
	}
	key, _ := sess.DataKey()
	token, err := s.users.Publish(sess.AccountID(), key, conn, target, expiry)
	if err != nil {
		return "", err
	}
	return token, nil
}

// partLives is how long the link of a letter's part lives: what was applied
// over the letter, else the letter's thirty days.
func partLives(p mailPart) string {
	if v := p.Terms.Get("lives"); v != "" {
		return v
	}
	return "30"
}

// partPublishQuery is the publish sheet of a part the letter sends as a
// link, over the letter: mail names the part (note 70 of Typecase).
func partPublishQuery(p mailPart, i int) url.Values {
	var q url.Values
	if p.Kind == view.KindFile {
		q = url.Values{"k": {filesSpec.segment}, "a": {p.Account}, "c": {p.Col}, "f": {p.Path}}
		if p.Folder {
			q.Set("dir", "1")
		}
	} else {
		q = publishQuery(segmentOf(p.Kind), p.Account, p.Col, p.UID)
	}
	q.Set("mail", strconv.Itoa(i))
	return q
}

// linkReason is why a part goes as a link, as the line under the list says
// it (s-mail-1): a folder always; a file over the attachment's ceiling; one
// the letter has no room left for.
func (s *Server) linkReason(p mailPart) string {
	switch {
	case p.Folder:
		return "a folder always goes as a link"
	case p.Size > s.mailPolicy().Attach:
		what := "«" + p.Title + "»"
		if i := strings.LastIndex(p.Name, "."); i >= 0 && i < len(p.Name)-1 {
			what = "the " + strings.ToUpper(p.Name[i+1:])
		}
		return what + " is over the " + string(formatSize(s.mailPolicy().Attach)) + " ceiling"
	}
	return "«" + p.Title + "» would take the letter over " + string(formatSize(s.mailPolicy().Letter))
}

// linksLine is the line under the list when something goes as a link
// (s-mail-1): how many and why, and where they are revoked — «Published
// links» in bold. Nil without links.
func linksLine(why []string) view.Runs {
	if len(why) == 0 {
		return nil
	}
	var reasons []string
	for _, w := range why {
		if !slices.Contains(reasons, w) {
			reasons = append(reasons, w)
		}
	}
	said := reasons[0]
	if n := len(reasons); n > 1 {
		said = strings.Join(reasons[:n-1], ", ") + ", and " + reasons[n-1]
	}
	n := countWord(len(why))
	head := strings.ToUpper(n[:1]) + n[1:] + " of these go as links, not as files: " + said + ". Each link can be revoked in "
	if len(why) == 1 {
		head = "One of these goes as a link, not as a file: " + said + ". The link can be revoked in "
	}
	return join(head, strongRun("Published links"), " — including for the people in this letter.")
}
