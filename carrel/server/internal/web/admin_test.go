package web_test

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

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
	if page.Result.Mailed != "SMTP not configured" {
		t.Fatalf("mailed = %q, want SMTP not configured", page.Result.Mailed)
	}
	temp := string(page.Result.Password)
	if _, _, ok, _ := h.users.Authenticate(context.Background(), "bob", temp); !ok {
		t.Fatal("cannot sign in with temporary password from recovery page")
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

	form := url.Values{"action": {"create"}, "login": {"carol"}, "email": {"carol@example.com"}}
	rec := h.do(postForm("/admin/invites", alice, form))
	if rec.Code != 303 {
		t.Fatalf("POST create invite: %d, want redirect", rec.Code)
	}
	snap := h.st.Snapshot()
	var link string
	for _, inv := range snap.Invites {
		if inv.Login == "carol" && inv.Secret != "" {
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
		if row.Login == "carol" && row.Link != "" {
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
