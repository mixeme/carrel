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
