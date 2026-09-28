package web_test

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// s-admin-13 read part by part: the title, the labels of the fields, the
// hint under the temporary password, and the buttons of the foot.
var (
	frameSheetTitle = regexp.MustCompile(`(?s)<b class="tc-sheet__title">(.*?)</b>`)
	frameSheetHint  = regexp.MustCompile(`(?s)<span class="tc-field__hint">(.*?)</span>`)
	frameSheetFoot  = regexp.MustCompile(`(?s)<div class="tc-sheet__foot">(.*?)</div>`)
)

// from-typecase note 18, to-typecase task 16: «Reset a password» stands in
// the head of the people's list (s-admin-2) and opens a sheet (s-admin-13):
// whose, what is destroyed, the temporary password, the lossless way through
// Key escrow. The row's «⋯» has no reset. The reset opens the account with
// the password typed; the answer shows it once, and the person changes it at
// the next sign-in.
func TestResetPasswordSheetIsTheFrame(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	bobID := h.invite(adminID, "bob", "bobpass123")
	carolID := h.invite(adminID, "carol", "carolpass123")
	if err := h.users.DisableUser(adminID, carolID); err != nil {
		t.Fatal(err)
	}
	alice := h.login("alice", "password123")
	h.login("bob", "bobpass123")

	page := decode[view.AdminUsersPage](t, h.do(get("/admin/", alice)))
	head := mockupFrame(t, "s-admin-2")
	head = head[strings.Index(head, `class="tc-head"`):]
	head = head[strings.Index(head, `class="tc-acts"`):]
	var want []string
	for _, m := range frameButton.FindAllStringSubmatch(head[:strings.Index(head, `</div>`)], -1) {
		want = append(want, frameText(m[1]))
	}
	if got := labels(page.Add); !equalStrings(got, want) {
		t.Errorf("users head = %q, want (s-admin-2) %q", got, want)
	}
	for _, r := range page.Rows {
		if a := menuAction(r.Menu, "admin.user.reset"); a != nil {
			t.Errorf("%s: the row still resets: %+v", r.Login, a)
		}
	}
	sayWhy(t, "users head", page.Add...)

	frame := mockupFrame(t, "s-admin-13")
	sheet := decode[view.PasswordResetSheet](t, h.do(get(actionHref(t, page.Add, "admin.reset"), alice)))
	if title := frameText(frameSheetTitle.FindStringSubmatch(frame)[1]); string(sheet.Title) != title {
		t.Errorf("title = %q, want %q", sheet.Title, title)
	}
	var fields, frameFields []string
	for _, set := range sheet.Form.Sets {
		for _, row := range set.Rows {
			// The login typed to confirm is the owner's (28.09.2026) and not
			// drawn in s-admin-13 yet: TestResetAsksTheLoginTyped holds it.
			if row.Kind != view.InputHidden && row.Name != "confirm" {
				fields = append(fields, string(row.Label))
			}
		}
	}
	for _, m := range frameSheetLabel.FindAllStringSubmatch(frame, -1) {
		frameFields = append(frameFields, frameText(m[1]))
	}
	if !equalStrings(fields, frameFields) {
		t.Errorf("fields = %q, want (s-admin-13) %q", fields, frameFields)
	}
	temp := rowNamed(t, sheet.Form, "password")
	if len(temp.Value) < 12 || string(temp.Hint) != frameText(frameSheetHint.FindStringSubmatch(frame)[1]) {
		t.Errorf("Temporary password = %q, hint %q", temp.Value, temp.Hint)
	}
	if sheet.Warning == "" || !strings.Contains(string(sheet.Warning), "destroys") {
		t.Errorf("warning = %q: what a reset destroys is said before the button", sheet.Warning)
	}
	var foot []string
	for _, m := range frameButton.FindAllStringSubmatch(frameSheetFoot.FindStringSubmatch(frame)[1], -1) {
		foot = append(foot, frameText(m[1]))
	}
	if got := labels(sheet.Actions); !equalStrings(got, foot) {
		t.Errorf("sheet actions = %q, want %q", got, foot)
	}
	reset := actionByID(sheet.Actions, "admin.reset.run")
	if reset == nil || !reset.Danger || reset.Method != http.MethodPost {
		t.Fatalf("Reset password = %+v", reset)
	}
	sayWhy(t, "reset sheet", sheet.Actions...)

	// Whose: the people of the list by login and role word; oneself and a
	// disabled account are there, off, with the reason.
	whose := rowNamed(t, sheet.Form, "target")
	byLabel := map[string]view.Option{}
	for _, o := range whose.Options {
		byLabel[string(o.Label)] = o
	}
	if o := byLabel["bob · User"]; o.Value != bobID || o.Disabled {
		t.Errorf("bob = %+v in %+v", o, whose.Options)
	}
	for _, who := range []string{"alice · Administrator", "carol · User"} {
		if o := byLabel[who]; !o.Disabled || o.Reason == "" {
			t.Errorf("%s = %+v, want off with the reason", who, o)
		}
	}
	chosen := decode[view.PasswordResetSheet](t, h.do(get(actionHref(t, page.Add, "admin.reset")+"?target="+bobID, alice)))
	if sel := selectedOf(rowNamed(t, chosen.Form, "target").Options); !equalStrings(sel, []string{bobID}) {
		t.Errorf("?target= chooses %q", sel)
	}

	// Refused: a password too short says so at its field, and nothing moved.
	short := decode[view.PasswordResetSheet](t, h.do(postForm(sheet.Form.Action, alice, url.Values{"target": {bobID}, "password": {"short"}, "confirm": {"bob"}})))
	if rowNamed(t, short.Form, "password").Error == "" || short.Issued != nil {
		t.Errorf("a short password: %+v", short)
	}
	if id, _, _, _ := h.users.Authenticate(context.Background(), "bob", "bobpass123"); id != bobID {
		t.Fatal("a refused reset changed the password")
	}

	done := decode[view.PasswordResetSheet](t, h.do(postForm(sheet.Form.Action, alice, url.Values{"target": {bobID}, "password": {"tide-oven-4c1-lamp"}, "confirm": {"bob"}})))
	if done.Issued == nil || done.Issued.Login != "bob" || done.Issued.Password != "tide-oven-4c1-lamp" {
		t.Fatalf("issued = %+v", done.Issued)
	}
	if id, _, _, _ := h.users.Authenticate(context.Background(), "bob", "tide-oven-4c1-lamp"); id != bobID {
		t.Error("the temporary password does not open the account")
	}
	snap := h.st.Snapshot()
	if u := snap.UserByID(bobID); u == nil || !u.MustChangePassword {
		t.Errorf("bob = %+v, want a password to change", u)
	}
	if h.mgr.LiveCount(bobID) != 0 {
		t.Error("bob's sessions outlived the reset")
	}
	journaled := false
	for _, e := range snap.Audit {
		if strings.Contains(e.Detail, "tide") {
			t.Errorf("the password is in the journal: %+v", e)
		}
		journaled = journaled || (e.What == "user.reset" && e.Who == "alice" && e.Whom == "bob")
	}
	if !journaled {
		t.Errorf("no user.reset by alice for bob in %+v", snap.Audit)
	}
	// Shown once: the page opened again does not carry it.
	again := decode[view.PasswordResetSheet](t, h.do(get(actionHref(t, page.Add, "admin.reset"), alice)))
	if again.Issued != nil {
		t.Errorf("the password is shown twice: %+v", again.Issued)
	}
}

