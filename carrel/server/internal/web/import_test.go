package web_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// The file another program exported: Ada, whose identity People already
// holds, and Grace, whom it does not.
const importedCards = "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:ada\r\nFN:Ада Лавлейс\r\nX-CUSTOM:keep\r\nEND:VCARD\r\n" +
	"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:grace\r\nFN:Grace Hopper\r\nEND:VCARD\r\n"

// importedCalendar holds one event and one task: a calendar file of another
// program does not sort itself by the sections of Carrel.
const importedCalendar = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n" +
	"BEGIN:VEVENT\r\nUID:meet\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260105T100000Z\r\nSUMMARY:Ковка\r\nEND:VEVENT\r\n" +
	"BEGIN:VTODO\r\nUID:buy\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Купить уголь\r\nEND:VTODO\r\n" +
	"END:VCALENDAR\r\n"

// importFixture is People with Ada in it, an empty Friends and a read-only
// Archive on one server.
func importFixture(t *testing.T) davFixture {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	srv.AddAddressBook(carddav.AddressBook{Path: peopleBook, Name: "People"})
	srv.AddAddressBook(carddav.AddressBook{Path: friendsBook, Name: "Friends"})
	srv.AddAddressBook(carddav.AddressBook{Path: archiveBook, Name: "Archive", ReadOnly: true})
	srv.Seed(peopleBook+"ada.vcf", movedCard)
	return connectFixture(t, srv, peopleBook, nil)
}

// importRequest sends a file the way the page does: without scripts _csrf
// comes first in the body, with them it goes in the header.
func importRequest(path string, sess *session.Session, header bool, name string, data []byte) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if !header {
		_ = w.WriteField("_csrf", sess.CSRFToken())
	}
	if name != "" {
		part, _ := w.CreateFormFile("file", name)
		_, _ = part.Write(data)
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if header {
		req.Header.Set("X-CSRF-Token", sess.CSRFToken())
	}
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	return req
}

func importOption(t *testing.T, page view.ImportPage, label string) view.TargetOption {
	t.Helper()
	for _, o := range page.Into.Options {
		if string(o.Label) == label {
			return o
		}
	}
	t.Fatalf("no collection %q in %+v", label, page.Into.Options)
	return view.TargetOption{}
}

// tableRow is the row of a preview or a report whose first column is what.
func tableRow(t *testing.T, tables []view.ImportTable, rubric, what string) []view.Text {
	t.Helper()
	for _, tb := range tables {
		if !strings.Contains(string(tb.Rubric), rubric) {
			continue
		}
		for _, row := range tb.Rows {
			if strings.Contains(string(row[0]), what) {
				return row
			}
		}
	}
	t.Fatalf("no row %q under %q in %+v", what, rubric, tables)
	return nil
}

// S-19: the destination is chosen and it is what the preview counts against;
// choosing another one counts the same file again.
func TestImportPreviewCountsAgainstTheChosenCollection(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	if page.Step != view.ImportStepSource || len(page.Preview) != 0 {
		t.Fatalf("first step = %q with %d tables", page.Step, len(page.Preview))
	}
	if len(page.Source.Kinds) != 1 || page.Source.Kinds[0] != ".vcf" || page.Source.Folder != nil {
		t.Errorf("source = %+v", page.Source)
	}
	archive := importOption(t, page, "Archive")
	if !archive.ReadOnly || archive.Reason != "Read-only" {
		t.Errorf("Archive = %+v", archive)
	}
	if page.Into.Chosen != importOption(t, page, "People").ID {
		t.Errorf("chosen = %q, want People", page.Into.Chosen)
	}

	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cards.vcf", []byte(importedCards))))
	page = asCopies(t, f, page)
	if page.Step != view.ImportStepPreview {
		t.Fatalf("step = %q", page.Step)
	}
	// The preview of each kind is its own: a card is told from a card by its
	// address, an event by its time.
	if cols := page.Preview[0].Columns; len(cols) != 4 || cols[2] != "UID" {
		t.Errorf("columns = %v", cols)
	}
	if row := previewRow(t, page.Preview, "Ада"); row[3] != "New UID will be assigned" {
		t.Errorf("Ada against People = %q", row[3])
	}
	if row := previewRow(t, page.Preview, "Grace"); row[3] != "Ready" {
		t.Errorf("Grace against People = %q", row[3])
	}

	// The same file against an empty collection: nothing is taken there.
	page = decode[view.ImportPage](t, f.h.do(get(importOption(t, page, "Friends").ID, f.sess)))
	if page.Step != view.ImportStepPreview {
		t.Fatalf("the file was lost when the collection changed: step = %q", page.Step)
	}
	for _, who := range []string{"Ада", "Grace"} {
		if row := previewRow(t, page.Preview, who); row[3] != "Ready" {
			t.Errorf("%s against Friends = %q", who, row[3])
		}
	}
}

