package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// Р-57 (the owner's decision of 12.10.2026; appearance-1, attachments-1): a
// settings page that is one form has Discard beside Save — a link to the same
// page, which drops what was not saved — and Save is the page's primary
// button.
func TestASettingsPageHasDiscardBesideSave(t *testing.T) {
	f := attachFixture(t, 0, nil)
	check := func(name string, actions []view.Action, save, discard, page string) {
		t.Helper()
		if len(actions) < 2 || actions[0].ID != discard || actions[1].ID != save {
			t.Fatalf("%s: actions = %+v, want Discard, then Save", name, actions)
		}
		if d := actions[0]; d.Label != "Discard" || d.Method != "GET" || d.Href != page || !d.Enabled || d.Primary {
			t.Errorf("%s: Discard = %+v, want a link to %s", name, d, page)
		}
		if s := actions[1]; s.Label != "Save" || !s.Primary || !s.Enabled {
			t.Errorf("%s: Save = %+v, want the primary button", name, s)
		}
	}
	appearance := decode[view.AppearancePage](t, f.h.do(get("/app/settings/appearance", f.sess)))
	check("appearance", appearance.Form.Actions, "appearance.save", "appearance.discard", "/app/settings/appearance")
	attachments := decode[view.AttachmentsPage](t, f.h.do(get("/app/settings/attachments", f.sess)))
	check("attachments", attachments.Form.Actions, "attachments.save", "attachments.discard", "/app/settings/attachments")
}
