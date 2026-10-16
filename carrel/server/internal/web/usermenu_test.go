package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// The menu of the account says who is signed in and where, before it offers
// anything (s-account-1). Two instances — the one at home and the one at
// work — look alike everywhere else on the screen.
func TestTheAccountMenuSaysWhoIsSignedInAndWhere(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	sess := h.login("mix", "password123")
	h.do(postForm("/admin/settings", sess, settingsForm(t, h, sess, map[string]string{
		"name": "carrel.example", "inviteTTL": "24", "idle": "2", "session": "7",
	})))

	page := decode[view.AccountPage](t, h.do(get("/app/settings/account", sess)))
	if got := page.Top.Signed; string(got.Login) != "mix" || string(got.Instance) != "carrel.example" {
		t.Errorf("signed in as %+v, want mix on carrel.example", got)
	}
}

// What the menu offers, in the order of the frame: Settings, Administration
// for an administrator, and Sign out past the line. Theme and Keyboard
// shortcuts are the client's — they have no route and no server behind them.
func TestTheAccountMenuOffersSettingsAdministrationAndSignOut(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("mix", "password123")
	h.invite(adminID, "bob", "bobpass123")

	admin := h.login("mix", "password123")
	page := decode[view.AccountPage](t, h.do(get("/app/settings/account", admin)))
	var ids []string
	for _, it := range page.Top.User {
		ids = append(ids, it.ID)
	}
	if want := []string{"user.settings", "user.admin", "user.logout"}; !equalStrings(ids, want) {
		t.Fatalf("the administrator's menu = %v, want %v", ids, want)
	}
	if out := page.Top.User[2]; !out.Separator || out.Method != "POST" || out.Href != "/logout" {
		t.Errorf("sign out = %+v", out)
	}

	plain := h.login("bob", "bobpass123")
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", plain)))
	ids = nil
	for _, it := range page.Top.User {
		ids = append(ids, it.ID)
	}
	if want := []string{"user.settings", "user.logout"}; !equalStrings(ids, want) {
		t.Fatalf("a person who is not an administrator sees %v, want %v", ids, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
