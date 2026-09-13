package discover

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/view"
)

func testGuard(t *testing.T) *dav.Guard {
	t.Helper()
	return dav.NewGuard(&conf.Config{
		DAVConnectTimeout:   5 * time.Second,
		DAVRequestTimeout:   10 * time.Second,
		DAVMaxResponseBytes: 1 << 20,
		DAVMaxRedirects:     5,
		DAVSSRFAllowlist:    "127.0.0.1",
	})
}

// Radicale-style trap: well-known at the origin redirects to a web UI (200 HTML)
// without a principal. Discovery must stay on the entered /test/ URL.
func TestWellKnownHTMLWithoutPrincipalStaysOnEnteredURL(t *testing.T) {
	const principal = "/principals/user/"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/.well-known/caldav":
			http.Redirect(w, r, "/", http.StatusFound)
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "<!DOCTYPE html><html></html>")
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/.well-known/"):
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/test") {
			http.NotFound(w, r)
			return
		}
		fakeDAV{principal: principal, homeSets: true}.handler().ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	entered := srv.URL + "/test/"
	res, trace, err := Run(t.Context(), testGuard(t), entered, "", "", "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.BaseURL != entered && res.BaseURL != strings.TrimRight(entered, "/") {
		t.Fatalf("BaseURL = %q, want entered %q", res.BaseURL, entered)
	}
	if !traceHas(trace, stepWellKnown, view.CheckBad) {
		t.Error("well-known step should fail when redirect target is not DAV")
	}
	if !traceHas(trace, stepDirect, view.CheckOK) {
		t.Error("discovery should fall back to the entered URL")
	}
	if !traceHas(trace, stepPrincipal, view.CheckOK) {
		t.Error("principal should be found on the entered URL")
	}
}

func TestAcceptWellKnownTargetRejectsHTTPDowngrade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakeDAV{principal: "/principals/me/", homeSets: true}.handler().ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	ok, note := acceptWellKnownTarget(
		t.Context(),
		testGuard(t),
		"https://example.test/app/",
		srv.URL+"/dav/",
		"",
		"",
		"",
	)
	if ok {
		t.Fatal("expected HTTPS→HTTP downgrade to be rejected")
	}
	if !strings.Contains(note, "HTTPS") {
		t.Fatalf("note = %q, want downgrade explanation", note)
	}
}

func TestWellKnownAcceptsValidRedirect(t *testing.T) {
	const principal = "/principals/user/"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/.well-known/caldav":
			http.Redirect(w, r, "/dav/", http.StatusFound)
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/.well-known/"):
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/dav") {
			http.NotFound(w, r)
			return
		}
		fakeDAV{principal: principal, homeSets: true}.handler().ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	want := srv.URL + "/dav/"
	res, trace, err := Run(t.Context(), testGuard(t), srv.URL+"/nested/", "", "", "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.BaseURL != want && res.BaseURL != strings.TrimRight(want, "/") {
		t.Fatalf("BaseURL = %q, want well-known target %q", res.BaseURL, want)
	}
	if !traceHas(trace, stepWellKnown, view.CheckOK) {
		t.Error("well-known step should succeed for a DAV redirect target")
	}
}
