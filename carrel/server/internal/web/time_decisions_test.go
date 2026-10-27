package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The owner's decisions of 12.10.2026 on the table of S-11 «an edit of the
// time → what is written» (Б23, Б25, Б26, Б28 of the audit of 11.10.2026).

func decisionEvent(lines string) string {
	return ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\n" + lines + "END:VEVENT")
}

// Б26: a frequency chosen in the simple fields is the repeat the person
// asked for. The parts of the rule those fields do not show belonged to the
// old frequency and go; every N, the end of the series and the start of the
// week stay, and so do the days ticked when the new frequency is weekly.
func TestAnotherFrequencyDropsWhatTheFieldsDoNotShow(t *testing.T) {
	cases := []struct {
		name, rule string
		set        url.Values
		want       string
	}{
		{"yearly to monthly", "FREQ=YEARLY;BYMONTH=1;BYMONTHDAY=12;INTERVAL=2;WKST=SU;COUNT=10",
			url.Values{"repeat.freq": {"MONTHLY"}}, "RRULE:FREQ=MONTHLY;INTERVAL=2;WKST=SU;COUNT=10"},
		{"numbered days to weekly", "FREQ=MONTHLY;BYDAY=2TU,-1FR;BYSETPOS=1",
			url.Values{"repeat.freq": {"WEEKLY"}, "repeat.on": {"TU"}}, "RRULE:FREQ=WEEKLY;BYDAY=TU"},
		{"weekly to monthly, the ticks still sent", "FREQ=WEEKLY;BYDAY=MO,WE;UNTIL=20260601T000000Z",
			url.Values{"repeat.freq": {"MONTHLY"}}, "RRULE:FREQ=MONTHLY;UNTIL=20260601T000000Z"},
		// The frequency untouched, what the fields do not show stays.
		{"every second, the same frequency", "FREQ=YEARLY;BYMONTH=1;BYMONTHDAY=12",
			url.Values{"repeat.interval": {"2"}}, "RRULE:FREQ=YEARLY;BYMONTH=1;BYMONTHDAY=12;INTERVAL=2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := calendarFixture(t, nil, "e.ics", decisionEvent("DTSTART:20260112T100000Z\nDTEND:20260112T110000Z\nSUMMARY:S\nRRULE:"+tc.rule+"\n"))
			code, page, after := shiftEdit(t, f, tc.set)
			if code != http.StatusSeeOther {
				t.Fatalf("save: %d %+v", code, page.Notices)
			}
			mustContain(t, after, tc.want+"\r\n")
		})
	}
}

// Б25: the start is always the first occurrence, and the days ticked are
// not moved by it. A series on Fridays given a Tuesday is written as typed,
// and the agenda shows that Tuesday and then the Fridays. (The week holds a
// Friday too: which records fall in a week is the server's answer, and the
// stand's does not count a start its rule does not name.)
func TestAStartOutsideTheRuleIsTheFirstOccurrence(t *testing.T) {
	f := calendarFixture(t, nil, "e.ics", decisionEvent("DTSTART:20260109T100000Z\nDTEND:20260109T110000Z\nSUMMARY:Sync\nRRULE:FREQ=WEEKLY;BYDAY=FR\n"))
	code, page, after := shiftEdit(t, f, url.Values{"start.date": {"2026-01-13"}, "end.date": {"2026-01-13"}})
	if code != http.StatusSeeOther {
		t.Fatalf("save: %d %+v", code, page.Notices)
	}
	mustContain(t, after, "DTSTART:20260113T100000Z", "RRULE:FREQ=WEEKLY;BYDAY=FR")
	agenda := decode[view.AgendaPage](t, f.h.do(get(f.path("calendar")+"?from=2026-01-12", f.sess)))
	var days []string
	for _, d := range agenda.Days {
		if len(d.Rows) > 0 {
			days = append(days, string(d.Rubric))
		}
	}
	if got := strings.Join(days, " | "); len(days) != 2 || !strings.Contains(days[0], "13") || !strings.Contains(days[1], "16") {
		t.Errorf("the week of 12 January shows %q, want Tuesday 13 and Friday 16", got)
	}
}

