package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// s-profile-3 (Нс18–Нс20): Show password of a connection opens the sheet
// that says whose password it asks for — «Show password», the connection —,
// one line on what the window opens and for how long, and Show and Cancel.
func TestThePasswordSheetSaysWhose(t *testing.T) {
	f := calendarFixture(t, nil)
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	pw := page.Accounts[0].Password
	if pw == nil || !strings.Contains(pw.Reveal.Href, "for="+f.id) {
		t.Fatalf("Show password = %+v", pw)
	}
	sheet := decode[view.UnlockSheet](t, f.h.do(get(pw.Reveal.Href, f.sess)))
	if sheet.Purpose != "Show password" || sheet.Subject != view.Text(page.Accounts[0].Title) {
		t.Errorf("head = %q · %q, want «Show password» · %q", sheet.Purpose, sheet.Subject, page.Accounts[0].Title)
	}
	if !strings.HasPrefix(string(sheet.Warning), "Unlocks credentials everywhere for ") {
		t.Errorf("line = %q", sheet.Warning)
	}
	if a := sheet.Form.Actions; len(a) != 2 || a[0].ID != "unlock.submit" || a[1].ID != "unlock.cancel" || a[1].Href == "" {
		t.Errorf("actions = %+v", a)
	}
}
