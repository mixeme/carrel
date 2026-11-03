package web_test

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Б42 of the audit of 11.10.2026: the folder of attachments renamed or moved
// in Files kept the setting at its old path — the next large attachment
// made the folder there again —, and deleting it, or a published folder,
// was not checked. The setting now goes with the folder, and the sheet of a
// deletion says what the folder is for.
func TestTheAttachmentFolderGoesWithItsFolder(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"carrel/attachments/2026-08/a.pdf", "pdf")
	key, _ := f.sess.DataKey()
	if err := f.h.users.SetAttachmentFolder(f.sess.AccountID(), key, sealed.AttachmentFolder{AccountID: f.id, Path: filesDocs, Folder: []string{"carrel", "attachments"}}); err != nil {
		t.Fatal(err)
	}

	// The sheet of deleting the folder above it says it holds the
	// attachments.
	del := opSheetOf(t, f, "delete", []string{"carrel"})
	if !strings.Contains(plainText(del.Warning), "attachments") {
		t.Errorf("the deletion says nothing of the attachment folder: %q", plainText(del.Warning))
	}
	// The owner's words of 03.11.2026: which attachment comes next, and where
	// the folder comes back.
	want := view.Reference{What: "Attachments", Then: "the attachment folder is in it; the next attachment too large to go inside its record creates it again, empty, at the same path"}
	if !slices.Contains(del.References, want) {
		t.Errorf("references = %+v, want %+v", del.References, want)
	}

	// Renamed: the setting names the new path.
	ren := opSheetOf(t, f, "rename", []string{"carrel"})
	f.h.do(postForm(confirmHref(t, ren), f.sess, url.Values{"name": {"store"}}))
	if _, ok := f.srv.File(filesDocs + "store/attachments/2026-08/a.pdf"); !ok {
		t.Fatal("the folder was not renamed")
	}
	got := catalogOf(t, f).Attachments
	if got == nil || !slices.Equal(got.Folder, []string{"store", "attachments"}) {
		t.Errorf("attachments = %+v, want store/attachments", got)
	}

	// A folder elsewhere renamed leaves it alone.
	f.srv.SeedFolder(filesDocs + "misc/")
	ren = opSheetOf(t, f, "rename", []string{"misc"})
	f.h.do(postForm(confirmHref(t, ren), f.sess, url.Values{"name": {"other"}}))
	if got := catalogOf(t, f).Attachments; !slices.Equal(got.Folder, []string{"store", "attachments"}) {
		t.Errorf("an unrelated rename moved the setting: %+v", got)
	}
	_ = view.FileDelete
}
