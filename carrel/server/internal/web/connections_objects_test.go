package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// s-profile-1 (Нс22): «Objects» of a collection is how many records it held
// when this session last read it; before any read, a dash.
func TestACollectionRowCountsItsObjects(t *testing.T) {
	f := calendarFixture(t, nil,
		"a.ics", ics("BEGIN:VEVENT\nUID:a\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T090000Z\nSUMMARY:A\nEND:VEVENT"),
		"b.ics", ics("BEGIN:VEVENT\nUID:b\nDTSTAMP:20260101T000000Z\nDTSTART:20260811T090000Z\nSUMMARY:B\nEND:VEVENT"))
	row := func() view.CollectionRow {
		t.Helper()
		p := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
		return p.Accounts[0].Collections[0]
	}
	if r := row(); r.Objects != nil {
		t.Errorf("nothing read, objects = %d", *r.Objects)
	}
	tickRail(t, f.h, f.sess, "calendar", f.path("calendar"))
	combined(t, f.h, f.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	if r := row(); r.Objects == nil || *r.Objects != 2 {
		t.Errorf("read, objects = %v", r.Objects)
	}
}
