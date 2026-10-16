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
