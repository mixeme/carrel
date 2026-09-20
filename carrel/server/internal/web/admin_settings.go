package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	cmail "carrel.dev/carrel/server/internal/mail"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// settingsFlash is what one POST leaves on the page.
type settingsFlash struct {
	form     *store.SMTPSettings // what was typed, when it was refused
	errField string
	errText  string
	testTo   string
	test     *view.MailTestResult
	set      *store.InstanceSettings // the Global or letters form as typed, when refused
}

// postInstanceSettings saves the Global form or the letters form; the
// values of the other stay as they are.
func (s *Server) postInstanceSettings(w http.ResponseWriter, r *http.Request, sess *session.Session, op string) {
	cur := s.users.Settings()
	in := user.InstanceUpdate{
		Name: cur.Name, IdleHours: cur.IdleHours, SessionDays: cur.SessionDays, InviteHours: cur.InviteHours,
		PeopleMail: s.mailPolicy().People, MailMaxAttachMB: cur.MailMaxAttachMB, MailMaxMessageMB: cur.MailMaxMessageMB,
		MailMaxRecipients: cur.MailMaxRecipients, MailMaxPerDay: cur.MailMaxPerDay,
	}
	atoi := func(name string) int {
		n, err := strconv.Atoi(strings.TrimSpace(r.FormValue(name)))
		if err != nil {
			return -1 // out of every range: the field says so
		}
		return n
	}
	if op == "instance" {
		in.Name = r.FormValue("name")
		if strings.TrimSpace(in.Name) == "Carrel" {
			in.Name = ""
		}
		in.InviteHours, in.IdleHours, in.SessionDays = atoi("inviteTTL"), atoi("idle"), atoi("session")
	} else {
		v := r.FormValue("people")
		in.PeopleMail = v == "on" || v == "true"
		in.MailMaxAttachMB, in.MailMaxMessageMB = atoi("attach"), atoi("letter")
		in.MailMaxRecipients, in.MailMaxPerDay = atoi("recipients"), atoi("perDay")
	}
	if err := s.users.SetInstance(sess.AccountID(), in); err != nil {
		typed := cur
		typed.Name, typed.IdleHours, typed.SessionDays, typed.InviteHours = in.Name, in.IdleHours, in.SessionDays, in.InviteHours
		typed.PeopleMail = &in.PeopleMail
		typed.MailMaxAttachMB, typed.MailMaxMessageMB = in.MailMaxAttachMB, in.MailMaxMessageMB
		typed.MailMaxRecipients, typed.MailMaxPerDay = in.MailMaxRecipients, in.MailMaxPerDay
		flash := settingsFlash{set: &typed, errField: "name", errText: string(formError(err))}
		var bad *user.ErrSetting
		if errors.As(err, &bad) {
			flash.errField, flash.errText = bad.Field, bad.Msg
		}
		s.writePage(w, r, http.StatusOK, s.adminSettingsPage(sess, flash))
		return
	}
	s.redirect(w, r, "/admin/settings")
}

func (s *Server) getAdminSettings(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	s.writePage(w, r, http.StatusOK, s.adminSettingsPage(sess, settingsFlash{}))
}

// postAdminSettings stores the relay (S-26). The password field is never
// filled in: empty keeps the stored one.
func (s *Server) postAdminSettings(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if op := r.FormValue("op"); op == "instance" || op == "limits" {
		s.postInstanceSettings(w, r, sess, op)
		return
	}
	port, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	in := store.SMTPSettings{
		Host: r.FormValue("host"), Port: port, Security: r.FormValue("security"),
		Username: r.FormValue("username"), Password: r.FormValue("password"),
		From: r.FormValue("from"), FromName: r.FormValue("fromName"), Sender: r.FormValue("sender"),
	}
	if err := s.users.SetSMTP(sess.AccountID(), in); err != nil {
		in.Password = ""
		flash := settingsFlash{form: &in, errField: "host", errText: string(formError(err))}
		var bad *user.ErrSMTP
		if errors.As(err, &bad) {
			flash.errField, flash.errText = bad.Field, bad.Msg
		}
		s.writePage(w, r, http.StatusOK, s.adminSettingsPage(sess, flash))
		return
	}
	s.redirect(w, r, "/admin/settings")
}