// S-19: an import always creates. The record whose identity is taken gets a
// new one and a line in the report; the record already there is not touched.
func TestImportAlwaysCreates(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cards.vcf", []byte(importedCards))))
	page = asCopies(t, f, page)
	page = decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{})))

	if page.Step != view.ImportStepReport || page.Report == nil {
		t.Fatalf("step = %q report = %+v", page.Step, page.Report)
	}
	if page.Report.Created != 2 || page.Report.Renamed != 1 || page.Report.Skipped != 0 {
		t.Fatalf("report = %+v", page.Report)
	}
	if !strings.Contains(string(page.Report.Note), "Nothing there was overwritten") {
		t.Errorf("note = %q", page.Report.Note)
	}
	if paths := f.srv.Paths(peopleBook); len(paths) != 3 {
		t.Fatalf("People holds %v", paths)
	}
	body, _, ok := f.srv.Object(peopleBook + "ada.vcf")
	if !ok || !strings.Contains(body, "X-CUSTOM:keep") || strings.Contains(body, "Ада") {
		t.Errorf("the record that was there changed: %q", body)
	}
	// The card that arrived under a new identity is a card all the same.
	row := tableRow(t, page.Report.Rows, "Created", "Ада")
	if !strings.Contains(string(row[2]), "already held the identity") {
		t.Errorf("row = %q", row[2])
	}
	if f.srv.CountRequests("PUT", peopleBook) != 2 {
		t.Errorf("requests = %v", f.srv.Requests())
	}
}

// A server that keeps a new card under an identity of its own (Google,
// docs/servers.md, SV-25): the report names the identity the card has, not
// the one it was sent with, and counts it among the renamed. A card the
// server does not show at all is not reported as simply created.
func TestImportNamesTheIdentityTheServerGave(t *testing.T) {
	run := func(t *testing.T, f davFixture) view.ImportPage {
		t.Helper()
		page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
		page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cards.vcf", []byte(importedCards))))
		page = asCopies(t, f, page)
		return decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{})))
	}
	t.Run("rewritten", func(t *testing.T) {
		f := importFixture(t)
		f.srv.CreateRewritesUID = true
		page := run(t, f)
		if page.Report == nil || page.Report.Created != 2 || page.Report.Renamed != 2 {
			t.Fatalf("report = %+v", page.Report)
		}
		row := tableRow(t, page.Report.Rows, "Created", "Grace")
		if !strings.Contains(string(row[2]), "Created under sv25-") || !strings.Contains(string(row[2]), "identity of its own") {
			t.Errorf("grace row = %q", row[2])
		}
		if !strings.Contains(string(page.Report.Note), "2 records under a new identity") {
			t.Errorf("note = %q", page.Report.Note)
		}
	})
	t.Run("hidden", func(t *testing.T) {
		f := importFixture(t)
		f.srv.CreateHidesTheRecord = true
		page := run(t, f)
		row := tableRow(t, page.Report.Rows, "Created", "Grace")
		if !strings.Contains(string(row[2]), "does not show it") {
			t.Errorf("grace row = %q", row[2])
		}
	})
	t.Run("honest", func(t *testing.T) {
		f := importFixture(t)
		page := run(t, f)
		if row := tableRow(t, page.Report.Rows, "Created", "Grace"); string(row[2]) != "Created" {
			t.Errorf("grace row = %q", row[2])
		}
		if n := f.srv.CountRequests("GET", peopleBook); n != 0 {
			t.Errorf("%d GETs: a record found under its identity was read again", n)
		}
	})
}