// A reset destroys what the person had (S-10.4), so the sheet asks their
// login typed, as a collection's deletion asks its name (owner's decision
// 28.09.2026): a person chosen by a slip of the list is not wiped. The frame
// s-admin-13 does not draw the field yet (to-typecase).
func TestResetAsksTheLoginTyped(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	bobID := h.invite(adminID, "bob", "bobpass123")
	alice := h.login("alice", "password123")
	sheet := decode[view.PasswordResetSheet](t, h.do(get("/admin/reset?target="+bobID, alice)))
	confirm := rowNamed(t, sheet.Form, "confirm")
	if !confirm.Required || confirm.Kind != view.InputText || confirm.Value != "" {
		t.Errorf("confirm row = %+v", confirm)
	}
	for _, typed := range []string{"", "Bob", "carol"} {
		got := decode[view.PasswordResetSheet](t, h.do(postForm(sheet.Form.Action, alice,
			url.Values{"target": {bobID}, "password": {"tide-oven-4c1-lamp"}, "confirm": {typed}})))
		if rowNamed(t, got.Form, "confirm").Error == "" || got.Issued != nil {
			t.Errorf("typed %q: %+v", typed, got)
		}
		if id, _, _, _ := h.users.Authenticate(context.Background(), "bob", "bobpass123"); id != bobID {
			t.Fatalf("typed %q reset the password", typed)
		}
	}
	done := decode[view.PasswordResetSheet](t, h.do(postForm(sheet.Form.Action, alice,
		url.Values{"target": {bobID}, "password": {"tide-oven-4c1-lamp"}, "confirm": {" bob "}})))
	if done.Issued == nil {
		t.Fatalf("the login typed did not reset: %+v", done)
	}
}
