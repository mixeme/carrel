package dav

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
)

func TestGuardBlocksLoopback(t *testing.T) {
	g := NewGuard(testConfig())
	client := g.HTTPClient()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := client.Get(srv.URL)
	if !errors.Is(err, ErrSSRF) && !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected SSRF block, got %v", err)
	}
}

func TestGuardAllowlistPermitsLocalhost(t *testing.T) {
	c := testConfig()
	c.DAVSSRFAllowlist = "127.0.0.1"
	g := NewGuard(c)
	client := g.HTTPClient()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("allowlisted localhost: %v", err)
	}
	resp.Body.Close()
}

func TestGuardBlocksPrivateAfterRedirect(t *testing.T) {
	g := NewGuard(testConfig())
	client := g.HTTPClient()
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer inner.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, inner.URL, http.StatusFound)
	}))
	defer redirector.Close()

	_, err := client.Get(redirector.URL)
	if err == nil {
		t.Fatal("expected redirect target to be blocked")
	}
}

func TestBlockedIPRanges(t *testing.T) {
	cases := []string{
		"127.0.0.1",
		"10.0.0.1",
		"192.168.1.1",
		"169.254.0.1",
		"100.64.0.1",
		"::1",
		"fe80::1",
		"fc00::1",
	}
	for _, s := range cases {
		if !blockedIP(net.ParseIP(s)) {
			t.Fatalf("%s should be blocked", s)
		}
	}
	if blockedIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("8.8.8.8 should be allowed")
	}
}

func TestGetReturnsStream(t *testing.T) {
	body := strings.Repeat("x", 4096)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc"`)
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := testConfig()
	c.DAVSSRFAllowlist = "127.0.0.1"
	g := NewGuard(c)
	client, err := NewClient(g, srv.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	rc, etag, err := client.Get(context.Background(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if etag != "abc" {
		t.Fatalf("etag = %q", etag)
	}
	buf := make([]byte, 32)
	n, err := rc.Read(buf)
	if err != nil || n == 0 {
		t.Fatalf("stream read: n=%d err=%v", n, err)
	}
}

func testConfig() *conf.Config {
	return &conf.Config{
		DAVConnectTimeout:   5 * time.Second,
		DAVRequestTimeout:   10 * time.Second,
		DAVMaxResponseBytes: 1 << 20,
		DAVMaxRedirects:     5,
	}
}
