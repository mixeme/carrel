package web

import (
	"errors"
	"log/slog"
	"net/http"
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
		user.ErrMasterPasswordShort, user.ErrMasterPasswordMismatch,
		user.ErrEscrowNotConfigured, user.ErrEscrowConfigured, user.ErrNotDeposited, user.ErrEscrowDisabled,
	} {
		if errors.Is(err, known) {
			return text(known.Error())
		}
	}
	return text("Something went wrong. Try again.")
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

func (s *Server) getHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) getAbout(w http.ResponseWriter, r *http.Request) {
	page := view.AboutPage{
		Plain: view.Plain{
			Title: text("About Carrel"),
			Lede:  text("One desk. Every server."),
		},
		Version: text(s.version),
		Commit:  text(s.commit),
		Source: view.Action{
			ID: "about.source", Label: "Source code", Href: "https://carrel.dev", Method: "GET", Enabled: true,
		},
		Third: view.Action{ID: "about.third", Label: "Third-party notices", Href: s.route("/about"), Method: "GET", Enabled: true},
		Back:  view.Action{ID: "about.back", Label: "Back", Href: s.route("/login"), Method: "GET", Enabled: true},
	}
	s.writePage(w, r, http.StatusOK, page)
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
	page := view.LoginPage{
		Plain: view.Plain{Title: text("Sign in"), Lede: text("One desk. Every server.")},
		Form: s.anonPlainForm(w, r, "/login", "POST",
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Required: true},
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true},
		),
		Forgot: view.Action{ID: "login.forgot", Label: "Forgotten your password?", Href: s.route("/forgot"), Method: "GET", Enabled: true},
		About:  view.Action{ID: "login.about", Label: "About", Href: s.route("/about"), Method: "GET", Enabled: true},
	}
	if s.users.RegisterOpen() {
		page.Form.Sets[0].Rows = append(page.Form.Sets[0].Rows,
			view.FormRow{Name: "register", Label: "", Kind: view.InputText, Extra: &view.Action{
				ID: "login.register", Label: "Create an account", Href: s.route("/register"), Method: "GET", Enabled: true,
			}},
		)
	}
	s.writePage(w, r, http.StatusOK, page)
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
		page := view.LoginPage{
			Plain: view.Plain{Title: text("Sign in"), Lede: text("One desk. Every server.")},
			Form: s.anonPlainForm(w, r, "/login", "POST",
				view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Value: text(login), Required: true},
				view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true, Error: text("Login or password is wrong.")},
			),
			Forgot: view.Action{ID: "login.forgot", Label: "Forgotten your password?", Href: "/forgot", Method: "GET", Enabled: true},
			About:  view.Action{ID: "login.about", Label: "About", Href: "/about", Method: "GET", Enabled: true},
		}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	// A new SID on every login (architecture.md): whatever session the
	// browser presented ends here, so the old cookie value opens nothing.
	if old, ok := s.sessionFrom(r); ok {
		s.sessions.End(old.ID())
	}
	sess, err := s.sessions.Create(accountID, normalizeLogin(login), dataKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, r, sess.ID())
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

