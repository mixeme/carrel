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
	now := mockupNow(t).Add(mockupSince)
	app.h.srv.SetClock(now)
	app.sess.SetUnlockUntil(now.Add(4*time.Minute + 12*time.Second))

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
	app.h.do(postForm("/app/settings/devices?tab=apple&op=profile", app.sess, url.Values{}))
	app.compare(t, "app-settings-devices/link.json", app.h.do(get("/app/settings/devices?tab=apple", app.sess)))

	// The other tabs with radicale ticked again, as s-plan-2 draws it.
	app.h.do(postForm("/app/settings/devices?tab=android&op=include&account="+app.realAccount(t, "proekty"), app.sess, url.Values{}))
	app.compare(t, "app-settings-devices/android.json", app.h.do(get("/app/settings/devices?tab=android", app.sess)))
	app.compare(t, "app-settings-devices/thunderbird.json", app.h.do(get("/app/settings/devices?tab=thunderbird", app.sess)))
	app.compare(t, "app-settings-devices/other.json", app.h.do(get("/app/settings/devices?tab=other", app.sess)))
}
