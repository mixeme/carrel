package web

import (
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

// The instance's settings are one form (s-admin-3, Ад17): Global, Mail,
// Letters from users and the test letter, Save and Discard in the head. A
// save stores all of it or nothing; the test letter is a button of the same
// form that sends what is typed and stores nothing (Ад37).

// settingsFlash is what one POST leaves on the page: the form as typed and
// the refusals at their rows, or the test letter's result.
type settingsFlash struct {
	typed   *typedSettings
	refused map[string]string
	testTo  string
	test    *view.MailTestResult
}

// typedSettings is the form as it was sent.
type typedSettings struct {
	set   user.InstanceUpdate
	relay store.SMTPSettings
}

// settingsTyped reads the whole form. What the form did not carry — the
// sender and the ceilings of people's letters while they are off, which the
// page does not show (Typecase's note 88) — stays as it is stored.
func (s *Server) settingsTyped(r *http.Request) typedSettings {
	_ = r.ParseForm()
	stored := s.users.Settings()
	kept := map[string]int{"attach": stored.MailMaxAttachMB, "letter": stored.MailMaxMessageMB,
		"recipients": stored.MailMaxRecipients, "perDay": stored.MailMaxPerDay}
	atoi := func(name string) int {
		if _, sent := r.Form[name]; !sent {
			if v, ok := kept[name]; ok {
				return v
			}
		}
		n, err := strconv.Atoi(strings.TrimSpace(r.FormValue(name)))
		if err != nil {
			return -1 // out of every range: the field says so
		}
		return n
	}
	in := user.InstanceUpdate{Name: r.FormValue("name")}
	if strings.TrimSpace(in.Name) == "Carrel" {
		in.Name = ""
	}
	in.InviteHours, in.IdleHours, in.SessionDays = atoi("inviteTTL"), atoi("idle"), atoi("session")
	v := r.FormValue("people")
	in.PeopleMail = v == "on" || v == "true"
	in.MailMaxAttachMB, in.MailMaxMessageMB = atoi("attach"), atoi("letter")
	in.MailMaxRecipients, in.MailMaxPerDay = atoi("recipients"), atoi("perDay")
	port, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	relay := store.SMTPSettings{
		Host: r.FormValue("host"), Port: port, Security: r.FormValue("security"),
		Username: r.FormValue("username"), Password: r.FormValue("password"),
		From: r.FormValue("from"), FromName: r.FormValue("fromName"), Sender: r.FormValue("sender"),
	}
	if _, sent := r.Form["sender"]; !sent {
		relay.Sender = s.users.SMTP().Sender
	}
	return typedSettings{set: in, relay: relay}
}

// keepDefaults is the form with each value the default shows as zero:
// zero keeps the default, and a form saved as shown changes nothing.
func (s *Server) keepDefaults(in user.InstanceUpdate) user.InstanceUpdate {
	zero := func(v *int, def int) {
		if *v == def {
			*v = 0
		}
	}
	zero(&in.InviteHours, int(conf.InviteTTL.Hours()))
	zero(&in.IdleHours, int(conf.SessionIdleTimeout.Hours()))
	zero(&in.SessionDays, int(conf.SessionAbsoluteTimeout.Hours()/24))
	zero(&in.MailMaxAttachMB, int(s.cfg.MailMaxAttachBytes>>20))
	zero(&in.MailMaxMessageMB, int(s.cfg.MailMaxMessageBytes>>20))
	zero(&in.MailMaxRecipients, s.cfg.MailMaxRecipients)
	zero(&in.MailMaxPerDay, s.cfg.MailMaxPerDay)
	return in
}

func (s *Server) getAdminSettings(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	s.writePage(w, r, http.StatusOK, s.adminSettingsPage(sess, settingsFlash{}))
}

// postAdminSettings stores the whole form (S-26 for the relay). The
// password field is never filled in: empty keeps the stored one.
func (s *Server) postAdminSettings(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	typed := s.settingsTyped(r)
	refused, err := s.users.SaveSettings(sess.AccountID(), s.keepDefaults(typed.set), typed.relay)
	if err != nil {
		refused = []*user.ErrSetting{{Field: "name", Msg: string(formError(err))}}
	}
	if len(refused) > 0 {
		typed.relay.Password = ""
		flash := settingsFlash{typed: &typed, refused: map[string]string{}}
		for _, e := range refused {
			if _, ok := flash.refused[e.Field]; !ok {
				flash.refused[e.Field] = e.Msg
			}
		}
		s.writePage(w, r, http.StatusOK, s.adminSettingsPage(sess, flash))
		return
	}
	s.redirect(w, r, "/admin/settings")
}

// postAdminMailTest sends a test letter through the relay as typed in the
// form — saved or not, so the relay is tried before Save — and stores
// nothing (Ад37; s-admin-3). An empty password is the stored one. The page
// comes back with what was typed and the relay's whole answer, or its whole
// refusal — never the relay's password, which is in neither
// (AdminSettingsPage.Test).
func (s *Server) postAdminMailTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	typed := s.settingsTyped(r)
	relay := typed.relay
	refused := user.CheckSMTP(&relay)
	stored := s.users.SMTP()
	if relay.Password == "" && relay.Username != "" && relay.Username == stored.Username {
		relay.Password = stored.Password
	}
	typed.relay.Password = ""
	to := strings.TrimSpace(r.FormValue("to"))
	flash := settingsFlash{typed: &typed, testTo: to, refused: map[string]string{}}
	var result view.MailTestResult
	addrs, err := cmail.Addresses([]string{to})
	switch {
	case relay.Host == "":
		result.Reply = text(mailNotConfigured)
	case len(refused) > 0:
		for _, e := range refused {
			if _, ok := flash.refused[e.Field]; !ok {
				flash.refused[e.Field] = e.Msg
			}
		}
		result.Reply = text("The relay as typed cannot be used: see the marked fields.")
	case err != nil || len(addrs) != 1:
		result.Reply = text("Enter one address to send the test letter to.")
	default:
		via := relayOf(relay)
		if via.FromName == "" {
			via.FromName = s.users.Settings().InstanceName()
		}
		reply, err := s.sendWith(r.Context(), via, addrs[0], "A test letter from Carrel",
			s.letter(r, "This is a test letter. The relay took it, and it reached you: letters from this instance work."))
		if err != nil {
			result.Reply = text(err.Error())
		} else {
			result.OK, result.Reply = true, text(reply)
		}
	}
	flash.test = &result
	// The page, with scripts or without: a result is no zone of C-7.2 (the
	// lead's decision П2, 02.10.2026).
	s.writePage(w, r, http.StatusOK, s.adminSettingsPage(sess, flash))
}

