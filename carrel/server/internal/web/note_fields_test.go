package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

func docRow(t *testing.T, d view.NoteDoc, name string) view.FormRow {
	t.Helper()
	for _, r := range d.Fields {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no field %q in %+v", name, d.Fields)
	return view.FormRow{}
}

// review Р-10, note (s-notes-4): Date and Time — the journal's DTSTART —,
// Tags and Related to beside the text. NoteDoc carried the title and the body
// only; its Fields carry the rest (C-6). A date without a time is a day.
func TestNoteDateTagsRelated(t *testing.T) {
	f := calendarFixture(t, nil,
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nDTSTART;VALUE=DATE:20260110\nSUMMARY:Plan\nDESCRIPTION:Body\nCATEGORIES:work\nX-KEEP:yes\nEND:VJOURNAL"),
		"n2.ics", ics("BEGIN:VJOURNAL\nUID:n2\nDTSTAMP:20260101T000000Z\nSUMMARY:Other note\nEND:VJOURNAL"))
	list := f.path("notes")
	rec := list + "/n1"
	_ = f.h.do(get(list, f.sess)) // the session loads the notebook
	edit := func() view.NoteDoc {
		t.Helper()
		return decode[view.NoteDoc](t, f.h.do(get(rec+"/edit", f.sess)))
	}
	save := func(set map[string]string) string {
		t.Helper()
		d := edit()
		form := url.Values{"summary": {string(d.Title)}, "description": {string(d.Body)}}
		for _, r := range d.Fields {
			form.Set(r.Name, string(r.Value))
		}
		for k, v := range set {
			form.Set(k, v)
		}
		href := actionHref(t, d.Bar, "record.save")
		u, _ := url.Parse(href)
		for k, v := range u.Query() {
			form[k] = v
		}
		if resp := f.h.do(postForm(u.EscapedPath(), f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return body(t, f, "n1.ics")
	}

	d := edit()
	if docRow(t, d, "start.date").Value != "2026-01-10" || docRow(t, d, "start.time").Value != "" ||
		docRow(t, d, "categories").Value != "work" || docRow(t, d, "categories").Label != "Tags" {
		t.Fatalf("fields = %+v", d.Fields)
	}
	for _, r := range d.Fields {
		if r.Name == "summary" || r.Name == "description" || r.Kind == view.InputHidden {
			t.Errorf("field %q belongs to the document or the address, not to Fields", r.Name)
		}
	}
	rel := docRow(t, d, "related")
	offered := false
	for _, o := range rel.Options {
		offered = offered || o.Value == "n2"
	}
	if !offered {
		t.Errorf("related options = %+v", rel.Options)
	}

	got := save(map[string]string{"summary": "Plan"})
	mustContain(t, got, "DTSTART;VALUE=DATE:20260110", "CATEGORIES:work", "X-KEEP:yes")

	got = save(map[string]string{"start.date": "2026-01-12", "start.time": "09:30", "categories": "work, ideas", "related": "n2"})
	mustContain(t, got, "DTSTART:20260112T093000Z", "CATEGORIES:work,ideas", "RELATED-TO:n2", "DESCRIPTION:Body")

	got = save(map[string]string{"start.time": ""})
	mustContain(t, got, "DTSTART;VALUE=DATE:20260112")

	// A refused value comes back on its field.
	d = edit()
	form := url.Values{"summary": {"Plan"}, "description": {"Body"}, "start.date": {"12 January"}}
	u, _ := url.Parse(actionHref(t, d.Bar, "record.save"))
	for k, v := range u.Query() {
		form[k] = v
	}
	page := decode[view.NotePage](t, f.h.do(postForm(u.EscapedPath(), f.sess, form)))
	if docRow(t, page.Doc, "start.date").Error == "" {
		t.Errorf("the refused date carries no error: %+v", page.Doc.Fields)
	}
	if strings.Contains(body(t, f, "n1.ics"), "12 January") {
		t.Error("a refused date was written")
	}
}
