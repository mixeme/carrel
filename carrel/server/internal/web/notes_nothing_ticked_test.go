package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The combined notebook with nothing ticked says what the other kinds say
// (states-25): «0 of 3 notebooks · 0 accounts» — not «· VJOURNAL», which
// tells how notes lie on a server, and nothing is shown to lie anywhere.
func TestNothingTickedNotesSayWhatOtherKindsSay(t *testing.T) {
	app := newMockupApp(t)
	if rec := app.h.do(postForm("/app/sources/notes", app.sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("untick everything: %d", rec.Code)
	}
	p := decode[view.NotesPage](t, app.h.do(get("/app/notes", app.sess)))
	if got, frame := said(p.Subtitle), headSub(t, "states-25"); got != frame {
		t.Errorf("subtitle %q, the frame %q", got, frame)
	}
}
