package web

import (
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// The third kind of rail (C-6.3): the settings and the administration. Its
// first row leads out of them, because a person who opened the settings has
// nowhere else to go back to — the sections of the app are not in this rail,
// and the browser's own button is not a product.
//
// The counts are what the screen behind the row holds, the way the counts of
// collections are: a number nobody can see without opening the screen is
// what a rail is for.

// settingsRail is the rail of every screen under /app/settings (account-1
// and every frame beside it). open is the route of the screen showing.
func (s *Server) settingsRail(sess *session.Session, open string) view.Rail {
	blob := s.catalogOf(sess)
	rail := view.Rail{About: s.aboutAction(), Nav: []view.NavRow{
		{Action: view.Action{ID: "settings.back", Label: "Back to app", Icon: "i-back",
			Href: s.route("/app/contacts"), Method: "GET", Enabled: true}},
	}}
	rows := []struct {
		id, label, route string
		count            *int
	}{
		{"settings.account", "Account", "/app/settings/account", nil},
		{"settings.connections", "Connections", "/app/settings/connections", count(len(blob.Connections))},
		{"settings.attachments", "Attachments", "/app/settings/attachments", nil},
		{"settings.devices", "New device", "/app/settings/devices", nil},
		{"settings.davloom", "davloom", "/app/settings/davloom", count(len(s.users.Devices(sess.AccountID())))},
		{"settings.backup", "Backup", "/app/settings/backup", count(len(blob.BackupJobs))},
		{"settings.published", "Published links", "/app/files/published", count(len(s.users.PublishedLinks(sess.AccountID())))},
		{"settings.appearance", "Appearance", "/app/settings/appearance", nil},
	}
	for _, r := range rows {
		rail.Nav = append(rail.Nav, view.NavRow{
			Action: view.Action{ID: r.id, Label: text(r.label), Href: s.route(r.route), Method: "GET",
				Enabled: true, On: r.route == open},
			Count: r.count,
		})
	}
	return rail
}

// adminRail is the rail of every screen under /admin (s-admin-1).
func (s *Server) adminRail(sess *session.Session, open string) view.Rail {
	snap := s.store.Snapshot()
	now := s.now()
	live := 0
	for i := range snap.Invites {
		if inviteLive(&snap.Invites[i], now) {
			live++
		}
	}
	people := 0
	for _, u := range snap.Users {
		if len(u.LoginHash) == 0 && u.Disabled {
			continue
		}
		people++
	}
	rail := view.Rail{About: s.aboutAction(), Nav: []view.NavRow{
		{Action: view.Action{ID: "admin.back", Label: "Back to app", Icon: "i-back",
			Href: s.route("/app/contacts"), Method: "GET", Enabled: true}},
	}}
	rows := []struct {
		id, label, route string
		count            *int
	}{
		{"admin.invites", "Add users", "/admin/invites", count(live)},
		{"admin.users", "Users", "/admin/", count(people)},
		{"admin.settings", "Settings", "/admin/settings", nil},
		{"admin.install", "Installation", "/admin/install", nil},
		{"admin.dav", "DAV validator", "/admin/dav", nil},
		{"admin.escrow", "Key escrow", "/admin/escrow", nil},
		{"admin.audit", "Audit log", "/admin/audit", nil},
	}
	for _, r := range rows {
		rail.Nav = append(rail.Nav, view.NavRow{
			Action: view.Action{ID: r.id, Label: text(r.label), Href: s.route(r.route), Method: "GET",
				Enabled: true, On: r.route == open},
			Count: r.count,
		})
	}
	return rail
}

func count(n int) *int { return &n }

// withRail is the shell with its rail: the settings and the administration
// build theirs where the shell is made, not after it.
func (s *Server) withRail(sh view.Shell, rail view.Rail) view.Shell {
	sh.Rail = rail
	return sh
}