// mailNotConfigured is why the test letter cannot go (s-admin-3).
const mailNotConfigured = "Mail is not configured: there is nothing to send through yet."

func (s *Server) adminSettingsPage(sess *session.Session, flash settingsFlash) view.AdminSettingsPage {
	cur := s.users.SMTP()
	shown := cur
	set := s.users.Settings()
	people := s.mailPolicy().People
	if t := flash.typed; t != nil {
		shown = t.relay
		set.Name, set.IdleHours, set.SessionDays, set.InviteHours = t.set.Name, t.set.IdleHours, t.set.SessionDays, t.set.InviteHours
		set.MailMaxAttachMB, set.MailMaxMessageMB = t.set.MailMaxAttachMB, t.set.MailMaxMessageMB
		set.MailMaxRecipients, set.MailMaxPerDay = t.set.MailMaxRecipients, t.set.MailMaxPerDay
		people = t.set.PeopleMail
	}
	// «Settings», as the rail and the frame name it (Ад16; s-admin-3), with
	// what it holds under it.
	page := view.AdminSettingsPage{Shell: s.buildShell(sess, "Settings"), Subtitle: "Global settings and mail", Test: flash.test,
		Discard: view.Action{ID: "admin.settings.discard", Label: "Discard", Href: s.route("/admin/settings"), Method: "GET", Enabled: true}}
	page.Rail = s.adminRail(sess, "/admin/settings")
	num := func(n int) view.Text { return text(strconv.Itoa(n)) }
	orDefault := func(v, def int) view.Text {
		if v != 0 {
			return num(v)
		}
		return num(def)
	}
	mb := func(v int, bytes int64) view.Text {
		if v != 0 {
			return num(v)
		}
		return num(int(bytes >> 20))
	}

	origin := s.cfg.PublicURL
	if origin == "" {
		origin = "the address each request comes to"
	}
	// The ranges are not said beside the fields (Ад19): a value out of one
	// is refused at its row with it.
	global := view.FieldSet{Legend: "Global", Rows: []view.FormRow{
		csrfField(sess),
		{Name: "inviteTTL", Label: "Invite lifetime, h", Kind: view.InputNumber, Mono: true, Value: orDefault(set.InviteHours, int(conf.InviteTTL.Hours()))},
		{Name: "idle", Label: "Idle timeout, h", Kind: view.InputNumber, Mono: true, Value: orDefault(set.IdleHours, int(conf.SessionIdleTimeout.Hours()))},
		{Name: "session", Label: "Session life, d", Kind: view.InputNumber, Mono: true, Value: orDefault(set.SessionDays, int(conf.SessionAbsoluteTimeout.Hours()/24))},
		{Name: "name", Label: "Instance name", Kind: view.InputText, Value: text(set.InstanceName())},
		{Name: "publicURL", Label: "Public address", Kind: view.InputText, Mono: true, Value: text(origin), ReadOnly: true,
			Hint: "Links in letters start with it. Set with CARREL_PUBLIC_URL."},
	}}

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
		return view.Option{Value: v, Label: text(label), Selected: security == v, Separator: v == cmail.None}
	}
	// The relay's fields with the samples of the frame (Ад20); its rubric
	// says when there is none (Ад18).
	relay := view.FieldSet{Legend: "Mail", Rows: []view.FormRow{
		{Name: "host", Label: "Host", Kind: view.InputText, Mono: true, Value: text(shown.Host), Placeholder: "smtp.example.org"},
		{Name: "port", Label: "Port", Kind: view.InputNumber, Value: text(port), Mono: true},
		// The list of s-admin-9: None under a line.
		{Name: "security", Label: "Encryption", Kind: view.InputSelect, Value: text(security), Options: []view.Option{
			option(cmail.StartTLS, "STARTTLS"), option(cmail.TLS, "TLS"), option(cmail.None, "None"),
		}},
		{Name: "username", Label: "Username", Kind: view.InputText, Value: text(shown.Username),
			// A relay may take letters without a login (s-admin-3: «—»).
			Placeholder: "—"},
		{Name: "password", Label: "Password", Kind: view.InputPassword, Placeholder: "leave blank to keep the current one"},
		{Name: "from", Label: "From address", Kind: view.InputEmail, Value: text(shown.From), Placeholder: "carrel@example.org"},
		{Name: "fromName", Label: "From name", Kind: view.InputText, Value: text(shown.FromName), Placeholder: text(s.users.Settings().InstanceName())},
	}}
	if !cur.Configured() {
		relay.Badges = []view.Badge{{Label: "not configured", Role: view.BadgeAlert}}
	}

	// People's letters: whether they go, from which address (Ад21: the
	// sender, after the relay is set up), and their ceilings.
	// The envelope's address monospaced, as the frame sets it (s-admin-3).
	var envelope any = "the From address"
	if shown.From != "" {
		envelope = monoRun(shown.From)
	}
	// People's letters go only through the mail configured (the owner,
	// 19.10.2026; Typecase's note 88): without it the tick is off with its
	// reason; while they are off, as saved, the sender and the ceilings
	// are not shown at all (s-admin-3, -32, -33).
	tick := view.FormRow{Name: "people", Label: "Let users send letters and links from Carrel", Kind: view.InputCheck, Value: text(boolStr(people)),
		Hint: "Everyone signed in may send mail through this relay, from this domain. With self-registration on, that is an open sending form."}
	if !cur.Configured() {
		tick.Value, tick.Reason = "", mailNotConfigured
	}
	letters := view.FieldSet{Legend: "Letters from users", Rows: []view.FormRow{tick,
		{Name: "sender", Label: "Sender address", Kind: view.InputRadio, Options: senderOptions(shown),
			HintRuns: relaySenderHint(envelope)},
		{Name: "attach", Label: "Attachment, MB", Kind: view.InputNumber, Mono: true, Value: mb(set.MailMaxAttachMB, s.cfg.MailMaxAttachBytes)},
		{Name: "letter", Label: "Letter, MB", Kind: view.InputNumber, Mono: true, Value: mb(set.MailMaxMessageMB, s.cfg.MailMaxMessageBytes)},
		{Name: "recipients", Label: "Recipients", Kind: view.InputNumber, Mono: true, Value: orDefault(set.MailMaxRecipients, s.cfg.MailMaxRecipients)},
		{Name: "perDay", Label: "Letters a day", Kind: view.InputNumber, Mono: true, Value: orDefault(set.MailMaxPerDay, s.cfg.MailMaxPerDay)},
	}}
	switch {
	case !s.mailPolicy().People || !cur.Configured():
		letters.Rows = letters.Rows[:1]
		letters.Badges = []view.Badge{{Label: "off", Role: view.BadgeNeutral}}
	default:
		letters.Badges = []view.Badge{{Label: "on", Role: view.BadgeNeutral}}
	}

	test := view.FieldSet{Legend: "Test", Rows: []view.FormRow{
		{Name: "to", Label: "Send test message to", Kind: view.InputEmail, Value: text(flash.testTo), Placeholder: "you@example.org"},
	}}
	// The test letter sends what is typed (Ад37) and is never off: without
	// scripts a button off by what was saved would not come on with the
	// Host typed (Typecase's note 82, 195 p. 5). No relay typed, the answer
	// says so under the test (mailNotConfigured).
	send := view.Action{ID: "admin.settings.mailtest", Label: "Send test", Href: s.route("/admin/settings/mail-test"), Method: "POST", Submits: true, Enabled: true}

	page.Form = view.Form{
		Action: s.route("/admin/settings"), Method: "POST",
		Sets:    []view.FieldSet{global, relay, letters, test},
		Actions: []view.Action{{ID: "admin.settings.save", Label: "Save", Method: "POST", Primary: true, Enabled: true}, send},
	}
	for i := range page.Form.Sets {
		for j := range page.Form.Sets[i].Rows {
			if msg, ok := flash.refused[page.Form.Sets[i].Rows[j].Name]; ok {
				page.Form.Sets[i].Rows[j].Error = text(msg)
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

// relaySenderHint is the hint of «Sender address» (s-admin-32), the
// envelope sender in it.
func relaySenderHint(envelope any) view.Runs {
	return join("Your relay decides which of the two it lets out. The envelope sender stays ", envelope, ", so SPF is unaffected; DKIM and DMARC are set up on the relay.")
}
