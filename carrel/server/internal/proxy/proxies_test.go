package proxy_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/proxy"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
)

// Ф2-11 holds for davloom the way it holds for the web: behind a proxy in
// CARREL_TRUSTED_PROXIES the buckets key on the rightmost untrusted
// X-Forwarded-For address, whatever separates the list. davloom read the
// list on commas only, so "a; b" or "a b" — which the web accepts — trusted
// nobody, every device shared the proxy's address, and twenty wrong
// passwords from one stranger locked every device out (audit №3, Ф3-5).
func TestFailedSignInsBehindTrustedProxyKeyOnTheClient(t *testing.T) {
	dir := t.TempDir()
	key, err := crypt.CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir, cfg.DAVloomPort, cfg.TrustedProxies = dir, 8081, "10.0.0.0/8; 127.0.0.1"
	users := user.New(st, session.NewManager(cfg), cfg)
	if err := users.SetupFirstAdmin(context.Background(), "alice", "password123"); err != nil {
		t.Fatal(err)
	}
	px := proxy.New(cfg, users, dav.NewGuard(cfg), "test", "abc")
	send := func(login, client string) int {
		req := httptest.NewRequest("PROPFIND", "/principal/", nil)
		req.RemoteAddr = "127.0.0.1:4000"
		req.Header.Set("X-Forwarded-For", client+", 10.0.0.2")
		req.SetBasicAuth(login, "wrong")
		rec := httptest.NewRecorder()
		px.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < conf.LoginRatePerIP; i++ {
		if code := send(fmt.Sprintf("nobody%d", i), "203.0.113.250"); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i+1, code)
		}
	}
	if code := send("nobody-more", "203.0.113.250"); code != http.StatusTooManyRequests {
		t.Fatalf("one client past its limit: %d, want 429", code)
	}
	if code := send("nobody-else", "203.0.113.251"); code != http.StatusUnauthorized {
		t.Fatalf("another client through the same proxy: %d, want 401", code)
	}
}
