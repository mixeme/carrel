package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// The owner's decision of 11.10.2026: a series given another zone or another
// hour takes its exceptions with it — the day left out stays left out, the
// day moved stays the same day's. Found by the probe of twelve servers
// (docs/servers.md, SV-59): Carrel rewrote the start of the series alone,
// the EXDATE and the RECURRENCE-ID stayed at ten in Moscow, where the series
// no longer had an occurrence, and eleven servers kept them so; Mail.ru threw
// them away.
func TestSeriesTakesItsExceptionsWithIt(t *testing.T) {
	f := calendarFixture(t, nil, "duty.ics", ics("BEGIN:VEVENT\nUID:duty\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART;TZID=Europe/Moscow:20260112T100000\nDTEND;TZID=Europe/Moscow:20260112T110000\nSUMMARY:Duty\n"+
		"RRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=6\nEXDATE;TZID=Europe/Moscow:20260119T100000\nEND:VEVENT\n"+
		"BEGIN:VEVENT\nUID:duty\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID;TZID=Europe/Moscow:20260126T100000\n"+
		"DTSTART;TZID=Europe/Moscow:20260126T120000\nDTEND;TZID=Europe/Moscow:20260126T130000\nSUMMARY:Duty moved\nEND:VEVENT"))
	rec := f.path("calendar") + "/duty"
	save := func(set map[string]string) string {
		t.Helper()
		form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return body(t, f, "duty.ics")
	}

	// A save that does not touch the time leaves them as they are.
	got := save(map[string]string{"summary": "Duty, weekly"})
	mustContain(t, got, "EXDATE;TZID=Europe/Moscow:20260119T100000", "RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000")

	// Another zone, the same clock: ten in Yekaterinburg, and the exceptions
	// with it. The time the day was moved to is the person's own and stays.
	got = save(map[string]string{"tz": "Asia/Yekaterinburg"})
	mustContain(t, got, "DTSTART;TZID=Asia/Yekaterinburg:20260112T100000",
		"EXDATE;TZID=Asia/Yekaterinburg:20260119T100000",
		"RECURRENCE-ID;TZID=Asia/Yekaterinburg:20260126T100000",
		"DTSTART;TZID=Europe/Moscow:20260126T120000", "SUMMARY:Duty moved")

	// Another hour.
	got = save(map[string]string{"start.time": "11:30", "end.time": "12:30"})
	mustContain(t, got, "DTSTART;TZID=Asia/Yekaterinburg:20260112T113000",
		"EXDATE;TZID=Asia/Yekaterinburg:20260119T113000",
		"RECURRENCE-ID;TZID=Asia/Yekaterinburg:20260126T113000")
}
