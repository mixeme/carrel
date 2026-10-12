package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б32 of the audit of 11.10.2026: a note whose edit changed only its
// Notebook moves without the form's version being compared. What moves is
// the note as the server holds it at that moment, read for the move — so
// what another client wrote meanwhile into a part the form does not show
// goes with it, and nothing is lost. (A change in what the form does show
// makes the form differ from the record, and that is the conflict screen:
// TestNoteMoveSurvivesAConflict.)
func TestANoteMovedByItsNotebookTakesWhatWasWrittenMeanwhile(t *testing.T) {
	f := newNoteShelf(t)
	list := f.path("notes")
	_ = f.h.do(get(list, f.sess))
	p := decode[view.NotePage](t, f.h.do(get(list+"/n2/edit", f.sess)))

	stored, _, _ := f.srv.Object(f.col + "n2.ics")
	f.srv.Seed(f.col+"n2.ics", strings.Replace(stored, "X-OTHER:kept", "X-OTHER:kept\r\nX-WRITTEN-MEANWHILE:yes", 1))

	rec := shelfSave(t, f, p, url.Values{"notebook": {f.notebook(f.diary)}})
	if rec.Code != 303 || !strings.HasPrefix(rec.Header().Get("Location"), f.notebook(f.diary)+"/n2") {
		t.Fatalf("save into Diary: %d %s %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	moved, _, ok := f.srv.Object(f.diary + "n2.ics")
	if !ok {
		t.Fatal("the note is not in Diary")
	}
	mustContain(t, unfolded(moved), "X-WRITTEN-MEANWHILE:yes", "X-OTHER:kept")
	if _, _, still := f.srv.Object(f.col + "n2.ics"); still {
		t.Error("the note stayed in Notes as well")
	}
}
