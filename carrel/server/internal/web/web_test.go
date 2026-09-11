package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/internal/web"
)

// harness wires the request layer to the real user.Service: the tests below
// check authorisation, and a stub that answered "viewerID == accountParam"
// proved nothing about it (server.md, 05.09.2026).
type harness struct {
	t     *testing.T
	srv   *web.Server
	st    *store.Store
	mgr   *session.Manager
	cfg   *conf.Config
	users *user.Service
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	key, err := crypt.CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = dir
	mgr := session.NewManager(cfg)
	users := user.New(st, mgr, cfg)
	h := &harness{t: t, st: st, mgr: mgr, cfg: cfg, users: users}
	h.rebuild()
	return h
}

// rebuild recreates the Server so a changed cfg (trusted proxies) applies.
func (h *harness) rebuild() {
	h.srv = web.New(h.cfg, h.st, h.mgr, h.users, web.JSONRenderer{}, "test", "test")
}

func (h *harness) admin(login, password string) string {
	h.t.Helper()
	if err := h.users.SetupFirstAdmin(context.Background(), login, password); err != nil {
		h.t.Fatal(err)
	}
	snap := h.st.Snapshot()
	return snap.UserByLogin(login).ID
}

func (h *harness) invite(adminID, login, password string) string {
	h.t.Helper()
	secret, err := h.users.CreateInvite(adminID, login, "")
	if err != nil {
		h.t.Fatal(err)
	}
	if err := h.users.CompleteInvite(context.Background(), secret, password); err != nil {
		h.t.Fatal(err)
	}
	snap := h.st.Snapshot()
	return snap.UserByLogin(login).ID
}

// login runs the real password path and returns a live session.
func (h *harness) login(login, password string) *session.Session {
	h.t.Helper()
	id, key, _, ok := h.users.Authenticate(context.Background(), login, password)
	if !ok {
		h.t.Fatalf("login %q failed", login)
	}
	s, err := h.mgr.Create(id, login, key)
	if err != nil {
		h.t.Fatal(err)
	}
	return s
}

func (h *harness) putConnections(login string, key [32]byte, ids ...string) {
	h.t.Helper()
	var b sealed.Blob
	for _, id := range ids {
		b.Connections = append(b.Connections, sealed.Connection{ID: id, Server: "https://dav.example.com"})
	}
	blob, err := sealed.Seal(key, b, login)
	if err != nil {
		h.t.Fatal(err)
	}
	err = h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByLogin(login)
		u.SealedBlob = blob
		return nil, nil
	})
	if err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.srv.Handler().ServeHTTP(rec, req)
	return rec
}

func get(path string, sess *session.Session) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if sess != nil {
		req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	}
	return req
}

