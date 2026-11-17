package web_test

import (
	"reflect"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A file dropped on the page of an event, a task or a note is attached to it
// (Typecase's note 136, s-io-20): the drop sheet says so with the record's
// title, and where a file over the record's limit goes — the folder for
// attachments with the record keeping a link, or, without one, that such a
// file is not attached. The file's size is not known while it is carried,
// so the sheet says the rule, not the outcome.
func TestARecordPageSaysWhatADropDoes(t *testing.T) {
	seed := []string{
		"e1.ics", attachEvent("e1", "Синк-ап по релизу", ""),
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nEND:VTODO"),
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Plan\nDESCRIPTION:Text\nEND:VJOURNAL"),
	}
	pages := func(f davFixture) map[string]*view.AttachDrop {
		return map[string]*view.AttachDrop{
			"event": decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/e1", f.sess))).Drop,
			"task":  decode[view.TaskPage](t, f.h.do(get(f.path("tasks")+"/t1", f.sess))).Drop,
			"note":  decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1", f.sess))).Drop,
		}
	}
	titles := map[string]string{"event": "Синк-ап по релизу", "task": "Slides", "note": "Plan"}

	with := attachFixture(t, 0, []string{"carrel"}, seed...)
	for kind, d := range pages(with) {
		if d == nil {
			t.Fatalf("the %s's page names no drop", kind)
		}
		same(t, kind+": lead", d.Lead, view.Text("Drop to attach"))
		sameRuns(t, kind+": line", d.Line, view.Runs{{Text: "to "}, {Text: view.Text(titles[kind]), Mark: view.RunStrong}})
		sameRuns(t, kind+": note", d.Note, view.Runs{{Text: "Over 2 MB it goes to "}, {Text: "Docs / carrel", Mark: view.RunMono}, {Text: view.Text(" and the " + kind + " keeps a link")}})
	}

	// The record's own limit, when its collection takes less (S-20).
	small := attachFixture(t, 1<<20, []string{"carrel"}, seed...)
	same(t, "a smaller limit", pages(small)["event"].Note[0].Text, view.Text("Over 1 MB it goes to "))

	without := attachFixture(t, 0, nil, seed...)
	for kind, d := range pages(without) {
		if d == nil {
			t.Fatalf("without a folder the %s's page names no drop", kind)
		}
		sameRuns(t, kind+": note without a folder", d.Note, view.Runs{{Text: "Over 2 MB it is not attached: no folder for attachments is chosen"}})
	}
}

// same fails the test when got is not want, both shown as the model says them.
func same[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func sameRuns(t *testing.T, what string, got, want view.Runs) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %+v, want %+v", what, got, want)
	}
}