// The draft is dropped once it is written: the report is not a second import
// waiting to happen.
func TestImportRunsOnce(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	run := actionHref(t, page.Actions, "import.choose")
	page = decode[view.ImportPage](t, f.h.do(importRequest(run, f.sess, true, "cards.vcf", []byte(importedCards))))
	page = asCopies(t, f, page)
	run = actionHref(t, page.Actions, "import.run")
	if page = decode[view.ImportPage](t, f.h.do(postForm(run, f.sess, url.Values{}))); page.Report.Created != 2 {
		t.Fatalf("report = %+v", page.Report)
	}
	page = decode[view.ImportPage](t, f.h.do(postForm(run, f.sess, url.Values{})))
	if page.Step != view.ImportStepSource || len(page.Notices) == 0 {
		t.Fatalf("a second run: step = %q notices = %+v", page.Step, page.Notices)
	}
	if paths := f.srv.Paths(peopleBook); len(paths) != 3 {
		t.Fatalf("People holds %v", paths)
	}
}

// The preview holds a file in the session, so its Cancel is the server's: it
// drops the file and leaves for the collection, as the frame draws it. The
// next import there starts from the choice of a file, not from the old one.
func TestImportCancelDropsTheFile(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cards.vcf", []byte(importedCards))))
	var cancel view.Action
	for _, a := range page.Actions {
		if a.ID == "import.cancel" {
			cancel = a
		}
	}
	if cancel.Label != "Cancel" || cancel.Method != "POST" {
		t.Fatalf("preview cancel = %+v", cancel)
	}
	rec := f.h.do(postForm(cancel.Href, f.sess, url.Values{}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != f.path("contacts") {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	page = decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	if page.Step != view.ImportStepSource {
		t.Fatalf("the cancelled file came back: step = %q", page.Step)
	}
	if paths := f.srv.Paths(peopleBook); len(paths) != 1 {
		t.Fatalf("People holds %v", paths)
	}
}

// S-19: a record of another kind in the same file is skipped and named, with
// where it belongs. Nothing goes missing in silence.
func TestImportNamesWhatItDoesNotWrite(t *testing.T) {
	f := calendarFixture(t, nil)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("calendar")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cal.ics", []byte(importedCalendar))))

	if row := previewRow(t, page.Preview, "VTODO"); !strings.HasSuffix(string(row[3]), "import it into a list of tasks") {
		t.Errorf("the task was not sent anywhere: %q", row)
	}
	if row := previewRow(t, page.Preview, "Ковка"); row[3] != "Ready" || !strings.Contains(string(row[2]), "January") {
		t.Errorf("event = %q", row)
	}
	if cols := page.Preview[0].Columns; len(cols) != 4 || cols[2] != "When" {
		t.Errorf("columns = %v", cols)
	}
	page = decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{})))
	if page.Report.Created != 1 || page.Report.Skipped != 1 {
		t.Fatalf("report = %+v", page.Report)
	}
	// Им15 (s-io-14): the report names what it left out, title first.
	if sk := page.Report.Skips; len(sk) != 1 || sk[0] != "Купить уголь · VTODO: a calendar takes events — import it into a list of tasks" {
		t.Errorf("skips = %q", sk)
	}
	if paths := f.srv.Paths(davtest.CalendarHome + "work/"); len(paths) != 1 {
		t.Fatalf("the calendar holds %v", paths)
	}
}

