package web_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// selectedLabel is the label of the option a row has chosen.
func selectedLabel(row *view.FormRow) string {
	for _, o := range row.Options {
		if o.Selected {
			return string(o.Label)
		}
	}
	return ""
}

// A new event is the edit's form, empty (s-agenda-15, Typecase task 77): one
// reminder by default — First, 10 minutes before —, Every 1, the start and
// the end filled — the next whole hour, an hour long, in the instance zone,
// which is chosen without «as sent» —, nothing kept as the server sent it and
// no Delete. Saved as it stands, it is written so: the defaults are values,
// not samples.
func TestNewEventIsTheFrame(t *testing.T) {
	frame, _, _ := strings.Cut(mockupFrame(t, "s-agenda-15"), "<figcaption")
	first := regexp.MustCompile(`<label>First</label><button class="tc-in is-pick" type="button">([^<]*)</button>`).FindStringSubmatch(frame)
	if first == nil || strings.Count(frame, "<label>Reminder</label>") != 0 {
		t.Fatal("s-agenda-15 draws no one reminder First: read the frame again")
	}
	every := frameInputValue(t, frame, "Every")
	start, end := frameInputValue(t, frame, "Start time"), frameInputValue(t, frame, "End time")
	if strings.Contains(frame, "Kept as the server sent them") || strings.Contains(frame, "as sent") || strings.Contains(frame, "Delete") {
		t.Fatal("s-agenda-15 draws what a new event has not: read the frame again")
	}
	length := func(a, b string) time.Duration {
		x, _ := time.Parse("15:04", a)
		y, _ := time.Parse("15:04", b)
		return y.Sub(x)
	}

	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "calendar", "lichnyj")
	page := decode[view.EventEditPage](t, app.h.do(get(list+"/new", app.sess)))
	if r := formRowOf(page.Form, "reminder.0"); r == nil || selectedLabel(r) != first[1] || r.Label != "First" {
		t.Errorf("reminder.0 = %+v, want First %q (s-agenda-15)", r, first[1])
	}
	if r := formRowOf(page.Form, "reminder.1"); r != nil {
		t.Errorf("a second reminder: %+v", r)
	}
	if r := formRowOf(page.Form, "repeat.interval"); r == nil || string(r.Value) != every {
		t.Errorf("Every = %+v, want %q", r, every)
	}
	if r := formRowOf(page.Form, "repeat.freq"); r == nil || r.Value != "" {
		t.Errorf("Frequency = %+v, want no repeat", r)
	}
	zone, err := time.LoadLocation(mockupZone)
	if err != nil {
		t.Fatal(err)
	}
	now := mockupNow(t).Add(mockupSince).In(zone)
	next := time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, 0, 0, 0, now.Location())
	got := map[string]string{}
	for _, n := range []string{"start.date", "start.time", "end.date", "end.time"} {
		if r := formRowOf(page.Form, n); r != nil {
			got[n] = string(r.Value)
		}
	}
	want := map[string]string{"start.date": next.Format("2006-01-02"), "start.time": next.Format("15:04"),
		"end.date": next.Add(length(start, end)).Format("2006-01-02"), "end.time": next.Add(length(start, end)).Format("15:04")}
	for n, v := range want {
		if got[n] != v {
			t.Errorf("%s = %q, want %q (the next whole hour, as long as s-agenda-15's %s–%s)", n, got[n], v, start, end)
		}
	}
	if tz := formRowOf(page.Form, "tz"); tz == nil || tz.ValueAside != "" {
		t.Errorf("tz = %+v, want no «as sent»", tz)
	}
	if len(page.Foreign) != 0 || page.Delete != nil {
		t.Errorf("foreign %+v, delete %+v", page.Foreign, page.Delete)
	}

	form := formFields(page.Form)
	form.Set("summary", "Planning")
	rec := app.h.do(postForm(list+"/new", app.sess, form))
	if rec.Code != 303 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	body, _, _ := app.servers["nextcloud"].Object(davtest.CalendarHome + "lichnyj/" + loc[strings.LastIndex(loc, "/")+1:] + ".ics")
	mustContain(t, unfolded(body), "TRIGGER:-PT10M", "DTSTART;TZID=Europe/Moscow:"+next.Format("20060102T150405"))
	if strings.Contains(body, "RRULE") {
		t.Errorf("a new event that does not repeat got a rule:\n%s", body)
	}
}
