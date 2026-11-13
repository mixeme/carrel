package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б42 of the audit of 11.10.2026, its rest: a published file or folder
// renamed or moved in Files kept its link at the old path, and the guest read
// «No longer there», while a record's link goes where Carrel moves it (Б41).
// Now a link goes where Files takes what it opens, and so does a link to
// anything inside a folder renamed or moved.

func downloads(t *testing.T, f davFixture, link, want string) {
	t.Helper()
	if got := f.h.do(anon(link + "/download")); got.Code != http.StatusOK || got.Body.String() != want {
		t.Errorf("%s/download: %d %q, want %q", link, got.Code, got.Body.String(), want)
	}
}

func noLinkGone(t *testing.T, f davFixture) {
	t.Helper()
	page := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess)))
	for _, r := range page.Rows {
		if r.Gone != "" {
			t.Errorf("%s is called dead: %q", r.Title, r.Gone)
		}
	}
}

func TestAPublishedFileFollowsARename(t *testing.T) {
	f := pubDocs(t)
	file := publishFile(t, f, "share/readme.txt", false)
	folder := publishFile(t, f, "share/sub", true)
	other := publishFile(t, f, "secret.txt", false)

	// The folder that holds both renamed: both links open them there.
	ren := opSheetOf(t, f, "rename", []string{"share"})
	f.h.do(postForm(confirmHref(t, ren), f.sess, url.Values{"name": {"shared"}}))
	if _, ok := f.srv.File(filesDocs + "shared/readme.txt"); !ok {
		t.Fatal("the folder was not renamed")
	}
	downloads(t, f, file, "hello from share")
	if page := publicPage(t, f, folder); page.Folder == nil || len(page.Folder.Rows) != 1 {
		t.Errorf("the folder link after the rename: %+v, notices %+v", page.Folder, page.Notices)
	}

	// The file itself renamed.
	ren = opSheetOf(t, f, "rename", []string{"readme.txt"}, "shared")
	f.h.do(postForm(confirmHref(t, ren), f.sess, url.Values{"name": {"README.txt"}}))
	downloads(t, f, file, "hello from share")
	// A link to something else stays where it was.
	downloads(t, f, other, "the neighbour")
	noLinkGone(t, f)
}

func TestAPublishedFolderFollowsAMoveToAnotherStorage(t *testing.T) {
	f, _, confirm := twoStorages(t)
	file := publishFile(t, f, "trip/a.txt", false)
	folder := publishFile(t, f, "trip", true)
	f.h.do(postForm(confirm, f.sess, url.Values{}))
	if _, ok := f.srv.File(filesDocs + "trip/a.txt"); ok {
		t.Fatal("the folder was not moved")
	}
	// Read on the other storage, with its connection's credentials.
	downloads(t, f, file, "alpha")
	page := publicPage(t, f, folder)
	if page.Folder == nil || len(page.Folder.Rows) != 2 {
		t.Errorf("the folder link after the move: %+v, notices %+v", page.Folder, page.Notices)
	}
	noLinkGone(t, f)
}

// The sheet of a rename no longer says that published links lose it.
func TestTheRenameSheetLeavesPublishedLinksOut(t *testing.T) {
	f := pubDocs(t)
	sheet := opSheetOf(t, f, "rename", []string{"readme.txt"}, "share")
	if w := plainText(sheet.Warning); strings.Contains(w, "published links") {
		t.Errorf("warning = %q", w)
	}
}
