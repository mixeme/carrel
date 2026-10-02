package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// frameInputValue is the value of the input a frame labels so.
func frameInputValue(t *testing.T, frame, label string) string {
	t.Helper()
	m := regexp.MustCompile(`<input[^>]*value="([^"]*)"[^>]*aria-label="` + regexp.QuoteMeta(label) + `"`).FindStringSubmatch(frame)
	if m == nil {
		t.Fatalf("the frame has no input %q with a value: read it again", label)
	}
	return m[1]
}

// An event is edited in its own zone (s-agenda-3, the owner's decision of
// 02.10.2026, from-typecase note 31): one from Asia/Yekaterinburg shows
// 12:00–13:00 as it was sent, that zone chosen and «as sent»; the times
// typed are in the zone chosen. A save that touches nothing writes no time,
// a new time is written in the event's zone, and another zone keeps the
// clock the form shows.
func TestEventIsEditedInItsOwnZone(t *testing.T) {
	frame := mockupFrame(t, "s-agenda-3")
	start, end := frameInputValue(t, frame, "Start time"), frameInputValue(t, frame, "End time")
	zone := regexp.MustCompile(`<summary class="tc-in is-pick">([^<]*)<span class="tc-dim[^"]*">as sent</span>`).FindStringSubmatch(frame)
	if zone == nil {
		t.Fatal("s-agenda-3 draws no zone «as sent»: read the frame again")
	}
	tz := strings.TrimSpace(zone[1])

	f := calendarFixture(t, nil, "sync.ics", ics("BEGIN:VEVENT\nUID:sync\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART;TZID="+tz+":20260810T"+strings.ReplaceAll(start, ":", "")+"00\n"+
		"DTEND;TZID="+tz+":20260810T"+strings.ReplaceAll(end, ":", "")+"00\nSUMMARY:Sync\nEND:VEVENT"))
	rec := f.path("calendar") + "/sync"
	page := decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	for name, want := range map[string]string{"start": start, "end": end} {
		row := formRowOf(page.Form, name+".time")
		if row == nil || row.Value != view.Text(want) {
			t.Errorf("%s = %+v, want the time %s of s-agenda-3", name, row, want)
		}
	}
	if row := formRowOf(page.Form, "tz"); row == nil || row.Value != view.Text(tz) || row.ValueAside != "as sent" {
		t.Errorf("tz = %+v, want %s as sent", row, tz)
	}

	stored := func() string {
		body, _, _ := f.srv.Object(davtest.CalendarHome + "work/sync.ics")
		return unfolded(body)
	}
	save := func(set map[string]string) {
		t.Helper()
		page := decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
		form := formFields(page.Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("save %v: %d %s", set, resp.Code, resp.Body.String())
		}
	}
	save(map[string]string{"summary": "Sync ·"})
	mustContain(t, stored(), "DTSTART;TZID="+tz+":20260810T120000")
	save(map[string]string{"start.time": "14:00", "end.time": "15:00"})
	mustContain(t, stored(), "DTSTART;TZID="+tz+":20260810T140000")
	mustContain(t, stored(), "DTEND;TZID="+tz+":20260810T150000")
	save(map[string]string{"tz": "Europe/Moscow"})
	mustContain(t, stored(), "DTSTART;TZID=Europe/Moscow:20260810T140000")
	mustContain(t, stored(), "DTEND;TZID=Europe/Moscow:20260810T150000")
}
