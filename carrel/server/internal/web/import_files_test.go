package web_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// importFiles is the import form with several files under «file», name and
// body in turn, as a drop of several sends them.
func importFiles(path string, sess *session.Session, files ...string) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("_csrf", sess.CSRFToken())
	for i := 0; i+1 < len(files); i += 2 {
		part, _ := w.CreateFormFile("file", files[i])
		_, _ = part.Write([]byte(files[i+1]))
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	return req
}

// Им5 of the scan of 15.10.2026 (s-io-1: only notes take more than one file
// at once): the second file of a drop was lost without a word — the first
// alone was read. Notes take every file as one set, each named, as a zip's
// are; the other kinds refuse the drop whole and say why. Им14: an empty
// file is no note by every way in — the set skips it, a file alone is
// refused with the same words, not «No file was chosen».
func TestImportTakesEveryFileOfADrop(t *testing.T) {
	f := notebookFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	choose := actionHref(t, page.Actions, "import.choose")
	page = decode[view.ImportPage](t, f.h.do(importFiles(choose, f.sess,
		"podryadchik.md", "---\ntitle: Встреча с подрядчиком\n---\n\nТекст\n",
		"crdt.md", "# Что почитать про CRDT\n\nСсылки.\n",
		"skan.pdf", "%PDF",
		"pustoj.md", "")))
	if page.Step != view.ImportStepPreview {
		t.Fatalf("step %q, notices %+v", page.Step, page.Notices)
	}
	if got := saidRuns(page.Subtitle); !strings.Contains(got, "4 files") {
		t.Errorf("subtitle = %q", got)
	}
	for title, want := range map[string]string{
		// Where the title and the date come from (Им12; s-io-8).
		"Встреча с подрядчиком": "Ready — title, date and tags from front matter",
		"Что почитать про CRDT": "Ready — no front matter: title from the first heading, date from the file",
		"skan.pdf":              "Skipped: not Markdown",
		"pustoj.md":             "Empty file: nothing to store and nothing to name it by",
	} {
		if row := previewRow(t, page.Preview, title); string(row[3]) != want {
			t.Errorf("%s = %q, want %q", title, row, want)
		}
	}

	empty := decode[view.ImportPage](t, f.h.do(importFiles(choose, f.sess, "pustoj.md", "  \n")))
	if len(empty.Notices) != 1 || !strings.Contains(string(empty.Notices[0].Body), "Empty file") {
		t.Errorf("an empty file alone: %+v", empty.Notices)
	}

	c := importFixture(t)
	page = decode[view.ImportPage](t, c.h.do(get(c.path("contacts")+"/import", c.sess)))
	page = decode[view.ImportPage](t, c.h.do(importFiles(actionHref(t, page.Actions, "import.choose"), c.sess,
		"one.vcf", "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:one\r\nFN:One\r\nEND:VCARD\r\n",
		"two.vcf", "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:two\r\nFN:Two\r\nEND:VCARD\r\n")))
	if page.Step == view.ImportStepPreview || len(page.Notices) != 1 || page.Notices[0].Title != "Nothing was read." ||
		page.Notices[0].Body != "One file at a time: only notes take several." {
		t.Errorf("two address books at once: step %q, notices %+v", page.Step, page.Notices)
	}
}
