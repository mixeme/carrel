package web_test

import (
	"net/http"
	"net/url"
	"slices"
	"testing"

	"carrel.dev/carrel/server/view"
)

// noteFormOf is what the edit of a note sends: the document's own form —
// title and text — and the fields beside it (s-notes-20).
func noteFormOf(t *testing.T, f davFixture, uid string) url.Values {
	t.Helper()
	page := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/"+uid+"/edit", f.sess)))
	form := formFields(*page.Doc.Form)
	for k, v := range formFields(view.Form{Sets: []view.FieldSet{{Rows: page.Doc.Fields}}}) {
		form[k] = v
	}
	return form
}

// A note another client wrote: its text in two DESCRIPTION lines (a journal
// may have several, RFC 5545 §3.6.3), a tag with a comma, a status, a class,
// a link and a relation Carrel's form does not show whole. One field is
// changed, and only its lines change.
func TestANoteSaveIsTheEdit(t *testing.T) {
	doc := ics("BEGIN:VJOURNAL\nUID:hard\nDTSTAMP:20260101T000000Z\nDTSTART;VALUE=DATE:20260105\nSUMMARY:Notes\n" +
		"DESCRIPTION:First\\n\nDESCRIPTION:Second part\nCATEGORIES:Smith\\, John,c\nSTATUS:DRAFT\nCLASS:PRIVATE\nX-JTX-BOARD:2\n" +
		"ATTACH;FMTTYPE=text/plain:https://elsewhere.example.org/x.txt\nRELATED-TO;RELTYPE=SIBLING:other-1\nEND:VJOURNAL")
	cases := []struct {
		name    string
		set     url.Values
		changed []string
		holds   []string
	}{
		{"nothing", url.Values{}, nil, nil},
		{"the title", url.Values{"summary": {"Notes 2"}}, []string{"SUMMARY"}, []string{"SUMMARY:Notes 2"}},
		// The text shown is the first DESCRIPTION; the second stays.
		{"the text", url.Values{"description": {"First\n\nmore"}}, []string{"DESCRIPTION"}, []string{"DESCRIPTION:First\\n\\nmore"}},
		{"one more tag", url.Values{"categories": {"Smith, John, c, d"}}, []string{"CATEGORIES"}, []string{"CATEGORIES:Smith\\, John,c,d"}},
		{"another day", url.Values{"start.date": {"2026-01-06"}}, []string{"DTSTART"}, []string{"DTSTART;VALUE=DATE:20260106"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := calendarFixture(t, nil, "hard.ics", doc)
			rec := f.path("notes") + "/hard"
			save := func(set url.Values) string {
				t.Helper()
				form := noteFormOf(t, f, "hard")
				for k, v := range set {
					form[k] = v
				}
				if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
					t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
				}
				return body(t, f, "hard.ics")
			}
			// The first save spells the record as Carrel read it (oneEdit).
			before := save(url.Values{"start.time": {"10:00"}})
			if tc.name == "another day" {
				tc.holds = []string{"20260106T100000"}
			}
			after := save(tc.set)
			if got := changedLines(before, after); !slices.Equal(got, tc.changed) {
				t.Errorf("lines changed: %v, want %v\n%s", got, tc.changed, after)
			}
			mustContain(t, after, append(tc.holds, "DESCRIPTION:Second part", "STATUS:DRAFT", "CLASS:PRIVATE", "X-JTX-BOARD:2",
				"ATTACH;FMTTYPE=text/plain:https://elsewhere.example.org/x.txt", "RELATED-TO;RELTYPE=SIBLING:other-1")...)
		})
	}
}
