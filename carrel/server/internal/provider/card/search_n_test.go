package card

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
)

// Radicale before 3.7.7 answers 500 to a text-match on a structured property
// (AttributeError on N), and Debian stable ships 3.5.3: the search REPORT
// names FN and never N.
func TestSearchSendsNoTextMatchOnN(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The search itself is the first request: an empty answer is then
		// checked against the book, and those requests are not the one under
		// test (SV-26).
		if body == nil {
			body, _ = io.ReadAll(r.Body)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?><d:multistatus xmlns:d="DAV:"/>`)
	}))
	t.Cleanup(srv.Close)
	g := dav.NewGuard(&conf.Config{
		DAVConnectTimeout: 5 * time.Second, DAVRequestTimeout: 10 * time.Second,
		DAVMaxResponseBytes: 1 << 20, DAVMaxRedirects: 5, DAVSSRFAllowlist: "127.0.0.1",
	})
	client, err := dav.NewClient(g, srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(client)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Search(t.Context(), "/book/", "ada"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`name="FN"`)) || bytes.Contains(body, []byte(`name="N"`)) {
		t.Errorf("search REPORT body:\n%s", body)
	}
}
