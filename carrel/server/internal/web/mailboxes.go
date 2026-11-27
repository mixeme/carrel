package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/mailbox"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// People's own mailboxes on the screens of connections (carrel-mailbox.md
// M-3; s-mailbox-22…28): connecting one — the address first, the provider
// known by its domain, the servers found, signing in to IMAP and SMTP —,
// the question of the folders the server did not name, the edit of one, and
// its card. Off on the instance, every address here answers as one that does
// not exist (M-2), except Remove: the person's blob is theirs, and the
// administrator cannot open it to remove a mailbox.

// The words of the mailbox's screens (s-mailbox-22…28).
const (
	mailboxConnectTitle = "Connect a mailbox"
	mailboxConnectSub   = "Read it and write from it over IMAP and SMTP. Letters stay on your mail server."
	mailboxFoldersTitle = "Which folders are these?"
	mailboxEscrowBold   = "Key escrow is on:"
	mailboxEscrowRest   = " the administrator can open this password too — and through a mailbox, reset your other passwords."
	mailboxOffNote      = "Passwords stay sealed in your profile; everything works again when mailboxes are back on."
	mailboxOffLine      = "not used while mail is off"
	mailboxSendOff      = "In «From» it stays off until Test passes."
	mailboxSentCopy     = "The server puts a copy of sent letters into Sent itself"
	mailboxSentCopyHint = "Off: Carrel puts the copy after sending. Turn it on if every sent letter shows up twice."
	mailboxBadAddress   = "Type the whole address: name@domain."
	mailboxNoPassword   = "Type the password: Carrel signs in to read and to send."
)

// mailboxRoles are the special folders in the order of the card's table
// (s-mailbox-26), with their names on the screens.
var mailboxRoles = []struct {
	role  view.MailRole
	label string
}{
	{view.MailSent, "Sent"}, {view.MailDrafts, "Drafts"}, {view.MailArchive, "Archive"},
	{view.MailJunk, "Spam"}, {view.MailTrash, "Trash"},
}

// The form fields of a role in the question of the folders (s-mailbox-24).
var roleField = map[view.MailRole]string{view.MailSent: "sent", view.MailDrafts: "drafts", view.MailArchive: "archive",
	view.MailJunk: "spam", view.MailTrash: "trash"}

func roleFolder(f sealed.MailFolders, role view.MailRole) string {
	return map[view.MailRole]string{view.MailSent: f.Sent, view.MailDrafts: f.Drafts, view.MailArchive: f.Archive,
		view.MailJunk: f.Junk, view.MailTrash: f.Trash}[role]
}

func setRoleFolder(f *sealed.MailFolders, role view.MailRole, name string) {
	switch role {
	case view.MailSent:
		f.Sent = name
	case view.MailDrafts:
		f.Drafts = name
	case view.MailArchive:
		f.Archive = name
	case view.MailJunk:
		f.Junk = name
	case view.MailTrash:
		f.Trash = name
	}
}

// mailboxForm is the dialog as typed: connecting, or the edit of Editing.
type mailboxForm struct {
	Editing                       string
	Address, Label, Colour        string
	FromName, Signature           string
	Autosave                      bool
	Manual                        bool
	IMAPHost, IMAPPort            string
	SMTPHost, SMTPPort            string
	AddressErr, LabelErr, PassErr view.Text
	ServersErr                    view.Text
	Found                         *mailbox.Found
	Tested                        bool
}

// mailboxesGate answers as a missing address while mailboxes are off on
// the instance (M-2): one check at the entry, not one per screen.
func (s *Server) mailboxesGate(w http.ResponseWriter, r *http.Request) (*session.Session, bool) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return nil, false
	}
	if _, live := sess.DataKey(); !live || !s.mailboxesOn() {
		s.notFound(w, r)
		return nil, false
	}
	return sess, true
}

// ownMailbox is the mailbox {mailbox} of the person, or a missing address.
func (s *Server) ownMailbox(w http.ResponseWriter, r *http.Request, sess *session.Session) (sealed.Mailbox, bool) {
	m, ok := s.catalogOf(sess).Mailbox(r.PathValue("mailbox"))
	if !ok {
		s.notFound(w, r)
	}
	return m, ok
}

func (s *Server) mailboxHref(id, tail string) string {
	return s.route("/app/settings/mailboxes/" + url.PathEscape(id) + tail)
}

// readMailboxForm is the dialog as it was sent.
func readMailboxForm(r *http.Request) mailboxForm {
	_ = readForm(r)
	return mailboxForm{
		Address:   strings.TrimSpace(r.FormValue("address")),
		Label:     connectionLabel(r.FormValue("label")),
		Colour:    strings.ToLower(strings.TrimSpace(r.FormValue("colour"))),
		FromName:  strings.TrimSpace(r.FormValue("fromName")),
		Signature: strings.TrimRight(strings.ReplaceAll(r.FormValue("signature"), "\r\n", "\n"), "\n "),
		Autosave:  r.FormValue("autosave") == "on",
		Manual:    r.FormValue("servers") == "manual" || r.FormValue("imap.host") != "",
		IMAPHost:  strings.TrimSpace(r.FormValue("imap.host")), IMAPPort: strings.TrimSpace(r.FormValue("imap.port")),
		SMTPHost: strings.TrimSpace(r.FormValue("smtp.host")), SMTPPort: strings.TrimSpace(r.FormValue("smtp.port")),
	}
}

