package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

func (h *harness) enableEscrow(adminLogin, adminPassword, master string) {
	h.t.Helper()
	adminID := h.admin(adminLogin, adminPassword)
	alice := h.login(adminLogin, adminPassword)
	form := url.Values{
		"action":  {"enable"},
		"master":  {master},
		"master2": {master},
	}
	rec := h.do(postForm("/admin/escrow", alice, form))
	if rec.Code != 303 {
		h.t.Fatalf("POST enable escrow: %d, want redirect", rec.Code)
	}
	_ = adminID
}

func TestPostAdminEscrowRecoverShowsTempPassword(t *testing.T) {
	h := newHarness(t)
	const master = "sixteen-char-pass!"
	h.enableEscrow("alice", "password123", master)
	alice := h.login("alice", "password123")
	snap := h.st.Snapshot()
	adminID := snap.UserByLogin("alice").ID
	h.invite(adminID, "bob", "bobpass123")

	form := url.Values{"action": {"recover"}, "login": {"bob"}, "master": {master}}
	rec := h.do(postForm("/admin/escrow", alice, form))
	if rec.Code != 200 {
		t.Fatalf("POST recover: %d, want 200 with temp password on page", rec.Code)
	}
	var page view.AdminEscrowPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Result == nil || page.Result.Password == "" {
		t.Fatal("recovery result missing temporary password")
	}
	// Issued as a reset issues a password (Ад34), with why no letter went.
	if page.Result.Note != "No letter went: mail is not configured. Tell them yourself." || page.Result.Copy.Icon != "i-dup" || page.Result.Done.Href != "/admin/escrow" {
		t.Fatalf("result = %+v", page.Result)
	}
	temp := string(page.Result.Password)
	if _, _, ok, _ := h.users.Authenticate(context.Background(), "bob", temp); !ok {
		t.Fatal("cannot sign in with temporary password from recovery page")
	}
}

func TestEscrowWithdrawActionCarriesAction(t *testing.T) {
	h := newHarness(t)
	const master = "sixteen-char-pass!"
	h.enableEscrow("alice", "password123", master)
	snap := h.st.Snapshot()
	adminID := snap.UserByLogin("alice").ID
	h.invite(adminID, "bob", "bobpass123")
	bob := h.login("bob", "bobpass123")

	rec := h.do(get("/app/settings/escrow", bob))
	var page view.EscrowPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	// The page's button opens the sheet; the sheet's carries the action.
	open := actionHref(t, page.Status.Actions, "escrow.withdraw")
	var sheet view.EscrowWithdrawSheet
	if err := json.Unmarshal(h.do(get(open, bob)).Body.Bytes(), &sheet); err != nil {
		t.Fatal(err)
	}
	if snap := h.st.Snapshot(); !store.UserDeposited(snap.UserByLogin("bob")) {
		t.Fatal("opening the sheet withdrew the key")
	}
	href := actionHref(t, sheet.Actions, "escrow.withdraw")
	u, err := url.Parse(href)
	if err != nil || u.Query().Get("action") != "withdraw" {
		t.Fatalf("withdraw href = %q, want action=withdraw", href)
	}
	rec = h.do(postForm(href, bob, url.Values{}))
	if rec.Code != 303 {
		t.Fatalf("POST withdraw href: %d, want redirect", rec.Code)
	}
	snap = h.st.Snapshot()
	if store.UserDeposited(snap.UserByLogin("bob")) {
		t.Fatal("bob still deposited after withdraw via href")
	}
}

func TestGetAdminEscrowMasterFormWhenEnabled(t *testing.T) {
	h := newHarness(t)
	h.enableEscrow("alice", "password123", "sixteen-char-pass!")
	alice := h.login("alice", "password123")

	rec := h.do(get("/admin/escrow", alice))
	if rec.Code != 200 {
		t.Fatalf("GET escrow: %d", rec.Code)
	}
	var page view.AdminEscrowPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Master.Sets) == 0 || len(page.Master.Sets[0].Rows) == 0 {
		t.Fatal("master form missing when escrow is enabled")
	}
	found := false
	for _, row := range page.Master.Sets[0].Rows {
		if row.Name == "action" && row.Value == "master" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("master form missing action=master when escrow is enabled")
	}
}

