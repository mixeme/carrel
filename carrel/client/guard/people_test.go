package guard_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"carrel.dev/carrel/client/guard"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/server/view"
)

// A task's and a note's people (Р3 of the server's kinds-parity list) have
// no frame yet (Typecase's task 179): the task's card and panel and the
// note's column and panel name them after Related, in its shape — the name
// leading to the card when the books know it, else the address, what each
// said muted. Drawn, each page is still its frame's and its draft's kin.
func TestTasksAndNotesNameTheirPeople(t *testing.T) {
	l := load(t)
	people := []view.AttendeeRow{
		{Name: "Анна Ковалёва", Email: "a.kovaleva@example.org", Status: "organiser", Href: "/app/contacts/a-nc-home/kontakty/c1", Colour: "#4A6B52"},
		{Name: "v@example.org", Email: "v@example.org", Status: "no reply"},
	}
	for _, c := range []struct {
		fixture, model string
		give           func(model any)
	}{
		{"app-task/normal.json", "TaskPage", func(m any) { m.(*view.TaskPage).Attendees = people }},
		{"app-note/normal.json", "NotePage", func(m any) { m.(*view.NotePage).Meta.Attendees = people }},
		{"app-tasks-all/normal.json", "TasksPage", func(m any) { m.(*view.TasksPage).Details.Attendees = people }},
		{"app-notes-all/normal.json", "NotesPage", func(m any) { m.(*view.NotesPage).Details.Attendees = people }},
	} {
		path := filepath.Join("..", "..", "..", "docs", "spec", "fixtures", filepath.FromSlash(c.fixture))
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		var scr screens.Screen
		for _, s := range screens.All {
			if modelName(s) == c.model {
				scr = s
			}
		}
		model, err := decode(scr, path)
		if err != nil {
			t.Fatal(err)
		}
		c.give(model)
		root := scr.Draw(model)
		html := root.Render()
		for _, want := range []string{">Attendees<", "Анна Ковалёва", `href="/app/contacts/a-nc-home/kontakty/c1"`, "v@example.org", "· organiser", "· no reply"} {
			if !strings.Contains(html, want) {
				t.Errorf("%s: %q is not drawn", c.fixture, want)
			}
		}
		// A state that waits for the library (U-5.5) is held to the CSS
		// only, as the screens' own check holds it.
		frame := l.frameOf(path)
		if _, waiting := waits(scr, key(path)); waiting {
			for _, f := range classFindings(l, c.fixture, html) {
				t.Errorf("frame: %s", f)
			}
		} else {
			for _, f := range htmlFindings(l, frame, c.fixture, html, "") {
				t.Errorf("frame: %s", f)
			}
		}
		d, err := guard.Drafts(module, []string{frame})
		if err != nil {
			t.Fatal(err)
		}
		if err := guard.Kin(d[frame], guard.Skeleton(root)); err != nil {
			t.Errorf("%s (%s): %v", c.fixture, frame, err)
		}
	}
}

// The column «Attendees» of a task's list (after Р3; s-tasks-12, last and
// off): chosen, a row draws how many people it has as a plain badge, as the
// agenda's column draws an event's. A notebook's list has none (Typecase's
// Р-64).
func TestTasksHaveAnAttendeesColumn(t *testing.T) {
	for _, c := range []struct {
		fixture, model string
		give           func(model any)
	}{
		{"app-tasks/normal.json", "TasksPage", func(m any) {
			p := m.(*view.TasksPage)
			p.Bar.Columns.Items = append(p.Bar.Columns.Items, view.ColumnItem{ID: "attendees", Label: "Attendees", On: true})
			p.Groups[0].Rows[0].Attendees = "2 attendees"
		}},
	} {
		path := filepath.Join("..", "..", "..", "docs", "spec", "fixtures", filepath.FromSlash(c.fixture))
		var scr screens.Screen
		for _, s := range screens.All {
			if modelName(s) == c.model {
				scr = s
			}
		}
		model, err := decode(scr, path)
		if err != nil {
			t.Fatal(err)
		}
		c.give(model)
		if html := scr.Draw(model).Render(); !strings.Contains(html, ">2 attendees<") {
			t.Errorf("%s: the column draws no people", c.fixture)
		}
	}
}