// typedServer is a server typed by hand: the port says the way in — 993 and
// 465 over TLS, 143 and 587 with STARTTLS; another port is the guard's to
// refuse (S-5.1).
func typedServer(host, port string, imap bool) sealed.MailServer {
	n, _ := strconv.Atoi(port)
	if n == 0 {
		n = 993
		if !imap {
			n = 465
		}
	}
	sec := sealed.MailTLS
	if n == 143 || n == 587 {
		sec = sealed.MailSTARTTLS
	}
	return sealed.MailServer{Host: strings.ToLower(host), Port: n, Security: sec}
}

// labelFromAddress is the label a mailbox is offered: the first word of its
// domain, «yandex» of yandex.ru (s-mailbox-22).
func labelFromAddress(address string) string {
	d := mailbox.Domain(address)
	if i := strings.Index(d, "."); i > 0 {
		return d[:i]
	}
	return d
}

func validMailAddress(a string) bool {
	at := strings.LastIndex(a, "@")
	return at > 0 && strings.Contains(a[at+1:], ".") && !strings.ContainsAny(a, " <>,")
}

// escrowCovers says whether key escrow opens this person's data key, and so
// the mailbox's password too (S-29.2).
func (s *Server) escrowCovers(sess *session.Session) bool {
	snap := s.store.Snapshot()
	u := snap.UserByID(sess.AccountID())
	return u != nil && store.EscrowActive(&snap) && store.UserDeposited(u)
}

// profileName is the person's name, the default of a mailbox's Name in From
// (M-3).
func (s *Server) profileName(sess *session.Session) string {
	snap := s.store.Snapshot()
	if u := snap.UserByID(sess.AccountID()); u != nil && u.DisplayName != "" {
		return u.DisplayName
	}
	return sess.Login()
}

