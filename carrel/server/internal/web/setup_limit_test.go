package web_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/view"
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
	// The form again, the words at the login — not a bare «too many
	// requests» (as Ад39).
	again := h.anonPostForm("/setup", "/setup", url.Values{"login": {"admin"}, "password": {"password123"}, "password2": {"nope"}})
	again.RemoteAddr = "198.51.100.7:4000"
	var page view.SetupPage
	if err := json.Unmarshal(h.do(again).Body.Bytes(), &page); err != nil || formRow(page.Form, "login") == nil ||
		formRow(page.Form, "login").Error != "Too many attempts just now. Wait a few minutes and try again." {
		t.Errorf("past the ceiling: %v %+v", err, page.Form)
	}
	if code := post("other"); code != http.StatusOK {
		t.Fatalf("another login from the same address: %d, want 200", code)
	}
	if h.users.FirstRun() != true {
		t.Fatal("the instance was set up by a refused attempt")
	}
}
