package web_test

import (
	"context"
	"net/url"
	"slices"
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
	// The list has no form: Add device in the head opens the dialog, a state
	// of the page (s-davloom-1, -2; Нс47).
	if page.Dialog != nil {
		t.Fatalf("the list carries a dialog: %+v", page.Dialog)
	}
	add := actionByID(page.Actions, "davloom.add")
	if add == nil || !add.Primary || !add.Enabled || add.Method != "GET" {
		t.Fatalf("Add device = %+v", add)
	}
	page = decode[view.DavloomPage](t, h.do(get(add.Href, alice)))
	if page.Dialog == nil || page.Dialog.Title != "Add device" || page.Dialog.Warning == "" {
		t.Fatalf("the dialog of Add device = %+v", page.Dialog)
	}
	// Create password sends the dialog; without the window the answer is
	// the password's sheet over it, what was typed held (s-davloom-5,
	// Typecase's note 86), and nothing is made yet.
	if a := page.Dialog.Form.Actions[0]; a.Method != "POST" || a.Href != "" {
		t.Fatalf("issue before unlock = %+v", a)
	}
	form := url.Values{"op": {"issue"}, "name": {"Телефон · DAVx5"}, "kind.event": {"on"}, "kind.contact": {"on"},
		"col.a1|" + seedBook: {"on"}}
	rec := h.do(postForm("/app/settings/davloom", alice, form))
	if rec.Code != 303 || !strings.Contains(rec.Header().Get("Location"), "/app/settings/unlock?") || len(h.users.Devices(alice.AccountID())) != 0 {
		t.Fatalf("Create password without the window: %d %s, devices %d", rec.Code, rec.Header().Get("Location"), len(h.users.Devices(alice.AccountID())))
	}
	sheet := decode[view.UnlockSheet](t, h.do(get(rec.Header().Get("Location"), alice)))
	if sheet.Purpose != "Add device" || sheet.Subject != "Телефон · DAVx5" || sheet.Form.Actions[0].Label != "Create password" {
		t.Fatalf("the sheet = %+v", sheet)
	}
	// Cancel is the dialog as it was typed.
	held := decode[view.DavloomPage](t, h.do(get(sheet.Form.Actions[1].Href, alice)))
	if held.Dialog == nil || formRow(held.Dialog.Form, "name").Value != "Телефон · DAVx5" ||
		formRow(held.Dialog.Form, "kind.note").Value != "" || formRow(held.Dialog.Form, "kind.event").Value == "" {
		t.Fatalf("Cancel lost what was typed: %+v", held.Dialog)
	}
	unlock := url.Values{"for": {"davloom"}, "back": {"/app/settings/davloom?add=1&held=1"}, "password": {"nope"}}
	if rec := h.do(postForm("/app/settings/unlock", alice, unlock)); rec.Code != 200 || len(h.users.Devices(alice.AccountID())) != 0 {
		t.Fatalf("a wrong password: %d", rec.Code)
	}
	unlock.Set("password", "password123")
	if rec := h.do(postForm("/app/settings/unlock", alice, unlock)); rec.Code != 303 || rec.Header().Get("Location") != "/app/settings/davloom" {
		t.Fatalf("the sheet's Create password: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	page = decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice)))
	if page.Dialog == nil || page.Dialog.Password == nil || page.Dialog.Title != "Телефон · DAVx5" {
		t.Fatalf("the password issued = %+v", page.Dialog)
	}
	password := string(page.Dialog.Password.Value)
	if len(password) < 40 || len(page.Devices) != 1 {
		t.Fatalf("issued = %q, devices %+v", password, page.Devices)
	}
	if again := decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice))); again.Dialog != nil {
		t.Error("the password was shown twice")
	}
	// Inside the window a scope that meets nothing is refused in the dialog.
	empty := url.Values{"op": {"issue"}, "name": {"Empty"}, "kind.note": {"on"}, "col.a1|" + seedBook: {"on"}}
	page = decode[view.DavloomPage](t, h.do(postForm("/app/settings/davloom", alice, empty)))
	if page.Dialog == nil || len(page.Dialog.Form.Errors) == 0 || len(h.users.Devices(alice.AccountID())) != 1 {
		t.Fatalf("a scope that meets nothing was taken: %+v", page.Dialog)
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

	del := decode[view.CollectionDeleteSheet](t, h.do(get("/app/collections/delete?"+url.Values{"a": {"a1"}, "c": {seedBook}}.Encode(), alice)))
	if len(del.References) != 1 || del.References[0].What != "1 device through davloom" {
		t.Errorf("references = %v", del.References)
	}

	rec = h.do(postForm(actionHref(t, page.Devices[0].Actions, "davloom.revoke"), alice, url.Values{}))
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

	page := decode[view.DavloomPage](t, h.do(get("/app/settings/davloom?add=1", alice)))
	access := profileRow(page.Dialog.Form, "access")
	if access.Kind != view.InputRadio || access.Value != "write" || len(access.Options) != 2 || access.Label != "What this device may do" {
		t.Fatalf("access row = %+v", access)
	}

	form := url.Values{"op": {"issue"}, "name": {"Phone"}, "access": {"read"}, "kind.contact": {"on"}, "col.a1|" + seedBook: {"on"}}
	if rec := h.do(postForm("/app/settings/davloom", alice, form)); rec.Code != 303 {
		t.Fatalf("issue: %d %s", rec.Code, rec.Body)
	}
	page = decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice)))
	if page.Dialog == nil || page.Dialog.Title != "Phone" || page.Dialog.Subtitle != "Password created · read only · 1 collection" {
		t.Errorf("the password's dialog = %+v", page.Dialog)
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

// Нс73: the person's password typed in the dialog itself opens the window
// and the device's password is made at once — one step, nothing typed lost.
func TestDavloomPasswordInTheDialog(t *testing.T) {
	h := newHarness(t)
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	key, _ := alice.DataKey()
	h.putConnections("alice", key, "a1")
	// The dialog asks no password of its own any more: the sheet does, and
	// makes the device from what was typed (s-davloom-5).
	form := url.Values{"op": {"issue"}, "name": {"Ноутбук"}, "kind.contact": {"on"}, "col.a1|" + seedBook: {"on"}, "password": {"password123"}}
	if rec := h.do(postForm("/app/settings/davloom", alice, form)); rec.Code != 303 || len(h.users.Devices(alice.AccountID())) != 0 {
		t.Fatalf("issue with a password in the dialog: %d", rec.Code)
	}
	unlock := url.Values{"for": {"davloom"}, "back": {"/app/settings/davloom?add=1&held=1"}, "password": {"password123"}}
	if rec := h.do(postForm("/app/settings/unlock", alice, unlock)); rec.Code != 303 {
		t.Fatalf("the sheet: %d %s", rec.Code, rec.Body)
	}
	if d := h.users.Devices(alice.AccountID()); len(d) != 1 || d[0].Name != "Ноутбук" {
		t.Errorf("devices = %+v", d)
	}
}

// Нс50, Нс51 (s-davloom-1): a device's row carries the marks of what it
// sees, each colour once; «What the client sees» is the synthetic levels of
// davloom and the collections under them, by the address a client is given.
func TestDavloomShowsWhatTheClientSees(t *testing.T) {
	h := newHarness(t)
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	key, _ := alice.DataKey()
	h.putConnections("alice", key, "a1")
	h.do(fragment(postForm("/app/settings/unlock", alice, url.Values{"password": {"password123"}}), "sheet"))
	form := url.Values{"op": {"issue"}, "name": {"Phone"}, "kind.contact": {"on"}, "col.a1|" + seedBook: {"on"}}
	if rec := h.do(postForm("/app/settings/davloom", alice, form)); rec.Code != 303 {
		t.Fatalf("issue: %d %s", rec.Code, rec.Body)
	}
	page := decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice)))
	if len(page.Devices) != 1 || len(page.Devices[0].Marks) != 1 {
		t.Errorf("the device's marks = %+v", page.Devices)
	}
	// The tree is a sheet behind «What the client sees» (s-davloom-9).
	if page.Sees.Href != "/app/settings/davloom/tree" {
		t.Errorf("sees = %+v", page.Sees)
	}
	tree := decode[view.DavloomTreeSheet](t, h.do(get(page.Sees.Href, alice)))
	var paths []string
	for _, r := range tree.Tree {
		paths = append(paths, string(r.Path))
	}
	line := strings.Join(paths, " ")
	for _, want := range []string{"/dav/", "principal/", "cal/", "card/", "files/", "c/a1/" + url.PathEscape(seedBook) + "/"} {
		if !strings.Contains(line, want) {
			t.Errorf("the tree has no %q: %s", want, line)
		}
	}
}

