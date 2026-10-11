package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б2 of the audit of 11.10.2026: a card of a .vcf that cannot be read — cut
// short, without its END:VCARD — is counted «with errors» and named in the
// preview, and the cards after it are the file's all the same. Read in one
// run, the broken card swallowed the next one: two people came out as one
// contact, and the preview showed a clean file.
func TestImportNamesACardItCannotRead(t *testing.T) {
	f := importFixture(t)
	file := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:g1\r\nFN:Grace Hopper\r\nEND:VCARD\r\n" +
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:b1\r\nFN:Cut Short\r\nTEL:+1\r\n" +
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:c1\r\nFN:Charles Babbage\r\nEND:VCARD\r\n"
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cards.vcf", []byte(file))))
	if page.Step != view.ImportStepPreview {
		t.Fatalf("step = %q notices = %+v", page.Step, page.Notices)
	}
	counts := map[string]int{}
	for _, c := range page.Counts {
		counts[string(c.Label)] = c.Count
	}
	if counts["ready"] != 2 || counts["with errors"] != 1 {
		t.Errorf("counts = %+v", page.Counts)
	}
	said := false
	for _, table := range page.Preview {
		for _, row := range table.Rows {
			line := ""
			for _, cell := range row {
				line += string(cell) + " "
			}
			if strings.Contains(line, "Cut Short") && strings.Contains(line, "END:VCARD") {
				said = true
			}
			if strings.Contains(line, "Cut Short") && strings.Contains(line, "Charles") {
				t.Errorf("two cards in one row: %q", line)
			}
		}
	}
	if !said {
		t.Errorf("the preview does not name the card it could not read: %+v", page.Preview)
	}
}
