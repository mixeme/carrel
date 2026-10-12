package cal

import (
	"testing"
	"time"
)

// The start of a series is always its first occurrence; the rest go by the
// rule (the owner's decision of 12.10.2026, Б25 of the audit of 11.10.2026).
// A weekly series on Mondays given a start on a Tuesday is that Tuesday and
// then the Mondays — RFC 5545 leaves a start outside its rule undefined, and
// clients differ; Carrel shows what it writes.
func TestTheStartIsTheFirstOccurrence(t *testing.T) {
	series := func(extra string) string {
		return "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VEVENT\nUID:w\nDTSTAMP:20260101T000000Z\n" +
			"DTSTART:20260113T100000Z\nRRULE:FREQ=WEEKLY;BYDAY=MO" + extra + "\nEND:VEVENT\nEND:VCALENDAR\n"
	}
	from, to := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	days := func(doc string) []int {
		t.Helper()
		times, err := Expand(parseICS(t, doc), from, to, time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		var out []int
		for _, at := range times {
			out = append(out, at.Day())
		}
		return out
	}
	equal := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	if got := days(series("")); !equal(got, []int{13, 19, 26}) {
		t.Errorf("a start outside the rule: days %v, want Tuesday 13 and then the Mondays 19 and 26", got)
	}
	// A start the rule names stands once.
	on := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VEVENT\nUID:w\nDTSTAMP:20260101T000000Z\n" +
		"DTSTART:20260112T100000Z\nRRULE:FREQ=WEEKLY;BYDAY=MO\nEND:VEVENT\nEND:VCALENDAR\n"
	if got := days(on); !equal(got, []int{12, 19, 26}) {
		t.Errorf("a start the rule names: days %v, want 12, 19, 26", got)
	}
	// The first occurrence left out is left out.
	if got := days(series("\nEXDATE:20260113T100000Z")); !equal(got, []int{19, 26}) {
		t.Errorf("the start left out: days %v, want 19, 26", got)
	}
	cur, next, ok, err := NextTime(parseICS(t, series("")), time.UTC)
	if err != nil || !ok || cur.Day() != 13 || next.Day() != 19 {
		t.Errorf("the time after the start: %v then %v (%v, %v), want 13 then 19", cur, next, ok, err)
	}
}
