package cal

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func parseICS(t *testing.T, ics string) *modelObject {
	t.Helper()
	obj, err := Parse("/c/x.ics", "e", strings.NewReader(strings.ReplaceAll(ics, "\n", "\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

func TestTimingReadsZoneAllDayAndDuration(t *testing.T) {
	obj := parseICS(t, "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VEVENT\nUID:z\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART;TZID=America/New_York:20260115T100000\nDURATION:PT90M\nEND:VEVENT\nEND:VCALENDAR\n")
	tm, err := TimingOf(obj, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if tm.Zone != "America/New_York" || !tm.Start.Equal(time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC)) ||
		!tm.End.Equal(time.Date(2026, 1, 15, 16, 30, 0, 0, time.UTC)) || tm.AllDay {
		t.Fatalf("timing = %+v", tm)
	}

	day := parseICS(t, "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VEVENT\nUID:d\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART;VALUE=DATE:20260120\nEND:VEVENT\nEND:VCALENDAR\n")
	berlin, _ := time.LoadLocation("Europe/Berlin")
	tm, err = TimingOf(day, berlin)
	if err != nil {
		t.Fatal(err)
	}
	if !tm.AllDay || tm.Zone != "" || !tm.Start.Equal(time.Date(2026, 1, 20, 0, 0, 0, 0, berlin)) {
		t.Fatalf("all-day timing = %+v", tm)
	}
}

// Outlook writes Windows zone names Go does not know. The record is read as a
// wall clock in the instance zone, not dropped.
func TestUnknownZoneIsReadAsWallClock(t *testing.T) {
	obj := parseICS(t, "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VEVENT\nUID:w\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART;TZID=W. Europe Standard Time:20260115T100000\nRRULE:FREQ=DAILY;COUNT=2\nEND:VEVENT\nEND:VCALENDAR\n")
	tm, err := TimingOf(obj, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if tm.Zone != "W. Europe Standard Time" || !tm.Start.Equal(time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("timing = %+v", tm)
	}
	times, err := Expand(obj, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), time.UTC)
	if err != nil || len(times) != 2 {
		t.Fatalf("expand = %v, %v", times, err)
	}
}

func TestOverridesMatchExpandInstants(t *testing.T) {
	data, err := os.ReadFile("testdata/series.ics")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := Parse("/c/s.ics", "e", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	ovs, err := Overrides(obj, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(ovs) != 1 || ovs[0].Summary != "Exception" ||
		!ovs[0].Timing.Start.Equal(time.Date(2026, 1, 22, 11, 0, 0, 0, time.UTC)) {
		t.Fatalf("overrides = %+v", ovs)
	}
	times, _ := Expand(obj, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.UTC)
	var matched bool
	for _, at := range times {
		if at.Equal(ovs[0].RecurrenceID) {
			matched = true
		}
	}
	if !matched {
		t.Errorf("RECURRENCE-ID %v is not an Expand instant of %v", ovs[0].RecurrenceID, times)
	}
}

func TestAttendeesRelatedAndText(t *testing.T) {
	obj := parseICS(t, "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VEVENT\nUID:a\nDTSTAMP:20260101T000000Z\n"+
		"SUMMARY:Lunch\\, then a walk\nDESCRIPTION:line one\\nline two\n"+
		"ATTENDEE;CN=Ada Lovelace;PARTSTAT=ACCEPTED:mailto:ada@example.com\nATTENDEE:MAILTO:bob@example.com\n"+
		"RELATED-TO:parent-uid\nEND:VEVENT\nEND:VCALENDAR\n")
	if got := Text(obj, "SUMMARY"); got != "Lunch, then a walk" {
		t.Errorf("summary = %q", got)
	}
	if got := Description(obj); got != "line one\nline two" {
		t.Errorf("description = %q", got)
	}
	if got := EscapeText("a, b\nc"); got != `a\, b\nc` {
		t.Errorf("escaped = %q", got)
	}
	att := Attendees(obj)
	if len(att) != 2 || att[0].Name != "Ada Lovelace" || att[0].Email != "ada@example.com" || att[0].Status != "ACCEPTED" || att[1].Email != "bob@example.com" {
		t.Errorf("attendees = %+v", att)
	}
	if rel := RelatedTo(obj); len(rel) != 1 || rel[0] != "parent-uid" {
		t.Errorf("related = %v", rel)
	}
}

func TestFormatDuration(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		days bool
		want string
	}{
		{time.Hour, false, "PT1H"},
		{150 * time.Minute, false, "PT2H30M"},
		{45 * time.Second, false, "PT45S"},
		{0, false, "PT0S"},
		{48 * time.Hour, true, "P2D"},
		{48 * time.Hour, false, "PT48H"},
		{36 * time.Hour, true, "PT36H"},
		{-15 * time.Minute, false, "-PT15M"},
	} {
		if got := FormatDuration(tc.d, tc.days); got != tc.want {
			t.Errorf("FormatDuration(%v, %v) = %q, want %q", tc.d, tc.days, got, tc.want)
		}
	}
}
