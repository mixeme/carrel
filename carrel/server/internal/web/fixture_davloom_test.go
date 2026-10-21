package web_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// C-14.2: davloom as its frames draw it (Нс52) — three devices, each its
// collections and its right (s-davloom-1); Add device open (s-davloom-2);
// the password of a device just issued, shown this once (s-davloom-3).
func TestFixtureDavloomStates(t *testing.T) {
	// The settings' instance of the frames (s-davloom-1): its devices,
	// each last heard from, and the jobs of the rail's foot.
	app := newMockupApp(t)
	settingsState(t, app)
	app.compare(t, "app-settings-davloom/normal.json", app.h.do(get("/app/settings/davloom", app.sess)))
	// Add device as it is filled in (s-davloom-2): the name typed, read and
	// write, calendars, address books and folders, the collections of
	// nextcloud and sftpgo's Documents. Sent outside the unlock window it
	// is held while the password's sheet is asked (s-davloom-5), and the
	// form comes back as typed.
	typed := url.Values{"op": {"issue"}, "name": {"Телефон · DAVx5"}, "access": {"write"},
		"kind.event": {"on"}, "kind.contact": {"on"}, "kind.file": {"on"}}
	for _, slug := range []string{"lichnyj", "rabota", "kontakty", "Documents"} {
		typed.Set("col."+app.realAccount(t, slug)+"|"+app.realPath(t, slug), "on")
	}
	if rec := app.h.do(postForm("/app/settings/davloom", app.sess, typed)); rec.Code != http.StatusSeeOther {
		t.Fatalf("add: %d %s", rec.Code, rec.Body.String())
	}
	app.compare(t, "app-settings-davloom/add.json", app.h.do(get("/app/settings/davloom?add=1&held=1", app.sess)))

	// A device just made: its password is shown on the next answer, once.
	// It is made here as Create password makes it — the unlock window of a
	// real sign-in would put the wall clock in the top bar.
	key, _ := app.sess.DataKey()
	name := "Ноутбук · DAVx5"
	id, password, err := app.h.users.IssueDevice(context.Background(), app.sess.AccountID(), key, name,
		user.DeviceScope{Kinds: []view.Kind{view.KindEvent, view.KindContact}, Cols: deviceCols(t, app, "lichnyj", "rabota", "kontakty")})
	if err != nil {
		t.Fatal(err)
	}
	app.sess.SetDraft(session.DraftIssuedDevice, []byte(name+"\x00"+password+"\x00"+id))
	app.compare(t, "app-settings-davloom/issued.json", app.h.do(get("/app/settings/davloom", app.sess)))
	if err := app.h.users.RevokeDevice(app.sess.AccountID(), id); err != nil {
		t.Fatal(err)
	}

	// Typecase's note 96: what the phone sees, its sheet over davloom
	// (s-davloom-7); what a client sees at the address (s-davloom-9).
	page := decode[view.DavloomPage](t, app.h.do(get("/app/settings/davloom", app.sess)))
	app.compare(t, "app-settings-davloom-scope/normal.json", app.h.do(get(actionHref(t, page.Devices[0].Actions, "davloom.scope"), app.sess)))
	app.compare(t, "app-settings-davloom-tree/normal.json", app.h.do(get(page.Sees.Href, app.sess)))

	// davloom off on the instance (s-davloom-8): the devices stay and can be
	// revoked, none is added.
	app.h.cfg.DAVloomPort = 0
	app.h.rebuild()
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	app.h.srv.SetBuild(mockupVersion, mockupCommit)
	app.h.srv.SetPatience()
	app.compare(t, "app-settings-davloom/off.json", app.h.do(get("/app/settings/davloom", app.sess)))
}

// Typecase's note 96: no devices yet (s-davloom-6) — the rubric without a
// count, the address in its place, the rail's davloom without a count; the
// devices of the frames were revoked, and davloom's last answer to them
// stays in the rail's foot.
func TestFixtureDavloomEmpty(t *testing.T) {
	app := newMockupApp(t)
	settingsDevices(t, app)
	mockupBackups(t, app)
	for _, d := range app.h.users.Devices(app.sess.AccountID()) {
		if err := app.h.users.RevokeDevice(app.sess.AccountID(), d.ID); err != nil {
			t.Fatal(err)
		}
	}
	app.compare(t, "app-settings-davloom/empty.json", app.h.do(get("/app/settings/davloom", app.sess)))
}

// mockupDevices are the three devices of s-davloom-1, each its collections
// and its right; every frame that says «davloom · running · 3 devices» in
// the rail's foot (s-publish-1) stands on them. It answers their IDs.
func mockupDevices(t *testing.T, app *mockupApp) []string {
	t.Helper()
	key, _ := app.sess.DataKey()
	every := []string{}
	for _, c := range mockupSet {
		every = append(every, c.slug)
	}
	var ids []string
	for _, d := range []struct {
		name string
		sc   user.DeviceScope
	}{
		{"Телефон · DAVx5", user.DeviceScope{Kinds: []view.Kind{view.KindEvent, view.KindContact},
			Cols: deviceCols(t, app, "lichnyj", "rabota", "kontakty", "arhiv", "proekty")}},
		{"Ноутбук · Thunderbird", user.DeviceScope{Kinds: []view.Kind{view.KindEvent, view.KindContact, view.KindTask, view.KindNote, view.KindFile},
			Cols: deviceCols(t, app, every...), ReadOnly: true}},
		{"Планшет · Календарь iOS", user.DeviceScope{Kinds: []view.Kind{view.KindEvent},
			Cols: deviceCols(t, app, "lichnyj", "rabota", "dezhurstva"), ReadOnly: true}},
	} {
		id, _, err := app.h.users.IssueDevice(context.Background(), app.sess.AccountID(), key, d.name, d.sc)
		if err != nil {
			t.Fatalf("issue %s: %v", d.name, err)
		}
		ids = append(ids, id)
	}
	return ids
}

// deviceCols are collections of the set by their slugs, as a device sees them.
func deviceCols(t *testing.T, app *mockupApp, slugs ...string) []store.DeviceScope {
	t.Helper()
	var out []store.DeviceScope
	for _, slug := range slugs {
		out = append(out, store.DeviceScope{AccountID: app.realAccount(t, slug), Path: app.realPath(t, slug)})
	}
	return out
}