func (s *Server) getSetup(w http.ResponseWriter, r *http.Request) {
	if !s.users.FirstRun() {
		s.notFound(w, r)
		return
	}
	page := view.SetupPage{
		Plain: view.Plain{
			Title: text("Create administrator"),
			Lede:  text("This instance has no accounts yet. Choose a login and password for the first administrator."),
		},
		Form: s.anonPlainForm(w, r, "/setup", "POST",
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Required: true},
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true, Hint: text("At least 8 characters.")},
			view.FormRow{Name: "password2", Label: "Repeat password", Kind: view.InputPassword, Required: true},
		),
	}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postSetup(w http.ResponseWriter, r *http.Request) {
	if !s.users.FirstRun() {
		s.notFound(w, r)
		return
	}
	login := r.FormValue("login")
	password := r.FormValue("password")
	password2 := r.FormValue("password2")
	if password != password2 {
		page := view.SetupPage{
			Plain: view.Plain{Title: text("Create administrator"), Lede: text("This instance has no accounts yet.")},
			Form: s.anonPlainForm(w, r, "/setup", "POST",
				view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Value: text(login), Required: true},
				view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true, Error: text("Passwords do not match.")},
				view.FormRow{Name: "password2", Label: "Repeat password", Kind: view.InputPassword, Required: true},
			),
		}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	if err := s.users.SetupFirstAdmin(r.Context(), login, password); err != nil {
		page := view.SetupPage{
			Plain: view.Plain{Title: text("Create administrator"), Lede: text("This instance has no accounts yet.")},
			Form: s.anonPlainForm(w, r, "/setup", "POST",
				view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Value: text(login), Required: true, Error: formError(err)},
				view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true},
				view.FormRow{Name: "password2", Label: "Repeat password", Kind: view.InputPassword, Required: true},
			),
		}
		s.writePage(w, r, http.StatusOK, page)
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
	page := view.RegisterPage{
		Plain: view.Plain{Title: text("Create account"), Lede: text("Registration is open on this instance.")},
		Form: s.anonPlainForm(w, r, "/register", "POST",
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Required: true},
			view.FormRow{Name: "email", Label: "Email", Kind: view.InputEmail, Required: true},
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true},
		),
	}
	s.writePage(w, r, http.StatusOK, page)
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
	// The confirmation token goes to the mail layer (S-26), never into a
	// response.
	token, err := s.users.Register(r.Context(), r.FormValue("login"), r.FormValue("email"), r.FormValue("password"))
	if err != nil {
		page := view.RegisterPage{
			Plain: view.Plain{Title: text("Create account"), Lede: text("Registration is open on this instance.")},
			Form: s.anonPlainForm(w, r, "/register", "POST",
				view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Value: text(r.FormValue("login")), Error: formError(err)},
				view.FormRow{Name: "email", Label: "Email", Kind: view.InputEmail, Value: text(r.FormValue("email"))},
				view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword},
			),
		}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	mailSent := token != "" && s.sendConfirmMail(r.Context(), r.FormValue("login"), r.FormValue("email"), token)
	page := view.RegisterPage{
		Plain: view.Plain{Title: text("Create account")},
	}
	if mailSent {
		page.Plain.Lede = text("Check your email to confirm your address.")
		page.Sent = true
	} else {
		page.Plain.Lede = text("Your account was created, but the confirmation email could not be sent yet. Outbound mail is not available on this instance. Ask an administrator.")
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
	valid := s.users.InviteValid(r.Context(), token)
	if !valid {
		s.tokenNotFound(w, r, start)
		return
	}
	page := view.InvitePage{
		Plain: view.Plain{Title: text("Accept invitation"), Lede: text("Choose a password for your new account.")},
		Valid: true,
		Form: s.anonPlainForm(w, r, "/invite/"+token, "POST",
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true},
			view.FormRow{Name: "password2", Label: "Repeat password", Kind: view.InputPassword, Required: true},
		),
	}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postInvite(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	token := r.PathValue("token")
	if !s.users.InviteValid(r.Context(), token) {
		s.tokenNotFound(w, r, start)
		return
	}
	if err := passwordPair(r.FormValue("password"), r.FormValue("password2")); err != nil {
		page := view.InvitePage{
			Plain: view.Plain{Title: text("Accept invitation"), Lede: text("Choose a password.")},
			Valid: true,
			Form: s.anonPlainForm(w, r, "/invite/"+token, "POST",
				view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true, Error: formError(err)},
				view.FormRow{Name: "password2", Label: "Repeat password", Kind: view.InputPassword, Required: true},
			),
		}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	// Whatever else goes wrong (the login got taken meanwhile, the token
	// died between GET and POST) answers like a dead token: the form has
	// nothing more to say to someone holding an invalid secret.
	if err := s.users.CompleteInvite(r.Context(), token, r.FormValue("password")); err != nil {
		s.tokenNotFound(w, r, start)
		return
	}
	s.redirect(w, r, "/login")
}

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
		Plain: view.Plain{Title: text("Email confirmed"), Lede: text("You can sign in now.")},
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
	page := view.ForgotPage{
		Plain: view.Plain{
			Title: text("Forgotten password"),
			Lede:  text("Carrel cannot recover your encrypted data. You can wipe the account and set a new password, or ask an administrator."),
		},
		Form: s.anonPlainForm(w, r, "/forgot", "POST",
			view.FormRow{Name: "login", Label: "Login or email", Kind: view.InputText, Required: true},
		),
	}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postForgot(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := s.clientIP(r)
	if !s.limit.allowAnon(ip, start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	// The wipe token goes to the mail layer (S-26), never into a response.
	token, issued, asked, hasMail, err := s.users.ForgotRequest(r.Context(), r.FormValue("login"))
	if err != nil {
		// The person sees the same page as when nothing could be issued;
		// the operator sees why. No identifier from the form goes to the
		// log: it is whatever was typed.
		slog.Error("forgot: issuing wipe token failed", "error", err)
		issued, asked = false, false
	}
	mailSent := issued && token != "" && s.sendForgotWipeMail(r.Context(), r.FormValue("login"), token)
	page := view.ForgotPage{
		Plain:   view.Plain{Title: text("Forgotten password"), Lede: text("Carrel cannot recover your encrypted data.")},
		Form:    s.anonPlainForm(w, r, "/forgot", "POST", view.FormRow{Name: "login", Label: "Login or email", Kind: view.InputText}),
		Sent:    mailSent,
		Asked:   asked,
		HasMail: hasMail,
	}
	if issued && !mailSent && hasMail && !asked {
		page.Plain.Lede = text("A recovery link was prepared but could not be emailed: outbound mail is not available on this instance yet. Ask an administrator.")
	}
	if !hasMail && !issued && !asked {
		page.HasMail = false
		page.Plain.Lede = text("Without a confirmed email address on the account, sign-in recovery from here is not possible.")
	}
	constantTimeDelay(start, 300*time.Millisecond)
	s.writePage(w, r, http.StatusOK, page)
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
		if r.FormValue("confirm") != "yes" {
			page := s.forgotWipePage(w, r, token, login, step)
			page.Form.Sets[0].Rows[0].Error = text("Confirmation is required.")
			s.writePage(w, r, http.StatusOK, page)
			return
		}
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
			page.Form.Sets[0].Rows[0].Error = formError(err)
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

func (s *Server) forgotWipePage(w http.ResponseWriter, r *http.Request, token, login, step string) view.ForgotWipePage {
	page := view.ForgotWipePage{
		Plain: view.Plain{Title: text("Reset account"), Lede: text("This destroys encrypted connections and cannot be undone.")},
		Login: text(login),
		Warning: view.Notice{
			Role:  view.NoticeAlert,
			Title: text("Encrypted data will be destroyed"),
			Body:  text("Connections become unreadable and are removed. Data on remote servers is not touched."),
		},
	}
	switch step {
	case string(view.WipeConfirm):
		page.Step = view.WipeConfirm
		page.Form = s.anonPlainForm(w, r, "/forgot/"+token, "POST",
			view.FormRow{Name: "confirm", Label: "Type yes to wipe encrypted data", Kind: view.InputText, Required: true},
		)
	case string(view.WipePassword):
		page.Step = view.WipePassword
		page.Form = s.anonPlainForm(w, r, "/forgot/"+token, "POST",
			view.FormRow{Name: "password", Label: "New password", Kind: view.InputPassword, Required: true},
			view.FormRow{Name: "password2", Label: "Repeat password", Kind: view.InputPassword, Required: true},
		)
		page.Warning = view.Notice{
			Role: view.NoticeInfo,
			Body: text("Choose a new password. The same login and role are kept."),
		}
	}
	return page
}

func (s *Server) postBackupRun(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	s.tokenNotFound(w, r, start)
}

func (s *Server) getBackupStatus(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	s.tokenNotFound(w, r, start)
}

func (s *Server) getBackupDownload(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limit.allowAnon(s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	s.tokenNotFound(w, r, start)
}

func (s *Server) getBackupDownloadLegacy(w http.ResponseWriter, r *http.Request) {
	s.getBackupDownload(w, r)
}