// mailboxPage is the dialog of connecting a mailbox (s-mailbox-22, -23) or of
// editing one (-27), in the main zone alone.
func (s *Server) mailboxPage(sess *session.Session, f mailboxForm, stored *sealed.Mailbox) view.MailboxPage {
	page := view.MailboxPage{Shell: s.settingsShell(sess, mailboxConnectTitle, "/app/settings/connections", "/app/settings/connections"),
		Subtitle: view.Runs{{Text: mailboxConnectSub}}}
	action, test := s.route("/app/settings/mailboxes/new"), s.route("/app/settings/mailboxes/new/test")
	if stored != nil {
		page.Shell.Title = text(stored.Name())
		page.Subtitle = view.Runs{monoRun(stored.Address), {Text: " · mailbox"}}
		action, test = s.mailboxHref(stored.ID, "/edit"), s.mailboxHref(stored.ID, "/edit/test")
	}
	form := view.Form{Action: action, Method: "POST"}
	var rows []view.FormRow
	rows = append(rows, csrfField(sess))
	provider, known := mailbox.Provider{}, false
	if stored == nil {
		rows = append(rows, view.FormRow{Name: "address", Label: "Email address", Kind: view.InputEmail, Value: text(f.Address),
			Placeholder: "name@example.org", Error: f.AddressErr})
		provider, known = mailbox.ProviderOf(f.Address)
	} else {
		rows = append(rows, view.FormRow{Name: "address", Kind: view.InputHidden, Value: text(stored.Address)})
		provider, known = mailbox.ProviderOf(stored.Address)
	}
	connect := view.Action{ID: "mailbox.connect", Label: "Connect", Method: "POST", Primary: true, Submits: true, Enabled: true}
	if stored != nil {
		connect.ID, connect.Label = "mailbox.save", "Save"
	}
	testOnly := view.Action{ID: "mailbox.test", Label: "Test only", Href: test, Method: "POST", Submits: true, Enabled: true}
	cancel := view.Action{ID: "mailbox.cancel", Label: "Cancel", Href: s.route("/app/settings/connections"), Method: "GET", Enabled: true}

	// Microsoft closed signing in by password (s-mailbox-23): said by the
	// domain before a password is typed, and Connect is off with it.
	if known && provider.Closed {
		page.Closed = true
		page.Provider = view.Runs{strongRun("This mailbox cannot be connected."),
			{Text: " Outlook.com and Microsoft 365 no longer let mail programs in with a password, and Carrel does not sign in with a Microsoft account. It is not your password that is wrong."}}
		connect.Enabled, connect.Reason = false, "This mailbox cannot be connected."
		form.Sets = []view.FieldSet{{Rows: rows}}
		page.Form = &form
		page.Actions = []view.Action{connect, cancel}
		return page
	}

	label := f.Label
	if label == "" && stored == nil {
		label = labelFromAddress(f.Address)
	}
	sample := label
	if sample == "" {
		sample = "home"
	}
	labelRow := view.FormRow{Name: "label", Label: "Label", Kind: view.InputText, Value: text(label), Error: f.LabelErr}
	if stored == nil {
		// Said where the label is chosen first (s-mailbox-22); the edit
		// does not repeat it (-27).
		labelRow.Hint = text("How this mailbox is named in the rail: «" + sample + " · Inbox».")
	}
	rows = append(rows, labelRow)
	colour := f.Colour
	if colour == "" {
		colour = string(collectionColour("mailbox " + f.Address))
	}
	rows = append(rows, paletteRow(colour, "", ""))
	if stored != nil {
		name := f.FromName
		if name == "" {
			name = s.profileName(sess)
		}
		rows = append(rows,
			view.FormRow{Name: "fromName", Label: "Name in From", Kind: view.InputText, Value: text(name),
				Hint: text("Recipients see «" + name + " <" + stored.Address + ">».")},
			view.FormRow{Name: "signature", Label: "Signature", Kind: view.InputArea, Value: text(f.Signature),
				Hint: "Under your text, above the quote. Plain text."})
		auto := view.FormRow{Name: "autosave", Label: "Save the draft by itself every minute", Kind: view.InputCheck, Value: text(boolStr(f.Autosave))}
		if stored.Folders.Drafts != "" {
			auto.Hint = text("Into " + stored.Name() + " · " + stored.Folders.Drafts + ", the new version replacing the last.")
		} else {
			auto.Value, auto.Reason = "", text(stored.Name()+" has no folder for drafts.")
		}
		rows = append(rows, auto)
	}
	// A known provider says what it lets in before the password is typed,
	// and the field is named so (s-mailbox-22).
	passLabel := "Password"
	if known && provider.AppPassword {
		passLabel = "App password"
		page.Provider = appPasswordHint(provider.Name)
	}
	pass := view.FormRow{Name: "password", Label: text(passLabel), Kind: view.InputPassword, Error: f.PassErr}
	if stored != nil {
		pass.Placeholder = "leave blank to keep the current one"
	} else if f.Tested && f.PassErr == "" {
		pass.Hint = "Type it again to connect: Carrel does not send a password back."
	}
	rows = append(rows, pass)

	if f.Manual {
		imapPort, smtpPort := f.IMAPPort, f.SMTPPort
		if imapPort == "" {
			imapPort = "993"
		}
		if smtpPort == "" {
			smtpPort = "465"
		}
		rows = append(rows,
			view.FormRow{Name: "imap.host", Label: "IMAP server", Kind: view.InputText, Mono: true, Value: text(f.IMAPHost), Placeholder: "imap.example.org", Error: f.ServersErr},
			view.FormRow{Name: "imap.port", Label: "Port", Kind: view.InputNumber, Mono: true, Value: text(imapPort)},
			view.FormRow{Name: "smtp.host", Label: "SMTP server", Kind: view.InputText, Mono: true, Value: text(f.SMTPHost), Placeholder: "smtp.example.org"},
			view.FormRow{Name: "smtp.port", Label: "Port", Kind: view.InputNumber, Mono: true, Value: text(smtpPort)})
	} else {
		manual := s.route("/app/settings/mailboxes/new?servers=manual")
		if stored != nil {
			manual = s.mailboxHref(stored.ID, "/edit?servers=manual")
		}
		switch {
		case stored != nil:
			page.Servers = &view.MailServers{Lines: serverLines(stored.IMAP, stored.SMTP), From: text(stored.ServersFrom)}
		case f.Found != nil && (len(f.Found.IMAP) > 0 || len(f.Found.SMTP) > 0):
			var imapSrv, smtpSrv sealed.MailServer
			if len(f.Found.IMAP) > 0 {
				imapSrv = f.Found.IMAP[0]
			}
			if len(f.Found.SMTP) > 0 {
				smtpSrv = f.Found.SMTP[0]
			}
			page.Servers = &view.MailServers{Badges: []view.Badge{{Label: "found", Role: view.BadgeLinked}},
				Lines: serverLines(imapSrv, smtpSrv), From: text(f.Found.From)}
		}
		if page.Servers != nil {
			page.Servers.Manual = view.Action{ID: "mailbox.manual", Label: "Enter servers by hand", Href: manual, Method: "GET", Enabled: true}
		}
	}
	// Said where the password is first given (s-mailbox-22); the edit does
	// not repeat it (-27).
	if stored == nil && s.escrowCovers(sess) {
		page.Escrow = view.Runs{strongRun(mailboxEscrowBold), {Text: mailboxEscrowRest}}
	}
	form.Sets = []view.FieldSet{{Rows: rows}}
	// The caret where the dialog is typed next: the password of a
	// connection (s-mailbox-22), the signature of an edit (-27).
	caret := "password"
	if stored != nil {
		caret = "signature"
	}
	if f.AddressErr != "" || f.Address == "" {
		caret = "address"
	}
	form = focused(form, caret)
	page.Form = &form
	page.Actions = []view.Action{connect, testOnly, cancel}
	return page
}

// appPasswordHint is what a provider that lets mail programs in by an app
// password asks, in the words of s-mailbox-22.
func appPasswordHint(name string) view.Runs {
	where := map[string]string{
		"Yandex":  "Yandex ID → Security → App passwords → Mail",
		"Gmail":   "Google Account → Security → App passwords",
		"Mail.ru": "Mail.ru → Settings → Security → Passwords for external applications",
		"iCloud":  "Apple Account → Sign-In and Security → App-Specific Passwords",
	}[name]
	return view.Runs{{Text: text(name + " lets mail programs in with an ")}, strongRun("app password"),
		{Text: text(", not with the password of your " + name + " account. Make one in " + where + ", and paste it here.")}}
}

