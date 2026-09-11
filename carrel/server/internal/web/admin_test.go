package web_test

import (
	"encoding/json"
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

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
