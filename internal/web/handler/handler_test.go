// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
	"gitea.mixdep.ru/mix/carrel/internal/ratelimit"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

func newServer(t *testing.T, proxies ...string) *Server {
	t.Helper()
	trust, err := NewProxyTrust(proxies)
	if err != nil {
		t.Fatalf("NewProxyTrust: %v", err)
	}
	return &Server{Trust: trust, Sessions: session.New(session.Options{})}
}

func login(t *testing.T, s *Server, u session.User) *session.Session {
	t.Helper()
	dek, err := crypto.NewDEK()
	if err != nil {
		t.Fatalf("NewDEK: %v", err)
	}
	sess, err := s.Sessions.Create(u, dek)
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}
	return sess
}

// ok is the terminal handler: reaching it means the chain let the request past.
var ok = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})

func TestSecurityHeaders(t *testing.T) {
	s := newServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	Chain(ok, SecurityHeaders(s.Trust)).ServeHTTP(rec, req)

	h := rec.Header()
	csp := h.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "frame-ancestors 'none'", "form-action 'self'", "object-src 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP is missing %q: %s", want, csp)
		}
	}
	// htmx works through attributes, so nothing inline is needed and the
	// escape hatch stays shut (§24.5).
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP allows inline or eval: %s", csp)
	}
	if got := h.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
	if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := h.Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q", got)
	}
	// HSTS from a plain-HTTP instance would lock users out for a year.
	if got := h.Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS sent over plain HTTP: %q", got)
	}
}

func TestHSTSOnlyOverTLS(t *testing.T) {
	s := newServer(t, "10.0.0.1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	req.Header.Set("X-Forwarded-Proto", "https")
	Chain(ok, SecurityHeaders(s.Trust)).ServeHTTP(rec, req)
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("no HSTS behind a trusted proxy reporting https")
	}

	// The same header from an untrusted client proves nothing.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:5000"
	req.Header.Set("X-Forwarded-Proto", "https")
	Chain(ok, SecurityHeaders(s.Trust)).ServeHTTP(rec, req)
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("a client's own X-Forwarded-Proto produced HSTS: %q", got)
	}
}

func TestNoReferrerOnTokenPages(t *testing.T) {
	s := newServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/invite/abc", nil)
	Chain(ok, SecurityHeaders(s.Trust), NoReferrer).ServeHTTP(rec, req)

	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy on an invite page = %q, want no-referrer", got)
	}
}

