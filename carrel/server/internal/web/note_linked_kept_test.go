package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The linked note shows «Kept as the server sent them», folded, «Show 4»,
// after «Where this note is kept», as the note itself does (s-notes-32, as
// s-notes-2; Typecase's note 181 p. 4, task 254).
func TestTheLinkedNoteShowsWhatIsKeptAsSent(t *testing.T) {
	if frame := mockupFrame(t, "s-notes-32"); !strings.Contains(frame, "Kept as the server sent them") || !strings.Contains(frame, "Show 4") {
		t.Fatal("s-notes-32 does not draw «Kept as the server sent them», «Show 4»: read the frame again")
	}
	app := newMockupApp(t)
	openList(t, app, "/app/notes")
	page := decode[view.NotePage](t, app.h.do(get("/app/notes/linked/5c81-77aa-0d64", app.sess)))
	if !page.LinkedRecord {
		t.Fatal("not the linked note's page")
	}
	// Folded: «Show 4» leads to the linked note with them open, not to a copy.
	show := page.Meta.ForeignShow
	if show == nil || show.Label != "Show 4" || len(page.Meta.Foreign) != 4 || !strings.HasSuffix(show.Href, "/app/notes/linked/5c81-77aa-0d64?foreign=1") {
		t.Fatalf("kept as sent: show %+v, %d fields; want «Show 4» to the linked note", show, len(page.Meta.Foreign))
	}
	open := decode[view.NotePage](t, app.h.do(get("/app/notes/linked/5c81-77aa-0d64?foreign=1", app.sess)))
	if open.Meta.ForeignShow != nil || len(open.Meta.Foreign) != 4 {
		t.Errorf("opened: show %+v, %d fields", open.Meta.ForeignShow, len(open.Meta.Foreign))
	}
}
