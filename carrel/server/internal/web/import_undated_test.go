package web_test

import (
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// Б35: a note without front matter whose file carries no time — a member
// of an archive with DOS date 0, a file sent by a browser — is dated the
// day of the import by the server's clock, and the preview says so instead
// of «date from the file». Before, the archive's member was dated 30
// November 1979 (the DOS date 0 read as a date) and the browser's file by
// the machine's clock.
func TestANoteWhoseFileHasNoTimeIsDatedTheDayOfTheImport(t *testing.T) {
	f := notebookFixture(t)
	f.h.srv.SetClock(time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC))
	const undated = "Ready — no front matter: title from the first heading; the file has no date — dated today"

	page := freshImport(t, f)
	archive := zipAt(t, map[string]time.Time{"idei/crdt.md": {}}, "idei/crdt.md", "# Что почитать про CRDT\n\nСсылки.\n")
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "заметки.zip", archive)))
	if row := previewRow(t, page.Preview, "Что почитать про CRDT"); row[2] != "10 August 2026" || row[3] != undated {
		t.Errorf("a member without a time = %q", row)
	}

	page = freshImport(t, f)
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "doroga.md", []byte("# Про дорогу до дачи\n\nПоворот после моста.\n"))))
	if row := previewRow(t, page.Preview, "Про дорогу до дачи"); row[2] != "10 August 2026" || row[3] != undated {
		t.Errorf("a file sent by the browser = %q", row)
	}
}
