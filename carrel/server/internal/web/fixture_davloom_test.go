package web_test

import (
	"context"
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
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	key, _ := app.sess.DataKey()
	cols := func(slugs ...string) []store.DeviceScope {
		var out []store.DeviceScope
		for _, slug := range slugs {
			out = append(out, store.DeviceScope{AccountID: app.realAccount(t, slug), Path: app.realPath(t, slug)})
		}
		return out
	}
	every := []string{}
	for _, c := range mockupSet {
		every = append(every, c.slug)
	}
	for _, d := range []struct {
		name string
		sc   user.DeviceScope
	}{
		{"Телефон · DAVx5", user.DeviceScope{Kinds: []view.Kind{view.KindEvent, view.KindContact},
			Cols: cols("lichnyj", "rabota", "dni-rozhdeniya", "kontakty", "arhiv")}},
		{"Ноутбук · Thunderbird", user.DeviceScope{Kinds: []view.Kind{view.KindEvent, view.KindContact, view.KindTask, view.KindNote, view.KindFile},
			Cols: cols(every...), ReadOnly: true}},
		{"Планшет · Календарь iOS", user.DeviceScope{Kinds: []view.Kind{view.KindEvent},
			Cols: cols("lichnyj", "rabota", "dni-rozhdeniya"), ReadOnly: true}},
	} {
		if _, _, err := app.h.users.IssueDevice(context.Background(), app.sess.AccountID(), key, d.name, d.sc); err != nil {
			t.Fatalf("issue %s: %v", d.name, err)
		}
	}
	app.compare(t, "app-settings-davloom/normal.json", app.h.do(get("/app/settings/davloom", app.sess)))
	app.compare(t, "app-settings-davloom/add.json", app.h.do(get("/app/settings/davloom?add=1", app.sess)))

	// A device just made: its password is shown on the next answer, once.
	// It is made here as Create password makes it — the unlock window of a
	// real sign-in would put the wall clock in the top bar.
	name := "Ноутбук · DAVx5"
	id, password, err := app.h.users.IssueDevice(context.Background(), app.sess.AccountID(), key, name,
		user.DeviceScope{Kinds: []view.Kind{view.KindEvent, view.KindContact}, Cols: cols("lichnyj", "rabota", "kontakty")})
	if err != nil {
		t.Fatal(err)
	}
	app.sess.SetDraft(session.DraftIssuedDevice, []byte(name+"\x00"+password+"\x00"+id))
	app.compare(t, "app-settings-davloom/issued.json", app.h.do(get("/app/settings/davloom", app.sess)))
}