func postForm(path string, sess *session.Session, form url.Values) *http.Request {
	if sess != nil {
		form.Set("_csrf", sess.CSRFToken())
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if sess != nil {
		req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	}
	return req
}

func fragment(req *http.Request, target string) *http.Request {
	req.Header.Set("Carrel-View", "fragment")
	req.Header.Set("Carrel-Target", target)
	return req
}

func TestHealthz(t *testing.T) {
	h := newHarness(t)
	rec := h.do(get("/healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
	}
}

func TestSetupRedirectWhenFirstRun(t *testing.T) {
	h := newHarness(t)
	rec := h.do(get("/login", nil))
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "/setup") {
		t.Fatalf("status = %d location = %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestUnknownRouteNotFound(t *testing.T) {
	h := newHarness(t)
	if rec := h.do(get("/no-such-route", nil)); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestFragmentHeader(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	sess := h.login("alice", "password123")
	rec := h.do(fragment(get("/app/contacts", sess), "main"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"target":"main"`) {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFRequiredForMutatingSessionRequest(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	sess := h.login("alice", "password123")
	req := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(""))
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	if rec := h.do(req); rec.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF: status = %d, want 403", rec.Code)
	}
	if _, ok := h.mgr.Get(sess.ID()); !ok {
		t.Fatal("session ended by a request without CSRF token")
	}
	if rec := h.do(postForm("/logout", sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("logout with token: status = %d", rec.Code)
	}
	if _, ok := h.mgr.Get(sess.ID()); ok {
		t.Fatal("session alive after logout")
	}
}

// Ф2-7. {account} is a connection in the viewer's sealed blob (C-4). Own
// connection answers, a foreign one and the viewer's own user ID answer 404
// like anything that does not exist (S-9.5).
func TestOwnershipByConnection(t *testing.T) {
	h := newHarness(t)
	aliceID := h.admin("alice", "password123")
	h.invite(aliceID, "bob", "bobpass123")
	alice := h.login("alice", "password123")
	bob := h.login("bob", "bobpass123")
	aliceKey, _ := alice.DataKey()
	bobKey, _ := bob.DataKey()
	h.putConnections("alice", aliceKey, "a-alice00001")
	h.putConnections("bob", bobKey, "a-bob0000001")

	cases := []struct {
		path string
		want int
	}{
		{"/app/contacts/a-alice00001/personal", http.StatusOK},
		{"/app/contacts/a-bob0000001/personal", http.StatusNotFound},
		{"/app/contacts/" + aliceID + "/personal", http.StatusNotFound},
		{"/app/contacts/a-nowhere000/personal", http.StatusNotFound},
		{"/app/files/a-alice00001/docs/", http.StatusOK},
		{"/app/files/a-bob0000001/docs/", http.StatusNotFound},
		{"/app/contacts/a-alice00001/personal/uid-1/conflict", http.StatusOK},
		{"/app/contacts/a-bob0000001/personal/uid-1/conflict", http.StatusNotFound},
	}
	for _, c := range cases {
		if rec := h.do(get(c.path, alice)); rec.Code != c.want {
			t.Errorf("%s: status = %d, want %d", c.path, rec.Code, c.want)
		}
	}
	if rec := h.do(get("/app/contacts/a-bob0000001/personal", bob)); rec.Code != http.StatusOK {
		t.Errorf("bob on his own connection: status = %d", rec.Code)
	}
	// ".." never reaches a listing: the mux redirects to the cleaned path
	// and rejectPathTraversal stands behind it.
	if rec := h.do(get("/app/files/a-alice00001/docs/../etc", alice)); rec.Code == http.StatusOK {
		t.Errorf("path traversal answered 200")
	}
}

// Ф2-8. Login issues a fresh SID and ends the session the browser came with.
func TestLoginRotatesSIDAndEndsOldSession(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	old := h.login("alice", "password123")
	rec := h.do(postForm("/login", old, url.Values{"login": {"  Alice "}, "password": {"password123"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login: status = %d body = %s", rec.Code, rec.Body.String())
	}
	var sid string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "carrel_sid" {
			sid = c.Value
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
				t.Fatalf("cookie flags: %+v", c)
			}
		}
	}
	if sid == "" || sid == old.ID() {
		t.Fatalf("new sid = %q old = %q", sid, old.ID())
	}
	if _, ok := h.mgr.Get(old.ID()); ok {
		t.Fatal("old session alive after re-login")
	}
	fresh, ok := h.mgr.Get(sid)
	if !ok {
		t.Fatal("new session missing")
	}
	if fresh.Login() != "alice" {
		t.Fatalf("session login = %q, want normalised", fresh.Login())
	}
}

func TestLoginWrongPasswordSameForm(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	bodies := map[string]string{}
	for _, c := range [][2]string{{"alice", "wrongpassword"}, {"nobody", "wrongpassword"}} {
		rec := h.do(postForm("/login", nil, url.Values{"login": {c[0]}, "password": {c[1]}}))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", c[0], rec.Code)
		}
		bodies[c[0]] = strings.ReplaceAll(rec.Body.String(), c[0], "LOGIN")
		if len(rec.Result().Cookies()) != 0 {
			t.Fatalf("%s: cookie set on failed login", c[0])
		}
	}
	if bodies["alice"] != bodies["nobody"] {
		t.Fatalf("unknown login and wrong password answer differently:\n%s\n%s", bodies["alice"], bodies["nobody"])
	}
}

// Ф2-1. Behind a trusted proxy the buckets key on X-Forwarded-For; from an
// untrusted peer the header is ignored and everyone shares the peer bucket.
func TestClientIPBehindTrustedProxy(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	send := func(remote, xff string) int {
		req := get("/forgot", nil)
		req.RemoteAddr = remote
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		return h.do(req).Code
	}
	// Untrusted peer: forged headers do not buy extra buckets.
	for i := 0; i < conf.AnonTokenRatePerIP; i++ {
		if code := send("198.51.100.7:4000", fmt.Sprintf("203.0.113.%d", i+1)); code != http.StatusOK {
			t.Fatalf("request %d from untrusted peer: %d", i, code)
		}
	}
	if code := send("198.51.100.7:4000", "203.0.113.200"); code != http.StatusTooManyRequests {
		t.Fatalf("untrusted peer past the limit: %d, want 429", code)
	}

	h.cfg.TrustedProxies = "10.0.0.0/8, 127.0.0.1"
	h.rebuild()
	for i := 0; i < conf.AnonTokenRatePerIP+5; i++ {
		// Rightmost untrusted hop counts; the proxy's own hop and a forged
		// leading hop do not.
		xff := fmt.Sprintf("203.0.113.1, 203.0.113.%d, 10.0.0.2", i+1)
		if code := send("127.0.0.1:4000", xff); code != http.StatusOK {
			t.Fatalf("request %d through trusted proxy: %d", i, code)
		}
	}
	for i := 0; i < conf.AnonTokenRatePerIP; i++ {
		send("127.0.0.1:4000", "203.0.113.250")
	}
	if code := send("127.0.0.1:4000", "203.0.113.250"); code != http.StatusTooManyRequests {
		t.Fatalf("one client through the proxy past the limit: %d, want 429", code)
	}
	if code := send("127.0.0.1:4000", "203.0.113.251"); code != http.StatusOK {
		t.Fatalf("another client through the same proxy: %d, want 200", code)
	}
	// A proxy that forwards nothing is the client itself.
	if code := send("10.0.0.9:4000", ""); code != http.StatusOK {
		t.Fatalf("trusted peer without header: %d", code)
	}
}

func TestSecureCookieFromTrustedProxyOnly(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	h.cfg.TrustedProxies = "127.0.0.1"
	h.rebuild()
	secure := func(remote string) bool {
		req := postForm("/login", nil, url.Values{"login": {"alice"}, "password": {"password123"}})
		req.RemoteAddr = remote
		req.Header.Set("X-Forwarded-Proto", "https")
		rec := h.do(req)
		for _, c := range rec.Result().Cookies() {
			if c.Name == "carrel_sid" {
				return c.Secure
			}
		}
		t.Fatalf("no cookie from %s: %d", remote, rec.Code)
		return false
	}
	if !secure("127.0.0.1:1") {
		t.Fatal("https via trusted proxy should set Secure")
	}
	if secure("198.51.100.7:1") {
		t.Fatal("X-Forwarded-Proto from an untrusted peer set Secure")
	}
}

// Ф2-9. The unlock sheet (S-10.7) opens only for the person's own password,
// in this session only, and is journaled.
func TestUnlockRequiresPassword(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	sess := h.login("alice", "password123")
	other := h.login("alice", "password123")

	rec := h.do(fragment(postForm("/app/settings/unlock", sess, url.Values{"password": {"wrongpassword"}}), "sheet"))
	if rec.Code != http.StatusOK {
		t.Fatalf("wrong password: status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Password is wrong") {
		t.Fatalf("wrong password: body = %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"unlocked"`) && !strings.Contains(rec.Body.String(), `"unlocked":null`) {
		t.Fatalf("top bar unlocked after a wrong password: %s", rec.Body.String())
	}
	if h.do(get("/app/contacts", sess)).Body.String() != h.do(get("/app/contacts", other)).Body.String() {
		t.Fatal("wrong password changed the shell")
	}
	rec = h.do(postForm("/app/settings/unlock", sess, url.Values{"password": {"wrongpassword"}}))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unlock as a page: status = %d, want 404", rec.Code)
	}

	rec = h.do(fragment(postForm("/app/settings/unlock", sess, url.Values{"password": {"password123"}}), "sheet"))
	if rec.Code != http.StatusOK {
		t.Fatalf("right password: status = %d", rec.Code)
	}
	var payload struct {
		Extra []struct {
			Target string          `json:"target"`
			Body   json.RawMessage `json:"body"`
		} `json:"extra"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Extra) != 1 || payload.Extra[0].Target != "topbar" || strings.Contains(string(payload.Extra[0].Body), `"unlocked":null`) {
		t.Fatalf("top bar not unlocked: %s", rec.Body.String())
	}
	if strings.Contains(h.do(get("/app/contacts", other)).Body.String(), `"unlocked":"`) {
		t.Fatal("another session of the same person got unlocked")
	}
	found := false
	for _, e := range h.st.Snapshot().Audit {
		if e.What == "secret.unlock" && e.Who == "alice" {
			found = true
		}
	}
	if !found {
		t.Fatal("unlock not in the journal")
	}
}

func TestUnlockGuessesShareLoginBucket(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	sess := h.login("alice", "password123")
	var last int
	for i := 0; i <= conf.LoginRatePerAccount; i++ {
		last = h.do(fragment(postForm("/app/settings/unlock", sess, url.Values{"password": {"guess"}}), "sheet")).Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("guess %d: status = %d, want 429", conf.LoginRatePerAccount+1, last)
	}
}

// Ф2-10. A valid invitation with a short or mismatched password shows the
// form error; before the fix the person got a 404.
func TestInviteFormErrorsAreNot404(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	secret, err := h.users.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	path := "/invite/" + secret
	if rec := h.do(get(path, nil)); rec.Code != http.StatusOK {
		t.Fatalf("GET invite: %d", rec.Code)
	}
	rec := h.do(postForm(path, nil, url.Values{"password": {"short"}, "password2": {"short"}}))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "at least 8") {
		t.Fatalf("short password: status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = h.do(postForm(path, nil, url.Values{"password": {"bobpass123"}, "password2": {"bobpass124"}}))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "do not match") {
		t.Fatalf("mismatch: status = %d body = %s", rec.Code, rec.Body.String())
	}
	if _, _, _, ok := h.users.Authenticate(context.Background(), "bob", "bobpass123"); ok {
		t.Fatal("password set despite the form error")
	}
	rec = h.do(postForm(path, nil, url.Values{"password": {"bobpass123"}, "password2": {"bobpass123"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("accept: status = %d", rec.Code)
	}
	if rec := h.do(get(path, nil)); rec.Code != http.StatusNotFound {
		t.Fatalf("used invite: %d, want 404", rec.Code)
	}
}

// The two-step wipe through HTTP, then the dead token answers exactly like
// a token that never existed (S-9.5).
func TestForgotWipeRouteAndDeadTokensAlike(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	sess := h.login("alice", "password123")
	link, err := h.users.IssueConsoleWipeLink("alice")
	if err != nil {
		t.Fatal(err)
	}
	if rec := h.do(get(link, nil)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"step":"confirm"`) {
		t.Fatalf("GET link: %d %s", rec.Code, rec.Body.String())
	}
	if rec := h.do(postForm(link, nil, url.Values{"confirm": {"no"}})); !strings.Contains(rec.Body.String(), "Confirmation is required") {
		t.Fatalf("confirm=no: %s", rec.Body.String())
	}
	if _, _, _, ok := h.users.Authenticate(context.Background(), "alice", "password123"); !ok {
		t.Fatal("wiped without confirmation")
	}
	rec := h.do(postForm(link, nil, url.Values{"confirm": {"yes"}}))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"step":"password"`) {
		t.Fatalf("confirm=yes: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := h.mgr.Get(sess.ID()); ok {
		t.Fatal("session alive after wipe")
	}
	// Ф2-10. Mismatched repeat is a form error, and no password is set.
	rec = h.do(postForm(link, nil, url.Values{"password": {"newpass123"}, "password2": {"newpass124"}}))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "do not match") {
		t.Fatalf("mismatch: %d %s", rec.Code, rec.Body.String())
	}
	if _, _, _, ok := h.users.Authenticate(context.Background(), "alice", "newpass123"); ok {
		t.Fatal("password set from a mismatched pair")
	}
	rec = h.do(postForm(link, nil, url.Values{"password": {"newpass123"}, "password2": {"newpass123"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("set password: %d %s", rec.Code, rec.Body.String())
	}
	if _, _, _, ok := h.users.Authenticate(context.Background(), "alice", "newpass123"); !ok {
		t.Fatal("new password does not work")
	}

	dead := h.do(get(link, nil))
	never := h.do(get("/forgot/"+strings.Repeat("A", 43), nil))
	garbage := h.do(get("/forgot/not-a-token", nil))
	for name, rec := range map[string]*httptest.ResponseRecorder{"dead": dead, "never": never, "garbage": garbage} {
		if rec.Code != http.StatusNotFound || rec.Body.String() != never.Body.String() {
			t.Fatalf("%s token: %d %q", name, rec.Code, rec.Body.String())
		}
	}
	if rec := h.do(postForm(link, nil, url.Values{"confirm": {"yes"}})); rec.Code != http.StatusNotFound {
		t.Fatalf("dead token wiped again: %d", rec.Code)
	}
}

// Ф2-11. /healthz and /about never see the session bucket.
func TestHealthzAndAboutUnmetered(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	sess := h.login("alice", "password123")
	for i := 0; i < conf.SessionRateLimit+10; i++ {
		if rec := h.do(get("/healthz", sess)); rec.Code != http.StatusOK {
			t.Fatalf("healthz %d: %d", i, rec.Code)
		}
		if rec := h.do(get("/about", sess)); rec.Code != http.StatusOK {
			t.Fatalf("about %d: %d", i, rec.Code)
		}
	}
	for i := 0; i < conf.SessionRateLimit; i++ {
		h.do(get("/app/contacts", sess))
	}
	if rec := h.do(get("/app/contacts", sess)); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("session bucket not applied to /app: %d", rec.Code)
	}
	if rec := h.do(get("/healthz", sess)); rec.Code != http.StatusOK {
		t.Fatalf("healthz metered: %d", rec.Code)
	}
}

func TestBodyOverLimitIs413BeforeHandler(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	sess := h.login("alice", "password123")
	big := strings.Repeat("a", conf.MaxFormBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader("_csrf="+big))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	if rec := h.do(req); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if _, ok := h.mgr.Get(sess.ID()); !ok {
		t.Fatal("oversized body reached the handler")
	}
}

// Admin routes answer 404 to a plain user, like a route that does not
// exist; the role is read from the store, not from the session.
func TestAdminRoutesNeedAdminRole(t *testing.T) {
	h := newHarness(t)
	aliceID := h.admin("alice", "password123")
	h.invite(aliceID, "bob", "bobpass123")
	alice := h.login("alice", "password123")
	bob := h.login("bob", "bobpass123")
	for _, path := range []string{"/admin/", "/admin/invites", "/admin/audit", "/admin/escrow"} {
		if rec := h.do(get(path, bob)); rec.Code != http.StatusNotFound {
			t.Errorf("%s as user: %d, want 404", path, rec.Code)
		}
		if rec := h.do(get(path, alice)); rec.Code != http.StatusOK {
			t.Errorf("%s as admin: %d, want 200", path, rec.Code)
		}
		if rec := h.do(get(path, nil)); rec.Code != http.StatusSeeOther {
			t.Errorf("%s anonymous: %d, want redirect to login", path, rec.Code)
		}
	}
	// Public links are not implemented yet: every token is a dead token.
	if rec := h.do(get("/p/"+strings.Repeat("A", 43), nil)); rec.Code != http.StatusNotFound {
		t.Errorf("/p/{token}: %d", rec.Code)
	}
}
