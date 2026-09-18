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
	port, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	in := store.SMTPSettings{
		Host: r.FormValue("host"), Port: port, Security: r.FormValue("security"),
		Username: r.FormValue("username"), Password: r.FormValue("password"),
		From: r.FormValue("from"), FromName: r.FormValue("fromName"),
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
	num := func(n int) view.Text { return text(strconv.Itoa(n)) }

	origin := s.cfg.PublicURL
	if origin == "" {
		origin = "the address each request comes to"
	}
	page.Instance = view.Form{Sets: []view.FieldSet{{Legend: "Global", Rows: []view.FormRow{
		{Name: "publicURL", Label: "Public address", Kind: view.InputText, Value: text(origin), ReadOnly: true,
			Hint: "Links in letters start with it. Set with CARREL_PUBLIC_URL."},
		{Name: "inviteTTL", Label: "Invite lifetime, h", Kind: view.InputNumber, Value: num(int(conf.InviteTTL.Hours())), ReadOnly: true},
		{Name: "idle", Label: "Idle timeout, h", Kind: view.InputNumber, Value: num(int(conf.SessionIdleTimeout.Hours())), ReadOnly: true},
		{Name: "session", Label: "Session life, d", Kind: view.InputNumber, Value: num(int(conf.SessionAbsoluteTimeout.Hours() / 24)), ReadOnly: true},
	}}}}

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
				Hint: "Every letter goes from this address, a person's too: the relay vouches for this domain, not for theirs."},
			{Name: "fromName", Label: "From name", Kind: view.InputText, Value: text(shown.FromName)},
		}}},
		Actions: []view.Action{{ID: "admin.settings.save", Label: "Save", Method: "POST", Enabled: true}},
	}
	for i := range page.SMTP.Sets[0].Rows {
		if flash.errText != "" && page.SMTP.Sets[0].Rows[i].Name == flash.errField {
			page.SMTP.Sets[0].Rows[i].Error = text(flash.errText)
		}
	}

	people := "off"
	if s.cfg.MailEnabled {
		people = "on"
	}
	page.Mail = view.Form{
		Action: s.route("/admin/settings/mail-test"), Method: "POST",
		Sets: []view.FieldSet{{Legend: "Letters", Rows: []view.FormRow{
			csrfField(sess),
			{Name: "people", Label: "Letters from users", Kind: view.InputText, Value: text(people), ReadOnly: true,
				Hint: "When on, everyone signed in may send mail through this relay, from this address, with their name and — once confirmed — their address to reply to. Set with CARREL_MAIL_ENABLED; off by default."},
			{Name: "sender", Label: "Sender address", Kind: view.InputText, Value: text(cur.From), ReadOnly: true,
				Hint: "A person's login is never the sender."},
			{Name: "to", Label: "Send test message to", Kind: view.InputEmail, Value: text(flash.testTo)},
		}}},
		Actions: []view.Action{{ID: "admin.settings.mailtest", Label: "Send test", Method: "POST", Enabled: cur.Configured()}},
	}

	mb := func(n int64) view.Text {
		return text(strconv.FormatFloat(float64(n)/(1<<20), 'f', -1, 64))
	}
	page.Limits = view.Form{Sets: []view.FieldSet{{Legend: "Limits of a person's letters", Rows: []view.FormRow{
		{Name: "attach", Label: "Attachment, MB", Kind: view.InputNumber, Value: mb(s.cfg.MailMaxAttachBytes), ReadOnly: true,
			Hint: "A larger file goes as a published link, shown as such before sending. CARREL_MAIL_MAX_ATTACH_BYTES."},
		{Name: "letter", Label: "Letter, MB", Kind: view.InputNumber, Value: mb(s.cfg.MailMaxMessageBytes), ReadOnly: true,
			Hint: "CARREL_MAIL_MAX_MESSAGE_BYTES."},
		{Name: "recipients", Label: "Recipients", Kind: view.InputNumber, Value: num(s.cfg.MailMaxRecipients), ReadOnly: true,
			Hint: "CARREL_MAIL_MAX_RECIPIENTS."},
		{Name: "perDay", Label: "Letters a day", Kind: view.InputNumber, Value: num(s.cfg.MailMaxPerDay), ReadOnly: true,
			Hint: "Per person, per UTC day. CARREL_MAIL_MAX_PER_DAY."},
	}}}}
	return page
}
