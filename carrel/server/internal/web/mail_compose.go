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
	Name    string // the file name in the letter
	Media   string
	Size    int64
	AsLink  bool
	LinkID  string // one of the person's live links, kept in the blob
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
// the attach sheet, remove drops a part, link starts a letter with a link
// just made, discard throws the letter away, send sends it.
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
	switch op {
	case "discard":
		saveMailDraft(sess, &mailDraft{})
		s.redirect(w, r, "/app/mail/compose")
		return
	case "link":
		if s.linkToken(sess, r.FormValue("id")) == "" {
			s.notFound(w, r)
			return
		}
		part, ok := s.linkPart(sess, r.FormValue("id"))
		if !ok {
			s.notFound(w, r)
			return
		}
		d = &mailDraft{Parts: []mailPart{part}, Subject: part.Title}
		saveMailDraft(sess, d)
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
		From: text(ls.From), ReplyTo: text(ls.ReplyTo), ReplyNote: ls.replyNote(),
	}
	for _, a := range d.To {
		sheet.To = append(sheet.To, view.Recipient{Address: text(a)})
	}
	if q != "" {
		sheet.Suggest, sheet.NotAsked = s.suggestRecipients(ctx, sess, q)
	}
	links := 0
	for i, p := range d.Parts {
		row := view.MailAttachRow{Kind: p.Kind, Title: text(p.Title), AsLink: p.AsLink,
			Remove: view.Action{ID: "mail.attach.remove", Label: "Remove", Method: "POST", Enabled: true,
				Href: s.postQuery("/app/mail/compose", url.Values{"op": {"remove"}, "i": {strconv.Itoa(i)}})}}
		switch {
		case p.Folder:
			row.Format = "folder"
		case p.Name != "":
			row.Format = text(p.Name[strings.LastIndex(p.Name, ".")+1:])
			row.Size = formatSize(p.Size)
		}
		if p.AsLink {
			links++
		}
		sheet.Attach = append(sheet.Attach, row)
	}

	left := s.users.LettersLeft(u.ID, s.mailPolicy().PerDay)
	limits := []string{
		fmt.Sprintf("%d of %d recipients. Everyone sees the whole list.", len(d.To), s.mailPolicy().Recipients),
		fmt.Sprintf("An attachment up to %s, the letter up to %s; a folder, and anything larger, goes as a link that lives 30 days and can be revoked in Published links.",
			formatSize(s.mailPolicy().Attach), formatSize(s.mailPolicy().Letter)),
		fmt.Sprintf("%s left today.", plural(left, "letter")),
	}
	if links > 0 {
		limits = append(limits, fmt.Sprintf("%s of these go as links; they are made when the letter goes.", plural(links, "attachment")))
	}
	if note != "" {
		limits = append([]string{note}, limits...)
	}
	sheet.Limits = text(strings.Join(limits, " "))

	for _, k := range attachKinds {
		sheet.Actions = append(sheet.Actions, view.Action{ID: "mail.attach." + k.What, Label: text(k.Label), Method: "POST", Enabled: true,
			Href: s.postQuery("/app/mail/compose", url.Values{"op": {"attach"}, "what": {k.What}})})
	}
	send := view.Action{ID: "mail.send", Label: "Send", Method: "POST", Enabled: true,
		Href: s.postQuery("/app/mail/compose", url.Values{"op": {"send"}})}
	switch {
	case len(d.To) == 0:
		send.Enabled, send.Reason = false, "Add a recipient."
	case len(d.To) > s.mailPolicy().Recipients:
		send.Enabled, send.Reason = false, text(fmt.Sprintf("A letter goes to %d recipients at most.", s.mailPolicy().Recipients))
	case left == 0:
		send.Enabled, send.Reason = false, text(user.ErrLetterLimit.Error())
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
			out = append(out, view.Recipient{Address: text(addr), Name: text(name), Label: text(strings.ToLower(v.Params["TYPE"]))})
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

// reckonLinks decides again what goes as a link: a folder always; a file
// over the attachment ceiling; and, in order, whatever would take the letter
// over its own. Run on every change, so the sheet shows the letter that
// will go.
func (s *Server) reckonLinks(parts []mailPart) []mailPart {
	var total int64
	for i := range parts {
		p := &parts[i]
		if p.LinkID != "" || p.Folder {
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

// linkPart is a letter's part for a link made in this session.
func (s *Server) linkPart(sess *session.Session, id string) (mailPart, bool) {
	for _, l := range s.users.PublishedLinks(sess.AccountID()) {
		if l.ID != id {
			continue
		}
		return mailPart{What: "link", Kind: view.Kind(l.Target.Kind), Account: l.Target.AccountID, Col: l.Target.Col,
			UID: l.Target.UID, Path: l.Target.Path, Folder: l.Target.Folder, Title: l.Target.Title, AsLink: true, LinkID: id}, true
	}
	return mailPart{}, false
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
			objs, _, err := c.calQuery(ctx, "VEVENT", nil, nil) // the whole calendar
			if err != nil {
				return nil, err
			}
			writeCalendar(&buf, objs)
			break
		}
		from, okFrom := parseDay(c.tf, p.From)
		to, okTo := parseDay(c.tf, p.To)
		if !okFrom || !okTo {
			return nil, errors.New("no range")
		}
		to = to.AddDate(0, 0, 1)
		objs, _, err := c.calQuery(ctx, "VEVENT", &from, &to)
		if err != nil {
			return nil, err
		}
		writeCalendar(&buf, objs)
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
			writeCalendar(&buf, []*model.Object{obj})
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
		token := ""
		if p.LinkID != "" {
			token = s.linkToken(sess, p.LinkID)
		}
		if token == "" {
			token, err = s.partLink(sess, p)
			if err != nil {
				return 0, fmt.Errorf("The link to «%s» could not be made. Remove it or try again.", p.Title)
			}
		}
		links = append(links, p.Title+" — "+s.absURL(r, "/p/"+token))
		kinds[string(p.Kind)+" link"]++
	}

	ls := s.senderOf(u)
	body := strings.TrimRight(strings.ReplaceAll(d.Body, "\r\n", "\n"), "\n") + "\n\n" + ls.Name + "\n"
	if len(links) > 0 {
		body += "\n" + strings.Join(links, "\n") + "\n"
	}
	if ls.ReplyTo == "" {
		body += "\n-- \nThis letter was sent from Carrel at " + s.publicOrigin(r) +
			". Do not reply to it: the sender's address is not confirmed, and a reply would reach nobody.\n"
	}
	data, err := cmail.Build(s.relay(), cmail.Message{
		To: to, ReplyTo: ls.ReplyTo, Sender: ls.Name, FromLogin: ls.FromLogin,
		Subject: d.Subject, Text: body, Attachments: files,
	}, time.Now())
	if err != nil {
		return 0, errors.New("The letter could not be written: " + strings.TrimPrefix(err.Error(), "mail: "))
	}
	sendCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if _, err := cmail.Send(sendCtx, s.relay(), to, data); err != nil {
		return 0, errors.New("The relay did not take the letter: " + err.Error())
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

// partLink publishes an attached thing for thirty days (S-25.1).
func (s *Server) partLink(sess *session.Session, p mailPart) (string, error) {
	blob := s.catalogOf(sess)
	conn, ok := blob.Connection(p.Account)
	if !ok || conn.Feed {
		return "", errors.New("not the person's connection")
	}
	expires := time.Now().Add(publish.DefaultTTL)
	target := user.PublishTarget{AccountID: p.Account, Kind: string(p.Kind), Col: p.Col, Title: p.Title}
	if p.Kind == view.KindFile {
		target.Path, target.Folder = p.Path, p.Folder
	} else {
		target.UID = p.UID // empty for a book or a calendar: the whole of it
	}
	key, _ := sess.DataKey()
	token, err := s.users.Publish(sess.AccountID(), key, conn, target, &expires)
	if err != nil {
		return "", err
	}
	return token, nil
}
