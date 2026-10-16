package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Note 69 of Typecase (tasks 176–178; the owner's Р-63 of 15.10.2026: kinds
// are equal in what is common, not in their content): a note's status and an
// event's priority are content of the kind the owner does not take. They are
// not fields of the forms, not lines of the page and the card, not columns
// of the lists — and what came from the server stays, shown in «Kept as the
// server sent them» and written back untouched (S-7).
func TestKindContentIsKeptAsSent(t *testing.T) {
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\nPRIORITY:5\nEND:VEVENT"),
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Plan\nSTATUS:DRAFT\nEND:VJOURNAL"))
	kept := func(fields []view.Field, name, value string) bool {
		for _, fl := range fields {
			if string(fl.Label) == name && string(fl.Value) == value {
				return true
			}
		}
		return false
	}

	event := decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/e1/edit", f.sess)))
	if formRow(event.Form, "priority") != nil || !kept(event.Foreign, "PRIORITY", "5") {
		t.Errorf("event edit: priority row %+v, kept %+v", formRow(event.Form, "priority"), event.Foreign)
	}
	card := decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/e1", f.sess)))
	for _, fl := range card.Fields {
		if fl.Label == "Priority" {
			t.Errorf("the event's card shows Priority: %+v", fl)
		}
	}
	form := formFields(event.Form)
	form.Set("summary", "Review again")
	if rec := f.h.do(postForm(f.path("calendar")+"/e1/edit", f.sess, form)); rec.Code != 303 {
		t.Fatalf("save event: %d %s", rec.Code, rec.Body)
	}
	if got := strings.ReplaceAll(body(t, f, "e1.ics"), "\r\n", "\n"); !strings.Contains(got, "\nPRIORITY:5\n") {
		t.Errorf("the event's priority was not kept:\n%s", got)
	}

	note := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1/edit", f.sess)))
	for _, r := range note.Doc.Fields {
		if r.Name == "status" {
			t.Errorf("the note's edit has Status: %+v", r)
		}
	}
	nf := noteFormOf(t, f, "n1")
	nf.Set("summary", "Plan B")
	if rec := f.h.do(postForm(f.path("notes")+"/n1/edit", f.sess, nf)); rec.Code != 303 {
		t.Fatalf("save note: %d %s", rec.Code, rec.Body)
	}
	if got := strings.ReplaceAll(body(t, f, "n1.ics"), "\r\n", "\n"); !strings.Contains(got, "\nSTATUS:DRAFT\n") {
		t.Errorf("the note's status was not kept:\n%s", got)
	}

	for _, c := range []struct {
		path string
		cols func() *view.Columns
	}{
		{"calendar", func() *view.Columns {
			return decode[view.AgendaPage](t, f.h.do(get(f.path("calendar")+"?from=2026-01-12", f.sess))).Bar.Columns
		}},
		{"notes", func() *view.Columns { return decode[view.NotesPage](t, f.h.do(get(f.path("notes"), f.sess))).Bar.Columns }},
	} {
		cols := c.cols()
		if cols == nil {
			t.Fatalf("%s: no columns", c.path)
		}
		for _, it := range cols.Items {
			if it.ID == "status" || it.ID == "priority" {
				t.Errorf("%s offers the column %q", c.path, it.ID)
			}
		}
	}
}
