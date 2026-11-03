package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Carrel's task 253, Typecase's note 162 (s-notes-2, -3, -27, print-5): a
// note's column has «Kept as the server sent them», as a contact's card
// does — a second DESCRIPTION and the properties Carrel does not know,
// together, folded under «Show 4»; without scripts «Show 4» is the note
// with them open. An edit keeps the second text as it was.
func TestANoteKeepsWhatTheServerSent(t *testing.T) {
	f := newNoteShelf(t)
	list := f.path("notes")
	stored, _, _ := f.srv.Object(f.col + "n2.ics")
	f.srv.Seed(f.col+"n2.ics", strings.Replace(stored, "DESCRIPTION:Text of Second", "DESCRIPTION:Text of Second\r\nDESCRIPTION:Договорились заморозить ветку в четверг.", 1))
	_ = f.h.do(get(list, f.sess))

	page := decode[view.NotePage](t, f.h.do(get(list+"/n2", f.sess)))
	var kept []string
	for _, k := range page.Meta.Foreign {
		if !k.Mono {
			t.Errorf("%s is not monospaced, as Source is", k.Label)
		}
		kept = append(kept, string(k.Label)+" · "+string(k.Value))
	}
	want := []string{"DESCRIPTION · Договорились заморозить ветку в четверг.", "X-JTX-COLOR · -16711936", "X-JTX-PINNED · 1", "X-OTHER · kept"}
	if !equalStrings(kept, want) {
		t.Errorf("kept = %q, want %q", kept, want)
	}
	if page.Doc.Body != "Text of Second" {
		t.Errorf("the text = %q, want the first DESCRIPTION", page.Doc.Body)
	}
	show := page.Meta.ForeignShow
	if show == nil || show.Label != "Show 4" || show.Method != "GET" || !show.Enabled || show.Href != list+"/n2?foreign=1" {
		t.Fatalf("Show = %+v, want «Show 4», the note with them open", show)
	}
	open := decode[view.NotePage](t, f.h.do(get(show.Href, f.sess)))
	if open.Meta.ForeignShow != nil || len(open.Meta.Foreign) != 4 {
		t.Errorf("opened: show %+v, kept %d", open.Meta.ForeignShow, len(open.Meta.Foreign))
	}
	// A note with nothing kept has no section.
	f.srv.Seed(f.col+"bare.ics", ics("BEGIN:VJOURNAL\nUID:bare\nDTSTAMP:20260101T000000Z\nSUMMARY:Bare\nEND:VJOURNAL"))
	_ = f.h.do(get(list, f.sess))
	if bare := decode[view.NotePage](t, f.h.do(get(list+"/bare", f.sess))); len(bare.Meta.Foreign) != 0 || bare.Meta.ForeignShow != nil {
		t.Errorf("a bare note keeps %+v, show %+v", bare.Meta.Foreign, bare.Meta.ForeignShow)
	}

	// The edit changes the first text; the second goes back untouched.
	ed := decode[view.NotePage](t, f.h.do(get(list+"/n2/edit", f.sess)))
	if rec := shelfSave(t, f, ed, url.Values{"description": {"Text, changed"}}); rec.Code != 303 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	saved, _, _ := f.srv.Object(f.col + "n2.ics")
	mustContain(t, unfolded(saved), "DESCRIPTION:Text\\, changed", "DESCRIPTION:Договорились заморозить ветку в четверг.", "X-OTHER:kept")

	// The two texts the same, the first changed: the save is the edit — the
	// check before it is sent tells the first from the second by its place,
	// not by its words (found on the fixture of s-conflict-…, 04.11.2026).
	stored, _, _ = f.srv.Object(f.col + "n3.ics")
	f.srv.Seed(f.col+"n3.ics", strings.Replace(stored, "DESCRIPTION:Text of Third", "DESCRIPTION:Text of Third\r\nDESCRIPTION:Text of Third", 1))
	_ = f.h.do(get(list, f.sess))
	ed = decode[view.NotePage](t, f.h.do(get(list+"/n3/edit", f.sess)))
	if rec := shelfSave(t, f, ed, url.Values{"description": {"Third, changed"}}); rec.Code != 303 {
		t.Fatalf("save of the first of two equal texts: %d %s", rec.Code, rec.Body.String())
	}
	saved, _, _ = f.srv.Object(f.col + "n3.ics")
	mustContain(t, unfolded(saved), "DESCRIPTION:Third\\, changed", "DESCRIPTION:Text of Third")
}
