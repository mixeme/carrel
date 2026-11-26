package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// autosave is the library's auto-save of an edit (Typecase's note 229): the
// edit's form as Save sends it, with Tc-Autosave.
func autosave(path string, f davFixture, form url.Values) *http.Request {
	req := postForm(path, f.sess, form)
	req.Header.Set("Tc-Autosave", "1")
	return req
}

// An edit that auto-saves: each write is 204 and stays on the page, the
// page's form — which still says the version it was opened at — writes
// again without a conflict with itself, and its Save goes to the note as
// always. A field the server refuses is «not written» (not 2xx): the script
// keeps the text a draft.
func TestAnEditAutoSaves(t *testing.T) {
	doc := ics("BEGIN:VJOURNAL\nUID:auto\nDTSTAMP:20260101T000000Z\nDTSTART;VALUE=DATE:20260105\nSUMMARY:Notes\nDESCRIPTION:One\nEND:VJOURNAL")
	f := calendarFixture(t, nil, "auto.ics", doc)
	rec := f.path("notes") + "/auto"
	form := noteFormOf(t, f, "auto")

	for i, text := range []string{"One two", "One two three"} {
		form.Set("description", text)
		resp := f.h.do(autosave(rec+"/edit", f, form))
		if resp.Code != http.StatusNoContent || resp.Body.Len() != 0 || resp.Header().Get("Location") != "" {
			t.Fatalf("auto-save %d: %d %q %s", i+1, resp.Code, resp.Header().Get("Location"), resp.Body.String())
		}
		mustContain(t, body(t, f, "auto.ics"), "DESCRIPTION:"+text)
	}

	// A field refused: not written, no page.
	bad := url.Values{}
	for k, v := range form {
		bad[k] = v
	}
	bad.Set("start.date", "2026-13-45")
	if resp := f.h.do(autosave(rec+"/edit", f, bad)); resp.Code != http.StatusConflict || resp.Body.Len() != 0 {
		t.Fatalf("auto-save of a refused field: %d %s", resp.Code, resp.Body.String())
	}

	// Save of the same form, as the person presses it.
	form.Set("description", "One two three four")
	resp := f.h.do(postForm(rec+"/edit", f.sess, form))
	if resp.Code != http.StatusSeeOther || !strings.HasSuffix(resp.Header().Get("Location"), rec) {
		t.Fatalf("Save after the auto-saves: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	mustContain(t, body(t, f, "auto.ics"), "DESCRIPTION:One two three four")
}

// A new note that auto-saves: the first write creates it and sends the form
// to the note's edit; the next writes — the same form, which names no
// version — edit it there.
func TestANewNoteAutoSaves(t *testing.T) {
	f := calendarFixture(t, nil)
	list := f.path("notes")
	_ = f.h.do(get(list, f.sess))
	p := decode[view.NotePage](t, f.h.do(get(list+"/new", f.sess)))
	form := formFields(*p.Doc.Form)
	for k, v := range formFields(view.Form{Sets: []view.FieldSet{{Rows: p.Doc.Fields}}}) {
		form[k] = v
	}
	form.Set("summary", "Typed")
	form.Set("description", "First")
	resp := f.h.do(autosave(list+"/new", f, form))
	at := resp.Header().Get("Location")
	if resp.Code != http.StatusSeeOther || !strings.HasPrefix(at, list+"/") || !strings.HasSuffix(at, "/edit") {
		t.Fatalf("the first auto-save of a new note: %d %q %s", resp.Code, at, resp.Body.String())
	}
	if got := f.h.do(get(at, f.sess)); got.Code != http.StatusOK {
		t.Fatalf("GET %s, where the script follows: %d", at, got.Code)
	}
	uid := strings.TrimSuffix(strings.TrimPrefix(at, list+"/"), "/edit")

	form.Set("description", "First and second")
	if resp := f.h.do(autosave(at, f, form)); resp.Code != http.StatusNoContent {
		t.Fatalf("the next auto-save, at the note's edit: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, uid+".ics"), "SUMMARY:Typed", "DESCRIPTION:First and second")
}
