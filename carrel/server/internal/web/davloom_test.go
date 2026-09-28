package web_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// S-27.2: a device password is issued after the unlock sheet and shown
// once; it opens the same data key as the sign-in password; changing the
// sign-in password leaves it working; revoking stops it; an empty scope is
// refused; the delete sheet of a collection counts the devices that see it.
func TestDavloomDevices(t *testing.T) {
	h := newHarness(t)
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	key, _ := alice.DataKey()
	h.putConnections("alice", key, "a1")

	page := decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice)))
	if !page.Enabled || page.Address != "http://127.0.0.1:8081/" {
		t.Errorf("enabled %v, address %q", page.Enabled, page.Address)
	}
	if a := page.Add.Actions[0]; a.Method != "GET" || !strings.HasSuffix(a.Href, "/app/settings/unlock") {
		t.Fatalf("issue before unlock = %+v", a)
	}
	form := url.Values{"op": {"issue"}, "name": {"Телефон · DAVx5"}, "kind.event": {"on"}, "kind.contact": {"on"},
		"col.a1|" + seedBook: {"on"}}
	if rec := h.do(postForm("/app/settings/davloom", alice, form)); rec.Code != 200 {
		t.Fatalf("issued without the password: %d", rec.Code)
	}
	if len(h.users.Devices(alice.AccountID())) != 0 {
		t.Fatal("a device was made without the unlock sheet")
	}

	h.do(fragment(postForm("/app/settings/unlock", alice, url.Values{"password": {"password123"}}), "sheet"))
	empty := url.Values{"op": {"issue"}, "name": {"Empty"}, "kind.note": {"on"}, "col.a1|" + seedBook: {"on"}}
	page = decode[view.DavloomPage](t, h.do(postForm("/app/settings/davloom", alice, empty)))
	if len(page.Add.Errors) == 0 || len(h.users.Devices(alice.AccountID())) != 0 {
		t.Fatalf("a scope that meets nothing was taken: %+v", page.Add.Errors)
	}
	if rec := h.do(postForm("/app/settings/davloom", alice, form)); rec.Code != 303 {
		t.Fatalf("issue: %d %s", rec.Code, rec.Body)
	}
	page = decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice)))
	password := string(profileRow(page.Add, "password").Value)
	if len(password) < 40 || len(page.Devices) != 1 {
		t.Fatalf("issued = %q, devices %+v", password, page.Devices)
	}
	if again := decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice))); profileRow(again.Add, "password").Value != "" {
		t.Error("the password was shown twice")
	}
	snap := h.st.Snapshot()
	for _, e := range snap.Audit {
		if strings.Contains(e.Detail, password) {
			t.Fatal("the journal holds the device password")
		}
	}

	ctx := context.Background()
	_, deviceID, got, ok := h.users.UnlockDevice(ctx, "alice", password)
	if !ok || got != key {
		t.Fatal("the device password does not open the data key")
	}
	if _, _, _, ok := h.users.UnlockDevice(ctx, "alice", "password123"); ok {
		t.Error("the sign-in password opened a device wrapper")
	}
	if err := h.users.ChangePassword(ctx, alice.AccountID(), alice.ID(), "password123", "newpassword456", "newpassword456"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := h.users.UnlockDevice(ctx, "alice", password); !ok {
		t.Error("changing the sign-in password disconnected the device")
	}

	sheet := decode[view.CollectionDeleteSheet](t, h.do(get("/app/collections/delete?"+url.Values{"a": {"a1"}, "c": {seedBook}}.Encode(), alice)))
	if len(sheet.References) != 1 || !strings.HasPrefix(string(sheet.References[0]), "1 device through davloom — ") {
		t.Errorf("references = %v", sheet.References)
	}

	rec := h.do(postForm(actionHref(t, page.Devices[0].Actions, "davloom.revoke"), alice, url.Values{}))
	if rec.Code != 303 {
		t.Fatalf("revoke: %d", rec.Code)
	}
	if _, _, _, ok := h.users.UnlockDevice(ctx, "alice", password); ok {
		t.Error("a revoked device still opens the key")
	}
	if _, ok := h.users.DeviceOf(alice.AccountID(), deviceID); ok {
		t.Error("the revoked device is still there")
	}
}

// Owner's decision 24.09.2026 (s-davloom-1…3): a device is read only or read
// and write. The add form asks, read and write by default; the password's
// legend says what was chosen; the device's row switches it without a new
// password, the switch showing which is in force.
func TestDavloomDeviceAccess(t *testing.T) {
	h := newHarness(t)
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	key, _ := alice.DataKey()
	h.putConnections("alice", key, "a1")
	h.do(fragment(postForm("/app/settings/unlock", alice, url.Values{"password": {"password123"}}), "sheet"))

	page := decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice)))
	access := profileRow(page.Add, "access")
	if access.Kind != view.InputRadio || access.Value != "write" || len(access.Options) != 2 || access.Label != "What this device may do" {
		t.Fatalf("access row = %+v", access)
	}

	form := url.Values{"op": {"issue"}, "name": {"Phone"}, "access": {"read"}, "kind.contact": {"on"}, "col.a1|" + seedBook: {"on"}}
	if rec := h.do(postForm("/app/settings/davloom", alice, form)); rec.Code != 303 {
		t.Fatalf("issue: %d %s", rec.Code, rec.Body)
	}
	page = decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice)))
	if legend := string(page.Add.Sets[0].Legend); legend != "Phone · Password created · read only · 1 collection" {
		t.Errorf("legend = %q", legend)
	}
	devices := h.users.Devices(alice.AccountID())
	if len(devices) != 1 || !devices[0].ReadOnly || !page.Devices[0].ReadOnly {
		t.Fatalf("devices = %+v, rows %+v", devices, page.Devices)
	}
	if a := actionByID(page.Devices[0].Actions, "davloom.access.read"); a == nil || !a.On {
		t.Errorf("read only is not shown in force: %+v", page.Devices[0].Actions)
	}

	rec := h.do(postForm(actionHref(t, page.Devices[0].Actions, "davloom.access.write"), alice, url.Values{}))
	if rec.Code != 303 {
		t.Fatalf("switch: %d", rec.Code)
	}
	page = decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice)))
	if page.Devices[0].ReadOnly || !actionByID(page.Devices[0].Actions, "davloom.access.write").On {
		t.Errorf("after the switch: %+v", page.Devices[0])
	}
	if d := h.users.Devices(alice.AccountID())[0]; d.ReadOnly || string(d.WrappedKey) != string(devices[0].WrappedKey) {
		t.Error("the switch did not take, or it made a new password")
	}
}
