package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// The owner's decision of 24.11.2026: a copy of the only administrator's key
// is of no use — recovery is done by an administrator signed in with the
// master password, and there is no other. While they are the only one,
// «Deposit my key» is off with its reason, and a deposit asked anyway is
// refused with it; a second administrator turns it on. Nobody recovers
// themselves: the signed-in administrator's own login is refused in
// Recover.
func TestTheOnlyAdministratorDepositsNoKey(t *testing.T) {
	const reason = "No other administrator could recover it: you are the only one."
	h := newHarness(t)
	h.enableEscrow("alice", "password123", "sixteen-char-pass!")
	alice := h.login("alice", "password123")

	deposit := func(p view.EscrowStatus) *view.Action {
		for i, a := range p.Actions {
			if a.ID == "escrow.deposit" {
				return &p.Actions[i]
			}
		}
		return nil
	}
	for _, path := range []string{"/app/settings/account", "/app/settings/escrow"} {
		var st view.EscrowStatus
		if path == "/app/settings/escrow" {
			st = decode[view.EscrowPage](t, h.do(get(path, alice))).Status
		} else {
			st = decode[view.AccountPage](t, h.do(get(path, alice))).Escrow
		}
		if a := deposit(st); a == nil || a.Enabled || string(a.Reason) != reason {
			t.Errorf("%s: deposit = %+v, want off with %q", path, a, reason)
		}
	}
	h.do(postForm("/app/settings/escrow", alice, url.Values{"action": {"deposit"}, "password": {"password123"}}))
	if snap := h.st.Snapshot(); store.UserDeposited(snap.UserByLogin("alice")) {
		t.Error("the only administrator's key was deposited")
	}

	// A second administrator: the deposit is on.
	snap := h.st.Snapshot()
	adminID := snap.UserByLogin("alice").ID
	bobID := h.invite(adminID, "bob", "bobpass123")
	if err := h.users.SetUserRole(adminID, bobID, store.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if a := deposit(decode[view.EscrowPage](t, h.do(get("/app/settings/escrow", alice))).Status); a == nil || !a.Enabled {
		t.Errorf("with two administrators deposit = %+v", a)
	}
	h.do(postForm("/app/settings/escrow", alice, url.Values{"action": {"deposit"}, "password": {"password123"}}))
	if snap := h.st.Snapshot(); !store.UserDeposited(snap.UserByLogin("alice")) {
		t.Fatal("with two administrators the deposit did not go")
	}

	// alice cannot recover herself; bob can recover her.
	rec := h.do(postForm("/admin/escrow", alice, url.Values{"action": {"recover"}, "login": {"alice"}, "master": {"sixteen-char-pass!"}, "temp": {"temporary-pass-1"}}))
	if !strings.Contains(rec.Body.String(), "You are signed in as alice: change your password on Account.") {
		t.Errorf("recovering oneself: %d %.300s", rec.Code, rec.Body.String())
	}
	bob := h.login("bob", "bobpass123")
	if rec := h.do(postForm("/admin/escrow", bob, url.Values{"action": {"recover"}, "login": {"alice"}, "master": {"sixteen-char-pass!"}, "temp": {"temporary-pass-1"}})); strings.Contains(rec.Body.String(), "recoverErr") || rec.Code >= 400 {
		t.Errorf("bob recovering alice: %d", rec.Code)
	}
}
