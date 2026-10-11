package web_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/view"
)

func presetHref(t *testing.T, rng *view.Range, id string) string {
	t.Helper()
	if rng == nil {
		t.Fatal("no range on the bar")
	}
	for _, a := range rng.Presets {
		if a.ID == id {
			u, _ := url.Parse(a.Href)
			return u.RequestURI()
		}
	}
	t.Fatalf("no preset %q in %+v", id, rng.Presets)
	return ""
}

func agendaTitles(p view.AgendaPage) string {
	var out []string
	for _, d := range p.Days {
		for _, r := range d.Rows {
			out = append(out, string(r.Summary))
		}
	}
	return strings.Join(out, ",")
}

// review Р-12 (s-agenda-1): the agenda is a list, and Day, Week and Month
// are how much of it is shown — with a range of one's own beside them. A day
// without events is not printed (the owner's decision of 02.10.2026, task
// 77): the window is the range's, and only the days with events stand.
func TestAgendaWindows(t *testing.T) {
	tz := time.UTC
	today := time.Now().In(tz)
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, tz)
	at := func(d time.Time, h int) string {
		return time.Date(d.Year(), d.Month(), d.Day(), h, 0, 0, 0, tz).Format("20060102T150405Z")
	}
	// Today, and a day of this month that is not today (the 1st, or the 2nd
	// when today is the 1st).
	other := first
	if other.Day() == today.Day() {
		other = first.AddDate(0, 0, 1)
	}
	f := calendarFixture(t, nil,
		"now.ics", ics("BEGIN:VEVENT\nUID:now\nDTSTAMP:20260101T000000Z\nDTSTART:"+at(today, 10)+"\nDTEND:"+at(today, 11)+"\nSUMMARY:Today event\nEND:VEVENT"),
		"other.ics", ics("BEGIN:VEVENT\nUID:other\nDTSTAMP:20260101T000000Z\nDTSTART:"+at(other, 9)+"\nDTEND:"+at(other, 10)+"\nSUMMARY:Other day\nEND:VEVENT"))
	list := f.path("calendar")

	span := func(p view.AgendaPage) int {
		t.Helper()
		a, err1 := time.Parse("2 January 2006", string(p.Bar.Range.From))
		b, err2 := time.Parse("2 January 2006", string(p.Bar.Range.To))
		if err1 != nil || err2 != nil {
			t.Fatalf("range %q — %q", p.Bar.Range.From, p.Bar.Range.To)
		}
		return int(b.Sub(a).Hours()/24) + 1
	}
	week := decode[view.AgendaPage](t, f.h.do(get(list, f.sess)))
	if n := span(week); n != 7 {
		t.Fatalf("a week spans %d days", n)
	}
	if len(week.Days) == 0 || len(week.Days) > 2 {
		t.Errorf("the week prints %d days, want those with events", len(week.Days))
	}
	for _, d := range week.Days {
		if len(d.Rows) == 0 {
			t.Errorf("%s: a day without events is printed", d.Rubric)
		}
	}

	day := decode[view.AgendaPage](t, f.h.do(get(presetHref(t, week.Bar.Range, "range.day"), f.sess)))
	if len(day.Days) != 1 || agendaTitles(day) != "Today event" {
		t.Errorf("Day = %+v (%q)", day.Days, agendaTitles(day))
	}
	month := decode[view.AgendaPage](t, f.h.do(get(presetHref(t, week.Bar.Range, "range.month"), f.sess)))
	if n := span(month); n < 28 || n > 31 || len(month.Days) != 2 {
		t.Errorf("Month spans %d days and prints %d", n, len(month.Days))
	}
	titles := agendaTitles(month)
	if !strings.Contains(titles, "Today event") || !strings.Contains(titles, "Other day") {
		t.Errorf("Month = %q, want the whole month read", titles)
	}

	// A range of one's own: the form carries the days, and Show opens them.
	form := week.Bar.Range.Form
	if form == nil {
		t.Fatal("the range has no form for a range of its own")
	}
	v := formFields(*form)
	v.Set("from", first.Format("2006-01-02"))
	v.Set("to", first.Format("2006-01-02"))
	own := decode[view.AgendaPage](t, f.h.do(get(list+"?"+v.Encode(), f.sess)))
	if len(own.Days) != 1 || own.Days[0].Rubric == "" || agendaTitles(own) != "Other day" && first.Day() == other.Day() {
		t.Errorf("a range of one day = %+v", own.Days)
	}
	if own.Bar.Range.From != own.Bar.Range.To {
		t.Errorf("range = %q — %q, want one day", own.Bar.Range.From, own.Bar.Range.To)
	}

	// A range that ends before it starts is not obeyed: the week stands.
	bad := decode[view.AgendaPage](t, f.h.do(get(list+"?view=range&from="+first.AddDate(0, 0, 3).Format("2006-01-02")+"&to="+first.Format("2006-01-02"), f.sess)))
	if n := span(bad); n != 7 {
		t.Errorf("a backwards range spans %d days, want the week", n)
	}
}

