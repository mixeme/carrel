package web

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// formError turns a service error into the text a form may show. Only the
// errors a person can act on are named; anything else (a failed write, a
// path in an os error) stays out of the response.
func formError(err error) view.Text {
	for _, known := range []error{
		user.ErrPasswordShort, user.ErrPasswordMismatch, user.ErrLoginTaken, user.ErrLastAdmin, user.ErrLoginConfirm,
		user.ErrLoginEmpty, user.ErrSelfRegNeedsMail, user.ErrBadAddress,
		user.ErrMasterPasswordShort, user.ErrMasterPasswordMismatch,
		user.ErrEscrowNotConfigured, user.ErrEscrowConfigured, user.ErrNotDeposited, user.ErrEscrowDisabled,
	} {
		if errors.Is(err, known) {
			return text(refusal(known.Error()))
		}
	}
	return text("Something went wrong. Try again.")
}

// refusal is an error said at a field or over a page: capitalised and
// ended with a full stop, «Login is already taken.» (s-admin-19). Go keeps
// its error strings lower-case and bare.
func refusal(s string) string {
	if s = sentence(s); s != "" && !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

// passwordPair validates a new password and its repeat the way every
// password form does (S-10.2, S-10.4): length first, then equality.
func passwordPair(password, password2 string) error {
	if len(password) < conf.MinPasswordLen {
		return user.ErrPasswordShort
	}
	if password != password2 {
		return user.ErrPasswordMismatch
	}
	return nil
}

// passwordError says a refused new password at its field: a repeat that
// differs at the repeat, anything else at the password (S-10.2). pair is
// the password's row and its repeat's.
func passwordError(pair []view.FormRow, err error) {
	switch {
	case err == nil:
	case errors.Is(err, user.ErrPasswordMismatch):
		pair[1].Error = formError(err)
	default:
		pair[0].Error = formError(err)
	}
}

// The tagline in its two lines (C-1): the sign-in and «about» say it, and
// nothing else does (s-auth-1, rest-3).
const (
	taglineLead = "One desk. Every server."
	taglineText = "Calendars, contacts, tasks, notes and files — one place."
)

// passwordWarning stands over the fields of every form that sets a password
// from outside the shell (s-auth-2, s-auth-4, s-auth-11, s-auth-12).
const passwordWarning = "Your password is the only key to your own data. Nobody — including this server — can recover it for you."

func passwordNotice() []view.Notice {
	return []view.Notice{{Role: view.NoticeAlert, Body: passwordWarning}}
}

// backToSignIn is the one door of a page outside the shell that has done
// its work (s-auth-3, s-auth-5 to s-auth-9, rest-3).
func (s *Server) backToSignIn(id string) view.Action {
	return view.Action{ID: id, Label: "Back to sign in", Href: s.route("/login"), Method: "GET", Enabled: true}
}

func (s *Server) getHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

// getAbout is rest-3. Its button knows where one came from: without a
// session it leads to sign-in, as drawn; from the shell, back to the page
// of the shell one was on (U-6.14).
func (s *Server) getAbout(w http.ResponseWriter, r *http.Request) {
	page := view.AboutPage{
		Plain:   view.Plain{Title: "Carrel", Lede: taglineLead},
		Tagline: taglineText,
		Version: text(s.version),
		Commit:  text(s.commit),
		Licence: "AGPL-3.0-or-later: if you run a modified version for others, you must make the source available to them.",
		Source: view.Action{
			ID: "about.source", Label: "Source code", Href: "https://carrel.dev", Method: "GET", Enabled: true,
		},
		Third: view.Action{ID: "about.third", Label: "Third-party licences", Href: s.route("/about"), Method: "GET", Enabled: true},
		Back:  s.backToSignIn("about.back"),
	}
	if _, ok := s.sessionFrom(r); ok {
		page.Back.Label, page.Back.Href = "Back", s.cameFrom(r)
	}
	s.writePage(w, r, http.StatusOK, page)
}

// cameFrom is the page of the shell a request came from: the Referer when
// it is this origin and under /app, else the first screen. Nothing of
// another origin is ever led to.
func (s *Server) cameFrom(r *http.Request) string {
	home := s.route("/app/contacts")
	ref, err := url.Parse(r.Referer())
	if err != nil || ref.Host != r.Host || ref.User != nil || !strings.HasPrefix(ref.EscapedPath(), s.route("/app/")) {
		return home
	}
	back := ref.EscapedPath()
	if ref.RawQuery != "" {
		back += "?" + ref.RawQuery
	}
	return back
}

func (s *Server) getLogin(w http.ResponseWriter, r *http.Request) {
	if s.users.FirstRun() {
		s.redirect(w, r, "/setup")
		return
	}
	if sess, ok := s.sessionFrom(r); ok {
		s.redirect(w, r, "/app/contacts")
		_ = sess
		return
	}
	s.writePage(w, r, http.StatusOK, s.loginPage(w, r, "", ""))
}

// loginPage is the sign-in form (s-auth-1), empty or back with what was
// typed and why it failed. The sign stands in place of the title, which
// stays the document's; under it the tagline. Under Sign in, the links:
// «Create an account» only while registration is open — a closed instance
// does not advertise a form that refuses everyone —, then «Forgotten your
// password?». Every link goes through the base path.
func (s *Server) loginPage(w http.ResponseWriter, r *http.Request, login string, errText view.Text) view.LoginPage {
	page := view.LoginPage{
		Plain:   view.Plain{Title: text(s.signInTitle()), Lede: taglineLead},
		Tagline: taglineText,
		Form: withSubmit(s.anonPlainForm(w, r, "/login", "POST",
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Value: text(login), Required: true},
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true, Error: errText},
		), "login.submit", "Sign in"),
		Forgot: view.Action{ID: "login.forgot", Label: "Forgotten your password?", Href: s.route("/forgot"), Method: "GET", Enabled: true},
	}
	if s.users.RegisterOpen() {
		page.Register = &view.Action{ID: "login.register", Label: "Create an account", Href: s.route("/register"), Method: "GET", Enabled: true}
	}
	return page
}

