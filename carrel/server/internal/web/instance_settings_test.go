package web_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// Owner's decision 19.09.2026: the name, the lifetimes, people's mail and
// its ceilings are settings of the screen; zero keeps the default, and a
// value out of range is refused on its field.
func TestInstanceSettingsAreEdited(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	h.relayed()

	page := decode[view.AdminSettingsPage](t, h.do(get("/admin/settings", alice)))
	if f := formFields(page.Instance); f.Get("session") != "30" || f.Get("inviteTTL") != "72" || f.Get("idle") != "12" || f.Get("name") != "Carrel" {
		t.Errorf("defaults = %v", f)
	}
	if f := formFields(page.Limits); f.Get("attach") != "10" || f.Get("recipients") != "20" {
		t.Errorf("mail defaults = %v", f)
	}

	rec := h.do(postForm("/admin/settings", alice, url.Values{"op": {"instance"}, "name": {"Контора"}, "inviteTTL": {"24"}, "idle": {"2"}, "session": {"7"}}))
	if rec.Code != 303 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	set := h.users.Settings()
	if set.Name != "Контора" || set.InviteHours != 24 || set.IdleHours != 2 || set.SessionDays != 7 {
		t.Errorf("stored = %+v", set)
	}
	if _, err := h.users.CreateInvite(adminID, "bob", ""); err != nil {
		t.Fatal(err)
	}
	snap := h.st.Snapshot()
	for _, inv := range snap.Invites {
		if inv.Login == "bob" && inv.ExpiresAt.Sub(inv.CreatedAt) != 24*time.Hour {
			t.Errorf("invitation lives %v", inv.ExpiresAt.Sub(inv.CreatedAt))
		}
	}
	// Idle applies to a session already open.
	if !alice.Expired(time.Now().Add(2*time.Hour + time.Minute)) {
		t.Error("the new idle timeout did not reach a live session")
	}
	login := decode[view.LoginPage](t, h.do(get("/login", nil)))
	if login.Title != "Sign in to Контора" {
		t.Errorf("sign-in title = %q", login.Title)
	}

	page = decode[view.AdminSettingsPage](t, h.do(postForm("/admin/settings", alice, url.Values{"op": {"limits"}, "attach": {"30"}, "letter": {"20"}, "recipients": {"5"}, "perDay": {"9"}})))
	var refused string
	for _, row := range page.Limits.Sets[0].Rows {
		if row.Error != "" {
			refused = row.Name
		}
	}
	if refused != "attach" {
		t.Errorf("an attachment over the letter was not refused on its field: %q", refused)
	}

	rec = h.do(postForm("/admin/settings", alice, url.Values{"op": {"limits"}, "people": {"on"}, "attach": {"1"}, "letter": {"20"}, "recipients": {"5"}, "perDay": {"9"}}))
	if rec.Code != 303 {
		t.Fatalf("limits: %d %s", rec.Code, rec.Body)
	}
	sheet := decode[view.MailSheet](t, h.do(get("/app/mail/compose", alice)))
	if !strings.Contains(string(sheet.Limits), "0 of 5 recipients") || !strings.Contains(string(sheet.Limits), "9 letters left") {
		t.Errorf("limits = %q", sheet.Limits)
	}
	if h.users.Settings().Name != "Контора" {
		t.Error("the letters form changed the Global one")
	}
}