// A file over the ceiling is refused whole: half of it is not written and
// then called an import (C-12).
func TestImportRefusesAFileOverTheCeiling(t *testing.T) {
	f := importFixture(t)
	f.h.cfg.ImportMaxRecords = 1
	f.h.rebuild()
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cards.vcf", []byte(importedCards))))
	if page.Step != view.ImportStepSource || !hasNotice(page.Notices, "Nothing was imported") {
		t.Fatalf("step = %q notices = %+v", page.Step, page.Notices)
	}
	if paths := f.srv.Paths(peopleBook); len(paths) != 1 {
		t.Fatalf("something was written: %v", paths)
	}
}

func TestImportWithoutAFileSaysSo(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "", nil)))
	if !hasNotice(page.Notices, "No file was chosen") {
		t.Fatalf("notices = %+v", page.Notices)
	}
}

// Notes have a second source: a folder of Markdown files on a server of this
// person (S-19).
func TestImportNotesFromAFolder(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	notebook := davtest.CalendarHome + "notes/"
	srv.AddCalendar(caldav.Calendar{Path: notebook, Name: "Notebook",
		SupportedComponentSet: []string{"VJOURNAL"}})
	srv.SeedFolder(davtest.FileRoot + "docs/")
	srv.SeedFile(davtest.FileRoot+"docs/kovka.md", "---\ntitle: Ковка\ncategories: work\n---\n\nПервая строка\n")
	srv.SeedFile(davtest.FileRoot+"docs/plain.md", "# Уголь\n\nЗаказать мешок.\n")
	srv.SeedFile(davtest.FileRoot+"docs/photo.jpg", "not a note")
	f := connectFixture(t, srv, notebook, nil)
	addFileCollection(t, f, davtest.FileRoot+"docs/", "Docs")

	page := decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	if page.Source.Folder == nil || len(page.Source.Folder.Nodes) != 1 {
		t.Fatalf("no folder to take notes from: %+v", page.Source)
	}
	// The folder is chosen in the tree, and «Preview folder» takes it
	// (s-io-4).
	page = decode[view.ImportPage](t, f.h.do(get(page.Source.Folder.Nodes[0].ID, f.sess)))
	page = decode[view.ImportPage](t, f.h.do(postForm(page.Source.Preview.Href, f.sess, url.Values{})))
	if page.Step != view.ImportStepPreview {
		t.Fatalf("step = %q notices = %+v", page.Step, page.Notices)
	}
	if row := previewRow(t, page.Preview, "Ковка"); row[0] != "kovka.md" || row[3] != "Ready" {
		t.Errorf("Ковка = %q", row)
	}
	if row := previewRow(t, page.Preview, "Уголь"); row[0] != "plain.md" || row[3] != "Ready" {
		t.Errorf("Уголь = %q", row)
	}
	if row := previewRow(t, page.Preview, "photo.jpg"); row[3] != "Skipped: not Markdown" {
		t.Errorf("the photo = %q", row)
	}

	page = decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{})))
	if page.Report.Created != 2 {
		t.Fatalf("report = %+v", page.Report)
	}
	paths := f.srv.Paths(notebook)
	if len(paths) != 2 {
		t.Fatalf("the notebook holds %v", paths)
	}
	body, _, _ := f.srv.Object(paths[0])
	if !strings.Contains(body, "VJOURNAL") {
		t.Errorf("what was written is no note: %q", body)
	}
}

// addFileCollection puts a file collection of this connection in the catalog,
// as discovery would.
func addFileCollection(t *testing.T, f davFixture, path, name string) {
	t.Helper()
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{
			AccountID: f.id, Path: path, Name: name, Kinds: []view.Kind{view.KindFile},
		})
	})
}