// serverLines are a mailbox's servers as the rubric Servers says them.
func serverLines(imapSrv, smtpSrv sealed.MailServer) []view.MailServerLine {
	var out []view.MailServerLine
	for _, l := range []struct {
		role string
		srv  sealed.MailServer
	}{{"IMAP", imapSrv}, {"SMTP", smtpSrv}} {
		if l.srv.Host == "" {
			continue
		}
		out = append(out, view.MailServerLine{Role: text(l.role), Address: text(l.srv.Host + ":" + strconv.Itoa(l.srv.Port)),
			Security: text(mailbox.SecuritySaid(l.srv.Security))})
	}
	return out
}

func (s *Server) getMailboxNew(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.mailboxesGate(w, r)
	if !ok {
		return
	}
	f := mailboxForm{Manual: r.URL.Query().Get("servers") == "manual", Address: strings.TrimSpace(r.URL.Query().Get("address"))}
	// An address given (?address=, what a script sends when the field is
	// left): the servers are found and shown before the password is typed
	// (s-mailbox-22); finding them reads DNS and a file, it changes nothing.
	if validMailAddress(f.Address) && !f.Manual {
		if p, known := mailbox.ProviderOf(f.Address); !known || !p.Closed {
			found := s.mail.Discover(r.Context(), f.Address, func(string, string, bool) {})
			f.Found = &found
		}
	}
	s.writePage(w, r, http.StatusOK, s.mailboxPage(sess, f, nil))
}

// checkTyped runs the check of the dialog as typed: the servers found or
// typed, the login, the password; what was typed by hand goes as it is.
func (s *Server) checkTyped(ctx context.Context, f mailboxForm, password string, stored *sealed.Mailbox) (*mailbox.Result, *view.DiagnosticTrace, error) {
	in := mailbox.Input{Address: f.Address, Password: password}
	if stored != nil {
		in.Address, in.Login, in.CertFingerprint = stored.Address, stored.Login, stored.CertFingerprint
		in.IMAP, in.SMTP = stored.IMAP, stored.SMTP
	}
	if f.Manual {
		in.IMAP = typedServer(f.IMAPHost, f.IMAPPort, true)
		in.SMTP = typedServer(f.SMTPHost, f.SMTPPort, false)
	}
	return s.mail.Check(ctx, in)
}

// postMailboxNew connects a mailbox (M-3): the address checked, the provider
// that closed passwords refused by its domain, the label in the space of the
// connections' labels; no password — the servers are found and the password
// asked; then IMAP and SMTP are signed in to. IMAP that does not let in is
// the refusal, nothing saved; SMTP that does not is saved with reading only
// (s-mailbox-25). Roles the server did not name are asked once (-24).
func (s *Server) postMailboxNew(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.mailboxesGate(w, r)
	if !ok {
		return
	}
	key, _ := sess.DataKey()
	f := readMailboxForm(r)
	if f.Label == "" {
		f.Label = labelFromAddress(f.Address)
	}
	if !inPalette(f.Colour) {
		f.Colour = ""
	}
	if !validMailAddress(f.Address) {
		f.AddressErr = mailboxBadAddress
		s.writePage(w, r, http.StatusOK, s.mailboxPage(sess, f, nil))
		return
	}
	if p, known := mailbox.ProviderOf(f.Address); known && p.Closed {
		s.writePage(w, r, http.StatusOK, s.mailboxPage(sess, f, nil))
		return
	}
	blob := s.catalogOf(sess)
	if labelTaken(blob, f.Label, "") {
		f.LabelErr = labelTakenText(f.Label)
		s.writePage(w, r, http.StatusOK, s.mailboxPage(sess, f, nil))
		return
	}
	if f.Manual && (f.IMAPHost == "" || f.SMTPHost == "") {
		f.ServersErr = "Type both servers, or go back to the servers found."
		s.writePage(w, r, http.StatusOK, s.mailboxPage(sess, f, nil))
		return
	}
	password := r.FormValue("password")
	if password == "" {
		// The servers are found first and shown; the password is asked
		// (the owner's rule of 24.11.2026: no server is asked with an
		// empty password).
		if !f.Manual {
			found := s.mail.Discover(r.Context(), f.Address, func(string, string, bool) {})
			f.Found = &found
		}
		f.PassErr = mailboxNoPassword
		s.writePage(w, r, http.StatusOK, s.mailboxPage(sess, f, nil))
		return
	}
	res, trace, err := s.checkTyped(r.Context(), f, password, nil)
	if err != nil || res == nil {
		s.writePage(w, r, http.StatusOK, s.mailboxRefused(sess, f, trace, err, "Nothing was saved."))
		return
	}
	colour := f.Colour
	if colour == "" {
		colour = string(collectionColour("mailbox " + f.Address))
	}
	m := sealed.Mailbox{Label: f.Label, Address: f.Address, Colour: colour, Login: res.Login,
		Credentials: sealed.MailCredentials{Method: sealed.MailPassword, Password: password},
		IMAP:        res.IMAP, SMTP: res.SMTP, SendRefused: res.SendRefused, SendTrace: res.SendTrace,
		MaxLetter: res.MaxLetter, ServersFrom: res.From, ServerKeepsSent: res.KeepsSent,
		Folders: announcedOnly(res.Folders, res.Unannounced)}
	id, err := s.users.AddMailbox(sess.AccountID(), key, m)
	if err != nil {
		http.Error(w, "could not save the mailbox", http.StatusInternalServerError)
		return
	}
	sess.NoteCheck(id, trace)
	next := "/app/settings/connections"
	if len(res.Unannounced) > 0 {
		next = "/app/settings/mailboxes/" + url.PathEscape(id) + "/folders"
	}
	if !res.CanSend() {
		m.ID = id
		s.writePage(w, r, http.StatusOK, s.mailboxSaved(sess, m, trace, next))
		return
	}
	s.redirect(w, r, next)
}