// Typecase Р-56 (note 45; the owner's decision of 11.10.2026): the calendar
// shows events and nothing else — tasks and notes live in their own
// sections, and only the screens that cross kinds (search, duplicates, a
// person's timeline) show them together. An address that still carries the
// ticks of before (with=tasks) is the calendar all the same.
func TestTheCalendarShowsEventsAndNothingElse(t *testing.T) {
	tz := time.UTC
	today := time.Now().In(tz)
	at := func(h int) string {
		return time.Date(today.Year(), today.Month(), today.Day(), h, 0, 0, 0, tz).Format("20060102T150405Z")
	}
	srv := searchServer(t)
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work", SupportedComponentSet: []string{"VEVENT", "VTODO", "VJOURNAL"}})
	srv.Seed(searchWork+"e.ics", ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART:"+at(10)+"\nDTEND:"+at(11)+"\nSUMMARY:Standup\nCATEGORIES:team\nEND:VEVENT"))
	srv.Seed(searchWork+"t.ics", ics("BEGIN:VTODO\nUID:t\nDTSTAMP:20260101T000000Z\nDUE:"+at(15)+"\nSUMMARY:Send the report\nEND:VTODO"))
	srv.Seed(searchWork+"n.ics", ics("BEGIN:VJOURNAL\nUID:n\nDTSTAMP:20260101T000000Z\nDTSTART:"+at(9)+"\nSUMMARY:Morning note\nEND:VJOURNAL"))
	f := connectFixture(t, srv, searchWork, nil)
	list := f.path("calendar")

	for _, address := range []string{list, list + "?with=events&with=tasks&with=notes", list + "?with="} {
		page := decode[view.AgendaPage](t, f.h.do(get(address, f.sess)))
		if got := agendaTitles(page); got != "Standup" {
			t.Errorf("%s: the line = %q", address, got)
		}
		for _, d := range page.Days {
			for _, r := range d.Rows {
				if r.Kind != view.KindEvent {
					t.Errorf("%s: a row of kind %q", address, r.Kind)
				}
			}
		}
	}
	// What Columns adds is in the row: the event's tags.
	page := decode[view.AgendaPage](t, f.h.do(get(list, f.sess)))
	if row := page.Days[0].Rows[0]; len(row.Tags) != 1 || row.Tags[0].Label != "team" {
		t.Errorf("the tags of the row = %+v", row.Tags)
	}

	tickAll(t, f.h, f.sess)
	agendaPoll := func(p view.AgendaPage) *view.PollBar { return p.Poll }
	for _, address := range []string{"/app/calendar", "/app/calendar?with=events&with=tasks"} {
		if got := agendaTitles(combined(t, f.h, f.sess, address, agendaPoll)); got != "Standup" {
			t.Errorf("%s: the combined line = %q", address, got)
		}
	}
}

// The choice of the agenda's columns is this browser's, kept as the other
// lists keep theirs (S-9.6): Time and Title are not the form's to take away.
func TestAgendaColumnsAreChosen(t *testing.T) {
	f := calendarFixture(t, nil)
	list := f.path("calendar")
	page := decode[view.AgendaPage](t, f.h.do(get(list, f.sess)))
	form := url.Values{"col": {"tags", "where"}}
	resp := f.h.do(postForm(page.Bar.Columns.Form.Action, f.sess, form))
	if resp.Code != 303 {
		t.Fatalf("columns: %d %s", resp.Code, resp.Body.String())
	}
	req := get(list, f.sess)
	for _, c := range resp.Result().Cookies() {
		req.AddCookie(c)
	}
	page = decode[view.AgendaPage](t, f.h.do(req))
	var on []string
	for _, it := range page.Bar.Columns.Items {
		if it.On {
			on = append(on, it.ID)
		}
	}
	if got := strings.Join(on, ","); got != "time,title,tags,where" {
		t.Errorf("columns shown = %q", got)
	}
}
