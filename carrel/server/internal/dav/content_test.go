package dav

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
)

// A CalDAV or CardDAV server refuses a record PUT without Content-Type
// (go-webdav answers 400); the providers' writes used to send none.
func TestPutSendsTheBodyType(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get("Content-Type"))
		w.Header().Set("ETag", `"e"`)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	g := NewGuard(&conf.Config{
		DAVConnectTimeout: 5 * time.Second, DAVRequestTimeout: 10 * time.Second,
		DAVMaxResponseBytes: 1 << 20, DAVMaxRedirects: 5, DAVSSRFAllowlist: "127.0.0.1",
	})
	c, err := NewClient(g, srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	const ical = "text/calendar; charset=utf-8"
	if _, err := c.Put(t.Context(), "/c/a.ics", Typed(strings.NewReader("x"), ical), `"e"`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PutCreate(t.Context(), "/c/b.ics", Typed(strings.NewReader("x"), ical)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Put(t.Context(), "/f/plain.bin", strings.NewReader("x"), ""); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != ical || got[1] != ical || got[2] != "" {
		t.Fatalf("Content-Type = %q, want the typed bodies' type and none for a plain body", got)
	}
}
