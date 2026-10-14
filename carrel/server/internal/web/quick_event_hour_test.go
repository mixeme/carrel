package web_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б31 of the audit of 11.10.2026: an event made on the quick sheet with a
// start and no end was written without an end — it lasted no time at all —,
// while the full form gives a new event an hour. The quick sheet now does
// the same; an end typed stays the end typed, and a whole day stays a day.
func TestAQuickEventWithoutAnEndIsAnHour(t *testing.T) {
	f := calendarFixture(t, nil)
	quick := f.path("calendar") + "/new/quick"
	made := func(set map[string]string) string {
		t.Helper()
		before := f.srv.Paths(f.col)
		form := formFields(decode[view.NewSheet](t, f.h.do(get(quick, f.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(quick, f.sess, form)); resp.Code != http.StatusSeeOther {
			t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
		}
		for _, p := range f.srv.Paths(f.col) {
			if !slices.Contains(before, p) {
				b, _, _ := f.srv.Object(p)
				return unfolded(b)
			}
		}
		t.Fatal("nothing was made")
		return ""
	}

	got := made(map[string]string{"summary": "Call", "start.date": "2031-03-12", "start.time": "09:30"})
	start, end := lineOf(got, "DTSTART"), lineOf(got, "DTEND")
	if !strings.Contains(start, "20310312T093000") || !strings.Contains(end, "20310312T103000") {
		t.Errorf("no end: start %q, end %q, want an hour", start, end)
	}
	got = made(map[string]string{"summary": "Call", "start.date": "2031-03-12", "start.time": "09:30", "end.time": "09:45"})
	if end := lineOf(got, "DTEND"); !strings.Contains(end, "20310312T094500") {
		t.Errorf("an end typed became %q", end)
	}
	got = made(map[string]string{"summary": "Away", "start.date": "2031-03-13", "start.allday": "on"})
	if lineOf(got, "DTSTART") != "DTSTART;VALUE=DATE:20310313" || (lineOf(got, "DTEND") != "" && !strings.Contains(lineOf(got, "DTEND"), "VALUE=DATE")) {
		t.Errorf("a whole day: %q %q", lineOf(got, "DTSTART"), lineOf(got, "DTEND"))
	}
}
