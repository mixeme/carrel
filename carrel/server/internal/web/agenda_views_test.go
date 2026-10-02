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

// showQuery is the address the ticks of the bar (ListBar.Show) send, as a
// browser sends the form: every hidden field — the empty with too — and the
// kinds ticked.
func showQuery(t *testing.T, base string, form *view.Form, kinds ...string) string {
	t.Helper()
	if form == nil || form.Method != "GET" {
		t.Fatalf("the ticks of the bar = %+v", form)
	}
	q := url.Values{}
	for _, row := range form.Sets[0].Rows {
		if row.Kind == view.InputHidden {
			q.Add(row.Name, string(row.Value))
		}
	}
	for _, k := range kinds {
		q.Add("with", k)
	}
	return base + "?" + q.Encode()
}

// showTicks are the kinds the ticks of the bar say are on the line.
func showTicks(p view.AgendaPage) string {
	var on []string
	for _, row := range p.Bar.Show.Sets[0].Rows {
		if row.Kind != view.InputCheck {
			continue
		}
		for _, o := range row.Options {
			if o.Selected {
				on = append(on, o.Value)
			}
		}
	}
	return strings.Join(on, ",")
}

// review Р-12 (s-agenda-1, U-6), the client's scan Б7: events, tasks due and
// notes stand on the same line of days, each behind its own tick of one GET
// form; the plain address is the events.
func TestAgendaTasksAndNotesOnTheLine(t *testing.T) {
	tz := time.UTC
	today := time.Now().In(tz)
	at := func(h int) string {
		return time.Date(today.Year(), today.Month(), today.Day(), h, 0, 0, 0, tz).Format("20060102T150405Z")
	}
	srv := searchServer(t)
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work", SupportedComponentSet: []string{"VEVENT", "VTODO", "VJOURNAL"}})
	srv.Seed(searchWork+"e.ics", ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART:"+at(10)+"\nDTEND:"+at(11)+"\nSUMMARY:Standup\nEND:VEVENT"))
	srv.Seed(searchWork+"t.ics", ics("BEGIN:VTODO\nUID:t\nDTSTAMP:20260101T000000Z\nDUE:"+at(15)+"\nSUMMARY:Send the report\nEND:VTODO"))
	srv.Seed(searchWork+"n.ics", ics("BEGIN:VJOURNAL\nUID:n\nDTSTAMP:20260101T000000Z\nDTSTART:"+at(9)+"\nSUMMARY:Morning note\nEND:VJOURNAL"))
	f := connectFixture(t, srv, searchWork, nil)
	list := f.path("calendar")

	page := decode[view.AgendaPage](t, f.h.do(get(list, f.sess)))
	if got := showTicks(page); got != "events" {
		t.Fatalf("the ticks of the plain address = %q", got)
	}
	if got := agendaTitles(page); got != "Standup" {
		t.Errorf("the line of the plain address = %q", got)
	}

	withTasks := decode[view.AgendaPage](t, f.h.do(get(showQuery(t, list, page.Bar.Show, "events", "tasks"), f.sess)))
	if got := agendaTitles(withTasks); got != "Standup,Send the report" {
		t.Errorf("events and tasks = %q", got)
	}
	if got := showTicks(withTasks); got != "events,tasks" {
		t.Errorf("the ticks do not say what is on: %q", got)
	}
	var kinds []string
	for _, d := range withTasks.Days {
		for _, r := range d.Rows {
			kinds = append(kinds, string(r.Kind))
		}
	}
	if !strings.Contains(strings.Join(kinds, ","), "task") {
		t.Errorf("rows carry no kind: %v", kinds)
	}

	// All three, then the notes alone: Events is a tick like the others
	// (s-agenda-1 draws it ticked, not locked).
	all := decode[view.AgendaPage](t, f.h.do(get(showQuery(t, list, withTasks.Bar.Show, "events", "tasks", "notes"), f.sess)))
	if got := agendaTitles(all); !strings.Contains(got, "Morning note") || !strings.Contains(got, "Send the report") || !strings.Contains(got, "Standup") {
		t.Fatalf("all three = %q", got)
	}
	notesOnly := decode[view.AgendaPage](t, f.h.do(get(showQuery(t, list, all.Bar.Show, "notes"), f.sess)))
	if got := agendaTitles(notesOnly); got != "Morning note" {
		t.Errorf("the notes alone = %q", got)
	}
	// Nothing ticked is obeyed: the form spoke.
	none := decode[view.AgendaPage](t, f.h.do(get(showQuery(t, list, all.Bar.Show), f.sess)))
	if got := agendaTitles(none); got != "" || showTicks(none) != "" {
		t.Errorf("nothing ticked = %q (%q)", got, showTicks(none))
	}
}

// The ticks work the same in the combined calendar: the poll asks every
// ticked calendar for what they turned on (S-14).
func TestCombinedAgendaSwitches(t *testing.T) {
	tz := time.UTC
	today := time.Now().In(tz)
	at := func(h int) string {
		return time.Date(today.Year(), today.Month(), today.Day(), h, 0, 0, 0, tz).Format("20060102T150405Z")
	}
	srv := searchServer(t)
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work", SupportedComponentSet: []string{"VEVENT", "VTODO"}})
	srv.Seed(searchWork+"e.ics", ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART:"+at(10)+"\nDTEND:"+at(11)+"\nSUMMARY:Standup\nEND:VEVENT"))
	srv.Seed(searchWork+"t.ics", ics("BEGIN:VTODO\nUID:t\nDTSTAMP:20260101T000000Z\nDUE:"+at(15)+"\nSUMMARY:Send the report\nEND:VTODO"))
	f := connectFixture(t, srv, searchWork, nil)
	tickAll(t, f.h, f.sess)

	agendaPoll := func(p view.AgendaPage) *view.PollBar { return p.Poll }
	page := combined(t, f.h, f.sess, "/app/calendar", agendaPoll)
	with := combined(t, f.h, f.sess, showQuery(t, "/app/calendar", page.Bar.Show, "events", "tasks"), agendaPoll)
	if got := agendaTitles(with); !strings.Contains(got, "Send the report") || !strings.Contains(got, "Standup") {
		t.Errorf("the combined agenda with tasks on = %q", got)
	}
	if got := agendaTitles(page); strings.Contains(got, "Send the report") {
		t.Errorf("the tick was off and the task is on the line: %q", got)
	}
}
