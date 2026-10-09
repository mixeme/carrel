package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// fieldOf is the row of a record's fields under a label.
func fieldOf(fields []view.Field, label string) *view.Field {
	for i := range fields {
		if string(fields[i].Label) == label {
			return &fields[i]
		}
	}
	return nil
}

func fieldLabels(fields []view.Field) string {
	var out []string
	for _, f := range fields {
		out = append(out, string(f.Label))
	}
	return strings.Join(out, ", ")
}

// The event as s-agenda-2 draws it, and in the panel of s-agenda-1 (the
// client's scan of the calendar, groups В and Г): When in pieces with the
// zone it was made in, Repeats with its end, Reminders, no Organiser among
// the fields — the organiser is the first of the people —, statuses as the
// frame writes them, the source's path and when it changed, the buttons of
// the head, and the panel beside the agenda without scripts.
func TestEventByItsFrames(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "calendar", "lichnyj")
	app.h.do(get(list+"?from=2026-08-10", app.sess))
	rec := list + "/8f1c-4a02-b7d3"
	page := decode[view.EventPage](t, app.h.do(get(rec, app.sess)))

	// Г2, Г3, Г4, Г5: the fields in the frame's order.
	if got := fieldLabels(page.Fields); got != "When, Repeats, Reminders, Where, Status, Tags" {
		t.Errorf("fields = %s", got)
	}
	when := fieldOf(page.Fields, "When")
	if when == nil || when.Value != "" || len(when.Runs) != 2 ||
		when.Runs[0] != (view.Run{Text: "10:00 – 11:00", Mark: view.RunMono}) || when.Runs[1].Text != ", Monday 10 August" ||
		when.Badge.Label != "created 12:00 Asia/Yekaterinburg" {
		t.Errorf("When = %+v", when)
	}
	repeats := fieldOf(page.Fields, "Repeats")
	if repeats == nil || repeats.Value != "Weekly on Mon" || repeats.Badge.Label != "until 2026-12-28" {
		t.Errorf("Repeats = %+v", repeats)
	}
	// Both reminders in one line, as the frame says them (Typecase's note
	// 40: «10 minutes before · 1 day before»).
	said := regexp.MustCompile(`<dt>Reminders</dt><dd>([^<]*)</dd>`).FindStringSubmatch(mockupFrame(t, "s-agenda-2"))
	if said == nil || !strings.Contains(said[1], " · ") {
		t.Fatalf("s-agenda-2 says %v of its reminders: read the frame again", said)
	}
	if r := fieldOf(page.Fields, "Reminders"); r == nil || string(r.Value) != frameText(said[1]) {
		t.Errorf("Reminders = %+v, the frame %q", r, frameText(said[1]))
	}
	// Г5, Г6: the organiser first among the people, statuses in lower case.
	var people []string
	for _, a := range page.Attendees {
		people = append(people, string(a.Name)+" · "+string(a.Status))
	}
	if got := strings.Join(people, ", "); got != "Михаил Енученко · organiser, Анна Ковалёва · accepted, Артём Белов · tentative, Виктор Гончаров · no reply" {
		t.Errorf("attendees = %s", got)
	}
	// Г1: Edit with its icon, New task, Print; Attach by its name.
	for id, icon := range map[string]string{"record.edit": "i-pencil", "event.task": "i-task", "record.print": "i-print", "attach.add": "i-up"} {
		if a := actionByID(page.Actions, id); a == nil || a.Icon != icon {
			t.Errorf("%s = %+v, want icon %s", id, a, icon)
		}
	}
	// Г7: the attachment lies in the event; its size as the frame writes it.
	if len(page.Attach) != 1 || !page.Attach[0].Inline || page.Attach[0].Size != "3 KB" {
		t.Errorf("attach = %+v", page.Attach)
	}
	// Г10: the path, and when it changed on the server.
	if page.Source.Path == "" || page.Source.Compared != "changed on server 3 days ago" {
		t.Errorf("source = %+v", page.Source)
	}
	// Г12: the file of From this computer travels in a field the action names.
	if a := actionByID(page.Actions, "attach.add"); a == nil || a.File == "" || len(a.Arrow) == 0 || a.Arrow[0].File == "" {
		t.Errorf("Attach names no field for the file: %+v", a)
	}

	// В1, В2, Б11: the panel beside the agenda, without scripts.
	open := ""
	agenda := decode[view.AgendaPage](t, app.h.do(get(list+"?from=2026-08-10", app.sess)))
	for _, d := range agenda.Days {
		for _, r := range d.Rows {
			if r.UID == "8f1c-4a02-b7d3" {
				open = r.Href
			}
		}
	}
	panel := decode[view.AgendaPage](t, app.h.do(get(list+"?from=2026-08-10&open="+url.QueryEscape(open), app.sess)))
	d := panel.Details
	if d == nil {
		t.Fatal("no panel with ?open=")
	}
	if got := fieldLabels(d.Fields); got != "When, Repeats, Reminders, Where, Status, Tags" {
		t.Errorf("the panel's fields = %s", got)
	}
	if d.Close == "" || strings.Contains(d.Close, "open=") || d.Source.Colour == "" || d.DeleteNote == "" {
		t.Errorf("panel: close %q, colour %q, delete note %q", d.Close, d.Source.Colour, d.DeleteNote)
	}
	on := 0
	for _, day := range panel.Days {
		for _, r := range day.Rows {
			if r.On {
				on++
				if r.Href != open {
					t.Errorf("row %s is on, the panel is %s", r.Href, open)
				}
			}
		}
	}
	if on == 0 {
		t.Error("no row is on beside the panel")
	}
}

// The panel of an event has its card's fields in its card's order (Typecase's
// note 40, s-agenda-1): When, Repeats, Reminders — both of them —, Where,
// Status, Tags.
func TestEventPanelHasTheCardsOrder(t *testing.T) {
	var want []string
	for _, m := range regexp.MustCompile(`<dt>([^<]+)</dt>`).FindAllStringSubmatch(framePanel(t, "s-agenda-1"), -1) {
		want = append(want, m[1])
	}
	if strings.Join(want, ", ") != "When, Repeats, Reminders, Where, Status, Tags" {
		t.Fatalf("the panel of s-agenda-1 = %q: read the frame again", want)
	}
	app := newMockupApp(t)
	first := combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	open := app.listPath(t, "calendar", "lichnyj") + "/8f1c-4a02-b7d3"
	page := decode[view.AgendaPage](t, app.h.do(get("/app/calendar?from=2026-08-10&task="+first.Poll.TaskID+"&open="+url.QueryEscape(open), app.sess)))
	if page.Details == nil {
		t.Fatal("no panel")
	}
	if got := fieldLabels(page.Details.Fields); got != strings.Join(want, ", ") {
		t.Errorf("panel fields = %s, want %q", got, want)
	}
	if r := fieldOf(page.Details.Fields, "Reminders"); r == nil || r.Value != "10 minutes before · 1 day before" {
		t.Errorf("panel Reminders = %+v", r)
	}
}