// announcedOnly are the roles the server announced: those the folders' names
// only suggest are asked (s-mailbox-24), not stored.
func announcedOnly(f sealed.MailFolders, unannounced []string) sealed.MailFolders {
	for _, role := range unannounced {
		setRoleFolder(&f, view.MailRole(role), "")
	}
	return f
}

// mailboxRefused is the dialog again with what was typed, the refusal above
// it and the trace under it; the password is not sent back.
func (s *Server) mailboxRefused(sess *session.Session, f mailboxForm, trace *view.DiagnosticTrace, err error, nothing string) view.MailboxPage {
	if errors.Is(err, mailbox.ErrRefused) {
		f.PassErr = "The server refused the login and password."
	}
	var stored *sealed.Mailbox
	if f.Editing != "" {
		if m, ok := s.catalogOf(sess).Mailbox(f.Editing); ok {
			stored = &m
		}
	}
	page := s.mailboxPage(sess, f, stored)
	n := &view.Notice{Role: view.NoticeAlert, Title: "Could not connect.", Body: text(nothing)}
	if stored != nil {
		n.Title = "The mailbox did not let in."
	}
	page.Shell.Notices = append(page.Shell.Notices, *n)
	page.Trace = trace
	return page
}

// mailboxSaved is the outcome of a mailbox that reads and does not send
// (s-mailbox-25): saved, said so, the trace of what was tried and what to
// do; Done goes on to the question of the folders when there is one.
func (s *Server) mailboxSaved(sess *session.Session, m sealed.Mailbox, trace *view.DiagnosticTrace, next string) view.MailboxPage {
	page := view.MailboxPage{Shell: s.settingsShell(sess, mailboxConnectTitle, "/app/settings/connections", "/app/settings/connections"),
		Subtitle: view.Runs{{Text: text(m.Name() + " · " + m.Address)}}}
	page.Saved = view.Runs{strongRun("Saved. Reading works, sending does not:"), {Text: text(" " + sendRefusedWords(m.SendRefused))}}
	page.Trace = trace
	page.TraceNote = text("Some servers keep a separate password for sending: ask whoever runs " + mailbox.Domain(m.Address) +
		". Test in Connections checks it without sending.")
	page.Actions = []view.Action{
		{ID: "mailbox.done", Label: "Done", Href: s.route(next), Method: "GET", Primary: true, Enabled: true},
		{ID: "mailbox.manual", Label: "Enter servers by hand", Href: s.mailboxHref(m.ID, "/edit?servers=manual"), Method: "GET", Enabled: true},
		{ID: "mailbox.password", Label: "Change the password", Href: s.mailboxHref(m.ID, "/edit"), Method: "GET", Enabled: true},
	}
	return page
}

// sendRefusedWords is what the SMTP server's refusal means, in plain words
// (s-mailbox-25); the answer itself stands in the trace.
func sendRefusedWords(answer string) string {
	switch {
	case strings.HasPrefix(answer, "535"):
		return "the server refused the password for sending."
	case strings.HasPrefix(answer, "No SMTP server"):
		return "no server for sending was found."
	}
	return "the server for sending answered " + answer + "."
}

// postMailboxNewTest is Test only of the dialog: the check of what is
// typed, nothing saved (as at a DAV connection).
func (s *Server) postMailboxNewTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.mailboxesGate(w, r)
	if !ok {
		return
	}
	f := readMailboxForm(r)
	if !validMailAddress(f.Address) {
		f.AddressErr = mailboxBadAddress
		s.writePage(w, r, http.StatusOK, s.mailboxPage(sess, f, nil))
		return
	}
	s.testTyped(w, r, sess, f, nil)
}

// testTyped checks the dialog as typed and shows the trace, saving nothing.
func (s *Server) testTyped(w http.ResponseWriter, r *http.Request, sess *session.Session, f mailboxForm, stored *sealed.Mailbox) {
	password := r.FormValue("password")
	if password == "" && stored != nil {
		password = stored.Credentials.Password
	}
	if password == "" {
		if !f.Manual && stored == nil {
			found := s.mail.Discover(r.Context(), f.Address, func(string, string, bool) {})
			f.Found = &found
		}
		f.PassErr = mailboxNoPassword
		s.writePage(w, r, http.StatusOK, s.mailboxPage(sess, f, stored))
		return
	}
	res, trace, err := s.checkTyped(r.Context(), f, password, stored)
	f.Tested = true
	if err != nil || res == nil {
		s.writePage(w, r, http.StatusOK, s.mailboxRefused(sess, f, trace, err, "Nothing was saved."))
		return
	}
	page := s.mailboxPage(sess, f, stored)
	page.Trace = trace
	n := view.Notice{Role: view.NoticeAccent, Title: "Test passed:", Body: "reading and sending work. Nothing was saved."}
	if !res.CanSend() {
		n.Title, n.Body = "Test passed for reading:", text(sendRefusedWords(res.SendRefused)+" Nothing was saved.")
	}
	page.Shell.Notices = append(page.Shell.Notices, n)
	s.writePage(w, r, http.StatusOK, page)
}

