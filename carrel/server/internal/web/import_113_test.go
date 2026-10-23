package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Typecase's note 113 (task 222): the answers to the twelve questions of the
// import after note 111.

// p. 1: a file another kind takes says where it goes; one no kind takes,
// that only this kind's files are imported.
func TestImportDropSaysWhereAFileGoes(t *testing.T) {
	f := importFixture(t)
	list := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	d := list.Bar.Drop
	if d == nil {
		t.Fatal("no drop")
	}
	says := map[string]string{}
	for _, e := range d.Elsewhere {
		says[string(e.Ext)] = saidRuns(e.Says)
	}
	if says[".ics"] != "Contacts take ⟨mono:.vcf⟩ — the calendar and the list of tasks take this one" {
		t.Errorf(".ics = %q", says[".ics"])
	}
	if saidRuns(d.Refused) != "Only ⟨mono:.vcf⟩ files are imported here" || d.OneOnly == "" {
		t.Errorf("refused %q, one only %q", saidRuns(d.Refused), d.OneOnly)
	}
	c := calendarFixture(t, nil)
	agenda := decode[view.AgendaPage](t, c.h.do(get(c.path("calendar"), c.sess)))
	for _, e := range agenda.Bar.Drop.Elsewhere {
		if e.Ext == ".vcf" && saidRuns(e.Says) != "Contacts take this one" {
			t.Errorf("a calendar's .vcf = %q", saidRuns(e.Says))
		}
	}
}

// p. 10, Р-76: a card without FN is named by its N, and written so; with
// neither, «Unnamed contact». Its row says which (p. 4).
func TestImportNamesACardByItsN(t *testing.T) {
	f := importFixture(t)
	cards := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:vera\r\nN:Лебедева;Вера;;;\r\nORG:Слово\r\nEND:VCARD\r\n" +
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:nobody\r\nEMAIL:x@example.org\r\nEND:VCARD\r\n"
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "mix.vcf", []byte(cards))))
	if row := previewRow(t, page.Preview, "Вера Лебедева"); row[3] != "Ready — name from N" {
		t.Errorf("Vera = %q", row)
	}
	if row := previewRow(t, page.Preview, "Unnamed contact"); row[3] != "Ready — no name" {
		t.Errorf("nobody = %q", row)
	}
	page = decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{})))
	if page.Report == nil || page.Report.Created != 2 {
		t.Fatalf("report = %+v", page.Report)
	}
	vera, _, _ := f.srv.Object(peopleBook + "vera.vcf")
	nobody, _, _ := f.srv.Object(peopleBook + "nobody.vcf")
	if !strings.Contains(vera, "FN:Вера Лебедева") || !strings.Contains(nobody, "FN:Unnamed contact") {
		t.Errorf("written:\n%s\n%s", vera, nobody)
	}
}

// pp. 3, 9: a second record of the file under the same UID stands under
// «With a new UID» with why; a record the parser refused is «‹name› ·
// ‹words›» under «Failed».
func TestImportSaysWhyAndWhatFailed(t *testing.T) {
	f := importFixture(t)
	cards := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:twin\r\nFN:Первая\r\nEND:VCARD\r\n" +
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:twin\r\nFN:Вторая\r\nEND:VCARD\r\n"
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "twins.vcf", []byte(cards))))
	page = decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{})))
	if r := page.Report; r == nil || len(r.NewUIDs) != 1 || !strings.HasPrefix(string(r.NewUIDs[0]), "Вторая · twin → ") ||
		!strings.HasSuffix(string(r.NewUIDs[0]), " · the file holds this UID twice") || len(r.Renames) != 0 {
		t.Errorf("report = %+v", page.Report)
	}
}

// pp. 5–7: a file read and found no good begins with «Nothing was read.».
func TestImportRefusesAFileReadAndFoundNoGood(t *testing.T) {
	for _, one := range []struct {
		name, body, want string
	}{
		{"none.vcf", "BEGIN:VCARD\r\nEND:VCARD\r\n", ""},
		{"empty.vcf", "   \r\n", "«empty.vcf» is empty."},
	} {
		f := importFixture(t)
		page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
		page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, one.name, []byte(one.body))))
		if one.want == "" {
			continue // a card the parser refuses is a row, not a refusal of the file
		}
		if len(page.Notices) != 1 || page.Notices[0].Title != "Nothing was read." || string(page.Notices[0].Body) != one.want {
			t.Errorf("%s: %+v", one.name, page.Notices)
		}
	}
	c := calendarFixture(t, nil)
	page := decode[view.ImportPage](t, c.h.do(get(c.path("calendar")+"/import", c.sess)))
	page = decode[view.ImportPage](t, c.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), c.sess, false, "empty.ics",
		[]byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//T//EN\r\nEND:VCALENDAR\r\n"))))
	if len(page.Notices) != 1 || page.Notices[0].Title != "Nothing was read." || page.Notices[0].Body != "This file holds no records." {
		t.Errorf("a calendar of nothing: %+v", page.Notices)
	}
	n := notebookFixture(t)
	page = freshImport(t, n)
	page = decode[view.ImportPage](t, n.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), n.sess, false, "broken.zip", []byte("PK\x03\x04 not really"))))
	if len(page.Notices) != 1 || page.Notices[0].Body != "«broken.zip» could not be opened as a zip archive." {
		t.Errorf("a broken zip: %+v", page.Notices)
	}
	page = freshImport(t, n)
	page = decode[view.ImportPage](t, n.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), n.sess, false, "pdfs.zip", zipOf(t, "a.pdf", "%PDF"))))
	if len(page.Notices) != 1 || page.Notices[0].Body != "The archive holds no Markdown file." {
		t.Errorf("a zip without Markdown: %+v", page.Notices)
	}
}

// p. 12: a zip among the files of a set is one file of it, its members
// named in it.
func TestImportNamesTheMembersOfAZipInASet(t *testing.T) {
	n := notebookFixture(t)
	page := freshImport(t, n)
	page = decode[view.ImportPage](t, n.h.do(importFiles(actionHref(t, page.Actions, "import.choose"), n.sess,
		"one.md", "# One\n", "more.zip", string(zipOf(t, "two.md", "# Two\n")))))
	if got := saidRuns(page.Subtitle); !strings.HasPrefix(got, "2 files · ") {
		t.Errorf("subtitle = %q", got)
	}
	if row := previewRow(t, page.Preview, "Two"); row[0] != "more.zip › two.md" {
		t.Errorf("a member = %q", row)
	}
}
