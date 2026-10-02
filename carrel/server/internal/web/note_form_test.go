package web_test

import (
	"encoding/json"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The client's scan of the notes, НД1 (U-8): a note in edit is sent without
// scripts. The document is a form — where it goes and how, its hidden rows,
// and the names its title and text go under — so what the person typed
// reaches the server under no name the client would have to know; the
// fields beside the text go with the same form, and Save of the bar sends it
// (Action.Submits). It was the bar's Save alone, its address carrying the
// version, and the server read summary and description nobody named.
func TestNoteEditIsAForm(t *testing.T) {
	f := calendarFixture(t, nil,
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nDTSTART;VALUE=DATE:20260110\nSUMMARY:Plan\nDESCRIPTION:Body\nCATEGORIES:work\nX-KEEP:yes\nEND:VJOURNAL"))
	list := f.path("notes")
	rec := list + "/n1"
	_ = f.h.do(get(list, f.sess)) // the session loads the notebook

	raw := f.h.do(get(rec+"/edit", f.sess))
	var generic struct {
		Doc map[string]json.RawMessage `json:"doc"`
	}
	if err := json.Unmarshal(raw.Body.Bytes(), &generic); err != nil {
		t.Fatal(err)
	}
	if v, ok := generic.Doc["form"]; !ok || string(v) == "null" {
		t.Fatalf("the edit of a note has no form: without scripts nothing names where its title and text go (НД1)")
	}
	page := decode[view.NotePage](t, raw)
	form := page.Doc.Form
	if form.Action != rec+"/edit" || form.Method != "POST" {
		t.Errorf("form goes %s %s, want POST %s", form.Method, form.Action, rec+"/edit")
	}
	rows := map[string]view.FormRow{}
	for _, set := range form.Sets {
		for _, r := range set.Rows {
			rows[r.Name] = r
		}
	}
	for _, name := range []string{"_csrf", "etag", "readat"} {
		if rows[name].Kind != view.InputHidden {
			t.Errorf("hidden %s = %+v", name, rows[name])
		}
	}
	if r := rows["summary"]; r.Kind != view.InputText || r.Value != page.Doc.Title {
		t.Errorf("the title's row = %+v, want the document's title %q", r, page.Doc.Title)
	}
	if r := rows["description"]; r.Kind != view.InputArea || r.Value != page.Doc.Body || !r.Focus {
		t.Errorf("the text's row = %+v, want the document's text with the caret (s-notes-20)", r)
	}
	for _, r := range page.Doc.Fields {
		if _, twice := rows[r.Name]; twice {
			t.Errorf("field %q stands in the document's form and in Fields", r.Name)
		}
	}
	save := actionByID(page.Doc.Bar, "record.save")
	if save == nil || !save.Submits {
		t.Errorf("Save of the bar = %+v, want the button that sends the document's form", save)
	}

	// Sent as a browser without scripts sends it.
	values := formFields(*form)
	for k, v := range formFields(view.Form{Sets: []view.FieldSet{{Rows: page.Doc.Fields}}}) {
		values[k] = v
	}
	values.Set("description", "Body, typed again")
	if resp := f.h.do(postForm(form.Action, f.sess, values)); resp.Code != 303 {
		t.Fatalf("POST the form: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "n1.ics"), "SUMMARY:Plan", "DESCRIPTION:Body\\, typed again", "CATEGORIES:work", "X-KEEP:yes")

	// A new note is the same form at its own address.
	fresh := decode[view.NotePage](t, f.h.do(get(list+"/new", f.sess)))
	if fresh.Doc.Form == nil || fresh.Doc.Form.Action != list+"/new" {
		t.Fatalf("the new note's form = %+v", fresh.Doc.Form)
	}
	values = formFields(*fresh.Doc.Form)
	for k, v := range formFields(view.Form{Sets: []view.FieldSet{{Rows: fresh.Doc.Fields}}}) {
		values[k] = v
	}
	values.Set("summary", "Fresh")
	values.Set("description", "A new text")
	if resp := f.h.do(postForm(fresh.Doc.Form.Action, f.sess, values)); resp.Code != 303 {
		t.Fatalf("POST the new form: %d %s", resp.Code, resp.Body.String())
	}
	found := false
	for _, p := range f.srv.Paths(f.col) {
		if b, _, ok := f.srv.Object(p); ok && strings.Contains(unfolded(b), "SUMMARY:Fresh") {
			found = true
		}
	}
	if !found {
		t.Error("the new note was not written")
	}
}
