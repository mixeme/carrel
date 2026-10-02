package web_test

import (
	"archive/zip"
	"bytes"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// zipOf is an archive of name → body, in that order.
func zipOf(t *testing.T, members ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for i := 0; i+1 < len(members); i += 2 {
		w, err := zw.Create(members[i])
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(members[i+1]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// freshImport is the first step of the notes' wizard, the file kept from
// before dropped by its Cancel.
func freshImport(t *testing.T, f davFixture) view.ImportPage {
	t.Helper()
	page := decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	if page.Step == view.ImportStepPreview {
		f.h.do(postForm(actionHref(t, page.Actions, "import.cancel"), f.sess, url.Values{}))
		page = decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	}
	return page
}

func notebookFixture(t *testing.T) davFixture {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	notebook := davtest.CalendarHome + "notes/"
	srv.AddCalendar(caldav.Calendar{Path: notebook, Name: "Notebook", SupportedComponentSet: []string{"VJOURNAL"}})
	return connectFixture(t, srv, notebook, nil)
}

// s-io-8 and S-29.4: notes come from a zip of Markdown files too — the
// files of its folders as well, each row naming its file —, what is not
// Markdown or is empty stands in the table with why, and the archive is
// read under the ceilings of an import: the bytes unpacked and the members
// counted before anything is parsed; a name that would climb out of the
// archive is only a name — nothing is written by it.
func TestImportNotesFromAZip(t *testing.T) {
	f := notebookFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	kinds := ""
	for _, k := range page.Source.Kinds {
		kinds += string(k) + " "
	}
	if !strings.Contains(kinds, ".zip") {
		t.Errorf("the notes take %s", kinds)
	}
	archive := zipOf(t,
		"2026-03-11-podryadchik.md", "---\ntitle: Встреча с подрядчиком\n---\n\nТекст\n",
		"idei/crdt.md", "# Что почитать про CRDT\n\nСсылки.\n",
		"skan.pdf", "%PDF",
		"pustoj.md", "",
		"../../evil.md", "# Злой\n",
	)
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "заметки-2026.zip", archive)))
	if page.Step != view.ImportStepPreview {
		t.Fatalf("step %q, notices %+v", page.Step, page.Notices)
	}
	if got := saidRuns(page.Subtitle); !strings.Contains(got, "заметки-2026.zip") || !strings.Contains(got, "5 files") {
		t.Errorf("subtitle = %q", got)
	}
	if row := previewRow(t, page.Preview, "Встреча с подрядчиком"); row[0] != "2026-03-11-podryadchik.md" || row[3] != "Ready" {
		t.Errorf("first = %q", row)
	}
	if row := previewRow(t, page.Preview, "Что почитать про CRDT"); row[0] != "idei/crdt.md" || row[3] != "Ready" {
		t.Errorf("in a folder = %q", row)
	}
	if row := previewRow(t, page.Preview, "skan.pdf"); row[3] != "Skipped: not Markdown" {
		t.Errorf("pdf = %q", row)
	}
	if row := previewRow(t, page.Preview, "pustoj.md"); row[3] != "Empty file: nothing to store and nothing to name it by" {
		t.Errorf("empty = %q", row)
	}
	if row := previewRow(t, page.Preview, "evil"); !strings.HasPrefix(string(row[3]), "Skipped:") {
		t.Errorf("a name out of the archive = %q", row)
	}

	// Past the ceilings: more members than an import takes, more bytes
	// unpacked than it takes — refused whole.
	f.h.cfg.ImportMaxRecords = 2
	page = freshImport(t, f)
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "many.zip", zipOf(t, "a.md", "# A", "b.md", "# B", "c.md", "# C"))))
	if page.Step != view.ImportStepSource || !hasNotice(page.Notices, "Nothing was imported") {
		t.Errorf("three members past a ceiling of two: step %q, %+v", page.Step, page.Notices)
	}
	f.h.cfg.ImportMaxRecords, f.h.cfg.ImportMaxBytes = 100, 4<<10
	bomb := zipOf(t, "big.md", "# Big\n\n"+strings.Repeat("a", 64<<10))
	page = freshImport(t, f)
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "bomb.zip", bomb)))
	if len(bomb) > 4<<10 || page.Step != view.ImportStepSource || !hasNotice(page.Notices, "Nothing was imported") {
		t.Errorf("64 KB unpacked from %d bytes past a ceiling of 4 KB: step %q, %+v", len(bomb), page.Step, page.Notices)
	}
}
