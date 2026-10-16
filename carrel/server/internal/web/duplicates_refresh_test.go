package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// Дб11 of the scan of duplicates (s-dup-1, -2, -6): the top bar has its
// circle and «2 min ago». The screen compares only what the session has
// read, so the words are how fresh the records compared are, and the circle
// compares them again; with nothing read, it is off and says why.
func TestDuplicatesHaveTheirRefresh(t *testing.T) {
	f := newDupFixture(t)
	before := decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess)))
	if r := before.Top.Refresh; r.Action.Enabled || r.Action.Reason == "" {
		t.Errorf("nothing read: %+v", r)
	}
	tickRail(t, f.h, f.sess, "contacts", f.book(f.id, peopleBook), f.book(f.otherID, workBook))
	combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	after := decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess)))
	if r := after.Top.Refresh; !r.Action.Enabled || r.Action.Href != "/app/duplicates" || r.Answered == "" {
		t.Errorf("after a list was read: %+v", r)
	}
}
