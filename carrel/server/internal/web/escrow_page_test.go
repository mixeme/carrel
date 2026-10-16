package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// account-12 (Нс14): the page without a deposit draws no sheet — Deposit my
// key opens it as its own answer; the sheet stands on the page only when a
// deposit came back refused without scripts.
func TestTheEscrowPageCarriesTheSheetOnlyWhenRefused(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	alice := h.login("alice", "password123")
	if rec := h.do(postForm("/admin/escrow", alice, url.Values{"action": {"enable"}, "master": {"sixteen-char-pass!"}, "master2": {"sixteen-char-pass!"}})); rec.Code >= 400 {
		t.Fatalf("enable escrow: %d", rec.Code)
	}
	bob := h.login("bob", "bobpass123")
	page := decode[view.EscrowPage](t, h.do(get("/app/settings/escrow", bob)))
	if page.Subtitle != "No copy of your data key is deposited" || page.Confirm != nil {
		t.Fatalf("page = subtitle %q, confirm %+v", page.Subtitle, page.Confirm)
	}
	refused := decode[view.EscrowPage](t, h.do(postForm("/app/settings/escrow", bob, url.Values{"action": {"deposit"}, "password": {"wrong-one"}})))
	if refused.Confirm == nil {
		t.Error("a refused deposit came back without its sheet")
	}
}
