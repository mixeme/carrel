package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Ад1, Ад2, Ад5, Ад6, Ад9, Ад16 of the scan of 15.10.2026 (s-admin-2, -3,
// -13): the rail of administration has its icons and no line of davloom;
// the list of people says what it holds above it, sifts by login and by
// All · Administrators · Disabled; the field that confirms a reset shows
// the login chosen; the settings are «Settings».
func TestAdminUsersAreTheFrame(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	h.invite(adminID, "anna", "annapass123")
	alice := h.login("alice", "password123")

	page := decode[view.AdminUsersPage](t, h.do(get("/admin/", alice)))
	if page.Subtitle != "3 accounts · 1 administrator · 0 covered by escrow" {
		t.Errorf("subtitle = %q", page.Subtitle)
	}
	icons := map[string]string{}
	for _, n := range page.Rail.Nav {
		icons[n.ID] = n.Icon
	}
	for id, want := range map[string]string{"admin.invites": "i-plus", "admin.users": "i-con", "admin.settings": "i-adm",
		"admin.install": "i-shield", "admin.dav": "i-search", "admin.escrow": "i-dup", "admin.audit": "i-clock"} {
		if icons[id] != want {
			t.Errorf("%s icon = %q, want %q", id, icons[id], want)
		}
	}
	if page.Rail.Davloom != "" {
		t.Errorf("the rail says davloom: %q", page.Rail.Davloom)
	}
	if page.Bar.FilterForm == nil || len(page.Bar.Segments) != 3 || !page.Bar.Segments[0].On || page.Bar.Segments[1].Label != "Administrators" {
		t.Fatalf("bar = %+v", page.Bar)
	}
	logins := func(p view.AdminUsersPage) string {
		var out []string
		for _, r := range p.Rows {
			out = append(out, string(r.Login))
		}
		return strings.Join(out, ",")
	}
	if got := logins(decode[view.AdminUsersPage](t, h.do(get("/admin/?filter=AN", alice)))); got != "anna" {
		t.Errorf("filtered = %q", got)
	}
	admins := decode[view.AdminUsersPage](t, h.do(get(page.Bar.Segments[1].Href, alice)))
	if got := logins(admins); got != "alice" || !admins.Bar.Segments[1].On {
		t.Errorf("administrators = %q, on %v", got, admins.Bar.Segments[1].On)
	}

	snap := h.st.Snapshot()
	sheet := decode[view.PasswordResetSheet](t, h.do(get("/admin/reset?target="+snap.UserByLogin("anna").ID, alice)))
	if c := formRow(sheet.Form, "confirm"); c == nil || c.Placeholder != "anna" {
		t.Errorf("confirm = %+v, want the login chosen as its placeholder", c)
	}
	if settings := decode[view.AdminSettingsPage](t, h.do(get("/admin/settings", alice))); settings.Title != "Settings" {
		t.Errorf("settings title = %q", settings.Title)
	}

	// Ад3, Ад26 (s-admin-7): the journal's moments in numbers; an action on
	// oneself has a dash for its target.
	audit := decode[view.AdminAuditPage](t, h.do(get("/admin/audit", alice)))
	self := false
	for _, r := range audit.Rows {
		if !regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d$`).MatchString(string(r.When)) {
			t.Errorf("when = %q", r.When)
			break
		}
		if r.Who == r.Whom {
			t.Errorf("an action on oneself names its target: %+v", r)
		}
		self = self || r.Whom == "—"
	}
	if !self {
		t.Error("no action on oneself has the dash")
	}
}