func TestPostAdminEscrowChangeMaster(t *testing.T) {
	h := newHarness(t)
	const oldMaster = "sixteen-char-pass!"
	const newMaster = "another-sixteen!!"
	h.enableEscrow("alice", "password123", oldMaster)
	alice := h.login("alice", "password123")
	snap := h.st.Snapshot()
	adminID := snap.UserByLogin("alice").ID
	h.invite(adminID, "bob", "bobpass123")

	form := url.Values{
		"action":  {"master"},
		"current": {oldMaster},
		"master":  {newMaster},
		"master2": {newMaster},
	}
	rec := h.do(postForm("/admin/escrow", alice, form))
	if rec.Code != 303 {
		t.Fatalf("POST change master: %d, want redirect", rec.Code)
	}
	form = url.Values{"action": {"recover"}, "login": {"bob"}, "master": {newMaster}}
	rec = h.do(postForm("/admin/escrow", alice, form))
	if rec.Code != 200 {
		t.Fatalf("POST recover with new master: %d", rec.Code)
	}
}

func TestPostAdminUsersDisable(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	bobID := h.invite(adminID, "bob", "bobpass123")
	alice := h.login("alice", "password123")
	h.login("bob", "bobpass123")

	form := url.Values{"action": {"disable"}, "target": {bobID}}
	rec := h.do(postForm("/admin/", alice, form))
	if rec.Code != 303 {
		t.Fatalf("POST disable: %d, want redirect", rec.Code)
	}
	snap := h.st.Snapshot()
	if u := snap.UserByID(bobID); u == nil || !u.Disabled {
		t.Fatal("bob not disabled")
	}
}

func TestPostAdminInvitesCreate(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	alice := h.login("alice", "password123")

	form := url.Values{"email": {"carol@example.com"}}
	rec := h.do(postForm("/admin/invites?action=link", alice, form))
	if rec.Code != 303 {
		t.Fatalf("POST create invite: %d, want redirect", rec.Code)
	}
	snap := h.st.Snapshot()
	var link string
	for _, inv := range snap.Invites {
		if inv.Secret != "" {
			link = "/invite/" + inv.Secret
			break
		}
	}
	if link == "" {
		t.Fatal("invite secret not stored for panel link")
	}
	rec = h.do(get("/admin/invites", alice))
	if rec.Code != 200 {
		t.Fatalf("GET invites: %d", rec.Code)
	}
	var page view.AdminInvitesPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range page.Rows {
		// A link goes as a link: the address typed beside it is not kept.
		if row.Link == text(link) && row.Address == "—" && row.Delivery == "link" {
			found = true
		}
	}
	if !found {
		t.Fatal("invite link not shown on panel")
	}
	_ = adminID
}

func TestPostAdminUsersEndSessions(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	bobID := h.invite(adminID, "bob", "bobpass123")
	h.login("bob", "bobpass123")

	form := url.Values{"action": {"sessions"}, "target": {bobID}}
	rec := h.do(postForm("/admin/", alice, form))
	if rec.Code != 303 {
		t.Fatalf("POST end sessions: %d, want redirect", rec.Code)
	}
	if h.mgr.LiveCount(bobID) != 0 {
		t.Fatal("bob sessions still live")
	}
}

func TestGetAdminAuditShowsJournal(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	_, _ = h.users.CreateInvite(adminID, "bob", "")

	rec := h.do(get("/admin/audit", alice))
	if rec.Code != 200 {
		t.Fatalf("GET audit: %d", rec.Code)
	}
	var page view.AdminAuditPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range page.Rows {
		if row.What == "user.invite.create" {
			found = true
		}
	}
	if !found {
		t.Fatal("audit page missing invite.create row")
	}
}

func TestGetAdminUsersListsPeople(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	alice := h.login("alice", "password123")

	rec := h.do(get("/admin/", alice))
	var page view.AdminUsersPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) < 2 {
		t.Fatalf("rows = %d, want alice and bob", len(page.Rows))
	}
}

