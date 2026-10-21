package web_test

import (
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// railStates are the lines of a rail's state block as the frames write them:
// «title / line», the line's marked pieces kept.
func railStates(r view.Rail) []string {
	var out []string
	for _, st := range r.Status {
		out = append(out, string(st.Tone)+": "+string(st.Title)+" / "+saidRuns(st.Line))
	}
	return out
}

// The state block of the settings and administration rails (components-35;
// account-*, s-profile-*, s-backup-*, s-admin-*): davloom and the copies
// under the settings, mail and the installation check under the
// administration, each a dot, a title and its last time monospaced.
func TestRailStatusBlock(t *testing.T) {
	stamp := `⟨mono:\d{4}-\d{2}-\d{2} \d{2}:\d{2}⟩`

	// davloom off, no job: «davloom is off», and no line about copies that
	// nobody set up.
	f := backupFixture(t)
	rail := decode[view.AccountPage](t, f.h.do(get("/app/settings/account", f.sess))).Rail
	if got := railStates(rail); strings.Join(got, "|") != "off: davloom is off / No device connects until it is started." {
		t.Errorf("settings rail, davloom off, no job = %q", got)
	}

	// A job that ran: «Backups are running», the last copy and verified.
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", ""))
	runJob(t, f, job)
	rail = decode[view.AccountPage](t, f.h.do(get("/app/settings/account", f.sess))).Rail
	got := railStates(rail)
	if len(got) != 2 || !regexp.MustCompile(`^on: Backups are running / Last backup `+stamp+` · verified$`).MatchString(got[1]) {
		t.Errorf("settings rail with a copy = %q", got)
	}
	// «verified» carries what it means (Нс5).
	if line := rail.Status[len(rail.Status)-1].Line; line[len(line)-1].Text != "verified" || line[len(line)-1].Hint == "" {
		t.Errorf("the verified piece = %+v", line)
	}

	// davloom on: running, its devices, and when one was last answered.
	h := newHarness(t)
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	h.admin("mix", "password123")
	mix := h.login("mix", "password123")
	// Nothing connected yet: the rail's foot says nothing (s-profile-25).
	if got = railStates(decode[view.AccountPage](t, h.do(get("/app/settings/account", mix))).Rail); len(got) != 0 {
		t.Errorf("settings rail, nothing connected = %q", got)
	}
	key, _ := mix.DataKey()
	h.putConnections("mix", key, "a-one")
	got = railStates(decode[view.AccountPage](t, h.do(get("/app/settings/account", mix))).Rail)
	if len(got) != 1 || got[0] != "on: davloom · running · no devices / No device has connected since the start." {
		t.Errorf("settings rail, davloom on = %q", got)
	}

	// Administration: mail and the check.
	got = railStates(decode[view.AdminUsersPage](t, h.do(get("/admin/", mix))).Rail)
	if strings.Join(got, "|") != "alert: SMTP not configured / No mail is going out.|off: Not checked yet / Run the check once Carrel is behind its proxy." {
		t.Errorf("admin rail before = %q", got)
	}
	relay := h.relayed()
	_ = relay
	_ = h.do(fragment(postForm("/admin/settings/mail-test", mix, settingsForm(t, h, mix, map[string]string{"to": "mix@example.org"})), "main"))
	live := httptest.NewServer(h.srv.Handler())
	defer live.Close()
	_ = h.do(postForm("/admin/install", mix, url.Values{"url": {live.URL}}))
	got = railStates(decode[view.AdminUsersPage](t, h.do(get("/admin/", mix))).Rail)
	if len(got) != 2 || !regexp.MustCompile(`^on: Mail is going out / Last sent `+stamp+`$`).MatchString(got[0]) ||
		!regexp.MustCompile(`^(on: Checks passed|alert: \d+ checks? failed) / Last check `+stamp+`$`).MatchString(got[1]) {
		t.Errorf("admin rail after = %q", got)
	}
}
