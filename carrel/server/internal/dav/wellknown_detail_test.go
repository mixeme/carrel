package dav

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A well-known step that fails in the client says why in its detail: the
// guard's refusal of an https→http redirect (docs/servers.md, SV-10) was an
// empty detail on the connection trace.
func TestResolveWellKnownSaysWhyItFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.test/dav.php", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("dav: redirect downgrades HTTPS to HTTP")
	}
	_, detail, err := ResolveWellKnown(t.Context(), client, srv.URL+"/dav.php", "/.well-known/caldav")
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