// getMailboxFolders asks the roles the server did not name (s-mailbox-24),
// or one role (?role=) from Change on the card: the server's folders are read
// again, a role suggested by a folder's name says so.
func (s *Server) getMailboxFolders(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.mailboxesGate(w, r)
	if !ok {
		return
	}
	m, ok := s.ownMailbox(w, r, sess)
	if !ok {
		return
	}
	page, err := s.foldersPage(r.Context(), sess, m, r.URL.Query().Get("role"))
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.mailboxNotAnswering(sess, m, err))
		return
	}
	s.writePage(w, r, http.StatusOK, page)
}

// mailboxNotAnswering is the connections page with the mailbox's refusal
// above it: its folders could not be read now.
func (s *Server) mailboxNotAnswering(sess *session.Session, m sealed.Mailbox, err error) view.ConnectionsPage {
	page := s.connectionsPage(sess, connectionForm{}, &view.Notice{Role: view.NoticeAlert,
		Title: text("Could not read the folders of " + m.Name() + "."), Body: text(err.Error())})
	return page
}

func (s *Server) foldersPage(ctx context.Context, sess *session.Session, m sealed.Mailbox, only string) (view.MailboxFoldersPage, error) {
	offered, unannounced, all, err := s.mail.Roles(ctx, m.IMAP, m.Login, m.Credentials.Password, m.CertFingerprint)
	if err != nil {
		return view.MailboxFoldersPage{}, err
	}
	page := view.MailboxFoldersPage{Shell: s.settingsShell(sess, mailboxFoldersTitle, "/app/settings/connections", "/app/settings/connections")}
	rows := []view.FormRow{csrfField(sess)}
	var asked []string
	for _, r := range mailboxRoles {
		role := string(r.role)
		switch {
		case only != "" && only != role:
			continue
		case only == "" && !slices.Contains(unannounced, role):
			continue
		}
		current := roleFolder(m.Folders, r.role)
		suggested := false
		if current == "" && !slices.Contains(m.Folders.Chosen, role) {
			current, suggested = roleFolder(offered, r.role), roleFolder(offered, r.role) != ""
		}
		row := view.FormRow{Name: roleField[r.role], Label: text(r.label), Kind: view.InputSelect, Value: text(current),
			Options: []view.Option{{Value: "", Label: "None", Selected: current == ""}}}
		for _, name := range all {
			row.Options = append(row.Options, view.Option{Value: name, Label: text(name), Selected: name == current})
		}
		if suggested {
			row.Hint = "Suggested by the name."
		}
		rows = append(rows, row)
		asked = append(asked, strings.ToLower(r.label))
	}
	if only != "" {
		rows = append(rows, view.FormRow{Name: "role", Kind: view.InputHidden, Value: text(only)})
	} else {
		rows = append(rows, view.FormRow{Name: "sentCopy", Label: mailboxSentCopy, Kind: view.InputCheck,
			Value: text(boolStr(m.ServerKeepsSent)), Hint: mailboxSentCopyHint})
	}
	page.Subtitle = text(m.Name() + " did not say where it keeps its " + andList(asked) + ".")
	page.Form = view.Form{Action: s.mailboxHref(m.ID, "/folders"), Method: "POST", Sets: []view.FieldSet{{Rows: rows}},
		Actions: []view.Action{
			{ID: "mailbox.folders.save", Label: "Save", Method: "POST", Primary: true, Submits: true, Enabled: true},
			{ID: "mailbox.folders.skip", Label: "Skip", Href: s.route("/app/settings/connections"), Method: "GET", Enabled: true},
		}}
	return page, nil
}

