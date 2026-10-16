package web_test

import (
	"regexp"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The tables of the settings and the administration write a moment in
// numbers, monospaced, as their frames do (Нс2; account-1, s-davloom-1,
// s-backup-1, s-admin-2): «2026-08-16 09:02», a day «2026-02-11»; when a
// session was last seen stays said in words («just now»).
func TestSettingsTablesWriteMomentsInNumbers(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	sess := h.login("mix", "password123")
	stamp := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$`)
	day := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	account := decode[view.AccountPage](t, h.do(get("/app/settings/account", sess)))
	if len(account.Sessions) == 0 || !stamp.MatchString(string(account.Sessions[0].Started)) {
		t.Errorf("sessions = %+v", account.Sessions)
	}
	users := decode[view.AdminUsersPage](t, h.do(get("/admin/", sess)))
	if len(users.Rows) == 0 || !day.MatchString(string(users.Rows[0].Created)) || !stamp.MatchString(string(users.Rows[0].LastLogin)) {
		t.Errorf("users = %+v", users.Rows)
	}
}
