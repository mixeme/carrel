// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
	"gitea.mixdep.ru/mix/carrel/internal/ratelimit"
	"gitea.mixdep.ru/mix/carrel/internal/session"
	"gitea.mixdep.ru/mix/carrel/internal/store"
	"gitea.mixdep.ru/mix/carrel/internal/web"
)

const (
	testPassword = "correct horse battery"
	badPassword  = "wrong horse battery"
)

// app is one instance with its own volume, driven through the real middleware
// chain. The browser is stood in for by a cookie map.
type app struct {
	*Server
	t       *testing.T
	handler http.Handler
	cookies map[string]string
}

// newApp opens an empty volume and wires the service. Argon2id is turned right
// down: the cost profiles are the crypto package's business, and at production
// settings a login per test case would dominate the run.
func newApp(t *testing.T, limiter *ratelimit.Limiter) *app {
	t.Helper()

	fast := crypto.Params{Time: 1, Memory: 8 * 1024, Threads: 1, KeyLen: crypto.KeyLen}
	dataDir := t.TempDir()
	st, err := store.OpenWith(dataDir, store.Options{Auth: fast, KEK: fast, Master: fast})
	if err != nil {
		t.Fatalf("OpenWith: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	templates, err := LoadTemplates(templateFS)
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}

	trust, err := NewProxyTrust(nil)
	if err != nil {
		t.Fatalf("NewProxyTrust: %v", err)
	}
	if limiter == nil {
		limiter = ratelimit.New(ratelimit.Options{})
	}

	srv := &Server{
		Trust:         trust,
		Sessions:      session.New(session.Options{}),
		Store:         st,
		Templates:     templates,
		LoginLimit:    limiter,
		RecoveryLimit: ratelimit.New(ratelimit.Options{}),
		DataDir:       dataDir,
	}
	t.Cleanup(srv.Sessions.Close)

	return &app{Server: srv, t: t, handler: srv.Handler(nil), cookies: map[string]string{}}
}

func (a *app) get(path string) *httptest.ResponseRecorder {
	a.t.Helper()
	return a.do(httptest.NewRequest(http.MethodGet, path, nil))
}

// post submits a form with the token the browser would have. Pass a nil form
// to submit nothing at all, which is what a cross-site attempt looks like.
func (a *app) post(path string, form url.Values) *httptest.ResponseRecorder {
	a.t.Helper()
	body := ""
	if form != nil {
		if form.Get(CSRFField) == "" {
			form.Set(CSRFField, a.token())
		}
		body = form.Encode()
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return a.do(req)
}

func (a *app) do(req *http.Request) *httptest.ResponseRecorder {
	a.t.Helper()
	req.RemoteAddr = "203.0.113.9:5000"
	for name, value := range a.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(a.cookies, c.Name)
			continue
		}
		a.cookies[c.Name] = c.Value
	}
	return rec
}

// token is what a rendered form would carry: the session's own token once
// signed in, the double-submit cookie before that.
func (a *app) token() string {
	if sess, ok := a.Sessions.Get(a.cookies[SessionCookie]); ok {
		return sess.CSRF
	}
	return a.cookies[CSRFCookie]
}

func (a *app) session() *session.Session {
	sess, _ := a.Sessions.Get(a.cookies[SessionCookie])
	return sess
}

func setupValues(login, password string) url.Values {
	return url.Values{
		fieldLogin:    {login},
		fieldEmail:    {"root@example.org"},
		fieldPassword: {password},
		fieldConfirm:  {password},
	}
}

func wantRedirect(t *testing.T, rec *httptest.ResponseRecorder, target string) {
	t.Helper()
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != target {
		t.Fatalf("Location = %q, want %q", got, target)
	}
}

// An empty volume sends every visitor to the setup screen, and the screen
// closes for good once it has been used (§5.1, §21).
func TestBootstrapFlow(t *testing.T) {
	a := newApp(t, nil)

	wantRedirect(t, a.get("/"), "/setup")

	rec := a.get("/setup")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /setup = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `name="csrf_token"`) {
		t.Error("the setup form carries no CSRF token")
	}
	if a.cookies[CSRFCookie] == "" {
		t.Fatal("no double-submit cookie was issued to the anonymous visitor")
	}

	// Creating the first administrator signs them in and lands them on the
	// panel.
	wantRedirect(t, a.post("/setup", setupValues("root", testPassword)), "/admin/")
	if a.cookies[SessionCookie] == "" {
		t.Fatal("no session cookie after setup")
	}
	sess := a.session()
	if sess == nil || !sess.Admin || sess.Login != "root" {
		t.Fatalf("session = %+v, want the new administrator", sess)
	}
	if sess.DEK() == nil {
		t.Error("the session has no DEK, so the account's data would be unreadable")
	}

	if rec := a.get("/admin/"); rec.Code != http.StatusOK {
		t.Errorf("GET /admin/ = %d, want 200", rec.Code)
	}

	// The setup screen is not a second way to mint an administrator.
	wantRedirect(t, a.get("/setup"), "/login")
	wantRedirect(t, a.post("/setup", setupValues("intruder", testPassword)), "/login")
	if _, err := a.Store.UserByLogin("intruder"); err == nil {
		t.Error("setup created a second administrator on a configured volume")
	}
}

func TestSetupRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		form url.Values
		want string
	}{
		{
			name: "passwords do not match",
			form: url.Values{fieldLogin: {"root"}, fieldPassword: {testPassword}, fieldConfirm: {badPassword}},
			want: "do not match",
		},
		{
			name: "password too short",
			form: url.Values{fieldLogin: {"root"}, fieldPassword: {"short"}, fieldConfirm: {"short"}},
			want: "at least",
		},
		{
			name: "login outside the allowed set",
			form: url.Values{fieldLogin: {"Root Admin"}, fieldPassword: {testPassword}, fieldConfirm: {testPassword}},
			want: "login",
		},
		{
			name: "unparseable email",
			form: setupValues("root", testPassword),
			want: "email",
		},
	}
	cases[3].form.Set(fieldEmail, "not-an-address")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t, nil)
			a.get("/setup")

			rec := a.post("/setup", tc.form)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if body := rec.Body.String(); !strings.Contains(strings.ToLower(body), tc.want) {
				t.Errorf("the page does not explain the problem (%q): %s", tc.want, body)
			}
			if !a.Store.NeedsBootstrap() {
				t.Error("a rejected form still created an account")
			}
			// The password is not echoed back into the re-rendered form.
			if strings.Contains(rec.Body.String(), testPassword) {
				t.Error("the submitted password came back in the page")
			}
		})
	}
}

// bootstrap sets the instance up and returns a signed-out app.
func bootstrap(t *testing.T, a *app) {
	t.Helper()
	a.get("/setup")
	if rec := a.post("/setup", setupValues("root", testPassword)); rec.Code != http.StatusSeeOther {
		t.Fatalf("setup: status %d", rec.Code)
	}
	a.post("/logout", url.Values{})
	a.cookies = map[string]string{}
}

// loginAdmin signs in as the bootstrap administrator.
func loginAdmin(t *testing.T, a *app) {
	t.Helper()
	a.get("/login")
	wantRedirect(t, a.post("/login", url.Values{fieldLogin: {"root"}, fieldPassword: {testPassword}}), "/admin/")
}

// signInReady signs in and clears a temporary password when one is set.
func (a *app) signInReady(login, password string) {
	a.t.Helper()
	a.signIn(login, password)
	if a.session() == nil || !a.session().MustChangePassword() {
		return
	}
	perm := password + "-permanent"
	rec := a.post("/app/password", url.Values{
		fieldCurrentPassword: {password},
		fieldNewPassword:     {perm},
		fieldConfirm:         {perm},
	})
	if rec.Code != http.StatusSeeOther {
		a.t.Fatalf("clear temp password for %s: status %d (body: %s)", login, rec.Code, rec.Body.String())
	}
}

func TestLoginAndLogout(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)

	a.get("/login")
	wantRedirect(t, a.post("/login", url.Values{fieldLogin: {"root"}, fieldPassword: {testPassword}}), "/admin/")

	sess := a.session()
	if sess == nil {
		t.Fatal("no session after a correct password")
	}
	if a.Store.Audit(store.AuditFilter{Action: store.ActionLogin, Limit: 1}) == nil {
		t.Error("the login was not recorded in the audit log")
	}
	if user, err := a.Store.User(sess.UserID); err != nil || user.LastLoginAt.IsZero() {
		t.Error("the last-login stamp was not updated")
	}

	// Logout ends the session and takes the DEK with it (§24.6).
	id := sess.ID
	wantRedirect(t, a.post("/logout", url.Values{}), "/login")
	if _, ok := a.Sessions.Get(id); ok {
		t.Error("the session survived logout")
	}
	if sess.DEK() != nil {
		t.Error("the DEK was left in memory after logout")
	}
	if a.cookies[SessionCookie] != "" {
		t.Error("the session cookie was not cleared")
	}
	if rec := a.get("/admin/"); rec.Code != http.StatusSeeOther {
		t.Errorf("the panel is still reachable after logout: status %d", rec.Code)
	}
}