// Typecase's note 96 (s-davloom-7): what a device sees is changed in its
// sheet without a new password — the collections by connection under the
// home sets it sees, the ticked ones its own —; none ticked is refused in
// the sheet.
func TestDavloomScopeSheet(t *testing.T) {
	h := newHarness(t)
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	key, _ := alice.DataKey()
	h.putConnections("alice", key, "a1")
	h.do(fragment(postForm("/app/settings/unlock", alice, url.Values{"password": {"password123"}}), "sheet"))
	form := url.Values{"op": {"issue"}, "name": {"Phone"}, "kind.contact": {"on"}, "kind.file": {"on"}, "col.a1|" + seedBook: {"on"}}
	if rec := h.do(postForm("/app/settings/davloom", alice, form)); rec.Code != 303 {
		t.Fatalf("issue: %d %s", rec.Code, rec.Body)
	}
	page := decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice)))
	change := actionByID(page.Devices[0].Actions, "davloom.scope")
	if change == nil || !strings.HasPrefix(change.Href, "/app/settings/davloom/scope?device=") {
		t.Fatalf("change = %+v", change)
	}
	sheet := decode[view.DavloomScopeSheet](t, h.do(get(change.Href, alice)))
	row := sheet.Form.Sets[len(sheet.Form.Sets)-1].Rows[0]
	if sheet.Subject != "Phone" || row.Name != "col" || len(row.Options) < 2 || row.Options[0].Group == "" {
		t.Fatalf("sheet = %+v", sheet)
	}
	ticked := 0
	for _, o := range row.Options {
		if o.Selected {
			ticked++
		}
	}
	if ticked != 1 {
		t.Errorf("ticked = %d, want the device's one book", ticked)
	}
	if rec := h.do(postForm(change.Href, alice, url.Values{})); rec.Code != 200 ||
		len(decode[view.DavloomScopeSheet](t, rec).Form.Errors) == 0 {
		t.Fatalf("none ticked was taken: %d", rec.Code)
	}
	if rec := h.do(postForm(change.Href, alice, url.Values{"col": {"a1|" + seedBook, "a1|" + seedFiles}})); rec.Code != 303 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	d := h.users.Devices(alice.AccountID())[0]
	if len(d.ScopeCols) != 2 || !slices.Contains(d.ScopeKinds, string(view.KindContact)) {
		t.Errorf("scope = %+v %+v", d.ScopeKinds, d.ScopeCols)
	}
}
