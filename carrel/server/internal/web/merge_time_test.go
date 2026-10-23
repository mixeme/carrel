package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// А6 of the audit of 11.10.2026 (the owner's decision of 12.10.2026;
// from-typecase note 117 p. 3): two events that differ in time are asked
// about it once, on the group's card — «When», each time in words —; the
// merge asks nothing, writes the time chosen whole, and says whose it is.
func TestMergeAsksWhen(t *testing.T) {
	srv := searchServer(t)
	duty := davtest.CalendarHome + "duty/"
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: duty, Name: "Duty"})
	srv.Seed(searchWork+"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup\nDTSTART:20260810T070000Z\nDTEND:20260810T080000Z\nEND:VEVENT"))
	srv.Seed(duty+"e2.ics", ics("BEGIN:VEVENT\nUID:e2\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup\nDTSTART:20260810T070000Z\nDTEND:20260810T073000Z\nRRULE:FREQ=WEEKLY;BYDAY=MO\nEND:VEVENT"))
	f := connectFixture(t, srv, searchWork, nil)
	cal := func(col string) string { return "/app/calendar/" + f.id + "/" + url.PathEscape(col) }
	for _, col := range []string{searchWork, duty} {
		f.h.do(get(cal(col)+"?from=2026-08-10", f.sess))
	}
	g := onlyGroup(t, decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess))))
	var when *view.MergeRow
	for i := range g.Choices {
		if g.Choices[i].Name == "choice.WHEN" {
			when = &g.Choices[i]
		}
	}
	if when == nil || when.Property != "When" || when.Many || len(when.Options) != 2 {
		t.Fatalf("no question about the time on the card: %+v", g.Choices)
	}
	weekly := -1
	for i, o := range when.Options {
		if strings.Contains(string(o), "RRULE") || strings.Contains(string(o), "DTSTART") {
			t.Errorf("option %q is not said in words", o)
		}
		if strings.Contains(string(o), ", weekly · ") {
			weekly = i
		}
	}
	if weekly < 0 || !strings.Contains(string(when.Options[1-weekly]), "10 Aug") {
		t.Fatalf("the options do not say the day and the repetition: %q", when.Options)
	}

	// The weekly time chosen on the card: the merge asks nothing and says
	// whose time is taken — a gain of the kept record (s-dup-4, Дб15).
	page := openMerge(t, f.h, f.sess, g, url.Values{"choice.WHEN": {when.Values[weekly]}})
	if !strings.HasSuffix(string(page.Target.Stamp), "Work") {
		page = decode[view.DuplicatesMergePage](t, f.h.do(get(mergeAction(t, page, "duplicates.merge.into").Href, f.sess)))
	}
	if len(page.Rows) != 0 {
		t.Errorf("the merge asks %+v", page.Rows)
	}
	lines := ""
	for _, l := range page.Gains {
		lines += string(l) + "\n"
	}
	if !strings.Contains(lines, "the time of ") {
		t.Errorf("the page does not say whose time is taken:\n%s", lines)
	}
	confirm := mergeAction(t, page, "duplicates.merge.confirm")
	if rec := f.h.do(postForm(confirm.Href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body.String())
	}
	body, _, ok := srv.Object(searchWork + "e1.ics")
	if !ok {
		t.Fatal("the kept event is gone")
	}
	mustContain(t, unfolded(body), "UID:e1", "DTEND:20260810T073000Z", "RRULE:FREQ=WEEKLY;BYDAY=MO")
	if _, _, ok := srv.Object(duty + "e2.ics"); ok {
		t.Error("the other event was not deleted")
	}
}
