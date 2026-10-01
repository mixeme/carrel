package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// review Р-10, "Повторение правится целиком" (S-11): the simple fields
// build RRULE and keep the parts they do not show; Advanced RRULE, when
// changed, wins; EXDATE, RDATE and overriding instances stay untouched.
func TestEventRepeats(t *testing.T) {
	f := calendarFixture(t, nil, "meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART:20260115T100000Z\nSUMMARY:Review\nRRULE:FREQ=WEEKLY;BYDAY=TH;WKST=MO;COUNT=5\n"+
		"EXDATE:20260122T100000Z\nEND:VEVENT\nBEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\n"+
		"RECURRENCE-ID:20260129T100000Z\nDTSTART:20260129T120000Z\nSUMMARY:Review moved\nEND:VEVENT"))
	rec := f.path("calendar") + "/meet"
	save := func(set map[string]string) string {
		t.Helper()
		form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
			if k == "repeat.on" { // the days ticked, one value each
				form[k] = strings.Split(v, ",")
			}
		}
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return body(t, f, "meet.ics")
	}
	kept := []string{"EXDATE:20260122T100000Z", "RECURRENCE-ID:20260129T100000Z", "SUMMARY:Review moved"}

	page := decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	form := formFields(page.Form)
	// Advanced RRULE is empty, the rule its sample (s-agenda-3).
	if form.Get("repeat.freq") != "WEEKLY" || form.Get("repeat.interval") != "1" || strings.Join(form["repeat.on"], ",") != "TH" || form.Get("repeat.count") != "5" || form.Get("repeat.until") != "" ||
		form.Get("repeat.rule") != "" {
		t.Fatalf("form = %v", form)
	}
	if r := formRowOf(page.Form, "repeat.rule"); r == nil || r.Placeholder != "FREQ=WEEKLY;BYDAY=TH;WKST=MO;COUNT=5" {
		t.Errorf("Advanced RRULE = %+v", r)
	}

	got := save(map[string]string{"summary": "Review again"})
	mustContain(t, got, "RRULE:FREQ=WEEKLY;BYDAY=TH;WKST=MO;COUNT=5")
	mustContain(t, got, kept...)

	// Every two weeks on Tuesday and Thursday, ten times: WKST stays.
	got = save(map[string]string{"repeat.interval": "2", "repeat.on": "TU,TH", "repeat.count": "10"})
	mustContain(t, got, "RRULE:FREQ=WEEKLY;BYDAY=TU,TH;WKST=MO;COUNT=10;INTERVAL=2")
	mustContain(t, got, kept...)

	// Until a day instead of a count: the end of that day, in UTC.
	got = save(map[string]string{"repeat.count": "", "repeat.until": "2026-03-31"})
	mustContain(t, got, "RRULE:FREQ=WEEKLY;BYDAY=TU,TH;WKST=MO;INTERVAL=2;UNTIL=20260331T235959Z")

	// Both is refused, and so is a raw rule that does not parse.
	for _, bad := range []map[string]string{
		{"repeat.count": "3", "repeat.until": "2026-03-31"},
		{"repeat.rule": "FREQ=SOMETIMES"},
	} {
		form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		for k, v := range bad {
			form.Set(k, v)
		}
		page := decode[view.EventEditPage](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
		if len(page.Form.Errors) == 0 || page.Form.Errors[0].Name != "repeat" {
			t.Errorf("%v: errors = %+v, want repeat refused", bad, page.Form.Errors)
		}
	}

	// The raw rule wins when it is what changed.
	got = save(map[string]string{"repeat.rule": "FREQ=MONTHLY;BYMONTHDAY=15"})
	mustContain(t, got, "RRULE:FREQ=MONTHLY;BYMONTHDAY=15")
	mustContain(t, got, kept...)

	// Does not repeat: the rule goes, the exceptions are the server's to keep.
	got = save(map[string]string{"repeat.freq": ""})
	if strings.Contains(got, "RRULE") {
		t.Errorf("the rule stayed:\n%s", got)
	}
}

// A new event gets a rule from the simple fields.
func TestNewEventRepeats(t *testing.T) {
	f := calendarFixture(t, nil)
	list := f.path("calendar")
	form := formFields(decode[view.EventEditPage](t, f.h.do(get(list+"/new", f.sess))).Form)
	form.Set("summary", "Standup")
	form.Set("start.date", "2026-01-12")
	form.Set("start.time", "09:00")
	form.Set("repeat.freq", "DAILY")
	form.Set("repeat.count", "3")
	if resp := f.h.do(postForm(list+"/new", f.sess, form)); resp.Code != 303 {
		t.Fatalf("POST: %d %s", resp.Code, resp.Body.String())
	}
	var found bool
	for _, p := range f.srv.Paths(f.col) {
		if b, _, _ := f.srv.Object(p); strings.Contains(unfolded(b), "RRULE:FREQ=DAILY;COUNT=3") {
			found = true
		}
	}
	if !found {
		t.Error("the new event has no rule")
	}
}
