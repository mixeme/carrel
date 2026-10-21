package web_test

import (
	"net/url"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// C-14.2: a new device as its frames draw it (Нс43) — every tab inside the
// unlock window, 4:12 left. Apple with radicale left out and the link
// issued (s-plan-1), Android with a code for each ticked account (s-plan-2),
// Thunderbird (s-plan-4) and anything else (s-plan-5) with the passwords
// in the account rows.
func TestFixtureDevicesStates(t *testing.T) {
	app := newMockupApp(t)
	// Over the instance of the settings' frames (the rail's foot).
	settingsState(t, app)
	// The calendars read once: baikal refused the saved password, and its
	// row goes in nowhere (s-plan-1, -2, -4; Typecase's note 94).
	combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	page := decode[view.DevicesPage](t, app.h.do(get("/app/settings/devices?tab=apple", app.sess)))
	left := false
	for _, a := range page.Accounts {
		if a.Label == "radicale · vps" && a.Toggle != nil {
			app.h.do(postForm(a.Toggle.Href, app.sess, url.Values{}))
			left = true
		}
	}
	if !left {
		t.Fatalf("no tick on radicale: %+v", page.Accounts)
	}
	// The window closed (s-plan-6, note 93): radicale left out, Generate
	// profile asks for the password first, no link is issued yet.
	app.compare(t, "app-settings-devices/locked.json", app.h.do(get("/app/settings/devices?tab=apple", app.sess)))
	now := mockupNow(t).Add(mockupSince)
	app.sess.SetUnlockUntil(now.Add(4*time.Minute + 12*time.Second))
	app.h.do(postForm("/app/settings/devices?tab=apple&op=profile", app.sess, url.Values{}))
	app.compare(t, "app-settings-devices/link.json", app.h.do(get("/app/settings/devices?tab=apple", app.sess)))
	// The page's own frame is this state (s-plan-1); the one before the
	// link has none (Typecase's task 193 p. 5).
	app.compare(t, "app-settings-devices/normal.json", app.h.do(get("/app/settings/devices", app.sess)))

	// The other tabs with radicale ticked again, as s-plan-2 draws it.
	app.h.do(postForm("/app/settings/devices?tab=android&op=include&account="+app.realAccount(t, "proekty"), app.sess, url.Values{}))
	app.compare(t, "app-settings-devices/android.json", app.h.do(get("/app/settings/devices?tab=android", app.sess)))
	app.compare(t, "app-settings-devices/thunderbird.json", app.h.do(get("/app/settings/devices?tab=thunderbird", app.sess)))
	app.compare(t, "app-settings-devices/other.json", app.h.do(get("/app/settings/devices?tab=other", app.sess)))
}
