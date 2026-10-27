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

// Typecase's Р-60 (states-36) outside the edit: a merge, an import and a
// rewrite of an address read back what they wrote, as a move does
// (move_checks_whole_record_test.go), and a day of a series the server
// dropped is said as a day of the series, not as EXDATE.

const skippedSeries = "BEGIN:VEVENT\nUID:%s\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup\nDTSTART:20260810T070000Z\n" +
	"DTEND:20260810T073000Z\nRRULE:FREQ=WEEKLY;COUNT=4\nEXDATE:20260817T070000Z\nEND:VEVENT"

func saysTheDay(t *testing.T, what, said string) {
	t.Helper()
	if !strings.Contains(said, "one day of the series") || strings.Contains(said, "EXDATE") {
		t.Errorf("%s says %q, want the day of the series and not EXDATE", what, said)
	}
}

func TestAMergeSaysTheDaysOfASeries(t *testing.T) {
	srv := searchServer(t)
	duty := davtest.CalendarHome + "duty/"
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: duty, Name: "Duty"})
	srv.Seed(searchWork+"e1.ics", ics(strings.Replace(skippedSeries, "%s", "e1", 1)))
	srv.Seed(duty+"e2.ics", ics(strings.Replace(skippedSeries, "%s", "e2", 1)))
	f := connectFixture(t, srv, searchWork, nil)
	cal := func(col string) string { return "/app/calendar/" + f.id + "/" + url.PathEscape(col) }
	for _, col := range []string{searchWork, duty} {
		f.h.do(get(cal(col)+"?from=2026-08-10", f.sess))
	}
	g := onlyGroup(t, decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess))))
	page := openMerge(t, f.h, f.sess, g, url.Values{})
	srv.Drop = []string{"EXDATE"}
	rec := f.h.do(postForm(mergeAction(t, page, "duplicates.merge.confirm").Href, f.sess, url.Values{}))
	if rec.Code != http.StatusOK {
		t.Fatalf("merge: %d, want what the server did not keep", rec.Code)
	}
	got := decode[view.DuplicatesMergePage](t, rec)
	if got.Warning.Title != "Written, but nothing was deleted" {
		t.Fatalf("warning = %+v", got.Warning)
	}
	saysTheDay(t, "the merge", string(got.Warning.Body))
}

func TestAnImportSaysTheDaysOfASeries(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	work := davtest.CalendarHome + "work/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	srv.Drop = []string{"EXDATE"}
	f := connectFixture(t, srv, work, nil)
	file := strings.ReplaceAll("BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other//Other//EN\n"+
		strings.Replace(skippedSeries, "%s", "standup", 1)+"\nEND:VCALENDAR\n", "\n", "\r\n")
	page := runImportOf(t, f, "calendar", "standup.ics", file)
	if page.Report.Lost != 1 || len(page.Report.Losts) != 1 {
		t.Fatalf("not kept = %d %q", page.Report.Lost, page.Report.Losts)
	}
	saysTheDay(t, "the report", string(page.Report.Losts[0]))
}

func TestARewriteSaysTheDaysOfASeries(t *testing.T) {
	s := newRewriteStand(t)
	s.srv.Seed(rewriteWork+"mine.ics", strings.Replace(meeting("mine", "alice@example.org", "20990105T100000Z"),
		"SUMMARY:mine", "SUMMARY:mine\r\nRRULE:FREQ=WEEKLY;COUNT=4\r\nEXDATE:20990112T100000Z", 1))
	resp := s.save(t, func(f url.Values) { f.Set("email.0", "ada@new.example") })
	sheet := decode[view.RewriteSheet](t, s.h.do(get(resp.Header.Get("Location"), s.sess)))
	s.srv.Drop = []string{"EXDATE"}
	page := decode[view.ContactPage](t, s.h.do(postForm(rewriteAction(t, sheet.Actions, "contact.rewrite.confirm").Href, s.sess, url.Values{})))
	for _, n := range page.Notices {
		if !strings.HasPrefix(string(n.Title), "Rewritten in") {
			continue
		}
		if n.Details == nil || len(n.Details.Lines) != 1 {
			t.Fatalf("outcome = %+v", n)
		}
		saysTheDay(t, "the outcome", string(n.Details.Lines[0]))
		return
	}
	t.Fatalf("notices = %+v", page.Notices)
}
