package web_test

import (
	"regexp"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Acceptance round 7 (24.11.2026): every settings frame draws the top bar's
// Refresh as the time of the last answer — «2 min ago» (account-1,
// s-backup-1, s-davloom-1) —, and Carrel drew «Refresh». It is when the
// session last read a server: its newest reading, as the files' root says
// it; nothing read yet, the word.
func TestSettingsRefreshSaysWhenServersLastAnswered(t *testing.T) {
	if frame := mockupFrame(t, "account-1"); !regexp.MustCompile(`\d+ min ago`).MatchString(frame) {
		t.Fatal("account-1 draws no time in the top bar: read the frame again")
	}
	f := calendarFixture(t, nil)
	cold := decode[view.AccountPage](t, f.h.do(get("/app/settings/account", f.sess)))
	if cold.Top.Refresh.Answered != "" || !cold.Top.Refresh.Action.Enabled {
		t.Errorf("nothing read: refresh %+v", cold.Top.Refresh)
	}
	tickRail(t, f.h, f.sess, "calendar", f.path("calendar"))
	combined(t, f.h, f.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	warm := decode[view.AccountPage](t, f.h.do(get("/app/settings/account", f.sess)))
	if warm.Top.Refresh.Answered == "" {
		t.Errorf("after a read the settings' refresh says no time: %+v", warm.Top.Refresh)
	}
}
