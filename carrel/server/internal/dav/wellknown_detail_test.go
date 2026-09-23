package dav

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A well-known step that fails says why in its detail: the refusal of an
// https→http redirect (docs/servers.md, SV-10) was an empty detail on the
// connection trace. The probe makes that refusal itself — it reads the
// redirect rather than walking into it (SV-45) — so it is also the one that
// has to give the reason.
func TestResolveWellKnownSaysWhyItFailed(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.test/dav.php", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	_, detail, err := ResolveWellKnown(t.Context(), srv.Client(), srv.URL+"/dav.php", "/.well-known/caldav")
	if err == nil {
		t.Fatal("a refused redirect resolved")
	}
	if detail != "GET /.well-known/caldav: dav: redirect downgrades HTTPS to HTTP" {
		t.Errorf("detail = %q", detail)
	}
	if strings.Contains(detail, "example.test") {
		t.Errorf("detail repeats the URL the client wrapped around the reason: %q", detail)
	}
}

// docs/servers.md, SV-45: the well-known probe carries no credentials, and
// where it points is a place a server may well protect. Stalwart sends
// /.well-known/caldav to /dav/cal and answers 401 there to anyone who has
// not signed in. RFC 6764 §6 asks for the Location, not for the resource, so
// the redirect is read and not walked into: the address is what it gives.
func TestResolveWellKnownTakesTheAddressNotTheResource(t *testing.T) {
	var walked bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/caldav" {
			http.Redirect(w, r, "/dav/cal", http.StatusTemporaryRedirect)
			return
		}
		walked = true
		w.Header().Set("WWW-Authenticate", `Basic realm="dav"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	got, detail, err := ResolveWellKnown(t.Context(), srv.Client(), srv.URL+"/", "/.well-known/caldav")
	if err != nil {
		t.Fatalf("err = %v, detail %q", err, detail)
	}
	if want := srv.URL + "/dav/cal"; got != want {
		t.Errorf("resolved to %q, want %q", got, want)
	}
	if walked {
		t.Error("the probe fetched the address it was given: a protected one would refuse it")
	}
}
