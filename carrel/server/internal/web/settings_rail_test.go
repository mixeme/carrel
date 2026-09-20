package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

func railLabels(rows []view.NavRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, string(r.Label))
	}
	return out
}

// Every screen of the settings carries the rail of the settings, and its
// first row is the way out of them (C-6.3, account-1 and every frame beside
// it). Without it a person who opened the settings has no way back but the
// browser's own.
func TestTheSettingsCarryTheirRail(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	sess := h.login("mix", "password123")

	want := []string{"Back to app", "Account", "Connections", "Attachments",
		"New device", "davloom", "Backup", "Published links", "Appearance"}
	for _, route := range []string{"/app/settings/account", "/app/settings/connections",
		"/app/settings/attachments", "/app/settings/devices", "/app/settings/davloom",
		"/app/settings/backup", "/app/settings/appearance"} {
		page := decode[view.AccountPage](t, h.do(get(route, sess)))
		if got := railLabels(page.Rail.Nav); !equalStrings(got, want) {
			t.Errorf("%s: rail = %v,\n        want %v", route, got, want)
		}
	}
}

// The administration has its own, and the open screen is marked.
func TestTheAdministrationCarriesItsRail(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	sess := h.login("mix", "password123")

	want := []string{"Back to app", "Add users", "Users", "Settings",
		"Installation", "DAV validator", "Key escrow", "Audit log"}
	page := decode[view.AdminUsersPage](t, h.do(get("/admin/", sess)))
	if got := railLabels(page.Rail.Nav); !equalStrings(got, want) {
		t.Fatalf("rail = %v,\nwant %v", got, want)
	}
	for _, r := range page.Rail.Nav {
		if r.Label == "Users" && !r.On {
			t.Errorf("the open screen is not marked: %+v", r)
		}
	}
}
