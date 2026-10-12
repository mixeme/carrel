package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// S-11, the table «an edit of the time → what is written» (Б21–Б28 of the
// audit of 11.10.2026, docs/plan/scans/silent-choices.md). A series is more
// than its start: the days left out, the days added, the day it ends on are
// all written by the start's clock. Move the start and each of them goes
// with it, or the save is not the edit the person made.

const shiftZone = "BEGIN:VTIMEZONE\nTZID:Europe/Berlin\nBEGIN:STANDARD\nDTSTART:19701025T030000\nTZOFFSETFROM:+0200\nTZOFFSETTO:+0100\n" +
	"RRULE:FREQ=YEARLY;BYMONTH=10;BYDAY=-1SU\nEND:STANDARD\nBEGIN:DAYLIGHT\nDTSTART:19700329T020000\nTZOFFSETFROM:+0100\nTZOFFSETTO:+0200\n" +
	"RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU\nEND:DAYLIGHT\nEND:VTIMEZONE\n"

// shiftSeries is a weekly series as another client writes it: UNTIL is the
// start of its last occurrence, one day is left out and one is added.
func shiftSeries() string {
	return ics(shiftZone + "BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Berlin:20260112T100000\n" +
		"DTEND;TZID=Europe/Berlin:20260112T110000\nSUMMARY:Sync\nRRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20260302T090000Z\n" +
		"RDATE;TZID=Europe/Berlin:20260114T100000\nEXDATE;TZID=Europe/Berlin:20260119T100000\nEND:VEVENT")
}

func shiftAllDay() string {
	return ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART;VALUE=DATE:20260112\nDTEND;VALUE=DATE:20260113\n" +
		"SUMMARY:Day\nRRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20260302\nRDATE;VALUE=DATE:20260114\nEND:VEVENT")
}

// shiftEdit opens the form of the event, sets what set names (an empty
// value takes the field out, as an unticked box is not sent), saves, and
// answers with the reply and the record after it.
func shiftEdit(t *testing.T, f davFixture, set url.Values) (int, editedForm, string) {
	t.Helper()
	rec := f.path("calendar") + "/e"
	form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	for k, v := range set {
		if len(v) == 1 && v[0] == "" {
			delete(form, k)
			continue
		}
		form[k] = v
	}
	resp := f.h.do(postForm(rec+"/edit", f.sess, form))
	var page editedForm
	if resp.Code == http.StatusOK {
		page = decode[editedForm](t, resp)
	}
	return resp.Code, page, unfolded(body(t, f, "e.ics"))
}

func TestASeriesGoesWithItsStart(t *testing.T) {
	cases := []struct {
		name  string
		doc   string
		set   url.Values
		holds []string
		until string // the last day the form shows afterwards
	}{
		// The last day stays the last day: UNTIL was the start of the last
		// occurrence, and an hour later that occurrence would be past it.
		{"an hour later", shiftSeries(), url.Values{"start.time": {"11:00"}, "end.time": {"12:00"}},
			[]string{"DTSTART;TZID=Europe/Berlin:20260112T110000", "UNTIL=20260302T225959Z",
				"RDATE;TZID=Europe/Berlin:20260114T110000", "EXDATE;TZID=Europe/Berlin:20260119T110000"}, "2026-03-02"},
		// Earlier, the day is reached all the same, and UNTIL is not touched.
		{"an hour earlier", shiftSeries(), url.Values{"start.time": {"09:00"}, "end.time": {"10:00"}},
			[]string{"DTSTART;TZID=Europe/Berlin:20260112T090000", "UNTIL=20260302T090000Z",
				"RDATE;TZID=Europe/Berlin:20260114T090000"}, "2026-03-02"},
		{"another zone", shiftSeries(), url.Values{"tz": {"UTC"}},
			[]string{"DTSTART:20260112T100000Z", "UNTIL=20260302T235959Z", "RDATE:20260114T100000Z", "EXDATE:20260119T100000Z"}, "2026-03-02"},
		// RFC 5545 §3.3.10: UNTIL is a date when the start is, a UTC time
		// when the start is a time.
		{"a whole day now", shiftSeries(), url.Values{"start.allday": {"on"}},
			[]string{"DTSTART;VALUE=DATE:20260112", "UNTIL=20260302;", "RDATE;VALUE=DATE:20260114", "EXDATE;VALUE=DATE:20260119"}, "2026-03-02"},
		{"a time now", shiftAllDay(), url.Values{"start.allday": {""}, "start.time": {"10:00"}, "end.date": {"2026-01-12"}, "end.time": {"11:00"}},
			[]string{"UNTIL=20260302T235959Z", "T100000"}, "2026-03-02"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := calendarFixture(t, nil, "e.ics", tc.doc)
			code, page, after := shiftEdit(t, f, tc.set)
			if code != http.StatusSeeOther {
				t.Fatalf("save: %d %+v\n%s", code, page.Notices, after)
			}
			// UNTIL may stand last in its rule or not.
			mustContain(t, strings.ReplaceAll(after, "\r\n", ";\r\n"), tc.holds...)
			form := formFields(decode[editedForm](t, f.h.do(get(f.path("calendar")+"/e/edit", f.sess))).Form)
			if got := form.Get("repeat.until"); got != tc.until {
				t.Errorf("the form shows the last day %q, want %q\n%s", got, tc.until, after)
			}
		})
	}
}

