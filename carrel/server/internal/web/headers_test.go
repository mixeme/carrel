package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// S-29.1 and C-11: the content policy without inline scripts and styles,
// nosniff and no framing are promised in the headers of every answer — the
// JSON renderer included, since a client renders what it gets on this
// origin. Personal pages are not to be kept by a cache on the way (audit
// №3, Ф3-1).
func TestSecurityHeadersOnEveryAnswer(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	sess := h.login("alice", "password123")
	check := func(name string, rec http.Header, personal bool) {
		t.Helper()
		csp := rec.Get("Content-Security-Policy")
		switch {
		case csp == "":
			t.Errorf("%s: no Content-Security-Policy", name)
		case strings.Contains(csp, "unsafe-inline"), strings.Contains(csp, "unsafe-eval"):
			t.Errorf("%s: CSP allows inline: %q", name, csp)
		case !strings.Contains(csp, "frame-ancestors 'none'"), !strings.Contains(csp, "object-src 'none'"):
			t.Errorf("%s: CSP = %q", name, csp)
		}
		if got := rec.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q", name, got)
		}
		if got := rec.Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options = %q", name, got)
		}
		if got := rec.Get("Referrer-Policy"); got == "" {
			t.Errorf("%s: no Referrer-Policy", name)
		}
		if personal {
			if got := rec.Get("Cache-Control"); !strings.Contains(got, "no-store") {
				t.Errorf("%s: Cache-Control = %q, want no-store", name, got)
			}
		}
	}
	check("GET /login", h.do(get("/login", nil)).Header(), false)
	check("GET /about", h.do(get("/about", nil)).Header(), false)
	check("GET /healthz", h.do(get("/healthz", nil)).Header(), false)
	check("GET /nowhere", h.do(get("/nowhere", nil)).Header(), false)
	check("GET /app/contacts", h.do(get("/app/contacts", sess)).Header(), true)
	check("GET /app/settings/account", h.do(get("/app/settings/account", sess)).Header(), true)
	check("GET /admin/", h.do(get("/admin/", sess)).Header(), true)
	// A refusal before the handler carries them too.
	req := postForm("/app/settings/account", nil, url.Values{})
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	check("POST without _csrf", h.do(req).Header(), true)
	// Fragments are pages of the same origin.
	check("fragment", h.do(fragment(get("/app/contacts", sess), "main")).Header(), true)
}

// A secret in the path still answers no-referrer: the stricter policy
// wins over the default of every page.
func TestSecretPathKeepsNoReferrerOverDefault(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	rec := h.do(get("/p/"+strings.Repeat("A", 43), nil))
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q", got)
	}
	if got := h.do(get("/login", nil)).Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Errorf("/login Referrer-Policy = %q, want same-origin", got)
	}
}
