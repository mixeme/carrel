package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Им3 of the scan of import (s-io-1): a file dropped on a collection's list
// opens the wizard on its preview, the collection chosen; before the drop
// the list says what will happen, where to, and that nothing is written
// yet, and refuses a file of another kind with where it belongs.
func TestADroppedFileOpensThePreview(t *testing.T) {
	f := bookFixture(t, nil, adaCard())
	list := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	d := list.Bar.Drop
	if d == nil {
		t.Fatal("the list takes no drop")
	}
	if d.Lead != "Drop to import" || d.Many || len(d.Accept) != 1 || d.Accept[0] != ".vcf" || d.Action.File != "file" || d.Action.Method != "POST" ||
		!strings.HasPrefix(saidRuns(d.Line), "One ⟨mono:.vcf⟩ file into ") || d.Note != "The preview opens first: nothing is written until it has been read" {
		t.Errorf("drop = %+v, line %q", d, saidRuns(d.Line))
	}
	says := map[string]string{}
	for _, e := range d.Elsewhere {
		says[string(e.Ext)] = saidRuns(e.Says)
	}
	if says[".ics"] != "Contacts take ⟨mono:.vcf⟩ — the calendar, the list of tasks and a notebook take this one" {
		t.Errorf("elsewhere = %q", says)
	}

	page := decode[view.ImportPage](t, f.h.do(importRequest(d.Action.Href, f.sess, false, "people.vcf",
		[]byte("BEGIN:VCARD\nVERSION:3.0\nUID:grace\nFN:Grace Hopper\nEND:VCARD\n"))))
	if page.Step != view.ImportStepPreview || page.Into.Chosen == "" {
		t.Errorf("step %q, into %q, notices %+v", page.Step, page.Into.Chosen, page.Notices)
	}
}

// Им4 (s-io-1): a file dropped on a combined view opens the first step with
// the file in hand and «Into» empty — the cards are not laid into the first
// book that comes; choosing «Into» opens the preview of this file.
func TestADropOnACombinedViewAsksWhereItGoes(t *testing.T) {
	f := bookFixture(t, nil, adaCard())
	list := decode[view.ContactsListPage](t, f.h.do(get("/app/contacts", f.sess)))
	d := list.Bar.Drop
	if d == nil || d.Action.Href != "/app/contacts/import" || !strings.HasSuffix(saidRuns(d.Line), "— where it goes is asked next") {
		t.Fatalf("drop = %+v", d)
	}
	page := decode[view.ImportPage](t, f.h.do(importRequest(d.Action.Href, f.sess, false, "people.vcf",
		[]byte("BEGIN:VCARD\nVERSION:3.0\nUID:grace\nFN:Grace Hopper\nEND:VCARD\n"))))
	if page.Step != view.ImportStepSource || page.Into.Chosen != "" || !strings.Contains(saidRuns(page.Source.Taken), "people.vcf") {
		t.Fatalf("step %q, into %q, taken %q, notices %+v", page.Step, page.Into.Chosen, saidRuns(page.Source.Taken), page.Notices)
	}
	// Still in hand when the page is opened again.
	again := decode[view.ImportPage](t, f.h.do(get("/app/contacts/import", f.sess)))
	if !strings.Contains(saidRuns(again.Source.Taken), "people.vcf") {
		t.Errorf("the file was lost: %q", saidRuns(again.Source.Taken))
	}
	var into string
	for _, o := range page.Into.Options {
		if !o.ReadOnly {
			into = o.Href
		}
	}
	preview := decode[view.ImportPage](t, f.h.do(get(into, f.sess)))
	if preview.Step != view.ImportStepPreview || preview.Into.Chosen == "" || len(preview.Preview) != 1 || len(preview.Preview[0].Lines) != 1 {
		t.Errorf("after «Into»: step %q, into %q, preview %+v", preview.Step, preview.Into.Chosen, preview.Preview)
	}
}