func (s *Server) postLogin(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := s.clientIP(r)
	login := r.FormValue("login")
	if !s.limit.allowLogin(login, ip, start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	password := r.FormValue("password")
	accountID, dataKey, mustChange, ok := s.users.Authenticate(r.Context(), login, password)
	if !ok {
		constantTimeDelay(start, 300*time.Millisecond)
		page := s.loginPage(w, r, login, text("Login or password is wrong."))
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	// A new SID on every login (security.md): whatever session the
	// browser presented ends here, so the old cookie value opens nothing.
	if old, ok := s.sessionFrom(r); ok {
		s.sessions.End(old.ID())
	}
	sess, err := s.sessions.Create(accountID, normalizeLogin(login), dataKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sess.SetClient(agentLabel(r.UserAgent()), s.clientIP(r))
	s.setSessionCookie(w, r, sess.ID())
	// A job that runs at sign-in when its last copy is old starts now: the
	// session holds the data key it reads with (S-24.4).
	s.backupOnSignIn(sess)
	if mustChange {
		s.redirect(w, r, "/app/password")
		return
	}
	s.redirect(w, r, "/app/contacts")
}

func (s *Server) postLogout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := s.sessionFrom(r); ok {
		s.sessions.End(sess.ID())
	}
	s.clearSessionCookie(w, r)
	s.redirect(w, r, "/login")
}

// /setup is a route without a session (S-29.3) and its POST derives a key,
// so it lives under the sign-in buckets: five a minute for one login,
// twenty for one address (audit №3, Ф3-4).
func (s *Server) getSetup(w http.ResponseWriter, r *http.Request) {
	if !s.limit.allowAnon(s.clientIP(r), time.Now()) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	if !s.users.FirstRun() {
		s.notFound(w, r)
		return
	}
	s.writePage(w, r, http.StatusOK, s.setupPage(w, r, "", "", nil, nil))
}

// setupPage is the first account's form (s-auth-4): what it is, the warning
// over the fields, Login with the sample of what it takes, Email
// (optional), the password twice. A refusal is said at its field.
func (s *Server) setupPage(w http.ResponseWriter, r *http.Request, login, email string, loginErr, passErr error) view.SetupPage {
	rows := []view.FormRow{
		{Name: "login", Label: "Login", Kind: view.InputText, Value: text(login), Required: true, Placeholder: "a-z 0-9 . _ -"},
		{Name: "email", Label: "Email", LabelAside: "(optional)", Kind: view.InputEmail, Value: text(email), Placeholder: "—"},
		{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true},
		{Name: "password2", Label: "Repeat password", Kind: view.InputPassword, Required: true},
	}
	if loginErr != nil {
		rows[0].Error = formError(loginErr)
	}
	passwordError(rows[2:], passErr)
	return view.SetupPage{
		Plain: view.Plain{
			Title:   "Set up Carrel",
			Lede:    "The account you create here is the first one, and it can manage everyone else.",
			Notices: passwordNotice(),
		},
		Form: withSubmit(s.anonPlainForm(w, r, "/setup", "POST", rows...), "setup.submit", "Create administrator"),
	}
}

func (s *Server) postSetup(w http.ResponseWriter, r *http.Request) {
	login := r.FormValue("login")
	if !s.limit.allowLogin(login, s.clientIP(r), time.Now()) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	if !s.users.FirstRun() {
		s.notFound(w, r)
		return
	}
	password := r.FormValue("password")
	email := r.FormValue("email")
	if err := passwordPair(password, r.FormValue("password2")); err != nil {
		s.writePage(w, r, http.StatusOK, s.setupPage(w, r, login, email, nil, err))
		return
	}
	if err := s.users.SetupFirstAdminWith(r.Context(), login, password, email); err != nil {
		if errors.Is(err, user.ErrPasswordShort) {
			s.writePage(w, r, http.StatusOK, s.setupPage(w, r, login, email, nil, err))
		} else {
			s.writePage(w, r, http.StatusOK, s.setupPage(w, r, login, email, err, nil))
		}
		return
	}
	s.redirect(w, r, "/login")
}

func (s *Server) getRegister(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	if !s.users.RegisterOpen() {
		s.notFound(w, r)
		return
	}
	s.writePage(w, r, http.StatusOK, s.registerPage(w, r, "", "", nil, nil))
}

// registerPage is the registration form (s-auth-2): what happens to the
// address, the warning over the fields, Login, Email, the password twice,
// and the way to sign-in for one who has an account.
func (s *Server) registerPage(w http.ResponseWriter, r *http.Request, login, email string, loginErr, passErr error) view.RegisterPage {
	rows := []view.FormRow{
		{Name: "login", Label: "Login", Kind: view.InputText, Value: text(login), Required: true},
		{Name: "email", Label: "Email", Kind: view.InputEmail, Value: text(email), Required: true},
		{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true},
		{Name: "password2", Label: "Repeat password", Kind: view.InputPassword, Required: true},
	}
	if loginErr != nil {
		rows[0].Error = formError(loginErr)
	}
	passwordError(rows[2:], passErr)
	return view.RegisterPage{
		Plain: view.Plain{
			Title:   "Create an account",
			Lede:    "A confirmation link will be sent to the address you give; the account cannot be used until you open it.",
			Notices: passwordNotice(),
		},
		Form:   withSubmit(s.anonPlainForm(w, r, "/register", "POST", rows...), "register.submit", "Create account"),
		SignIn: view.Action{ID: "register.login", Label: "Already have an account? Sign in", Href: s.route("/login"), Method: "GET", Enabled: true},
	}
}

func (s *Server) postRegister(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := s.clientIP(r)
	if !s.limit.allowAnon(ip, start) || !s.limit.allowLogin(r.FormValue("login"), ip, start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	if !s.users.RegisterOpen() {
		s.notFound(w, r)
		return
	}
	login, email := r.FormValue("login"), strings.TrimSpace(r.FormValue("email"))
	if err := passwordPair(r.FormValue("password"), r.FormValue("password2")); err != nil {
		s.writePage(w, r, http.StatusOK, s.registerPage(w, r, login, email, nil, err))
		return
	}
	// The confirmation token goes to the mail layer (S-26), never into a
	// response.
	token, err := s.users.Register(r.Context(), login, email, r.FormValue("password"))
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.registerPage(w, r, login, email, err, nil))
		return
	}
	mailSent := token != "" && s.sendConfirmMail(r, login, email, token)
	if mailSent {
		_ = s.users.ConfirmMailed(token)
	}
	// The form went: the page says where the letter went, and its one door
	// leads to sign-in (s-auth-3).
	page := view.RegisterPage{
		Plain:  view.Plain{Title: "Create an account"},
		SignIn: s.backToSignIn("register.login"),
	}
	if mailSent {
		page.Title = "Check your email"
		page.Lede = text("The letter went to " + email + ". The account cannot be used until you open the link in it.\n\n" +
			"Sending again with the same login and address, while the account is unconfirmed, simply replaces the letter and the password.")
		page.Sent = true
	} else {
		page.Lede = "Your account was created, but the confirmation email could not be sent. Ask an administrator."
	}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) getInvite(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	token := r.PathValue("token")
	inv, valid := s.users.InviteFor(r.Context(), token)
	if !valid {
		s.tokenNotFound(w, r, start)
		return
	}
	s.writePage(w, r, http.StatusOK, s.invitePage(w, r, token, inv, "", nil, nil))
}

// invitePage is the form an invited person fills (s-auth-12): what Carrel
// does not know yet — the login and the password. The address of a letter
// is known and shown, not changed; a link has none, it is set later in the
// account. An invitation made before v0.2.68 names its login, shown the
// same way. The line under the title is by the kind of invitation, and
// without a record of who invited it has no first sentence (the frame's
// caption). The warning is the registration's (s-auth-2).
func (s *Server) invitePage(w http.ResponseWriter, r *http.Request, token string, inv user.OpenInvite, typed string, loginErr, passErr error) view.InvitePage {
	lede := "Choose a login and a password."
	login := view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Required: true, Value: text(typed)}
	rows := []view.FormRow{login}
	switch {
	case inv.Login != "":
		lede = "Choose a password; the login is the one the invitation names."
		rows[0].Value, rows[0].ReadOnly, rows[0].Required = text(inv.Login), true, false
	case inv.Email != "":
		lede = "Choose a login and a password; the address is the one the invitation went to."
	}
	if inv.Email != "" {
		rows = append(rows, view.FormRow{Name: "email", Label: "Email", Kind: view.InputEmail, Value: text(inv.Email), ReadOnly: true})
	}
	if inv.By != "" {
		lede = inv.By + " invited you. " + lede
	}
	if loginErr != nil {
		rows[0].Error = formError(loginErr)
	}
	password := view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true}
	if passErr != nil {
		password.Error = formError(passErr)
	}
	rows = append(rows, password,
		view.FormRow{Name: "password2", Label: "Repeat password", Kind: view.InputPassword, Required: true})
	return view.InvitePage{
		Plain: view.Plain{
			Title:   text("Join " + s.users.Settings().InstanceName()),
			Lede:    text(lede),
			Notices: passwordNotice(),
		},
		Valid: true,
		Form:  withSubmit(s.anonPlainForm(w, r, "/invite/"+token, "POST", rows...), "invite.submit", "Create account"),
	}
}