// postAdminMailTest sends a test letter and shows the relay's whole answer,
// or its whole refusal — never the relay's password, which is in neither.
func (s *Server) postAdminMailTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	to := strings.TrimSpace(r.FormValue("to"))
	var result view.MailTestResult
	addrs, err := cmail.Addresses([]string{to})
	switch {
	case !s.mailConfigured():
		result.Reply = text("SMTP is not configured.")
	case err != nil || len(addrs) != 1:
		result.Reply = text("Enter one address to send the test letter to.")
	default:
		reply, err := s.sendSystemMail(r.Context(), addrs[0], "A test letter from Carrel",
			s.letter(r, "This is a test letter. The relay took it, and it reached you: letters from this instance work."))
		if err != nil {
			result.Reply = text(err.Error())
		} else {
			result.OK, result.Reply = true, text(reply)
		}
	}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetMain, result, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, s.adminSettingsPage(sess, settingsFlash{testTo: to, test: &result}))
}

func (s *Server) adminSettingsPage(sess *session.Session, flash settingsFlash) view.AdminSettingsPage {
	cur := s.users.SMTP()
	shown := cur
	if flash.form != nil {
		shown = *flash.form
	}
	page := view.AdminSettingsPage{Shell: s.buildShell(sess, "Instance settings"), Test: flash.test}
	page.Rail = s.adminRail(sess, "/admin/settings")
	num := func(n int) view.Text { return text(strconv.Itoa(n)) }

	origin := s.cfg.PublicURL
	if origin == "" {
		origin = "the address each request comes to"
	}
	set := s.users.Settings()
	if flash.set != nil {
		set = *flash.set
	}
	orDefault := func(v, def int) view.Text {
		if v != 0 {
			return num(v)
		}
		return num(def)
	}
	page.Instance = view.Form{
		Action: s.route("/admin/settings"), Method: "POST",
		Sets: []view.FieldSet{{Legend: "Global", Rows: []view.FormRow{
			csrfField(sess),
			{Name: "op", Kind: view.InputHidden, Value: "instance"},
			{Name: "inviteTTL", Label: "Invite lifetime, h", Kind: view.InputNumber, Value: orDefault(set.InviteHours, int(conf.InviteTTL.Hours())),
				Hint: "From 1 to 720. A new invitation, and one extended, live this long."},
			{Name: "idle", Label: "Idle timeout, h", Kind: view.InputNumber, Value: orDefault(set.IdleHours, int(conf.SessionIdleTimeout.Hours())),
				Hint: "From 1 to 168. Applies to sessions already open."},
			{Name: "session", Label: "Session life, d", Kind: view.InputNumber, Value: orDefault(set.SessionDays, int(conf.SessionAbsoluteTimeout.Hours()/24)),
				Hint: "From 1 to 365. Applies to sessions started from now on."},
			{Name: "name", Label: "Instance name", Kind: view.InputText, Value: text(set.InstanceName()),
				Hint: "On the sign-in screen and in the letters this instance sends."},
			{Name: "publicURL", Label: "Public address", Kind: view.InputText, Value: text(origin), ReadOnly: true,
				Hint: "Links in letters start with it. Set with CARREL_PUBLIC_URL."},
		}}},
		Actions: []view.Action{{ID: "admin.settings.instance", Label: "Save", Method: "POST", Enabled: true}},
	}

	port := ""
	if shown.Port > 0 {
		port = strconv.Itoa(shown.Port)
	} else if shown.Host == "" {
		port = "587"
	}
	security := shown.Security
	if security == "" {
		security = cmail.StartTLS
	}
	option := func(v, label string) view.Option {
		return view.Option{Value: v, Label: text(label), Selected: security == v}
	}
	var pwHint view.Text
	if cur.Password != "" {
		pwHint = "Leave blank to keep the current one."
	}
	legend := view.Text("Mail — not configured")
	if cur.Configured() {
		legend = "Mail"
	}
	page.SMTP = view.Form{
		Action: s.route("/admin/settings"), Method: "POST",
		Sets: []view.FieldSet{{Legend: legend, Rows: []view.FormRow{
			csrfField(sess),
			{Name: "host", Label: "Host", Kind: view.InputText, Value: text(shown.Host),
				Hint: "Empty turns mail off: invitations and links are then handed over in person."},
			{Name: "port", Label: "Port", Kind: view.InputNumber, Value: text(port)},
			{Name: "security", Label: "Encryption", Kind: view.InputSelect, Options: []view.Option{
				option(cmail.StartTLS, "STARTTLS"), option(cmail.TLS, "TLS"), option(cmail.None, "None — a relay on this host or network"),
			}},
			{Name: "username", Label: "Username", Kind: view.InputText, Value: text(shown.Username)},
			{Name: "password", Label: "Password", Kind: view.InputPassword, Hint: pwHint},
			{Name: "from", Label: "From address", Kind: view.InputEmail, Value: text(shown.From),
				Hint: "The instance's own letters go from it, and it is the envelope sender of every letter. A person's letter never goes from their own domain: the relay vouches for this one."},
			{Name: "fromName", Label: "From name", Kind: view.InputText, Value: text(shown.FromName)},
			{Name: "sender", Label: "Sender address", Kind: view.InputRadio, Options: senderOptions(shown),
				Hint: "Of people's letters. Your relay decides which of the two it will let out. The envelope sender stays the From address either way, so SPF is unaffected; DKIM and DMARC for this domain are set up on the relay, not here. Replies go to the person's confirmed address in both cases."},
		}}},
		Actions: []view.Action{{ID: "admin.settings.save", Label: "Save", Method: "POST", Enabled: true}},
	}
	for i := range page.SMTP.Sets[0].Rows {
		if flash.errText != "" && page.SMTP.Sets[0].Rows[i].Name == flash.errField {
			page.SMTP.Sets[0].Rows[i].Error = text(flash.errText)
		}
	}

	page.Mail = view.Form{
		Action: s.route("/admin/settings/mail-test"), Method: "POST",
		Sets: []view.FieldSet{{Legend: "Test", Rows: []view.FormRow{
			csrfField(sess),
			{Name: "to", Label: "Send test message to", Kind: view.InputEmail, Value: text(flash.testTo)},
		}}},
		Actions: []view.Action{{ID: "admin.settings.mailtest", Label: "Send test", Method: "POST", Enabled: cur.Configured()}},
	}

	people := s.mailPolicy().People
	if flash.set != nil && flash.set.PeopleMail != nil {
		people = *flash.set.PeopleMail
	}
	mb := func(v int, bytes int64) view.Text {
		if v != 0 {
			return num(v)
		}
		return num(int(bytes >> 20))
	}
	page.Limits = view.Form{
		Action: s.route("/admin/settings"), Method: "POST",
		Sets: []view.FieldSet{{Legend: "Letters from users", Rows: []view.FormRow{
			csrfField(sess),
			{Name: "op", Kind: view.InputHidden, Value: "limits"},
			{Name: "people", Label: "Let users send letters and links from Carrel", Kind: view.InputCheck, Value: text(boolStr(people)),
				Hint: "Everyone signed in may send mail through this relay, from this domain. With self-registration on, that is an open sending form."},
			{Name: "attach", Label: "Attachment, MB", Kind: view.InputNumber, Value: mb(set.MailMaxAttachMB, s.cfg.MailMaxAttachBytes),
				Hint: "From 1 to 100. A larger file goes as a published link, shown as such before sending."},
			{Name: "letter", Label: "Letter, MB", Kind: view.InputNumber, Value: mb(set.MailMaxMessageMB, s.cfg.MailMaxMessageBytes),
				Hint: "From 1 to 100, and not less than an attachment."},
			{Name: "recipients", Label: "Recipients", Kind: view.InputNumber, Value: orDefault(set.MailMaxRecipients, s.cfg.MailMaxRecipients),
				Hint: "From 1 to 100."},
			{Name: "perDay", Label: "Letters a day", Kind: view.InputNumber, Value: orDefault(set.MailMaxPerDay, s.cfg.MailMaxPerDay),
				Hint: "From 1 to 1000, per person, per UTC day."},
		}}},
		Actions: []view.Action{{ID: "admin.settings.limits", Label: "Save", Method: "POST", Enabled: true}},
	}
	for _, f := range []*view.Form{&page.Instance, &page.Limits} {
		for i := range f.Sets[0].Rows {
			if flash.errText != "" && f.Sets[0].Rows[i].Name == flash.errField {
				f.Sets[0].Rows[i].Error = text(flash.errText)
			}
		}
	}
	return page
}

// senderOptions are the two forms of a person's From (S-26.1), shown as the
// addresses they make.
func senderOptions(st store.SMTPSettings) []view.Option {
	service, login := st.From, "login@…"
	if at := strings.LastIndex(st.From, "@"); at >= 0 {
		login = "login" + st.From[at:]
	}
	if service == "" {
		service = "the From address"
	}
	return []view.Option{
		{Value: cmail.SenderService, Label: text(service), Selected: st.Sender != cmail.SenderLogin},
		{Value: cmail.SenderLogin, Label: text(login), Selected: st.Sender == cmail.SenderLogin},
	}
}

// signInTitle names the instance on the sign-in screen when it has a name
// of its own.
func (s *Server) signInTitle() string {
	if name := s.users.Settings().Name; name != "" {
		return "Sign in to " + name
	}
	return "Sign in"
}