// andList is «archive and spam», «sent, archive and spam».
func andList(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

// postMailboxFolders keeps the answer: each role sent is the folder chosen,
// None — no folder (M-3: not a refusal); a folder the server does not have
// is refused. The copy of sent letters goes with the first question only.
func (s *Server) postMailboxFolders(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.mailboxesGate(w, r)
	if !ok {
		return
	}
	m, ok := s.ownMailbox(w, r, sess)
	if !ok {
		return
	}
	_ = readForm(r)
	_, _, all, err := s.mail.Roles(r.Context(), m.IMAP, m.Login, m.Credentials.Password, m.CertFingerprint)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.mailboxNotAnswering(sess, m, err))
		return
	}
	key, _ := sess.DataKey()
	err = s.users.EditMailbox(sess.AccountID(), key, m.ID, "mailbox.folders", func(mb *sealed.Mailbox) {
		for _, rr := range mailboxRoles {
			vals, sent := r.Form[roleField[rr.role]]
			if !sent {
				continue
			}
			name := strings.TrimSpace(vals[0])
			if name != "" && !slices.Contains(all, name) {
				continue
			}
			setRoleFolder(&mb.Folders, rr.role, name)
			if !slices.Contains(mb.Folders.Chosen, string(rr.role)) {
				mb.Folders.Chosen = append(mb.Folders.Chosen, string(rr.role))
			}
		}
		mb.Folders.Asked = true
		// The mark stands in the first question only, not in Change.
		if r.FormValue("role") == "" {
			mb.ServerKeepsSent = r.FormValue("sentCopy") == "on"
		}
	})
	if err != nil {
		s.notFound(w, r)
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

// getMailboxEdit is the edit of a mailbox (s-mailbox-27): the label, colour,
// name in From, signature and the draft's autosave; the password and the
// servers, which are checked again before they are kept.
func (s *Server) getMailboxEdit(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.mailboxesGate(w, r)
	if !ok {
		return
	}
	m, ok := s.ownMailbox(w, r, sess)
	if !ok {
		return
	}
	f := mailboxFormOf(m)
	if r.URL.Query().Get("servers") == "manual" {
		f.Manual = true
	}
	s.writePage(w, r, http.StatusOK, s.mailboxPage(sess, f, &m))
}

// mailboxFormOf is the edit's form as the mailbox is stored.
func mailboxFormOf(m sealed.Mailbox) mailboxForm {
	return mailboxForm{Editing: m.ID, Address: m.Address, Label: m.Label, Colour: m.Colour, FromName: m.FromName,
		Signature: m.Signature, Autosave: m.Autosave,
		IMAPHost: m.IMAP.Host, IMAPPort: portOf(m.IMAP), SMTPHost: m.SMTP.Host, SMTPPort: portOf(m.SMTP)}
}

func portOf(srv sealed.MailServer) string {
	if srv.Port == 0 {
		return ""
	}
	return strconv.Itoa(srv.Port)
}

// postMailboxEdit keeps the edit. A new password or servers typed by hand
// are signed in to first; refused, nothing is kept.
func (s *Server) postMailboxEdit(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.mailboxesGate(w, r)
	if !ok {
		return
	}
	m, ok := s.ownMailbox(w, r, sess)
	if !ok {
		return
	}
	f := readMailboxForm(r)
	f.Editing, f.Address = m.ID, m.Address
	if !inPalette(f.Colour) {
		f.Colour = m.Colour
	}
	if f.Label == "" {
		f.Label = m.Label
	}
	if labelTaken(s.catalogOf(sess), f.Label, m.ID) {
		f.LabelErr = labelTakenText(f.Label)
		s.writePage(w, r, http.StatusOK, s.mailboxPage(sess, f, &m))
		return
	}
	if m.Folders.Drafts == "" {
		f.Autosave = false
	}
	if f.FromName == s.profileName(sess) {
		f.FromName = "" // the profile's name, as long as it is the profile's
	}
	password := r.FormValue("password")
	var res *mailbox.Result
	var trace *view.DiagnosticTrace
	if password != "" || f.Manual {
		if password == "" {
			password = m.Credentials.Password
		}
		var err error
		res, trace, err = s.checkTyped(r.Context(), f, password, &m)
		if err != nil || res == nil {
			s.writePage(w, r, http.StatusOK, s.mailboxRefused(sess, f, trace, err, "Nothing was changed."))
			return
		}
	}
	key, _ := sess.DataKey()
	err := s.users.EditMailbox(sess.AccountID(), key, m.ID, "mailbox.edit", func(mb *sealed.Mailbox) {
		mb.Label, mb.Colour, mb.FromName, mb.Signature, mb.Autosave = f.Label, f.Colour, f.FromName, f.Signature, f.Autosave
		if res != nil {
			mb.Credentials.Password = password
			mb.IMAP, mb.SMTP, mb.Login = res.IMAP, res.SMTP, res.Login
			mb.SendRefused, mb.SendTrace, mb.MaxLetter = res.SendRefused, res.SendTrace, res.MaxLetter
			if f.Manual {
				mb.ServersFrom = "typed by hand"
			}
		}
	})
	if err != nil {
		s.notFound(w, r)
		return
	}
	if trace != nil {
		sess.NoteCheck(m.ID, trace)
	}
	s.redirect(w, r, "/app/settings/connections")
}

func (s *Server) postMailboxEditTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.mailboxesGate(w, r)
	if !ok {
		return
	}
	m, ok := s.ownMailbox(w, r, sess)
	if !ok {
		return
	}
	f := readMailboxForm(r)
	f.Editing, f.Address = m.ID, m.Address
	s.testTyped(w, r, sess, f, &m)
}

