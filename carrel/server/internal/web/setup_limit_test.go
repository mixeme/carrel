package web_test

import (
	"net/http"
	"net/url"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
)

// S-29.3: every route without a session has a bucket. /setup had none, and
// its POST derives a key: a fresh instance could be kept busy by anybody
// who found it before its administrator did (audit №3, Ф3-4). The sign-in
// buckets apply: five a minute for one login, twenty for one address.
func TestSetupIsRateLimited(t *testing.T) {
	h := newHarness(t)
	post := func(login string) int {
		// Mismatched passwords answer before the derivation: the bucket is
		// what is measured here, not Argon2.
		req := h.anonPostForm("/setup", "/setup", url.Values{"login": {login}, "password": {"password123"}, "password2": {"nope"}})
		req.RemoteAddr = "198.51.100.7:4000"
		return h.do(req).Code
	}
	for i := 0; i < conf.LoginRatePerAccount; i++ {
		if code := post("admin"); code != http.StatusOK {
			t.Fatalf("attempt %d: %d", i+1, code)
		}
	}
	if code := post("admin"); code != http.StatusTooManyRequests {
		t.Fatalf("attempt %d on one login: %d, want 429", conf.LoginRatePerAccount+1, code)
	}
	if code := post("other"); code != http.StatusOK {
		t.Fatalf("another login from the same address: %d, want 200", code)
	}
	if h.users.FirstRun() != true {
		t.Fatal("the instance was set up by a refused attempt")
	}
}
