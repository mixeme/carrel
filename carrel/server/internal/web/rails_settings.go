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
	rail := view.Rail{About: s.aboutAction(), Davloom: s.davloomMark(sess), Nav: []view.NavRow{
		{Action: view.Action{ID: "settings.back", Label: "Back to app", Icon: "i-back",
			Href: s.route("/app/contacts"), Method: "GET", Enabled: true}},
	}}
	// Each row has its icon, as the sections' rows do (s-publish-1,
	// account-1); davloom's is its mark, which the screen draws.
	rows := []struct {
		id, label, icon, route string
		count                  *int
	}{
		{"settings.account", "Account", "i-user", "/app/settings/account", nil},
		{"settings.connections", "Connections", "i-server", "/app/settings/connections", count(blob.ConnectionCount())},
		{"settings.attachments", "Attachments", "i-file", "/app/settings/attachments", nil},
		{"settings.devices", "New device", "i-device", "/app/settings/devices", nil},
		{"settings.davloom", "davloom", "", "/app/settings/davloom", count(len(s.users.Devices(sess.AccountID())))},
		{"settings.backup", "Backup", "i-archive", "/app/settings/backup", countSome(len(blob.BackupJobs))},
		// No links, no number (Пб42; s-publish-18 writes no zero).
		{"settings.published", "Published links", "i-link", "/app/files/published", countSome(len(s.users.PublishedLinks(sess.AccountID())))},
		{"settings.appearance", "Appearance", "i-theme", "/app/settings/appearance", nil},
	}
	for _, r := range rows {
		rail.Nav = append(rail.Nav, view.NavRow{
			Action: view.Action{ID: r.id, Label: text(r.label), Icon: r.icon, Href: s.route(r.route), Method: "GET",
				Enabled: true, On: r.route == open},
			Count: r.count,
		})
	}
	rail.Status = s.settingsStatus(sess)
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
	// The rail of administration draws no line of davloom (Ад2; s-admin-*).
	rail := view.Rail{About: s.aboutAction(), Nav: []view.NavRow{
		{Action: view.Action{ID: "admin.back", Label: "Back to app", Icon: "i-back",
			Href: s.route("/app/contacts"), Method: "GET", Enabled: true}},
	}}
	rows := []struct {
		id, label, icon, route string
		count                  *int
	}{
		// Each with its icon, as the frames draw them (Ад1; s-admin-*).
		{"admin.invites", "Add users", "i-plus", "/admin/invites", count(live)},
		{"admin.users", "Users", "i-con", "/admin/", count(people)},
		{"admin.settings", "Settings", "i-adm", "/admin/settings", nil},
		{"admin.install", "Installation", "i-shield", "/admin/install", nil},
		{"admin.dav", "DAV validator", "i-search", "/admin/dav", nil},
		{"admin.escrow", "Key escrow", "i-dup", "/admin/escrow", nil},
		{"admin.audit", "Audit log", "i-clock", "/admin/audit", nil},
	}
	for _, r := range rows {
		rail.Nav = append(rail.Nav, view.NavRow{
			Action: view.Action{ID: r.id, Label: text(r.label), Icon: r.icon, Href: s.route(r.route), Method: "GET",
				Enabled: true, On: r.route == open},
			Count: r.count,
		})
	}
	rail.Status = s.adminStatus(sess)
	return rail
}

func count(n int) *int { return &n }

// countSome is a count drawn only when there is something to count: no
// backup job is no number by Backup (s-backup-6, the owner's decision of
// 02.10.2026).
func countSome(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

// withRail is the shell with its rail: the settings and the administration
// build theirs where the shell is made, not after it.
func (s *Server) withRail(sh view.Shell, rail view.Rail) view.Shell {
	sh.Rail = rail
	return sh
}
