package web

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
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
	// Back is the page the letter was opened from: the screen under it, its
	// Cancel, and where its attach sheets come back to (С21).
	Back string
	// Typed is what stands in «Name or address» and is no address yet: kept
	// with its suggestions until one is chosen.
	Typed string
	// Refused are those the relay would not take when the letter went, with
	// its words (s-mail-18): the letter is kept for «Send again to the
	// refused», and is gone once anything else is opened.
	Refused []cmail.Refusal
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
	// Token is the link made when the letter went: sent again to the
	// refused, it carries the same one.
	Token string
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
	if len(d.Refused) > 0 {
		// The letter went to some (s-mail-18): «Send again to the refused»
		// writes it to them; opened any other way, it is a new letter.
		if r.URL.Query().Get("to") == "refused" {
			d.To = nil
			for _, rf := range d.Refused {
				d.To = append(d.To, rf.Address)
			}
			d.Refused = nil
		} else {
			d = &mailDraft{Back: d.Back}
		}
		saveMailDraft(sess, d)
	}
	if back := r.URL.Query().Get("back"); back != "" {
		// Opened from a page (С21): the letter stands over it and goes back
		// to it.
		if p := s.sheetBack(r, back, false); p != "" && p != d.Back {
			d.Back = p
			saveMailDraft(sess, d)
		}
	}
	s.writeMailSheet(w, r, s.mailSheet(r, sess, d, "", ""))
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
		// «Close and lose the text» (s-mail-15): Carrel keeps no drafts.
		saveMailDraft(sess, &mailDraft{})
		s.redirect(w, r, mailBack(d))
		return
	}
	// What was typed goes with every press and is kept (s-mail-1): the
	// recipients picked, the line typed after them, the subject, the text.
	if r.Form.Has("to") || r.Form.Has("to.add") || r.Form.Has("subject") || r.Form.Has("text") {
		d.To = splitRecipients(r.Form["to"])
		d.Subject = r.FormValue("subject")
		d.Body = r.FormValue("text")
		d.Typed = strings.TrimSpace(r.FormValue("to.add"))
	}
	switch op {
	case "add":
		// Enter in «Name or address» without a script (Typecase's note 110
		// p. 3): an address joins the recipients; a name is looked up and
		// its addresses offered, the line kept.
		if typed := d.Typed; typed != "" {
			if a, found := s.typedRecipient(sess, d, typed); found {
				d.To, d.Typed = splitRecipients(append(d.To, a)), ""
				saveMailDraft(sess, d)
				s.writeMailSheet(w, r, s.mailSheet(r, sess, d, "", ""))
				return
			}
		}
		saveMailDraft(sess, d)
		s.writeMailSheet(w, r, s.mailSheet(r, sess, d, "", ""))
		return
	case "attach":
		saveMailDraft(sess, d)
		s.redirect(w, r, "/app/mail/attach?what="+url.QueryEscape(r.FormValue("what")))
		return
	case "change":
		// «Change…» of a link the letter makes (С16): what was typed kept,
		// the publish sheet over the letter with its terms.
		i, err := strconv.Atoi(r.FormValue("i"))
		if err != nil || i < 0 || i >= len(d.Parts) || !d.Parts[i].AsLink {
			s.notFound(w, r)
			return
		}
		saveMailDraft(sess, d)
		s.redirect(w, r, "/app/publish?"+partPublishQuery(d.Parts[i], i).Encode())
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
		d.Parts[i].Terms, d.Parts[i].Token = terms, ""
		saveMailDraft(sess, d)
		s.redirect(w, r, "/app/mail/compose")
		return
	case "remove":
		if i, err := strconv.Atoi(r.FormValue("i")); err == nil && i >= 0 && i < len(d.Parts) {
			d.Parts = append(d.Parts[:i], d.Parts[i+1:]...)
			d.Parts = s.reckonLinks(d.Parts)
		}
	case "close":
		// Cancel sends the form (s-mail-1, -14): nothing typed, the letter
		// closes and is forgotten; typed, it asks first (s-mail-15).
		if strings.TrimSpace(d.Body) == "" {
			saveMailDraft(sess, &mailDraft{})
			s.redirect(w, r, mailBack(d))
			return
		}
		saveMailDraft(sess, d)
		s.writeMailSheet(w, r, s.mailCloseSheet(r, sess, d))
		return
	case "preview", "", "send":
		// Send is the form's own button, no op (s-mail-1).
		if d.Typed != "" {
			// A line typed and not yet a recipient: added, as Enter would.
			if a, found := s.typedRecipient(sess, d, d.Typed); found {
				d.To, d.Typed = splitRecipients(append(d.To, a)), ""
			}
		}
		saveMailDraft(sess, d)
		if d.Typed != "" {
			s.writeMailSheet(w, r, s.mailSheet(r, sess, d, "Choose who «"+d.Typed+"» is, or type an address.", ""))
			return
		}
		if op == "preview" {
			s.writeMailSheet(w, r, s.mailPreviewSheet(r, sess, d))
			return
		}
		taken, refused, err := s.sendLetter(r, sess, d)
		switch {
		case err != nil:
			// It did not go (s-mail-17): the letter as it was, why first
			// under To.
			saveMailDraft(sess, d)
			s.writeMailSheet(w, r, s.mailSheet(r, sess, d, "", err.Error()))
		case len(refused) > 0:
			// It went to some (s-mail-18): the letter is kept for those it
			// did not reach.
			d.Refused = refused
			saveMailDraft(sess, d)
			s.writeMailSheet(w, r, s.mailSentSheet(r, sess, d, taken))
		default:
			// It went (s-mail-16): the sheet closes, the screen under it
			// says so once.
			saveMailDraft(sess, &mailDraft{})
			sayOnNextPage(sess, fmt.Sprintf("The letter went to %s.", plural(len(taken), "recipient")))
			s.redirect(w, r, mailBack(d))
		}
		return
	}
	saveMailDraft(sess, d)
	s.writeMailSheet(w, r, s.mailSheet(r, sess, d, "", ""))
}