// Б28: an event that becomes a whole day starts at midnight, and «15 minutes
// before» would ring the night before. A reminder closer than a day becomes
// «1 day before»; one a day away or at a set moment stays, and so does what
// the reminder does. The other way nothing is touched.
func TestAWholeDayTakesItsRemindersToTheDayBefore(t *testing.T) {
	timed := decisionEvent("DTSTART:20260112T100000Z\nDTEND:20260112T110000Z\nSUMMARY:S\n" +
		"BEGIN:VALARM\nACTION:AUDIO\nTRIGGER:-PT15M\nATTACH;VALUE=URI:Basso\nEND:VALARM\n" +
		"BEGIN:VALARM\nACTION:DISPLAY\nDESCRIPTION:Reminder\nTRIGGER:-P2D\nEND:VALARM\n" +
		"BEGIN:VALARM\nACTION:DISPLAY\nDESCRIPTION:Reminder\nTRIGGER;VALUE=DATE-TIME:20260111T080000Z\nEND:VALARM\n")
	f := calendarFixture(t, nil, "e.ics", timed)
	code, page, after := shiftEdit(t, f, url.Values{"start.allday": {"on"}})
	if code != http.StatusSeeOther {
		t.Fatalf("save: %d %+v", code, page.Notices)
	}
	mustContain(t, after, "DTSTART;VALUE=DATE:20260112", "ACTION:AUDIO", "TRIGGER:-P1D", "ATTACH;VALUE=URI:Basso",
		"TRIGGER:-P2D", "TRIGGER;VALUE=DATE-TIME:20260111T080000Z")
	if strings.Contains(after, "-PT15M") {
		t.Errorf("the reminder still rings a quarter to midnight:\n%s", after)
	}

	day := decisionEvent("DTSTART;VALUE=DATE:20260112\nDTEND;VALUE=DATE:20260113\nSUMMARY:S\n" +
		"BEGIN:VALARM\nACTION:DISPLAY\nDESCRIPTION:Reminder\nTRIGGER:-PT15M\nEND:VALARM\n")
	f = calendarFixture(t, nil, "e.ics", day)
	code, page, after = shiftEdit(t, f, url.Values{"start.allday": {""}, "start.time": {"10:00"}, "end.date": {"2026-01-12"}, "end.time": {"11:00"}})
	if code != http.StatusSeeOther {
		t.Fatalf("save: %d %+v", code, page.Notices)
	}
	mustContain(t, after, "TRIGGER:-PT15M")
}

// Б28 for a task (the kinds are equal): a task whose one «All day» is ticked
// takes its reminders closer than a day to the day before, the one counted
// from the due date as the one counted from the start.
func TestAWholeDayTaskTakesItsRemindersToTheDayBefore(t *testing.T) {
	f := calendarFixture(t, nil, "t.ics", ics("BEGIN:VTODO\nUID:t\nDTSTAMP:20260101T000000Z\nSUMMARY:T\n"+
		"DTSTART:20260112T100000Z\nDUE:20260112T180000Z\n"+
		"BEGIN:VALARM\nACTION:DISPLAY\nDESCRIPTION:Reminder\nTRIGGER;RELATED=END:-PT1H\nEND:VALARM\n"+
		"BEGIN:VALARM\nACTION:DISPLAY\nDESCRIPTION:Reminder\nTRIGGER:-P2D\nEND:VALARM\n"+
		"END:VTODO"))
	rec := f.path("tasks") + "/t"
	form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("allday", "on")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	after := unfolded(body(t, f, "t.ics"))
	mustContain(t, after, "DUE;VALUE=DATE:20260112", "TRIGGER;RELATED=END:-P1D", "TRIGGER:-P2D")
	if strings.Contains(after, "-PT1H") {
		t.Errorf("the reminder still rings an hour before midnight:\n%s", after)
	}
}

// Б20: an end given to another client's event that had none is written in
// the zone of its start, as the start is — not as an instant in UTC beside
// it.
func TestAnEndAddedIsWrittenInTheZoneOfTheStart(t *testing.T) {
	f := calendarFixture(t, nil, "e.ics", decisionEvent("DTSTART;TZID=Europe/Berlin:20260112T100000\nSUMMARY:Talk\n"))
	code, page, after := shiftEdit(t, f, url.Values{"end.date": {"2026-01-12"}, "end.time": {"11:30"}})
	if code != http.StatusSeeOther {
		t.Fatalf("save: %d %+v", code, page.Notices)
	}
	mustContain(t, after, "DTSTART;TZID=Europe/Berlin:20260112T100000", "DTEND;TZID=Europe/Berlin:20260112T113000")
}

// Б23: the end of a flight is written in the zone it lands in. Another zone
// chosen for the event moves the start to the same clock there and keeps the
// length; the end stays in its own zone.
func TestTheEndKeepsItsOwnZone(t *testing.T) {
	f := calendarFixture(t, nil, "e.ics", decisionEvent("DTSTART;TZID=Europe/Berlin:20260112T100000\nDTEND;TZID=Asia/Tokyo:20260113T060000\nSUMMARY:Flight\n"))
	code, page, after := shiftEdit(t, f, url.Values{"tz": {"Europe/Moscow"}})
	if code != http.StatusSeeOther {
		t.Fatalf("save: %d %+v", code, page.Notices)
	}
	// 10:00 in Moscow is 07:00 UTC; twelve hours later is 19:00 UTC, 04:00
	// in Tokyo.
	mustContain(t, after, "DTSTART;TZID=Europe/Moscow:20260112T100000", "DTEND;TZID=Asia/Tokyo:20260113T040000")
	form := formFields(decode[editedForm](t, f.h.do(get(f.path("calendar")+"/e/edit", f.sess))).Form)
	if form.Get("end.date") != "2026-01-12" || form.Get("end.time") != "22:00" {
		t.Errorf("the form shows the end %s %s, want 2026-01-12 22:00 as before", form.Get("end.date"), form.Get("end.time"))
	}
}
