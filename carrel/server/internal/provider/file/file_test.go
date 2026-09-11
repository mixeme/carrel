package file

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
)

func testProvider(t *testing.T, h http.Handler) *Provider {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := &conf.Config{
		DAVConnectTimeout:   5 * time.Second,
		DAVRequestTimeout:   10 * time.Second,
		DAVMaxResponseBytes: 1 << 20,
		DAVMaxRedirects:     5,
		DAVSSRFAllowlist:    "127.0.0.1",
	}
	client, err := dav.NewClient(dav.NewGuard(c), srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(client)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGetStreams(t *testing.T) {
	p := testProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method %q", r.Method)
		}
		w.Header().Set("ETag", `"e1"`)
		_, _ = w.Write([]byte("hello"))
	}))
	rc, etag, err := p.Get(t.Context(), "/a.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "hello" || etag != "e1" {
		t.Fatalf("body=%q etag=%q", b, etag)
	}
}

func TestUploadCreate412(t *testing.T) {
	p := testProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
	}))
	_, err := p.UploadCreate(t.Context(), "/taken.txt", strings.NewReader("x"))
	if !ErrOccupied(err) {
		t.Fatalf("got %v", err)
	}
}

func TestUploadCreateSendsIfNoneMatch(t *testing.T) {
	var hdr string
	p := testProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header.Get("If-None-Match")
		w.Header().Set("ETag", `"created"`)
		w.WriteHeader(http.StatusCreated)
	}))
	etag, err := p.UploadCreate(t.Context(), "/new.txt", strings.NewReader("data"))
	if err != nil || etag != "created" || hdr != "*" {
		t.Fatalf("etag=%q hdr=%q err=%v", etag, hdr, err)
	}
}

func TestGetPassesRange(t *testing.T) {
	var rng string
	p := testProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rng = r.Header.Get("Range")
		w.WriteHeader(http.StatusPartialContent)
	}))
	rc, _, err := p.Get(t.Context(), "/big.bin", &dav.Range{Start: 0, End: 4095})
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
	if rng != "bytes=0-4095" {
		t.Fatalf("Range = %q", rng)
	}
}
