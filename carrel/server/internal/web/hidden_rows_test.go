package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// A value the form carries and nobody fills in — the token, the version the
// form was read at, the step a form stands for — is a hidden row (C-8,
// InputHidden). As a text row with no label the client had no way to tell it
// from a field, and the edit form showed "Version" and "Read at" with a Unix
// time in it (C-3.1: every value arrives ready to show).
func TestCarriedValuesAreHiddenRows(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	edit := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/ada/edit", f.sess)))
	carried := map[string]bool{"_csrf": true, "etag": true, "readat": true}
	seen := 0
	for _, set := range edit.Form.Sets {
		for _, row := range set.Rows {
			if !carried[row.Name] {
				continue
			}
			seen++
			if row.Kind != view.InputHidden || row.Label != "" {
				t.Errorf("%s: kind %q, label %q — want a hidden row", row.Name, row.Kind, row.Label)
			}
		}
	}
	if seen != len(carried) {
		t.Errorf("found %d of the carried rows", seen)
	}
}
