package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Audit №4, А4-8, the owner's decision of 25.11.2026: a page of a
// neighbouring subdomain (SelfPost, Wynd under mixfed.ru) could set its own
// carrel_sid for the whole domain, and the person would work in the
// attacker's account. On an https public address at the root the cookies
// are __Host-carrel_sid and __Host-carrel_csrf — Secure, Path=/, no Domain,
// which no other host may set — and a cookie without the prefix is no
// session. Elsewhere (plain http, a base path) they keep their names: the
// prefix is refused there.
func TestCookiesAreHostOnlyOnHTTPS(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	h.cfg.PublicURL = "https://carrel.example.org"

	page := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := h.do(page)
	var csrf *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if strings.HasSuffix(c.Name, "carrel_csrf") {
			csrf = c
		}
	}
	if csrf == nil || csrf.Name != "__Host-carrel_csrf" || !csrf.Secure || csrf.Path != "/" || csrf.Domain != "" {
		t.Fatalf("the sign-in form's cookie = %+v", csrf)
	}
	form := url.Values{"login": {"alice"}, "password": {"password123"}, "_csrf": {csrf.Value}}
	login := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	login.AddCookie(&http.Cookie{Name: csrf.Name, Value: csrf.Value})
	rec = h.do(login)
	var sid *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if strings.HasSuffix(c.Name, "carrel_sid") {
			sid = c
		}
	}
	if sid == nil || sid.Name != "__Host-carrel_sid" || !sid.Secure || sid.Path != "/" || sid.Domain != "" {
		t.Fatalf("the session's cookie = %+v (%d)", sid, rec.Code)
	}
	ask := func(name string) int {
		req := httptest.NewRequest(http.MethodGet, "/app/settings/account", nil)
		req.AddCookie(&http.Cookie{Name: name, Value: sid.Value})
		// Past h.do, which would add the __Host- twin.
		rec := httptest.NewRecorder()
		h.srv.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	if code := ask("__Host-carrel_sid"); code != http.StatusOK {
		t.Errorf("with the __Host- cookie: %d", code)
	}
	if code := ask("carrel_sid"); code == http.StatusOK {
		t.Error("a cookie without the prefix is taken for a session")
	}
}
