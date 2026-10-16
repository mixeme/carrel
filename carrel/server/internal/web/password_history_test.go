package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The owner's decision of 16.10.2026 (Ад40): a letter is no guarantee — the
// administrator runs the relay. A deposit gives the account one permanent
// link to its password's history, always shown in Key escrow; by it,
// without signing in, the account's person sees what was done to the
// password and by whom: a recovery with the master password, a change.
func TestThePasswordHistoryLink(t *testing.T) {
	h := newHarness(t)
	const master = "sixteen-char-pass!"
	h.enableEscrow("alice", "password123", master)
	alice := h.login("alice", "password123")
	snap := h.st.Snapshot()
	h.invite(snap.UserByLogin("alice").ID, "bob", "bobpass123")
	bob := h.login("bob", "bobpass123")

	escrow := decode[view.EscrowPage](t, h.do(get("/app/settings/escrow", bob)))
	link := escrow.PasswordLink
	if link == nil || !strings.Contains(string(link.Address), "/password-history/") || link.Copy.Label != "Copy" {
		t.Fatalf("link = %+v", link)
	}
	u, _ := url.Parse(string(link.Address))
	empty := decode[view.PasswordHistoryPage](t, h.do(get(u.Path, nil)))
	if empty.Title != "Password history of bob" || len(empty.Rows) != 0 || empty.Empty == "" {
		t.Errorf("before = %+v", empty)
	}

	h.do(postForm("/admin/escrow", alice, url.Values{"action": {"recover"}, "login": {"bob"}, "master": {master}}))
	page := decode[view.PasswordHistoryPage](t, h.do(get(u.Path, nil)))
	if len(page.Rows) != 1 || page.Rows[0].Who != "alice" || !strings.HasPrefix(string(page.Rows[0].What), "Opened with the master password") {
		t.Errorf("after the recovery = %+v", page.Rows)
	}
	// The link is the same after it: nothing changes it.
	again := decode[view.EscrowPage](t, h.do(get("/app/settings/escrow", h.login("alice", "password123"))))
	_ = again
	if snap := h.st.Snapshot(); snap.UserByLogin("bob").PasswordLog == "" || !strings.HasSuffix(u.Path, snap.UserByLogin("bob").PasswordLog) {
		t.Error("the link changed")
	}
	if rec := h.do(get("/password-history/not-a-link", nil)); rec.Code != 404 {
		t.Errorf("an unknown link: %d", rec.Code)
	}
}