// A wrong password, an unknown login and a disabled account must look the same
// from outside; the audit log is where they differ (§5.1, §24.3).
func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	a.get("/login")

	var bodies []string
	for _, form := range []url.Values{
		{fieldLogin: {"root"}, fieldPassword: {badPassword}},
		{fieldLogin: {"nobody"}, fieldPassword: {badPassword}},
	} {
		rec := a.post("/login", form)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if a.cookies[SessionCookie] != "" {
			t.Fatal("a failed login produced a session")
		}
		if !strings.Contains(rec.Body.String(), badCredentials) {
			t.Errorf("the page does not show the generic message: %s", rec.Body.String())
		}
		bodies = append(bodies, rec.Body.String())
	}
	// The pages differ only in the login echoed back into the form.
	if strings.Replace(bodies[0], "root", "nobody", 1) != bodies[1] {
		t.Error("an unknown login is distinguishable from a wrong password")
	}
	if n := len(a.Store.Audit(store.AuditFilter{Action: store.ActionLoginFailed})); n != 2 {
		t.Errorf("audit recorded %d failed logins, want 2", n)
	}
}

func TestLoginRateLimited(t *testing.T) {
	a := newApp(t, ratelimit.New(ratelimit.Options{Free: 1}))
	bootstrap(t, a)
	a.get("/login")

	form := url.Values{fieldLogin: {"root"}, fieldPassword: {badPassword}}
	for i := 0; i < 2; i++ {
		if rec := a.post("/login", form); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401", i+1, rec.Code)
		}
	}

	rec := a.post("/login", form)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After on a throttled login")
	}
	// The throttle holds even once the password is right: the point is to stop
	// the guessing, not to reward the guess that lands.
	if rec := a.post("/login", url.Values{fieldLogin: {"root"}, fieldPassword: {testPassword}}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 while throttled", rec.Code)
	}
}

func TestLoginRequiresCSRFToken(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	a.get("/login")

	rec := a.post("/login", url.Values{fieldLogin: {"root"}, fieldPassword: {testPassword}, CSRFField: {"forged"}})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if a.cookies[SessionCookie] != "" {
		t.Error("a login without a valid token produced a session")
	}
}

// A plain user lands on their own page, and the panel stays shut.
func TestUserLandsOnAppAndCannotReachAdmin(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)

	admin, err := a.Store.UserByLogin("root")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	if _, err := a.Store.CreateUserWithPassword(
		store.Actor{ID: admin.ID, Login: admin.Login}, "ada", "", store.RoleUser, testPassword,
	); err != nil {
		t.Fatalf("CreateUserWithPassword: %v", err)
	}

	a.get("/login")
	wantRedirect(t, a.post("/login", url.Values{fieldLogin: {"ada"}, fieldPassword: {testPassword}}), "/app/password")

	rec := a.get("/app/")
	if rec.Code != http.StatusSeeOther {
		t.Errorf("GET /app/ = %d, want 303 to password change", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/app/password" {
		t.Errorf("Location = %q, want /app/password", loc)
	}
	if rec := a.get("/admin/"); rec.Code != http.StatusForbidden {
		t.Errorf("GET /admin/ = %d, want 403", rec.Code)
	}
	if sess := a.session(); sess == nil || !sess.MustChangePassword() {
		t.Error("a temporary password did not carry the forced-change flag")
	}
}

// The post-login destination is followed only when it is same-origin.
func TestLoginHonoursSafeNext(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)

	rec := a.get("/app/")
	wantRedirect(t, rec, "/login?next=%2Fapp%2F")

	a.get("/login?next=%2Fapp%2F")
	form := url.Values{fieldLogin: {"root"}, fieldPassword: {testPassword}, fieldNext: {"/app/"}}
	wantRedirect(t, a.post("/login", form), "/app/")

	a.post("/logout", url.Values{})
	a.get("/login")
	offsite := url.Values{fieldLogin: {"root"}, fieldPassword: {testPassword}, fieldNext: {"https://evil.example/"}}
	wantRedirect(t, a.post("/login", offsite), "/admin/")
}

// "Forgot password" leads to an explanation, not to a reset that would quietly
// destroy the account's data (§5.3).
func TestForgotOffersNoReset(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)

	rec := a.get("/forgot")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "not possible") {
		t.Errorf("the page does not say recovery is impossible: %s", body)
	}
	if strings.Contains(body, "<form") {
		t.Error("the page offers a form; there is nothing it could do")
	}
}

// An already signed-in visitor has no use for the login form.
func TestSignedInVisitorSkipsLoginForm(t *testing.T) {
	a := newApp(t, nil)
	a.get("/setup")
	a.post("/setup", setupValues("root", testPassword))

	wantRedirect(t, a.get("/login"), "/admin/")
	wantRedirect(t, a.get("/"), "/admin/")
}
