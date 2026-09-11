package dav

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestGuardAllowlistIPv6Literal(t *testing.T) {
	c := testConfig()
	c.DAVSSRFAllowlist = "::1"
	g := NewGuard(c)
	if !g.allowlisted("::1") {
		t.Fatal("::1 should match allowlist entry")
	}
}

func TestGuardBlocksPrivateAfterRedirect(t *testing.T) {
	c := testConfig()
	c.DAVSSRFAllowlist = "localhost"
	g := NewGuard(c)
	client := g.HTTPClient()

	var redirectorHits int32
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer inner.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&redirectorHits, 1)
		http.Redirect(w, r, inner.URL, http.StatusFound)
	}))
	defer redirector.Close()

	u := redirector.URL
	u = strings.Replace(u, "127.0.0.1", "localhost", 1)

	_, err := client.Get(u)
	if err == nil {
		t.Fatal("expected redirect target to be blocked")
	}
	if got := atomic.LoadInt32(&redirectorHits); got != 1 {
		t.Fatalf("redirector hits = %d, want 1", got)
	}
}

func TestGuardBlocksNAT64EmbeddedPrivate(t *testing.T) {
	g := NewGuard(testConfig())
	g.resolver = stubResolver{addrs: []net.IPAddr{{IP: net.ParseIP("64:ff9b::a00:1")}}}

	_, err := g.dialContext(context.Background(), "tcp", "evil.example:443")
	if !errors.Is(err, ErrSSRF) && !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected NAT64 private block, got %v", err)
	}
}

type stubResolver struct {
	addrs []net.IPAddr
}

func (s stubResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return s.addrs, nil
}

func TestBlockedIPRanges(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"169.254.0.1", true},
		{"100.64.0.1", true},
		{"::1", true},
		{"fe80::1", true},
		{"fc00::1", true},
		{"64:ff9b::10.0.0.1", true},
		{"64:ff9b::7f00:1", true},
		{"64:ff9b:1::a00:1", true},
		{"2002:0a00:0001::1", true},
		{"2002:7f00:0001::1", true},
		{"::10.0.0.1", true},
		{"::ffff:0:10.0.0.1", true},
		{"2001:0:1234::1", true},
		{"fec0::1", true},
		{"0.0.0.1", true},
		{"255.255.255.255", true},
		{"240.0.0.1", true},
		{"192.0.0.1", true},
		{"8.8.8.8", false},
	}
	for _, tc := range cases {
		got := blockedIP(net.ParseIP(tc.addr))
		if got != tc.want {
			t.Fatalf("%s blocked=%v, want %v", tc.addr, got, tc.want)
		}
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
