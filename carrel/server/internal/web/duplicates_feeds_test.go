package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Дб3 (s-dup-1: «Праздники РФ» in the rail, is-ext): a feed read in this
// session is a source the duplicates compare. Its records can be linked to
// another's, never merged — a feed is read, not written —, and its row in
// the rail says it is a feed.
func TestAFeedIsASourceOfDuplicates(t *testing.T) {
	f := feedHarness(t)
	f.srv.Seed(searchWork+"knowledge.ics", ics("BEGIN:VEVENT\nUID:mine\nDTSTAMP:20260801T000000Z\nSUMMARY:Day of Knowledge\nDTSTART;VALUE=DATE:20260901\nEND:VEVENT"))
	feed := newFeedServer(t, holidays, "")
	sub := subscribe(t, f, url.Values{"url": {feed.URL + "/ru.ics"}})
	tickRail(t, f.h, f.sess, "calendar", "/app/calendar/"+f.id+"/"+url.PathEscape(searchWork), "/app/calendar/"+sub.ID+"/feed")
	combined(t, f.h, f.sess, "/app/calendar?from=2026-08-31", func(p view.AgendaPage) *view.PollBar { return p.Poll })

	page := decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess)))
	g := onlyGroup(t, page)
	if g.Kind != view.KindEvent || len(g.Rows) != 2 {
		t.Fatalf("group = %+v", g)
	}
	if merge := actionOf(t, g, "duplicates.merge"); merge.Enabled {
		t.Errorf("a group with a feed's record can be merged: %+v", merge)
	}
	if link := actionOf(t, g, "duplicates.link"); !link.Enabled {
		t.Errorf("Link = %+v", link)
	}
	feedRow := false
	for _, sec := range page.Rail.Sections {
		for _, r := range sec.Rows {
			feedRow = feedRow || r.Feed
		}
	}
	if !feedRow {
		t.Errorf("no feed in the rail: %+v", page.Rail.Sections)
	}
}