func (s *Server) postInvite(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	token := r.PathValue("token")
	inv, valid := s.users.InviteFor(r.Context(), token)
	if !valid {
		s.tokenNotFound(w, r, start)
		return
	}
	typed := strings.TrimSpace(r.FormValue("login"))
	if err := passwordPair(r.FormValue("password"), r.FormValue("password2")); err != nil {
		s.writePage(w, r, http.StatusOK, s.invitePage(w, r, token, inv, typed, nil, err))
		return
	}
	// A login the person chose and cannot have is said on its field: the
	// invitation lives on. Whatever else goes wrong (the token died between
	// GET and POST) answers like a dead token: the form has nothing more to
	// say to someone holding an invalid secret.
	err := s.users.AcceptInvite(r.Context(), token, typed, r.FormValue("password"))
	switch {
	case err == nil:
		s.redirect(w, r, "/login")
	case inv.Login == "" && (errors.Is(err, user.ErrLoginTaken) || errors.Is(err, user.ErrLoginEmpty)):
		s.writePage(w, r, http.StatusOK, s.invitePage(w, r, token, inv, typed, err, nil))
	default:
		s.tokenNotFound(w, r, start)
	}
}

// getConfirmEmail is rest-2: the address confirmed, one door to sign-in.
func (s *Server) getConfirmEmail(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	token := r.PathValue("token")
	ok := s.users.ConfirmEmail(r.Context(), token)
	if !ok {
		s.tokenNotFound(w, r, start)
		return
	}
	page := view.ConfirmEmailPage{
		Plain: view.Plain{Title: "Email address", Lede: "Address confirmed. You can sign in now."},
		OK:    true,
		Next:  view.Action{ID: "confirm.login", Label: "Sign in", Href: s.route("/login"), Method: "GET", Enabled: true},
	}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) getForgot(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	// The first state of /forgot (s-auth-5): one field, the login or the
	// confirmed address, and why nothing here keeps the data.
	page := view.ForgotPage{
		Plain: view.Plain{
			Title: "Forgotten your password?",
			Lede: "Enter the login or the confirmed email of the account. There is no way to set a new password and keep the data stored here: " +
				"the encryption key is derived from the password, and the server never has that key.",
		},
		Form: withSubmit(s.anonPlainForm(w, r, "/forgot", "POST",
			view.FormRow{Name: "login", Label: "Login or email", Kind: view.InputText, Required: true},
		), "forgot.submit", "Continue"),
		Back: s.backToSignIn("forgot.back"),
	}
	s.writePage(w, r, http.StatusOK, page)
}

