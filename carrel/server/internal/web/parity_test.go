package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Р21 of scans/kinds-parity.md (the rule of 12.10.2026 «kinds are equal»):
// two tasks found duplicates that hold different places are asked which
// place stays, as two events are.
func TestDuplicateTasksAreAskedTheirPlace(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Pay rent\nDUE;VALUE=DATE:20260815\nLOCATION:Bank\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Pay rent\nDUE;VALUE=DATE:20260815\nLOCATION:Online\nEND:VTODO"))
	f.h.do(get(f.path("tasks"), f.sess))
	page := decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess)))
	if len(page.Groups) != 1 {
		t.Fatalf("groups = %+v", page.Groups)
	}
	var props []string
	for _, c := range page.Groups[0].Choices {
		props = append(props, c.Name)
	}
	if !strings.Contains(strings.Join(props, ","), "choice.LOCATION") {
		t.Errorf("choices = %v, want the place asked", props)
	}
}

// Р15 of scans/kinds-parity.md: New task related to the record is every
// kind's — an event and a person's timeline had it, a note did not.
func TestANoteHasNewTask(t *testing.T) {
	f := calendarFixture(t, nil, "n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Plan\nEND:VJOURNAL"))
	page := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1", f.sess)))
	task := actionByID(page.Doc.Bar, "note.task")
	if task == nil || !strings.Contains(task.Href, "/new/quick") || !strings.Contains(task.Href, "related=n1") {
		t.Errorf("New task = %+v", task)
	}
}

// Р17 of scans/kinds-parity.md: what Carrel does not know of a task is
// shown in its form as it came, as an event's is — it was shown nowhere.
func TestATaskFormShowsWhatIsKeptAsSent(t *testing.T) {
	f := calendarFixture(t, nil, "t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nPRIORITY:1\nX-OC-HIDESUBTASKS:1\nCLASS:PRIVATE\nEND:VTODO"))
	page := decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/t1/edit", f.sess)))
	var names []string
	for _, fl := range page.Foreign {
		names = append(names, string(fl.Label)+"="+string(fl.Value))
	}
	if strings.Join(names, ",") != "CLASS=PRIVATE,X-OC-HIDESUBTASKS=1" {
		t.Errorf("kept as sent = %v", names)
	}
}

// Р18 of scans/kinds-parity.md: the form of an event says where the event
// is and when it was read, as the task's form does.
func TestAnEventFormHasItsSource(t *testing.T) {
	f := calendarFixture(t, nil, "e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nSUMMARY:Review\nEND:VEVENT"))
	page := decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/e1/edit", f.sess)))
	if !strings.HasSuffix(string(page.Source.Path), "e1.ics") || page.Source.Stamp == "" {
		t.Errorf("source = %+v", page.Source)
	}
}

// Р10 of scans/kinds-parity.md: a task's list offers Where, as the agenda
// does, and a row carries the task's LOCATION.
func TestTasksHaveAWhereColumn(t *testing.T) {
	f := calendarFixture(t, nil, "t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nLOCATION:Room 4\nEND:VTODO"))
	page := decode[view.TasksPage](t, f.h.do(get(f.path("tasks")+"?seg=all", f.sess)))
	offered := false
	for _, it := range page.Bar.Columns.Items {
		offered = offered || it.ID == "where"
	}
	if !offered || len(page.Groups) == 0 || page.Groups[0].Rows[0].Where != "Room 4" {
		t.Errorf("columns %+v, groups %+v", page.Bar.Columns.Items, page.Groups)
	}
}

// Р20 of scans/kinds-parity.md: an event and a task hand over one record
// whole, as .ics; a note gave only its text, as Markdown, and its other
// properties had no way out. Its menu now has «Export .ics» too: the
// journal as the server holds it.
func TestANoteExportsAsCalendar(t *testing.T) {
	f := calendarFixture(t, nil, "n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Plan\nDESCRIPTION:Steps\nX-KEEP:1\nEND:VJOURNAL"))
	page := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1", f.sess)))
	var href string
	for _, a := range menuActions(page.Meta.Menu) {
		if string(a.Label) == "Export .ics" {
			href = a.Href
		}
	}
	if href == "" {
		t.Fatalf("menu = %+v, want Export .ics", menuActions(page.Meta.Menu))
	}
	resp := f.h.do(get(href, f.sess))
	got := strings.ReplaceAll(resp.Body.String(), "\r\n", "\n")
	if resp.Code != 200 || !strings.HasPrefix(resp.Header().Get("Content-Type"), "text/calendar") ||
		!strings.Contains(resp.Header().Get("Content-Disposition"), ".ics") ||
		!strings.Contains(got, "BEGIN:VJOURNAL") || !strings.Contains(got, "X-KEEP:1") {
		t.Errorf("%d %v\n%s", resp.Code, resp.Header(), got)
	}
	// The bar's Export Markdown stays what it was.
	if a := actionByID(page.Doc.Bar, "note.export"); a == nil || strings.Contains(a.Href, "as=ics") {
		t.Errorf("Export Markdown = %+v", a)
	}
}
