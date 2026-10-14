package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Р1 of scans/kinds-parity.md: an event's form edits its repetition and a
// task's did not, though the server reads a repeating task and moves it on
// when it is done (А2). Now a task repeats as an event does — the same
// rubric, the same rule kept whole (S-11) — from its start date, or from its
// due date when it has none, as the server reads it (cal.NextTime). A task
// with neither has nothing to repeat from: the rubric says so before the
// press, and a rule given to it is refused, nothing written.
func TestATaskRepeats(t *testing.T) {
	f := calendarFixture(t, nil,
		"both.ics", ics("BEGIN:VTODO\nUID:both\nDTSTAMP:20260101T000000Z\nSUMMARY:Report\nDTSTART:20260112T090000Z\nDUE:20260112T170000Z\nEND:VTODO"),
		"due.ics", ics("BEGIN:VTODO\nUID:due\nDTSTAMP:20260101T000000Z\nSUMMARY:Rent\nDUE;VALUE=DATE:20260105\nEND:VTODO"),
		"bare.ics", ics("BEGIN:VTODO\nUID:bare\nDTSTAMP:20260101T000000Z\nSUMMARY:Someday\nEND:VTODO"))
	save := func(uid string, set map[string]string) (view.TaskEditPage, string) {
		t.Helper()
		rec := f.path("tasks") + "/" + uid
		form := formFields(decode[view.TaskEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		resp := f.h.do(postForm(rec+"/edit", f.sess, form))
		if resp.Code == 303 {
			return view.TaskEditPage{}, body(t, f, uid+".ics")
		}
		return decode[view.TaskEditPage](t, resp), body(t, f, uid+".ics")
	}

	page := decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/both/edit", f.sess)))
	if r := formRowOf(page.Form, "repeat.freq"); r == nil {
		t.Fatalf("the task form has no Repeats: %+v", page.Form.Sets)
	}

	_, got := save("both", map[string]string{"repeat.freq": "WEEKLY", "repeat.count": "4"})
	mustContain(t, got, "RRULE:FREQ=WEEKLY;COUNT=4", "DTSTART:20260112T090000Z", "DUE:20260112T170000Z")

	// From the due date: until a day is the end of that day.
	_, got = save("due", map[string]string{"repeat.freq": "MONTHLY", "repeat.until": "2026-06-30"})
	mustContain(t, got, "RRULE:FREQ=MONTHLY;UNTIL=20260630", "DUE;VALUE=DATE:20260105")
	card := decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/due/edit", f.sess)))
	if form := formFields(card.Form); form.Get("repeat.freq") != "MONTHLY" || form.Get("repeat.until") != "2026-06-30" {
		t.Errorf("the rule read back = %v", form)
	}

	// Nothing to repeat from: said before, refused after.
	bare := decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/bare/edit", f.sess)))
	if r := formRowOf(bare.Form, "repeat.freq"); r == nil || r.Hint == "" {
		t.Errorf("a task without dates: Repeats = %+v, want a hint", r)
	}
	puts := f.srv.CountRequests("PUT", f.col)
	refused, got := save("bare", map[string]string{"repeat.freq": "DAILY"})
	if len(refused.Form.Errors) == 0 || refused.Form.Errors[0].Name != "repeat" || strings.Contains(got, "RRULE") {
		t.Errorf("errors = %+v\n%s", refused.Form.Errors, got)
	}
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 0 {
		t.Errorf("a refused rule sent %d PUT", n)
	}
}