// The states of /forgot after the first (s-auth-6 to s-auth-9), in their
// frames' words.
const (
	forgotNone = "This login has no confirmed email — an address that was entered but never confirmed does not count. " +
		"There is no letter we can send, and there is no password reset that would keep your data.\n\n" +
		"If you deposited the encryption key, ask the administrator in person. Otherwise this account cannot be recovered from the sign-in screen."
	forgotWipeBody = "We send a letter to the confirmed address. The link in it is not a reset: after you confirm, everything stored here is erased — " +
		"connections become unreadable and are deleted — and the same link then lets you set a new password. " +
		"Address books, calendars, tasks and notes on the servers stay where they are."
	forgotAskBody = "This account deposited its encryption key. The administrator can recover it with the instance master password, and your data is kept."
	forgotSent    = "If that account has a confirmed email, a letter is on its way. The link erases everything stored here after you confirm it, " +
		"and then asks for a new password. It cannot bring your data back."
	forgotAsked = "A recovery request is recorded for this account. The administrator still has to use the instance master password. " +
		"Until then, the sign-in screen will not accept a new password."
)

// postForgot answers the login or the address with what can be done
// (S-10.4): with a confirmed address, the wipe by letter, and asking the
// administrator only where the account deposited its key (s-auth-6);
// otherwise that nothing can (s-auth-7) — an unknown login, an unconfirmed
// address and a disabled account alike. Each option is a form of one
// button that carries what was typed: op=wipe sends the letter (s-auth-8),
// op=ask records the request (s-auth-9). The page names the account by what
// was typed, never by a login found for an address.
func (s *Server) postForgot(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := s.clientIP(r)
	if !s.limit.allowAnon(ip, start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	typed := strings.TrimSpace(r.FormValue("login"))
	page := view.ForgotPage{
		Plain: view.Plain{Title: "We cannot help from here", Lede: forgotNone},
		Back:  s.backToSignIn("forgot.back"),
	}
	switch r.FormValue("op") {
	case "wipe":
		// The wipe token goes to the mail layer (S-26), never into a response.
		token, err := s.users.ForgotRequest(r.Context(), typed)
		if err != nil {
			// The person sees the same page as when nothing could be issued;
			// the operator sees why. No identifier from the form goes to the
			// log: it is whatever was typed.
			slog.Error("forgot: issuing wipe token failed", "error", err)
		}
		switch {
		case token == "":
		case s.sendForgotWipeMail(r, typed, token):
			page.Title, page.Lede, page.Sent, page.HasMail = "Check your inbox", forgotSent, true, true
		default:
			page.Title, page.Lede, page.HasMail = "Forgotten your password?", "A recovery link was prepared but could not be emailed. Ask an administrator.", true
		}
	case "ask":
		asked, err := s.users.ForgotAsk(r.Context(), typed)
		if err != nil {
			slog.Error("forgot: recording the request failed", "error", err)
		}
		if asked {
			page.Title, page.Lede, page.Asked, page.HasMail = "The administrator has been asked", forgotAsked, true, true
		}
	default:
		if _, hasMail, canAsk := s.users.ForgotLookup(typed); hasMail {
			page.Title = "What we can do"
			page.Lede = text("Account " + typed + " has a confirmed email. A silent password reset does not exist: " +
				"a new password would be a new key, and everything stored under the old one would be lost anyway.")
			page.HasMail = true
			page.Options = []view.ForgotOption{s.forgotOption(w, r, typed, "wipe", "Wipe the account", forgotWipeBody, "Send a confirmation letter")}
			if canAsk {
				page.Options = append(page.Options, s.forgotOption(w, r, typed, "ask", "Ask the administrator", forgotAskBody, "Ask the administrator"))
			}
		}
	}
	constantTimeDelay(start, 300*time.Millisecond)
	s.writePage(w, r, http.StatusOK, page)
}

// forgotOption is a card of s-auth-6: what it does, and a form of one
// button that carries the login as typed.
func (s *Server) forgotOption(w http.ResponseWriter, r *http.Request, typed, op, title, body, button string) view.ForgotOption {
	return view.ForgotOption{
		Title: text(title),
		Body:  text(body),
		Form: withSubmit(s.anonPlainForm(w, r, "/forgot", "POST",
			view.FormRow{Name: "op", Kind: view.InputHidden, Value: text(op)},
			view.FormRow{Name: "login", Kind: view.InputHidden, Value: text(typed)},
		), "forgot."+op, button),
	}
}

func (s *Server) getForgotWipe(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	token := r.PathValue("token")
	login, step, valid := s.users.ForgotWipeState(r.Context(), token)
	if !valid {
		s.tokenNotFound(w, r, start)
		return
	}
	page := s.forgotWipePage(w, r, token, login, step)
	s.writePage(w, r, http.StatusOK, page)
}

// postForgotWipe takes the two steps of one link (S-10.4). The first is its
// button (s-auth-10): the page names the erasure and the button says it; a
// GET — a letter scanner following the link — wipes nothing. The second
// sets the password.
func (s *Server) postForgotWipe(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	token := r.PathValue("token")
	login, step, valid := s.users.ForgotWipeState(r.Context(), token)
	if !valid {
		s.tokenNotFound(w, r, start)
		return
	}
	switch step {
	case string(view.WipeConfirm):
		if err := s.users.ForgotConfirmWipe(r.Context(), token); err != nil {
			s.tokenNotFound(w, r, start)
			return
		}
		login, step, valid = s.users.ForgotWipeState(r.Context(), token)
		page := s.forgotWipePage(w, r, token, login, step)
		s.writePage(w, r, http.StatusOK, page)
	case string(view.WipePassword):
		if err := passwordPair(r.FormValue("password"), r.FormValue("password2")); err != nil {
			page := s.forgotWipePage(w, r, token, login, step)
			passwordError(page.Form.Sets[0].Rows[1:], err)
			s.writePage(w, r, http.StatusOK, page)
			return
		}
		if err := s.users.ForgotSetPassword(r.Context(), token, r.FormValue("password")); err != nil {
			s.tokenNotFound(w, r, start)
			return
		}
		s.redirect(w, r, "/login")
	default:
		s.tokenNotFound(w, r, start)
	}
}

// forgotWipePage is s-auth-10 before the wipe — what is erased and what
// stays, the button that erases, Cancel — and s-auth-11 after it: the
// account is empty, the warning over the password and its repeat.
func (s *Server) forgotWipePage(w http.ResponseWriter, r *http.Request, token, login, step string) view.ForgotWipePage {
	page := view.ForgotWipePage{Login: text(login)}
	switch step {
	case string(view.WipeConfirm):
		page.Step = view.WipeConfirm
		page.Title = "Erase everything and start again?"
		page.Lede = text("This is not a password recovery. The key to your data comes from the password you forgot, " +
			"so nothing stored under it can be read again — by you, by us, by anyone.\n\n" +
			"Confirming erases what Carrel keeps for " + login + ": saved connections and their passwords, subscriptions, tags and the settings of this desk. " +
			"Address books, calendars, tasks and notes on the servers are not touched — you connect to them again afterwards.\n\n" +
			"The account itself stays: same login, same confirmed address. Only the password is new, and you set it on the next screen.")
		page.Form = withSubmit(s.anonPlainForm(w, r, "/forgot/"+token, "POST"), "wipe.confirm", "Erase and set a new password")
		page.Form.Actions[0].Danger = true
		page.Cancel = &view.Action{ID: "wipe.cancel", Label: "Cancel", Href: s.route("/login"), Method: "GET", Enabled: true}
	case string(view.WipePassword):
		page.Step = view.WipePassword
		page.Title = "Set a new password"
		page.Lede = text("The account " + login + " is empty. Nothing was kept.")
		page.Notices = passwordNotice()
		page.Form = withSubmit(s.anonPlainForm(w, r, "/forgot/"+token, "POST",
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true},
			view.FormRow{Name: "password2", Label: "Repeat password", Kind: view.InputPassword, Required: true},
		), "wipe.password", "Set the password and sign in")
	}
	return page
}
