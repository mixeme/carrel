package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// А11 of the audit of 11.10.2026 (the owner's decision of 12.10.2026: «the
// usual behaviour of a file manager: merge the contents; on a conflict — a
// question»). «Replace» of a folder moved or copied onto a taken name used
// to delete the folder standing there. Now it merges: what has a free name
// there goes in, each name taken there is a question of its own in the
// queue — a folder among them merges the same way when it is answered so —,
// and nothing that stood there is deleted.
func TestReplaceOfAFolderMerges(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"src/team/a.txt", "mine a")
	f.srv.SeedFile(filesDocs+"src/team/b.txt", "mine b")
	f.srv.SeedFile(filesDocs+"src/team/sub/c.txt", "mine c")
	f.srv.SeedFile(filesDocs+"team/b.txt", "theirs b")
	f.srv.SeedFile(filesDocs+"team/sub/d.txt", "theirs d")

	sheet := opSheetOf(t, f, "move", []string{"team"}, "src")
	fields := formFields(*sheet.Form)
	fields.Set(sheet.Target.Field, f.at(""))
	f.h.do(postForm(sheet.Form.Action, f.sess, fields))
	queue := func() *view.TransferQueue {
		t.Helper()
		return decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Queue
	}
	ask := queueRow(t, queue(), "team")
	if ask.State != view.OpIdle {
		t.Fatalf("a folder moved onto a taken name = %+v", ask)
	}

	f.h.do(postForm(option(t, ask, "transfer.overwrite"), f.sess, url.Values{}))
	if got, ok := f.srv.File(filesDocs + "team/a.txt"); !ok || got != "mine a" {
		t.Errorf("a free name did not go in: %q %v", got, ok)
	}
	if _, ok := f.srv.File(filesDocs + "src/team/a.txt"); ok {
		t.Error("the moved file stayed where it was")
	}
	if got, _ := f.srv.File(filesDocs + "team/b.txt"); got != "theirs b" {
		t.Errorf("a taken name was overwritten without its question: %q", got)
	}
	if got, ok := f.srv.File(filesDocs + "team/sub/d.txt"); !ok || got != "theirs d" {
		t.Errorf("what stood in the folder is gone: %q %v", got, ok)
	}
	b, sub := queueRow(t, queue(), "b.txt"), queueRow(t, queue(), "sub")
	if b.State != view.OpIdle || sub.State != view.OpIdle {
		t.Fatalf("the taken names are not questions: %+v, %+v", b, sub)
	}

	// The folder inside answered «Replace» merges too.
	f.h.do(postForm(option(t, sub, "transfer.overwrite"), f.sess, url.Values{}))
	if got, ok := f.srv.File(filesDocs + "team/sub/c.txt"); !ok || got != "mine c" {
		t.Errorf("the inner folder did not merge: %q %v", got, ok)
	}
	if got, ok := f.srv.File(filesDocs + "team/sub/d.txt"); !ok || got != "theirs d" {
		t.Errorf("the inner merge deleted what stood there: %q %v", got, ok)
	}

	// The file answered «Skip» stays where it came from.
	f.h.do(postForm(option(t, b, "transfer.skip"), f.sess, url.Values{}))
	if got, _ := f.srv.File(filesDocs + "src/team/b.txt"); got != "mine b" {
		t.Errorf("a skipped file left its place: %q", got)
	}
	if got, _ := f.srv.File(filesDocs + "team/b.txt"); got != "theirs b" {
		t.Errorf("a skipped file overwrote: %q", got)
	}
}

// А11, the rest: a folder moved onto its name goes from where it came from
// once its questions are answered and nothing is left in it — as a file
// manager takes it away — not only when it empties at once. One question
// skipped keeps it: its file is still there.
func TestAMergedFolderGoesOnceItsQuestionsAreAnswered(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"src/team/a.txt", "mine a")
	f.srv.SeedFile(filesDocs+"src/team/b.txt", "mine b")
	f.srv.SeedFile(filesDocs+"src/team/c.txt", "mine c")
	f.srv.SeedFile(filesDocs+"team/b.txt", "theirs b")
	f.srv.SeedFile(filesDocs+"team/c.txt", "theirs c")

	sheet := opSheetOf(t, f, "move", []string{"team"}, "src")
	fields := formFields(*sheet.Form)
	fields.Set(sheet.Target.Field, f.at(""))
	f.h.do(postForm(sheet.Form.Action, f.sess, fields))
	queue := func() *view.TransferQueue {
		t.Helper()
		return decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Queue
	}
	f.h.do(postForm(option(t, queueRow(t, queue(), "team"), "transfer.overwrite"), f.sess, url.Values{}))
	b, c := queueRow(t, queue(), "b.txt"), queueRow(t, queue(), "c.txt")

	f.h.do(postForm(option(t, b, "transfer.both"), f.sess, url.Values{}))
	if f.srv.Names(filesDocs+"src/team/") == nil {
		t.Fatal("the folder went while a question about its file waited")
	}
	f.h.do(postForm(option(t, c, "transfer.overwrite"), f.sess, url.Values{}))
	if names := f.srv.Names(filesDocs + "src/team/"); names != nil {
		t.Errorf("the emptied folder stayed: %v", names)
	}
	if names := f.srv.Names(filesDocs + "src/"); names == nil {
		t.Error("the folder above it went too")
	}

	// Skipped, the file keeps its folder.
	f.srv.SeedFile(filesDocs+"src/team/d.txt", "mine d")
	f.srv.SeedFile(filesDocs+"team/d.txt", "theirs d")
	sheet = opSheetOf(t, f, "move", []string{"team"}, "src")
	fields = formFields(*sheet.Form)
	fields.Set(sheet.Target.Field, f.at(""))
	f.h.do(postForm(sheet.Form.Action, f.sess, fields))
	f.h.do(postForm(option(t, queueRow(t, queue(), "team"), "transfer.overwrite"), f.sess, url.Values{}))
	f.h.do(postForm(option(t, queueRow(t, queue(), "d.txt"), "transfer.skip"), f.sess, url.Values{}))
	if got, _ := f.srv.File(filesDocs + "src/team/d.txt"); got != "mine d" {
		t.Errorf("a skipped file left its folder: %q", got)
	}
}
