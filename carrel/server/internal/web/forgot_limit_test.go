package web_test

import (
	"net/url"
	"regexp"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// Audit №4, А4-7, the owner's decision of 25.11.2026: /forgot had a bucket
// by address only — anybody knowing a login could have hundreds of reset
// letters sent to that person, each a live link. Now one letter per
// account in five minutes (a request within them answers as one that went,
// and sends nothing), and one live link per account: a new letter burns the
// one before.
func TestForgotSendsOneLetterAndKeepsOneLink(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	h.confirmAddress("bob", "bob@example.org")
	relay := h.relayed()
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	h.users.SetClock(now)

	ask := func() view.ForgotPage {
		t.Helper()
		page := decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", "/forgot", url.Values{"login": {"bob"}})))
		if len(page.Options) == 0 {
			t.Fatalf("no options: %+v", page)
		}
		return decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", "/forgot", formFields(page.Options[0].Form))))
	}
	link := regexp.MustCompile(`/forgot/[A-Za-z0-9_-]+`)
	if p := ask(); !p.Sent {
		t.Fatalf("first request: %+v", p)
	}
	if p := ask(); !p.Sent {
		t.Errorf("a second request within five minutes answers otherwise: %+v", p)
	}
	if n := len(relay.Letters()); n != 1 {
		t.Fatalf("%d letters within five minutes, want 1", n)
	}
	_, first := letterBody(t, relay.Letters()[0])

	h.users.SetClock(now.Add(6 * time.Minute))
	if p := ask(); !p.Sent {
		t.Fatalf("after five minutes: %+v", p)
	}
	if n := len(relay.Letters()); n != 2 {
		t.Fatalf("%d letters, want 2", n)
	}
	live := 0
	snap := h.st.Snapshot()
	bob := snap.UserByLogin("bob").ID
	for _, tok := range snap.Tokens {
		if tok.UserID == bob && tok.Kind == "forgot_wipe" && !tok.Used {
			live++
		}
	}
	if live != 1 {
		t.Errorf("%d live reset links of bob, want 1", live)
	}
	old := link.FindString(first)
	if old == "" {
		t.Fatalf("no link in the first letter:\n%s", first)
	}
	if rec := h.do(get(old, nil)); rec.Code == 200 {
		t.Errorf("the first letter's link still opens after a new one went: %d", rec.Code)
	}
}