func TestAdminUserMenuHrefDisables(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	bobID := h.invite(adminID, "bob", "bobpass123")
	alice := h.login("alice", "password123")

	rec := h.do(get("/admin/", alice))
	var page view.AdminUsersPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	href := ""
	for _, row := range page.Rows {
		if row.Login != "bob" {
			continue
		}
		for _, item := range row.Menu {
			if item.ID == "admin.user.disable" {
				href = item.Href
			}
		}
	}
	if href == "" {
		t.Fatal("disable action missing for bob")
	}
	rec = h.do(postForm(href, alice, url.Values{}))
	if rec.Code != 303 {
		t.Fatalf("POST disable href %q: %d, want redirect", href, rec.Code)
	}
	snap := h.st.Snapshot()
	if u := snap.UserByID(bobID); u == nil || !u.Disabled {
		t.Fatal("bob not disabled via menu href")
	}
}

func TestAdminInviteActionsNameTheInvite(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	h.relayed()
	form := url.Values{"email": {"carol@example.com"}}
	if rec := h.do(postForm("/admin/invites?action=send", alice, form)); rec.Code != 303 {
		t.Fatalf("create invite: %d", rec.Code)
	}
	snap := h.st.Snapshot()
	var invID, secret string
	for _, inv := range snap.Invites {
		if inv.Email == "carol@example.com" {
			invID, secret = inv.ID, inv.Secret
			break
		}
	}
	if invID == "" {
		t.Fatal("invite not stored")
	}

	rec := h.do(get("/admin/invites", alice))
	var page view.AdminInvitesPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	var extend, revoke, resend string
	for _, row := range page.Rows {
		if row.Address != "carol@example.com" {
			continue
		}
		for _, a := range append(row.Actions, menuActions(row.Menu)...) {
			switch a.ID {
			case "admin.invite.extend":
				extend = a.Href
			case "admin.invite.revoke":
				revoke = a.Href
			case "admin.invite.resend":
				resend = a.Href
			}
		}
	}
	mustQuery := func(raw, key, want string) {
		t.Helper()
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if u.Query().Get(key) != want {
			t.Fatalf("%s %s = %q, want %q (href %q)", key, raw, u.Query().Get(key), want, raw)
		}
	}
	mustQuery(extend, "action", "extend")
	mustQuery(extend, "invite", invID)
	mustQuery(revoke, "action", "revoke")
	mustQuery(revoke, "invite", invID)
	mustQuery(resend, "action", "resend")
	mustQuery(resend, "invite", invID)

	if rec := h.do(postForm(extend, alice, url.Values{})); rec.Code != 303 {
		t.Fatalf("POST extend href: %d", rec.Code)
	}
	if !h.users.InviteValid(context.Background(), secret) {
		t.Fatal("extend via href broke the secret")
	}
	// Resend replaces the secret (S-10.3): the link sent before dies, the
	// invitation lives on with a new one.
	if rec := h.do(postForm(resend, alice, url.Values{})); rec.Code != 303 {
		t.Fatalf("POST resend href: %d %s", rec.Code, rec.Body)
	}
	if h.users.InviteValid(context.Background(), secret) {
		t.Fatal("the old link still works after Resend")
	}
	snap = h.st.Snapshot()
	for _, inv := range snap.Invites {
		if inv.ID == invID && (inv.Secret == secret || !h.users.InviteValid(context.Background(), inv.Secret)) {
			t.Fatalf("after Resend: %+v", inv)
		}
	}
	_ = adminID
}

// Ад39: Recover past the ceiling of attempts answers the page again, the
// refusal at its field — not a bare «too many requests».
func TestEscrowRecoverPastTheCeilingSaysSo(t *testing.T) {
	h := newHarness(t)
	const master = "sixteen-char-pass!"
	h.enableEscrow("alice", "password123", master)
	alice := h.login("alice", "password123")
	snap := h.st.Snapshot()
	h.invite(snap.UserByLogin("alice").ID, "bob", "bobpass123")
	form := url.Values{"action": {"recover"}, "login": {"bob"}, "master": {"not-the-master-pass"}}
	for i := 0; i < 60; i++ {
		rec := h.do(postForm("/admin/escrow", alice, form))
		if rec.Code != http.StatusTooManyRequests {
			continue
		}
		var page view.AdminEscrowPage
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("past the ceiling: %v\n%s", err, rec.Body.String())
		}
		said := false
		for _, set := range page.Recover.Sets {
			for _, r := range set.Rows {
				said = said || strings.Contains(string(r.Error), "Too many attempts")
			}
		}
		if !said {
			t.Errorf("the refusal is not at the field: %+v", page.Recover)
		}
		return
	}
	t.Fatal("no ceiling reached")
}