// A start moved past the last day leaves a series that ends before it
// begins: the form says so, and nothing is written.
func TestASeriesDoesNotStartAfterItsLastDay(t *testing.T) {
	f := calendarFixture(t, nil, "e.ics", shiftSeries())
	before := body(t, f, "e.ics")
	code, page, _ := shiftEdit(t, f, url.Values{"start.date": {"2026-03-09"}, "end.date": {"2026-03-09"}})
	if code != http.StatusOK {
		t.Fatalf("save: %d, want the form back", code)
	}
	var said view.Text
	for _, set := range page.Form.Sets {
		for _, row := range set.Rows {
			if row.Error != "" {
				said = row.Error
			}
		}
	}
	if !strings.Contains(string(said), "last day") {
		t.Errorf("the form's error = %q, want it to speak of the last day", said)
	}
	if after := body(t, f, "e.ics"); after != before {
		t.Errorf("the record was written:\n%s", after)
	}
}

// A reminder whose time is changed stays the reminder it was: its action,
// its sound and its text are not the form's to replace (Б28).
func TestAReminderMovedKeepsWhatItDoes(t *testing.T) {
	f := calendarFixture(t, nil, "e.ics", ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART:20260112T100000Z\nDTEND:20260112T110000Z\nSUMMARY:Sync\n"+
		"BEGIN:VALARM\nACTION:AUDIO\nTRIGGER:-PT15M\nATTACH;VALUE=URI:Basso\nX-WR-ALARMUID:keep-me\nEND:VALARM\n"+
		"BEGIN:VALARM\nACTION:EMAIL\nTRIGGER;VALUE=DATE-TIME:20260112T080000Z\nSUMMARY:Mail\nDESCRIPTION:Text\nATTENDEE:mailto:a@example.com\nEND:VALARM\nEND:VEVENT"))
	code, page, after := shiftEdit(t, f, url.Values{"reminder.0": {"-PT30M"}})
	if code != http.StatusSeeOther {
		t.Fatalf("save: %d %+v", code, page.Notices)
	}
	mustContain(t, after, "ACTION:AUDIO", "TRIGGER:-PT30M", "ATTACH;VALUE=URI:Basso", "X-WR-ALARMUID:keep-me",
		"ACTION:EMAIL", "TRIGGER;VALUE=DATE-TIME:20260112T080000Z", "ATTENDEE:mailto:a@example.com")
	if strings.Contains(after, "ACTION:DISPLAY") || strings.Contains(after, "TRIGGER:-PT15M") {
		t.Errorf("the reminder was replaced by a plain one:\n%s", after)
	}
}

// A whole day given a time is written as a new event is (S-11): in the zone
// of the instance, by name — not as an instant in UTC, which a series would
// then keep through the change of the clocks, an hour off for half a year
// (Б21).
func TestAWholeDayGivenATimeIsWrittenInAZone(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.Timezone = "Europe/Berlin" }, "e.ics", shiftAllDay())
	code, page, after := shiftEdit(t, f, url.Values{"start.allday": {""}, "start.time": {"10:00"}, "end.date": {"2026-01-12"}, "end.time": {"11:00"}})
	if code != http.StatusSeeOther {
		t.Fatalf("save: %d %+v", code, page.Notices)
	}
	mustContain(t, after, "DTSTART;TZID=Europe/Berlin:20260112T100000", "DTEND;TZID=Europe/Berlin:20260112T110000",
		"BEGIN:VTIMEZONE", "TZID:Europe/Berlin", "RDATE;TZID=Europe/Berlin:20260114T100000", "UNTIL=20260302T225959Z")
	form := formFields(decode[editedForm](t, f.h.do(get(f.path("calendar")+"/e/edit", f.sess))).Form)
	if form.Get("tz") != "Europe/Berlin" || form.Get("start.time") != "10:00" || form.Get("repeat.until") != "2026-03-02" {
		t.Errorf("the form afterwards: tz %q, start %q, until %q", form.Get("tz"), form.Get("start.time"), form.Get("repeat.until"))
	}
}

// The last day of a series stands in its form among the form's dates, and
// those are in the event's own zone (S-11): Mondays at 07:00 in Tokyo are
// Sunday evenings in Berlin, and a form whose start says Monday does not
// say the series ends on a Sunday (Б27).
func TestTheLastDayIsADayOfTheFormsZone(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.Timezone = "Europe/Berlin" }, "e.ics",
		ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Asia/Tokyo:20260112T070000\nDTEND;TZID=Asia/Tokyo:20260112T080000\n"+
			"SUMMARY:Early\nRRULE:FREQ=WEEKLY;UNTIL=20260301T220000Z\nEND:VEVENT"))
	form := formFields(decode[editedForm](t, f.h.do(get(f.path("calendar")+"/e/edit", f.sess))).Form)
	if form.Get("start.date") != "2026-01-12" || form.Get("repeat.until") != "2026-03-02" {
		t.Fatalf("the form: start %q, until %q, want Monday 12 January and Monday 2 March", form.Get("start.date"), form.Get("repeat.until"))
	}
	code, page, after := shiftEdit(t, f, url.Values{"repeat.until": {"2026-03-09"}})
	if code != http.StatusSeeOther {
		t.Fatalf("save: %d %+v", code, page.Notices)
	}
	// The end of 9 March in Tokyo.
	mustContain(t, after, "UNTIL=20260309T145959Z")
}