// The liveness probe must not disclose the version, the user count or the
// state of anything downstream (§24.3).
func TestHealthSaysNothing(t *testing.T) {
	rec := httptest.NewRecorder()
	Health(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "ok" {
		t.Errorf("body = %q, want just \"ok\"", body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestClientIP(t *testing.T) {
	s := newServer(t, "10.0.0.1", "192.168.0.0/24")

	cases := []struct {
		name      string
		remote    string
		forwarded string
		want      string
	}{
		{"direct connection", "203.0.113.9:5000", "", "203.0.113.9"},
		{"forged header from an untrusted client", "203.0.113.9:5000", "1.2.3.4", "203.0.113.9"},
		{"one trusted proxy", "10.0.0.1:5000", "203.0.113.9", "203.0.113.9"},
		{"chain of trusted proxies", "10.0.0.1:5000", "203.0.113.9, 192.168.0.7", "203.0.113.9"},
		{"client prepends its own hop", "10.0.0.1:5000", "1.2.3.4, 203.0.113.9", "203.0.113.9"},
		{"garbage in the chain", "10.0.0.1:5000", "not-an-ip", "10.0.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			if got := s.Trust.ClientIP(req); got != tc.want {
				t.Errorf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsHTTPS(t *testing.T) {
	s := newServer(t, "10.0.0.1")

	direct := httptest.NewRequest(http.MethodGet, "/", nil)
	direct.RemoteAddr = "203.0.113.9:5000"
	if s.Trust.IsHTTPS(direct) {
		t.Error("plain HTTP reported as TLS")
	}

	direct.TLS = &tls.ConnectionState{}
	if !s.Trust.IsHTTPS(direct) {
		t.Error("a real TLS connection was not recognised")
	}

	proxied := httptest.NewRequest(http.MethodGet, "/", nil)
	proxied.RemoteAddr = "10.0.0.1:5000"
	proxied.Header.Set("X-Forwarded-Proto", "https")
	if !s.Trust.IsHTTPS(proxied) {
		t.Error("a trusted proxy reporting https was not believed")
	}

	proxied.RemoteAddr = "203.0.113.9:5000"
	if s.Trust.IsHTTPS(proxied) {
		t.Error("an untrusted client's X-Forwarded-Proto was believed")
	}
}

func TestSessionCookieFlags(t *testing.T) {
	s := newServer(t, "10.0.0.1")
	sess := login(t, s, session.User{ID: "u1", Login: "ada"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:5000"
	s.SetSessionCookie(rec, req, sess)

	c := findCookie(t, rec.Result().Cookies(), SessionCookie)
	if !c.HttpOnly {
		t.Error("session cookie is readable from script")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if c.Secure {
		t.Error("Secure set on a plain-HTTP request; the cookie would never be sent")
	}
	if c.Value != sess.ID {
		t.Error("cookie does not carry the session identifier")
	}

	// Behind a proxy that terminates TLS, Secure must be set.
	rec = httptest.NewRecorder()
	req.RemoteAddr = "10.0.0.1:5000"
	req.Header.Set("X-Forwarded-Proto", "https")
	s.SetSessionCookie(rec, req, sess)
	if !findCookie(t, rec.Result().Cookies(), SessionCookie).Secure {
		t.Error("Secure not set behind an HTTPS proxy")
	}
}

func TestLoadSessionClearsStaleCookie(t *testing.T) {
	s := newServer(t)
	sess := login(t, s, session.User{ID: "u1", Login: "ada"})
	s.Sessions.Destroy(sess.ID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess.ID})

	var seen *session.Session
	s.LoadSession(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = SessionFrom(r)
	})).ServeHTTP(rec, req)

	if seen != nil {
		t.Error("a destroyed session was still attached to the request")
	}
	if c := findCookie(t, rec.Result().Cookies(), SessionCookie); c.MaxAge >= 0 {
		t.Error("the stale cookie was not cleared")
	}
}

func TestRequireAuth(t *testing.T) {
	s := newServer(t)
	guarded := Chain(ok, s.LoadSession, s.RequireAuth)

	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/profile", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("anonymous status = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?next=") {
		t.Fatalf("Location = %q, want the login form with a next parameter", loc)
	}
	if next := mustQuery(t, loc).Get("next"); next != "/app/profile" {
		t.Errorf("next = %q, want /app/profile", next)
	}

	sess := login(t, s, session.User{ID: "u1", Login: "ada"})
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/profile", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess.ID})
	guarded.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("logged-in status = %d, want 200", rec.Code)
	}
}

// An htmx fragment must not have the login page swapped into it; the browser
// is told to navigate instead.
func TestRequireAuthHTMX(t *testing.T) {
	s := newServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/profile", nil)
	req.Header.Set("HX-Request", "true")
	Chain(ok, s.LoadSession, s.RequireAuth).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("HX-Redirect"); !strings.HasPrefix(got, "/login") {
		t.Errorf("HX-Redirect = %q, want the login form", got)
	}
}

func TestRequireAdmin(t *testing.T) {
	s := newServer(t)
	guarded := Chain(ok, s.LoadSession, s.RequireAdmin)

	user := login(t, s, session.User{ID: "u1", Login: "ada"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: user.ID})
	guarded.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-admin status = %d, want 403", rec.Code)
	}

	admin := login(t, s, session.User{ID: "u2", Login: "root", Admin: true})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: admin.ID})
	guarded.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("admin status = %d, want 200", rec.Code)
	}
}

func TestBasePath(t *testing.T) {
	s := newServer(t)
	s.BasePath = "/carrel"

	if got := s.Path("/login"); got != "/carrel/login" {
		t.Errorf("Path = %q, want /carrel/login", got)
	}
	if got := s.cookiePath(); got != "/carrel/" {
		t.Errorf("cookiePath = %q, want /carrel/", got)
	}

	rec := httptest.NewRecorder()
	Chain(ok, s.LoadSession, s.RequireAuth).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/carrel/app", nil))
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/carrel/login") {
		t.Errorf("Location = %q, want a redirect under the base path", loc)
	}
}

func TestSafeRedirect(t *testing.T) {
	for _, dst := range []string{"", "//evil.example", "https://evil.example", "http://evil.example/x", "\\\\evil"} {
		if got := SafeRedirect(dst, "/app"); got != "/app" {
			t.Errorf("SafeRedirect(%q) = %q, want the fallback", dst, got)
		}
	}
	if got := SafeRedirect("/app/profile", "/app"); got != "/app/profile" {
		t.Errorf("SafeRedirect rejected a same-origin path: %q", got)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	s := newServer(t)
	l := ratelimit.New(ratelimit.Options{Free: 1})
	guarded := Chain(ok, s.LoadSession, s.RateLimit(l, "invite"))

	req := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/invite/abc", nil)
		r.RemoteAddr = "203.0.113.9:5000"
		return r
	}

	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req())
	if rec.Code != http.StatusOK {
		t.Fatalf("first attempt status = %d, want 200", rec.Code)
	}

	l.Fail("invite:203.0.113.9")
	l.Fail("invite:203.0.113.9")

	rec = httptest.NewRecorder()
	guarded.ServeHTTP(rec, req())
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("throttled status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After on a throttled response")
	}

	// A different address is unaffected.
	other := req()
	other.RemoteAddr = "203.0.113.10:5000"
	rec = httptest.NewRecorder()
	guarded.ServeHTTP(rec, other)
	if rec.Code != http.StatusOK {
		t.Errorf("another address was throttled: status %d", rec.Code)
	}
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %s cookie in the response", name)
	return nil
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u.Query()
}