// postMailboxTest is Test on the card: the stored mailbox signed in to
// again; what sending answered is kept, so a letter from it comes on or off
// with it (M-7), and the trace stands on the card for the session.
func (s *Server) postMailboxTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.mailboxesGate(w, r)
	if !ok {
		return
	}
	m, ok := s.ownMailbox(w, r, sess)
	if !ok {
		return
	}
	res, trace, err := s.mail.Check(r.Context(), mailbox.Input{Address: m.Address, Login: m.Login, Password: m.Credentials.Password,
		IMAP: m.IMAP, SMTP: m.SMTP, CertFingerprint: m.CertFingerprint})
	if trace != nil {
		sess.NoteCheck(m.ID, trace)
	}
	if err == nil && res != nil {
		key, _ := sess.DataKey()
		_ = s.users.EditMailbox(sess.AccountID(), key, m.ID, "mailbox.test", func(mb *sealed.Mailbox) {
			mb.SendRefused, mb.SendTrace = res.SendRefused, res.SendTrace
			if res.MaxLetter > 0 {
				mb.MaxLetter = res.MaxLetter
			}
		})
	}
	s.redirect(w, r, "/app/settings/connections")
}

// postMailboxSentCopy keeps the mark of the card's foot as it was sent
// (sentCopy): the server puts the copy of a sent letter into Sent itself,
// or Carrel does (M-3).
func (s *Server) postMailboxSentCopy(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.mailboxesGate(w, r)
	if !ok {
		return
	}
	m, ok := s.ownMailbox(w, r, sess)
	if !ok {
		return
	}
	key, _ := sess.DataKey()
	_ = readForm(r)
	on := r.FormValue("sentCopy") == "on"
	_ = s.users.EditMailbox(sess.AccountID(), key, m.ID, "mailbox.sentcopy", func(mb *sealed.Mailbox) {
		mb.ServerKeepsSent = on
	})
	s.redirect(w, r, "/app/settings/connections")
}

// postMailboxRemove takes the mailbox out of Carrel: its password and
// servers go, its letters stay on the mail server. Allowed while mail is off
// on the instance too (s-mailbox-28).
func (s *Server) postMailboxRemove(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	m, ok := s.ownMailbox(w, r, sess)
	if !ok {
		return
	}
	if err := s.users.RemoveMailbox(sess.AccountID(), key, m.ID); err != nil {
		s.notFound(w, r)
		return
	}
	sess.NoteCheck(m.ID, nil)
	s.redirect(w, r, "/app/settings/connections")
}

// mailboxCards are the person's mailboxes on the page of connections
// (s-mailbox-26), or, off on the instance, each with Remove only (-28).
func (s *Server) mailboxCards(sess *session.Session, blob sealed.Blob, unlocked bool) []view.MailboxCard {
	on := s.mailboxesOn()
	var out []view.MailboxCard
	for _, m := range blob.Mailboxes {
		card := view.MailboxCard{ID: m.ID, Label: text(m.Name()), Colour: view.Colour(m.Colour), Address: text(m.Address),
			IMAP: text(m.IMAP.Host + ":" + strconv.Itoa(m.IMAP.Port))}
		if m.SMTP.Host != "" {
			card.SMTP = text(m.SMTP.Host + ":" + strconv.Itoa(m.SMTP.Port))
		}
		remove := view.Action{ID: "mailbox.remove", Label: "Disconnect", Href: s.mailboxHref(m.ID, "/remove"), Method: "POST", Danger: true, Enabled: true}
		if !on {
			card.Off = mailboxOffLine
			remove.Label = "Remove"
			card.Actions = []view.Action{remove}
			out = append(out, card)
			continue
		}
		if m.MaxLetter > 0 {
			card.Limit = text(fmt.Sprintf("up to %d MB", m.MaxLetter>>20))
		}
		test := view.Action{ID: "mailbox.check", Label: "Test", Href: s.mailboxHref(m.ID, "/test"), Method: "POST", Enabled: true}
		if m.SendRefused != "" {
			card.SendRefused, card.Note, test.Label = true, mailboxSendOff, "Test again"
			card.Trace = &view.DiagnosticTrace{Steps: []view.TraceStep{{Name: "smtp", State: view.CheckBad, Detail: text(m.SendTrace)}}}
		}
		reveal := view.Action{ID: "mailbox.password", Label: "Show password", Href: s.route("/app/settings/unlock?" + url.Values{"for": {m.ID}}.Encode()), Method: "GET", Enabled: true}
		if unlocked {
			reveal.Href = s.postQuery("/app/settings/connections", url.Values{"reveal": {m.ID}})
		}
		card.Password = &view.SecretField{Reveal: reveal}
		for _, r := range mailboxRoles {
			row := view.MailFolderRow{Role: r.role, Label: text(r.label), Folder: text(roleFolder(m.Folders, r.role)),
				Change: view.Action{ID: "mailbox.role", Label: "Change", Href: s.mailboxHref(m.ID, "/folders?role="+string(r.role)), Method: "GET", Enabled: true}}
			switch {
			case row.Folder == "":
				row.How = "none"
			case slices.Contains(m.Folders.Chosen, string(r.role)):
				row.How = "you chose it"
			default:
				row.How = "announced by the server"
			}
			card.Folders = append(card.Folders, row)
		}
		card.KeepsSent = m.ServerKeepsSent
		card.SentToggle = &view.Action{ID: "mailbox.sentcopy", Label: mailboxSentCopy, Href: s.mailboxHref(m.ID, "/sentcopy"), Method: "POST", Enabled: true}
		card.Actions = []view.Action{test,
			{ID: "mailbox.edit", Label: "Edit", Href: s.mailboxHref(m.ID, "/edit"), Method: "GET", Enabled: true}, remove}
		out = append(out, card)
	}
	return out
}