// mailBack is where the letter closes to: the page it was opened over, the
// contacts without one (s-mail-1).
func mailBack(d *mailDraft) string {
	if d.Back == "" {
		return "/app/contacts"
	}
	return d.Back
}

// typedRecipient is the address a line typed in «Name or address» names
// without scripts: an address as it is; else the one address of the
// ticked books whose person's name holds the words — that one known to the
// letter by the name and the book's mark (Пб18).
func (s *Server) typedRecipient(sess *session.Session, d *mailDraft, typed string) (string, bool) {
	people := s.bookPeople(sess)
	if a, err := cmail.Addresses([]string{typed}); err == nil && len(a) == 1 {
		// An address the books know is the person's, named and marked as
		// one picked from the suggestions (s-mail-1).
		for _, p := range people {
			if strings.EqualFold(p.email, a[0]) {
				if d.Known == nil {
					d.Known = map[string]knownRecipient{}
				}
				d.Known[strings.ToLower(a[0])] = knownRecipient{Name: p.name, Colour: string(p.colour)}
				break
			}
		}
		return a[0], true
	}
	needle := strings.ToLower(typed)
	var found []bookPerson
	for _, p := range people {
		if strings.Contains(strings.ToLower(p.name), needle) && !slices.ContainsFunc(found, func(f bookPerson) bool { return f.email == p.email }) {
			found = append(found, p)
		}
	}
	if len(found) != 1 {
		return "", false
	}
	if d.Known == nil {
		d.Known = map[string]knownRecipient{}
	}
	d.Known[strings.ToLower(found[0].email)] = knownRecipient{Name: found[0].name, Colour: string(found[0].colour)}
	return found[0].email, true
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

// mailSheet is the letter as it will go, one form (s-mail-1): who it is from
// and where replies go, the recipients, what goes as a file and what as a
// link. note is said first under the recipients: what the last press did;
// failed — why the letter did not go, said there as an alarm (s-mail-17).
func (s *Server) mailSheet(r *http.Request, sess *session.Session, d *mailDraft, note, failed string) view.MailSheet {
	u := s.letterUser(sess)
	if u == nil {
		return view.MailSheet{}
	}
	ls := s.senderOf(u)
	sheet := view.MailSheet{
		From: text(ls.Address), FromName: text(ls.FromName), ReplyTo: text(ls.ReplyTo), ReplyNote: ls.replyNote(),
	}
	if d.Typed != "" {
		// A name typed without a script (op=add): its addresses offered.
		sheet.Suggest, sheet.NotAsked, _ = s.suggestRecipients(r.Context(), sess, d.Typed)
		if len(sheet.Suggest) > 0 {
			if d.Known == nil {
				d.Known = map[string]knownRecipient{}
			}
			for _, rc := range sheet.Suggest {
				d.Known[strings.ToLower(string(rc.Address))] = knownRecipient{Name: string(rc.Name), Colour: string(rc.Colour)}
			}
			saveMailDraft(sess, d)
		}
	}
	// The recipients as tags (s-mail-1): the name and the address after it,
	// the book's mark; one typed by hand, its address alone.
	to := view.FormRow{Name: "to", Label: "To", Kind: view.InputSelect, Multiple: true, Suggest: s.route("/app/mail/suggest"),
		Add: &view.FormRow{Name: "to.add", Label: "Add a recipient", Kind: view.InputText, Placeholder: "Name or address", Value: text(d.Typed)}}
	for _, a := range d.To {
		o := view.Option{Value: a, Label: text(a), Selected: true, Typed: true}
		if k, ok := d.Known[strings.ToLower(a)]; ok && k.Name != "" {
			o.Label, o.Meta, o.Colour, o.Typed = text(k.Name), text(a), view.Colour(k.Colour), false
		}
		to.Options = append(to.Options, o)
	}
	if len(to.Options) > 0 {
		to.Value = text(to.Options[0].Value)
	}
	// Under To, how many at most and that all see all (s-mail-1); what the
	// last press did, first; why the letter did not go, first in alarm
	// (s-mail-17).
	hint := fmt.Sprintf("Up to %d recipients. Each sees the others' addresses.", s.mailPolicy().Recipients)
	if note != "" {
		hint = sentence(note) + " " + hint
	}
	to.Hint = text(hint)
	if failed != "" {
		to.Hint = text(sentence(failed) + " " + hint)
		to.HintRuns = view.Runs{{Text: text(sentence(failed)), Mark: view.RunDanger}, {Text: text(" " + hint)}}
	}
	sheet.Form = s.plainForm("/app/mail/send", "POST", csrfField(sess), to,
		view.FormRow{Name: "subject", Label: "Subject", Kind: view.InputText, Value: text(d.Subject)},
		view.FormRow{Name: "text", Label: "Text", Kind: view.InputArea, Value: text(d.Body)})
	submit := func(q url.Values) string { return s.postQuery("/app/mail/send", q) }

	var why []string
	for i, p := range d.Parts {
		title := p.Title
		if p.Detail != "" {
			title += " · " + p.Detail
		}
		row := view.MailAttachRow{Kind: p.Kind, Title: text(title), AsLink: p.AsLink,
			Remove: view.Action{ID: "mail.attach.remove", Label: "Remove", Method: "POST", Enabled: true, Submits: true,
				Href: submit(url.Values{"op": {"remove"}, "i": {strconv.Itoa(i)}})}}
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
		// A link the letter makes says its terms, and «Change…» opens the
		// publish sheet over the letter with them, what was typed kept (note
		// 70 of Typecase; С16).
		if p.AsLink {
			why = append(why, s.linkReason(p))
			row.Terms = text(livesWords(partLives(p)))
			row.Change = &view.Action{ID: "mail.link.terms", Label: "Change…", Method: "POST", Enabled: true, Submits: true,
				Href: submit(url.Values{"op": {"change"}, "i": {strconv.Itoa(i)}})}
		}
		sheet.Attach = append(sheet.Attach, row)
	}
	sheet.Links = linksLine(why)

	for _, k := range attachKinds {
		sheet.Actions = append(sheet.Actions, view.Action{ID: "mail.attach." + k.What, Label: text(k.Label), Method: "POST", Enabled: true, Submits: true,
			Href: submit(url.Values{"op": {"attach"}, "what": {k.What}})})
	}
	send := view.Action{ID: "mail.send", Label: "Send", Icon: "i-mail", Method: "POST", Primary: true, Enabled: true, Submits: true, Href: s.route("/app/mail/send")}
	left := s.users.LettersLeft(u.ID, s.mailPolicy().PerDay)
	switch {
	case len(d.To) == 0 && d.Typed == "":
		send.Enabled, send.Reason = false, "Add a recipient."
	case len(d.To) > s.mailPolicy().Recipients:
		send.Enabled, send.Reason = false, text(fmt.Sprintf("A letter goes to %d recipients at most.", s.mailPolicy().Recipients))
	case left == 0:
		send.Enabled, send.Reason = false, text(user.ErrLetterLimit.Error())
	}
	sheet.Form.Actions = []view.Action{send,
		// Preview keeps what was typed and shows the letter as the
		// recipients get it (s-mail-14).
		{ID: "mail.preview", Label: "Preview", Method: "POST", Enabled: true, Submits: true, Href: submit(url.Values{"op": {"preview"}})},
		// Cancel sends what was typed (s-mail-1): with text, the sheet asks
		// before it closes (s-mail-15).
		{ID: "mail.cancel", Label: "Cancel", Method: "POST", Enabled: true, Submits: true, Href: submit(url.Values{"op": {"close"}})},
	}
	sheet.Back = s.route(mailBack(d))
	sheet.Under = s.underOf(r, d.Back, "/app/contacts")
	return sheet
}

// mailPreviewSheet is the letter as the recipients get it (s-mail-14): the
// headers with the names, the text with the signature, the files, the links
// — not made yet, so said by where they will be — and the line a letter
// without an address to answer closes with.
func (s *Server) mailPreviewSheet(r *http.Request, sess *session.Session, d *mailDraft) view.MailPreviewSheet {
	u := s.letterUser(sess)
	if u == nil {
		return view.MailPreviewSheet{}
	}
	ls := s.senderOf(u)
	address := func(a string) view.Run { return view.Run{Text: text("<" + a + ">"), Mark: view.RunMono} }
	sheet := view.MailPreviewSheet{
		From:    view.Runs{{Text: text(ls.FromName), Mark: view.RunStrong}, {Text: " "}, address(ls.Address)},
		Subject: text(d.Subject),
	}
	if ls.ReplyTo != "" {
		sheet.ReplyTo = view.Runs{{Text: text(ls.Name + " ")}, address(ls.ReplyTo)}
	} else {
		sheet.NoReply = text(s.noReplyLine(r))
	}
	for i, a := range d.To {
		if i > 0 {
			sheet.To = append(sheet.To, view.Run{Text: ", "})
		}
		if k, ok := d.Known[strings.ToLower(a)]; ok && k.Name != "" {
			sheet.To = append(sheet.To, view.Run{Text: text(k.Name + " ")}, address(a))
		} else {
			sheet.To = append(sheet.To, view.Run{Text: text(a)})
		}
	}
	parts := s.reckonLinks(slices.Clone(d.Parts))
	where := strings.TrimPrefix(strings.TrimPrefix(s.absURL(r, "/p/"), "https://"), "http://")
	for _, p := range parts {
		if !p.AsLink {
			sheet.Files = append(sheet.Files, view.MailPreviewFile{Name: text(p.Name), Kind: p.Kind})
			continue
		}
		token := p.Token
		if token == "" {
			token = "…"
		}
		sheet.Links = append(sheet.Links, view.MailPreviewLink{Title: text(linkTitle(p)), Address: text(where + token)})
	}
	sheet.Text = text(strings.TrimSuffix(letterText(d.Body, ls.Name), "\n"))
	sheet.Form = s.plainForm("/app/mail/send", "POST", csrfField(sess))
	sheet.Form.Actions = []view.Action{
		{ID: "mail.send", Label: "Send", Icon: "i-mail", Method: "POST", Primary: true, Enabled: true, Submits: true, Href: s.route("/app/mail/send")},
		{ID: "mail.edit", Label: "Edit", Method: "GET", Enabled: true, Href: s.route("/app/mail/compose")},
		{ID: "mail.cancel", Label: "Cancel", Method: "POST", Enabled: true, Submits: true, Href: s.postQuery("/app/mail/send", url.Values{"op": {"close"}})},
	}
	sheet.Under = s.underOf(r, d.Back, "/app/contacts")
	return sheet
}

// mailCloseSheet asks before a letter with text closes (s-mail-15): Carrel
// keeps no drafts, so closing loses the text.
func (s *Server) mailCloseSheet(r *http.Request, sess *session.Session, d *mailDraft) view.MailCloseSheet {
	sheet := view.MailCloseSheet{
		Warning: view.Runs{{Text: "Carrel keeps no drafts: "}, {Text: "closing loses the text", Mark: view.RunStrong}, {Text: "."}},
		Form:    s.plainForm("/app/mail/send?op=discard", "POST", csrfField(sess)),
	}
	if u := s.letterUser(sess); u != nil {
		sheet.From = text(s.senderOf(u).Address)
	}
	sheet.Form.Actions = []view.Action{
		{ID: "mail.discard", Label: "Close and lose the text", Method: "POST", Danger: true, Enabled: true, Href: s.postQuery("/app/mail/send", url.Values{"op": {"discard"}})},
		{ID: "mail.keep", Label: "Keep writing", Method: "GET", Primary: true, Enabled: true, Href: s.route("/app/mail/compose")},
	}
	sheet.Under = s.underOf(r, d.Back, "/app/contacts")
	return sheet
}

// mailSentSheet is a letter that went to some (s-mail-18): a row a
// recipient, sent or refused with the relay's words, and the letter again
// to the refused.
func (s *Server) mailSentSheet(r *http.Request, sess *session.Session, d *mailDraft, taken []string) view.MailSentSheet {
	sheet := view.MailSentSheet{Notice: "The letter went, but not to everyone. Sending again goes only to the refused."}
	if u := s.letterUser(sess); u != nil {
		sheet.From = text(s.senderOf(u).Address)
	}
	row := func(a string) view.MailSentRow {
		out := view.MailSentRow{Address: text(a)}
		if k, ok := d.Known[strings.ToLower(a)]; ok {
			out.Name, out.Colour = text(k.Name), view.Colour(k.Colour)
		}
		return out
	}
	for _, a := range d.To {
		if slices.ContainsFunc(taken, func(t string) bool { return strings.EqualFold(t, a) }) {
			rw := row(a)
			rw.Said = "sent"
			sheet.Rows = append(sheet.Rows, rw)
			continue
		}
		for _, rf := range d.Refused {
			if strings.EqualFold(rf.Address, a) {
				rw := row(a)
				rw.Said, rw.Refused = text("refused: «"+rf.Reply+"»"), true
				sheet.Rows = append(sheet.Rows, rw)
			}
		}
	}
	sheet.Actions = []view.Action{
		{ID: "mail.again", Label: "Send again to the refused", Method: "GET", Primary: true, Enabled: true,
			Href: s.postQuery("/app/mail/compose", url.Values{"to": {"refused"}})},
		{ID: "mail.close", Label: "Close", Method: "GET", Enabled: true, Href: s.route(mailBack(d))},
	}
	sheet.Under = s.underOf(r, d.Back, "/app/contacts")
	return sheet
}

// suggestRecipients asks the ticked address books for q, the same fanout as
// search (S-15), and offers every address of every card found — one row an
// address, labelled from the card. Books that did not answer are named: a
// silent list would read as "nobody by that name".
// It answers how many books were asked too.
func (s *Server) suggestRecipients(ctx context.Context, sess *session.Session, q string) ([]view.Recipient, []view.Stamp, int) {
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
		return nil, nil, 0
	}
	opts := s.searchOptions(sess)
	opts.Finished = nil // a suggestion is not a search of the history
	id, err := user.NewTaskID()
	if err != nil {
		return nil, nil, 0
	}
	task, err := fanout.Start(id, fanout.Query{Text: q}, sources, opts)
	if err != nil {
		return nil, nil, 0
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
	return out, notAsked, len(sources)
}

// getMailSuggest answers «Name or address» of a letter with the library's
// script (s-mail-3, С17): a row an address of the cards found — the name,
// the address, the card's label, the book's mark —, the line as typed, and
// the books that did not answer, named. A row's value is the address
// itself: a letter's recipient is no attendee.
func (s *Server) getMailSuggest(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if !s.canSendLetters(sess) {
		s.notFound(w, r)
		return
	}
	out := view.SuggestList{Items: []view.SuggestItem{}}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		s.writePage(w, r, http.StatusOK, out)
		return
	}
	found, notAsked, asked := s.suggestRecipients(r.Context(), sess, q)
	typed := true
	for _, rc := range found {
		if len(out.Items) >= suggestMost {
			break
		}
		out.Items = append(out.Items, view.SuggestItem{Name: line(string(rc.Name)), Email: rc.Address, Colour: rc.Colour, Label: rc.Label, Value: string(rc.Address)})
		if strings.EqualFold(string(rc.Address), q) {
			typed = false
		}
	}
	if typed {
		out.Items = append(out.Items, view.SuggestItem{Email: text(q), Value: q, Add: true})
	}
	if len(notAsked) > 0 {
		out.Searched = text(fmt.Sprintf("Searched %d of %d address books", asked-len(notAsked), asked))
		out.NotAsked = notAsked
	}
	s.writePage(w, r, http.StatusOK, out)
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
// files, and hands it to the relay recipient by recipient (s-mail-18). It
// returns those the relay took and those it refused; the error is what the
// sheet says first under To when the letter did not go (s-mail-17).
func (s *Server) sendLetter(r *http.Request, sess *session.Session, d *mailDraft) ([]string, []cmail.Refusal, error) {
	ctx := r.Context()
	u := s.letterUser(sess)
	if u == nil {
		return nil, nil, errors.New("no account")
	}
	to, err := cmail.Addresses(d.To)
	switch {
	case err != nil:
		return nil, nil, errors.New("A recipient is not an address: " + strings.TrimPrefix(err.Error(), "mail: "))
	case len(to) == 0:
		return nil, nil, errors.New("Add a recipient.")
	case len(to) > s.mailPolicy().Recipients:
		return nil, nil, fmt.Errorf("A letter goes to %d recipients at most.", s.mailPolicy().Recipients)
	}
	d.Parts = s.reckonLinks(d.Parts)

	var files []cmail.Attachment
	kinds := map[string]int{}
	var total int64
	for _, p := range d.Parts {
		if p.AsLink {
			continue
		}
		data, err := s.partContent(ctx, sess, p)
		if err != nil {
			return nil, nil, fmt.Errorf("«%s» could not be read: %s. Remove it or try again.", p.Title, sentence(err.Error()))
		}
		if int64(len(data)) > s.mailPolicy().Attach || total+int64(len(data)) > s.mailPolicy().Letter {
			return nil, nil, fmt.Errorf("«%s» has grown since it was attached and no longer fits the letter. Attach it again: it will go as a link.", p.Title)
		}
		total += int64(len(data))
		files = append(files, cmail.Attachment{Name: p.Name, Type: p.Media, Data: data})
		kinds[string(p.Kind)]++
	}

	if err := s.users.ReserveLetter(u.ID, s.mailPolicy().PerDay); err != nil {
		return nil, nil, err
	}
	sent := false
	defer func() {
		if !sent {
			s.users.ReleaseLetter(u.ID)
		}
	}()
	ls := s.senderOf(u)
	// The links are made once the relay has taken someone: a letter that
	// does not go leaves no link behind it (s-mail-17), save the ones
	// already asked for when one cannot be made. Sent again to the refused,
	// the letter carries the links it made the first time.
	letter := func() ([]byte, error) {
		var links []string
		for i := range d.Parts {
			p := &d.Parts[i]
			if !p.AsLink {
				continue
			}
			if p.Token == "" {
				token, err := s.partLink(sess, *p)
				if err != nil {
					return nil, fmt.Errorf("The link to «%s» could not be made. Remove it or try again.", p.Title)
				}
				p.Token = token
			}
			links = append(links, linkTitle(*p)+" — "+s.absURL(r, "/p/"+p.Token))
			kinds[string(p.Kind)+" link"]++
		}
		return s.writeLetter(r, ls, to, d.Subject, d.Body, links, files)
	}
	sendCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := cmail.SendEach(sendCtx, s.relay(), to, letter)
	if err != nil {
		return nil, out.Refused, relayRefused(err)
	}
	s.markSent()
	sent = true
	var taken []string
	for _, a := range to {
		if !slices.ContainsFunc(out.Refused, func(rf cmail.Refusal) bool { return rf.Address == a }) {
			taken = append(taken, a)
		}
	}
	var attached []string
	for _, k := range attachKinds {
		for _, suffix := range []string{"", " link"} {
			if n := kinds[string(k.Kind)+suffix]; n > 0 {
				attached = append(attached, fmt.Sprintf("%s%s ×%d", k.Kind, suffix, n))
				delete(kinds, string(k.Kind)+suffix)
			}
		}
	}
	s.users.LetterSent(u.ID, taken, strings.Join(attached, ", "))
	return taken, out.Refused, nil
}

// relayRefused is a letter that did not go as the sheet says it (s-mail-17):
// the relay's reply in its words; what the letter could not be written for,
// as it is.
func relayRefused(err error) error {
	var tp *textproto.Error
	if !errors.As(err, &tp) {
		return err
	}
	return errors.New("The letter did not go: the relay answered «" + cmail.Reply(err) + "».")
}

// linkTitle is a part that goes as a link as the letter names it before the
// link (s-mail-14): its title, and a folder what it holds.
func linkTitle(p mailPart) string {
	if p.Folder && p.Detail != "" {
		return p.Title + ", " + p.Detail
	}
	return p.Title
}

// letterText is what the person wrote and their name under «-- » (RFC 3676;
// s-mail-7): the letter's text as it goes and as Preview shows it.
func letterText(typed, name string) string {
	return strings.TrimRight(strings.ReplaceAll(typed, "\r\n", "\n"), "\n") + "\n\n-- \n" + name + "\n"
}

// noReplyLine closes a letter without an address to answer (s-mail-8): a
// reply would reach nobody, and the letter says so after the signature.
func (s *Server) noReplyLine(r *http.Request) string {
	return "This letter was sent from Carrel at " + s.publicOrigin(r) +
		". Do not reply to it: the sender's address is not confirmed, and a reply would reach nobody."
}

// writeLetter writes a letter as it goes: what the person wrote, the links
// each on a line of its own, the signature, and — without an address to
// answer — a line after it that says a reply reaches nobody (s-mail-8).
func (s *Server) writeLetter(r *http.Request, ls letterSender, to []string, subject, typed string, links []string, files []cmail.Attachment) ([]byte, error) {
	if len(links) > 0 {
		// Before the signature: a mail program greys what follows «-- » and
		// drops it from a reply.
		typed = strings.TrimRight(strings.ReplaceAll(typed, "\r\n", "\n"), "\n") + "\n\n" + strings.Join(links, "\n")
	}
	body := letterText(typed, ls.Name)
	if ls.ReplyTo == "" {
		body += "\n" + s.noReplyLine(r) + "\n"
	}
	data, err := cmail.Build(s.relay(), cmail.Message{
		To: to, ReplyTo: ls.ReplyTo, Sender: ls.Name, FromLogin: ls.FromLogin,
		Subject: subject, Text: body, Attachments: files,
	}, time.Now())
	if err != nil {
		return nil, errors.New("The letter could not be written: " + strings.TrimPrefix(err.Error(), "mail: "))
	}
	return data, nil
}

// relayLetter writes a letter and hands it to the relay whole: one
// recipient refused, it goes to nobody.
func (s *Server) relayLetter(r *http.Request, ls letterSender, to []string, subject, typed string, links []string, files []cmail.Attachment) error {
	if s.cfg.PublicURL == "" {
		return errNoPublicAddress // А4-6
	}
	data, err := s.writeLetter(r, ls, to, subject, typed, links, files)
	if err != nil {
		return err
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
	// Days of a calendar over the letter's ceiling go as a link to these
	// days, as a published range is (С19; the owner's decision of
	// 21.10.2026): never the whole calendar.
	target := user.PublishTarget{AccountID: p.Account, Kind: string(p.Kind), Col: p.Col, Title: p.Title, From: p.From, To: p.To}
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

// linkFolder is how the line under the list names a folder (s-mail-1).
const linkFolder = "the folder"

// linkReason is a part that goes as a link as the line under the list names
// it, why in brackets (s-mail-1): «the PDF (over 2 MB)», a folder — it
// always goes so —, one the letter has no room left for.
func (s *Server) linkReason(p mailPart) string {
	if p.Folder {
		return linkFolder
	}
	what := "«" + p.Title + "»"
	if p.Size > s.mailPolicy().Attach {
		if i := strings.LastIndex(p.Name, "."); i >= 0 && i < len(p.Name)-1 {
			what = "the " + strings.ToUpper(p.Name[i+1:])
		}
		return what + " (over " + string(formatSize(s.mailPolicy().Attach)) + ")"
	}
	return what + " (the letter would be over " + string(formatSize(s.mailPolicy().Letter)) + ")"
}

// linksLine is the line under the list when something goes as a link
// (s-mail-1): what and why, and that revoking one in «Published links», in
// bold, closes it for this letter too. Nil without links.
func linksLine(why []string) view.Runs {
	if len(why) == 0 {
		return nil
	}
	var reasons []string
	folders := 0
	for _, w := range why {
		if w == linkFolder {
			folders++
		}
		if !slices.Contains(reasons, w) {
			reasons = append(reasons, w)
		}
	}
	if folders > 1 {
		reasons[slices.Index(reasons, linkFolder)] = fmt.Sprintf("the %d folders", folders)
	}
	said := reasons[0]
	if n := len(reasons); n > 1 {
		said = strings.Join(reasons[:n-1], ", ") + " and " + reasons[n-1]
	}
	if rest, ok := strings.CutPrefix(said, "the "); ok {
		said = "The " + rest
	}
	if len(why) == 1 {
		return join(said+" goes as a link; revoking it in ", strongRun("Published links"), " closes it for this letter too.")
	}
	return join(said+" go as links; revoking one in ", strongRun("Published links"), " closes it for this letter too.")
}
