package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/caldav"
)

// Им2, Им8–Им10, Им12 of the scan of import (s-io-6, -7): the wizard is
// titled by its kind; a row of the preview says what will become of it —
// ready, failed, skipped —, a record without a title has the frame's
// placeholder, one of another kind its component as a badge and its own
// date; «Ready» says what is kept: an overdue task stays overdue, one
// without a date is a task, an event keeps the zone it was written in.
func TestImportLinesAreTheFrames(t *testing.T) {
	f := calendarFixture(t, nil)
	tasks := f.path("tasks")
	page := decode[view.ImportPage](t, f.h.do(get(tasks+"/import", f.sess)))
	if page.Title != "Import tasks" {
		t.Errorf("title = %q", page.Title)
	}
	const file = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//x//EN\n" +
		"BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Сдать отчёт\nDUE;VALUE=DATE:20200105\nEND:VTODO\n" +
		"BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Когда-нибудь\nEND:VTODO\n" +
		"BEGIN:VTODO\nUID:t3\nDTSTAMP:20260101T000000Z\nDESCRIPTION:без названия\nDUE;VALUE=DATE:20991231\nEND:VTODO\n" +
		"BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nSUMMARY:Созвон с подрядчиком\nDTSTART;VALUE=DATE:20260817\nEND:VEVENT\n" +
		"END:VCALENDAR\n"
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "tasks.ics", []byte(file))))
	if page.Title != "Import tasks" || len(page.Preview) != 1 {
		t.Fatalf("title %q, preview %+v, notices %+v", page.Title, page.Preview, page.Notices)
	}
	lines := map[string]view.ImportLine{}
	for _, l := range page.Preview[0].Lines {
		lines[string(l.Cells[1])] = l
	}
	if l := lines["Сдать отчёт"]; l.Result != view.ImportReady || l.Cells[3] != "Ready — overdue, and stays overdue" {
		t.Errorf("overdue = %+v", l)
	}
	if l := lines["Когда-нибудь"]; l.Cells[3] != "Ready — a task without a date is a task" {
		t.Errorf("no date = %+v", l)
	}
	if l := lines["— (no SUMMARY)"]; !l.Untitled {
		t.Errorf("untitled = %+v, want the placeholder marked", l)
	}
	if l := lines["Созвон с подрядчиком"]; l.Result != view.ImportSkipped || l.Component != "VEVENT" || !strings.Contains(string(l.Cells[2]), "2026") {
		t.Errorf("an event in a list of tasks = %+v, want skipped, VEVENT as a badge, its own date", l)
	}

	// An event written in another zone keeps it.
	cal := f.path("calendar")
	page = decode[view.ImportPage](t, f.h.do(get(cal+"/import", f.sess)))
	const events = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//x//EN\n" +
		"BEGIN:VEVENT\nUID:e2\nDTSTAMP:20260101T000000Z\nSUMMARY:Встреча\nDTSTART;TZID=Asia/Yekaterinburg:20260817T100000\nDTEND;TZID=Asia/Yekaterinburg:20260817T110000\nEND:VEVENT\n" +
		"END:VCALENDAR\n"
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cal.ics", []byte(events))))
	if page.Title != "Import events" || len(page.Preview) != 1 || page.Preview[0].Lines[0].Cells[3] != "Ready — time zone kept as sent" {
		t.Errorf("title %q, preview %+v", page.Title, page.Preview)
	}
}

// Им6 (s-io-4): the root of the tree of folders notes are read from says its
// connection only, not the collection again.
func TestImportTreeRootSaysItsConnection(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	notebook := davtest.CalendarHome + "notes/"
	srv.AddCalendar(caldav.Calendar{Path: notebook, Name: "Notebook", SupportedComponentSet: []string{"VJOURNAL"}})
	srv.SeedFolder(davtest.FileRoot + "docs/")
	f := connectFixture(t, srv, notebook, nil)
	addFileCollection(t, f, davtest.FileRoot+"docs/", "Docs")
	page := decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	if page.Title != "Import notes" || page.Source.Folder == nil || len(page.Source.Folder.Nodes) != 1 {
		t.Fatalf("title %q, source %+v", page.Title, page.Source)
	}
	if root := page.Source.Folder.Nodes[0]; root.Label != "Docs" || strings.Contains(string(root.Stamp), "Docs") || root.Stamp == "" {
		t.Errorf("root = %q · %q, want the collection and its connection alone", root.Label, root.Stamp)
	}
}

// Им12 of the scan of import (s-io-8): a single Markdown file says where its
// title and date come from, as a file of a set does; a date written by hand
// in its front matter is its date.
func TestOneMarkdownFileSaysWhereItsTitleComesFrom(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	notebook := davtest.CalendarHome + "notes/"
	srv.AddCalendar(caldav.Calendar{Path: notebook, Name: "Notebook", SupportedComponentSet: []string{"VJOURNAL"}})
	f := connectFixture(t, srv, notebook, nil)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "vstrecha.md",
		[]byte("---\ntitle: Встреча с подрядчиком\ndate: 2026-03-11\ncategories: работа\n---\n\nОбсудили смету.\n"))))
	if len(page.Preview) != 1 || len(page.Preview[0].Lines) != 1 {
		t.Fatalf("preview = %+v, notices %+v", page.Preview, page.Notices)
	}
	cells := page.Preview[0].Lines[0].Cells
	if cells[3] != "Ready — title, date and tags from front matter" || !strings.Contains(string(cells[2]), "March 2026") {
		t.Errorf("row = %q", cells)
	}
}
