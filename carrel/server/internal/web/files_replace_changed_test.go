package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б13 of the audit of 11.10.2026 (docs/plan/scans/silent-choices.md):
// «Replace» answered the question about the file that stood there when the
// question was asked — and wrote without a condition, however long the
// question had waited. A version somebody put there in between was
// overwritten unseen. The answer now holds for the version it was given
// about: another one stays, and the row says so.
func TestReplaceDoesNotOverwriteAFileChangedSince(t *testing.T) {
	ownTemp(t)
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"report.txt", "the version asked about")

	f.h.do(uploadForm(f.at("upload"), f.sess, false, upload{"report.txt", "mine"}))
	ask := queueRow(t, decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Queue, "report.txt")
	if ask.State != view.OpIdle {
		t.Fatalf("a taken name = %+v", ask)
	}
	// Somebody else writes the file while the question waits.
	f.srv.SeedFile(filesDocs+"report.txt", "written by another client meanwhile")

	// Replace of an upload brings the file again (S-17.3).
	f.h.do(answerUpload(t, ask, "transfer.overwrite", f.sess, upload{"report.txt", "mine"}))
	if got, _ := f.srv.File(filesDocs + "report.txt"); got != "written by another client meanwhile" {
		t.Errorf("the newer version was overwritten: %q", got)
	}
	row := queueRow(t, decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Queue, "report.txt")
	if row.State != view.OpFailed || !strings.Contains(string(row.Reason), "changed") {
		t.Errorf("the row does not say why: %+v", row)
	}
}

// The same of a file moved onto a taken name.
func TestReplaceByAMoveDoesNotOverwriteAFileChangedSince(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFile(filesDocs+"b.txt", "the version asked about")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	ren := opSheet(t, f, rowMenu(t, fileRow(t, page, "a.txt"), "file.rename"), url.Values{})
	f.h.do(postForm(confirmHref(t, ren), f.sess, url.Values{"name": {"b.txt"}}))
	ask := queueRow(t, decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Queue, "b.txt")
	if ask.State != view.OpIdle {
		t.Fatalf("rename onto a taken name = %+v", ask)
	}
	f.srv.SeedFile(filesDocs+"b.txt", "written by another client meanwhile")

	f.h.do(postForm(option(t, ask, "transfer.overwrite"), f.sess, url.Values{}))
	if got, _ := f.srv.File(filesDocs + "b.txt"); got != "written by another client meanwhile" {
		t.Errorf("the newer version was overwritten: %q", got)
	}
	if got, _ := f.srv.File(filesDocs + "a.txt"); got != "alpha" {
		t.Errorf("the file that was to move is gone: %q", got)
	}
}
