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

// А6 of the audit of 11.10.2026 (the owner's decision of 12.10.2026): two
// events that differ in time are merged with a question, «When», whose
// options say each time in words; the answer is written whole, and the page
// says what goes with it.
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
	page := decode[view.DuplicatesMergePage](t, f.h.do(get(actionOf(t, g, "duplicates.merge").Href, f.sess)))
	if !strings.HasSuffix(string(page.Target.Stamp), "Work") {
		page = decode[view.DuplicatesMergePage](t, f.h.do(get(mergeAction(t, page, "duplicates.merge.into").Href, f.sess)))
	}
	var when *view.MergeRow
	for i := range page.Rows {
		if page.Rows[i].Name == "choice.WHEN" {
			when = &page.Rows[i]
		}
	}
	if when == nil || when.Property != "When" || when.Union || len(when.Options) != 2 || when.Chosen != 0 {
		t.Fatalf("no question about the time: %+v", page.Rows)
	}
	for i, want := range []string{"10 August 2026", "weekly"} {
		if !strings.Contains(strings.ToLower(string(when.Options[i])), strings.ToLower(want)) {
			t.Errorf("option %d = %q, want it to say %q", i, when.Options[i], want)
		}
	}
	if when.Options[0] == when.Options[1] || strings.Contains(string(when.Options[1]), "RRULE") || strings.Contains(string(when.Options[1]), "DTSTART") {
		t.Errorf("the options are not said in words: %q", when.Options)
	}

	// The other's time chosen: the page says so, and the merge writes it.
	confirm := mergeAction(t, page, "duplicates.merge.confirm")
	u, _ := url.Parse(confirm.Href)
	q := u.Query()
	q.Set("choice.WHEN", "1")
	chosen := decode[view.DuplicatesMergePage](t, f.h.do(get("/app/duplicates/merge?"+url.Values{"group": q["group"], "into": q["into"], "choice.WHEN": {"1"}}.Encode(), f.sess)))
	lines := ""
	if chosen.Warning.Details != nil {
		for _, l := range chosen.Warning.Details.Lines {
			lines += string(l) + "\n"
		}
	}
	if !strings.Contains(lines, "takes the time of") {
		t.Errorf("the page does not say whose time is taken:\n%s", lines)
	}
	confirm = mergeAction(t, chosen, "duplicates.merge.confirm")
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
