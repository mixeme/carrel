package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// А2 of the audit of 11.10.2026 (docs/plan/scans/silent-choices.md), the
// owner's decision of 11.10.2026 («Первый»): a repeating task marked done is
// done this time, and the series goes on — its start and its due move to the
// next time and it stays open. Carrel completed the whole series: there was
// no next Monday, and nothing said so. After the last time it is done as any
// task is.

func repeatingTask(rule string) string {
	return ics("BEGIN:VTODO\nUID:water\nDTSTAMP:20260101T000000Z\nSUMMARY:Water the plants\n" +
		"DTSTART;TZID=Europe/Moscow:20260112T090000\nDUE;TZID=Europe/Moscow:20260112T180000\n" +
		"RRULE:" + rule + "\nSTATUS:IN-PROCESS\nPERCENT-COMPLETE:60\nEND:VTODO")
}

func TestARepeatingTaskMarkedDoneGoesOnToItsNextTime(t *testing.T) {
	f := calendarFixture(t, nil, "water.ics", repeatingTask("FREQ=WEEKLY;BYDAY=MO;COUNT=3"))
	rec := f.path("tasks") + "/water"
	done := func() string {
		t.Helper()
		page := decode[view.TaskPage](t, f.h.do(get(rec, f.sess)))
		if resp := f.h.do(postForm(actionHref(t, page.Actions, "task.done"), f.sess, url.Values{})); resp.Code != 303 {
			t.Fatalf("mark done: %d %s", resp.Code, resp.Body.String())
		}
		return body(t, f, "water.ics")
	}

	// The first Monday done: the second stands, open, and two are left.
	got := done()
	mustContain(t, got, "DTSTART;TZID=Europe/Moscow:20260119T090000", "DUE;TZID=Europe/Moscow:20260119T180000",
		"RRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=2", "STATUS:NEEDS-ACTION")
	for _, gone := range []string{"COMPLETED:", "PERCENT-COMPLETE", "STATUS:COMPLETED"} {
		if strings.Contains(got, gone) {
			t.Errorf("the next time of the task holds %q:\n%s", gone, got)
		}
	}
	if page := decode[view.TaskPage](t, f.h.do(get(rec, f.sess))); page.Done {
		t.Errorf("the task reads as done after its first time")
	}

	// The second: the third, the last one.
	mustContain(t, done(), "DTSTART;TZID=Europe/Moscow:20260126T090000", "RRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=1")

	// The last one done is the task done: there is no next time.
	got = done()
	mustContain(t, got, "DTSTART;TZID=Europe/Moscow:20260126T090000", "STATUS:COMPLETED", "COMPLETED:")
	if page := decode[view.TaskPage](t, f.h.do(get(rec, f.sess))); !page.Done {
		t.Errorf("the task does not read as done after its last time")
	}
}

// The form says what the tick will do before it is ticked, and the same
// tick in the form does the same.
func TestTheFormOfARepeatingTaskSaysWhatDoneDoes(t *testing.T) {
	f := calendarFixture(t, nil, "water.ics", repeatingTask("FREQ=WEEKLY;BYDAY=MO"))
	rec := f.path("tasks") + "/water"
	page := decode[view.TaskEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	hint := ""
	for _, set := range page.Form.Sets {
		for _, row := range set.Rows {
			if row.Name == "done" {
				hint = string(row.Hint)
			}
		}
	}
	if !strings.Contains(hint, "repeats") || !strings.Contains(hint, "19") {
		t.Errorf("hint under Done = %q", hint)
	}
	form := formFields(page.Form)
	form.Set("done", "on")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	got := body(t, f, "water.ics")
	mustContain(t, got, "DTSTART;TZID=Europe/Moscow:20260119T090000", "DUE;TZID=Europe/Moscow:20260119T180000", "RRULE:FREQ=WEEKLY;BYDAY=MO")
	if strings.Contains(got, "STATUS:COMPLETED") {
		t.Errorf("the series was completed:\n%s", got)
	}
}

// А2, the rest: a time of the series changed by hand (RECURRENCE-ID) that
// the task has moved past goes with the time it stood for — it was done —
// and does not stay in the file as a time the series no longer has, read by
// other clients as an open task of its own. One ahead of the new start
// stays.
func TestARepeatingTaskMovedOnDropsTheTimesItPassed(t *testing.T) {
	doc := ics("BEGIN:VTODO\nUID:water\nDTSTAMP:20260101T000000Z\nSUMMARY:Water the plants\n" +
		"DTSTART;TZID=Europe/Moscow:20260112T090000\nDUE;TZID=Europe/Moscow:20260112T180000\n" +
		"RRULE:FREQ=WEEKLY;BYDAY=MO\nEND:VTODO\n" +
		"BEGIN:VTODO\nUID:water\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID;TZID=Europe/Moscow:20260112T090000\nSUMMARY:Water the plants and the palm\n" +
		"DTSTART;TZID=Europe/Moscow:20260112T100000\nDUE;TZID=Europe/Moscow:20260112T180000\nEND:VTODO\n" +
		"BEGIN:VTODO\nUID:water\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID;TZID=Europe/Moscow:20260126T090000\nSUMMARY:Water the plants early\n" +
		"DTSTART;TZID=Europe/Moscow:20260126T070000\nDUE;TZID=Europe/Moscow:20260126T180000\nEND:VTODO")
	f := calendarFixture(t, nil, "water.ics", doc)
	rec := f.path("tasks") + "/water"
	page := decode[view.TaskPage](t, f.h.do(get(rec, f.sess)))
	if resp := f.h.do(postForm(actionHref(t, page.Actions, "task.done"), f.sess, url.Values{})); resp.Code != 303 {
		t.Fatalf("mark done: %d %s", resp.Code, resp.Body.String())
	}
	got := unfolded(body(t, f, "water.ics"))
	mustContain(t, got, "DTSTART;TZID=Europe/Moscow:20260119T090000", "RECURRENCE-ID;TZID=Europe/Moscow:20260126T090000")
	if strings.Contains(got, "RECURRENCE-ID;TZID=Europe/Moscow:20260112T090000") {
		t.Errorf("the time done stays as a time of its own:\n%s", got)
	}
}

// A task that repeats by its due day alone, a day skipped in the series.
func TestARepeatingTaskSkipsADayLeftOut(t *testing.T) {
	f := calendarFixture(t, nil, "bins.ics", ics("BEGIN:VTODO\nUID:bins\nDTSTAMP:20260101T000000Z\nSUMMARY:Bins\n"+
		"DUE;VALUE=DATE:20260112\nRRULE:FREQ=WEEKLY\nEXDATE;VALUE=DATE:20260119\nEND:VTODO"))
	rec := f.path("tasks") + "/bins"
	page := decode[view.TaskPage](t, f.h.do(get(rec, f.sess)))
	if resp := f.h.do(postForm(actionHref(t, page.Actions, "task.done"), f.sess, url.Values{})); resp.Code != 303 {
		t.Fatalf("mark done: %d %s", resp.Code, resp.Body.String())
	}
	got := body(t, f, "bins.ics")
	mustContain(t, got, "DUE;VALUE=DATE:20260126")
	if strings.Contains(got, "STATUS:COMPLETED") || strings.Contains(got, "DTSTART") {
		t.Errorf("after the first time:\n%s", got)
	}
}
