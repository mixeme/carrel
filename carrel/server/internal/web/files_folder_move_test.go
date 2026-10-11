package web_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Б14 of the audit of 11.10.2026 (docs/plan/scans/silent-choices.md): a
// folder moved to another storage is copied there and deleted here. The
// original was deleted without a look at either: a file put into it while
// it was being copied went with it, unseen, and a copy that broke off half
// way stood on the other storage under the folder's own name. Now the
// folder is written beside its place and takes it only whole, and the
// original goes only when it holds what it held and the copy holds all of
// it.

// twoStorages is a fixture with a second connection whose storage «Far» is
// ticked: the folder «trip» of the first is to move there.
func twoStorages(t *testing.T) (f davFixture, far *davtest.Server, confirm string) {
	t.Helper()
	f = filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"trip/a.txt", "alpha")
	f.srv.SeedFile(filesDocs+"trip/2020/b.txt", "beta")
	far = searchServer(t)
	far.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	far.SeedFolder(filesDocs)
	if rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{
		"server": {far.URL}, "login": {far.Login}, "password": {far.Password},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("second connection: %d", rec.Code)
	}
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	farID := blob.Connections[1].ID
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{AccountID: farID, Path: filesDocs, Name: "Far", Kinds: []view.Kind{view.KindFile}})
	})
	farRoot := "/app/files/" + farID + "/" + url.PathEscape(filesDocs) + "/"
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	sheet := opSheet(t, f, rowMenu(t, fileRow(t, page, "trip"), "file.move"), url.Values{"to": {farRoot}})
	return f, far, confirmHref(t, sheet)
}

func TestAFolderMovedBetweenStoragesArrivesWhole(t *testing.T) {
	f, far, confirm := twoStorages(t)
	f.h.do(postForm(confirm, f.sess, url.Values{}))
	if a, _ := far.File(filesDocs + "trip/a.txt"); a != "alpha" {
		t.Errorf("far trip/a.txt = %q", a)
	}
	if b, _ := far.File(filesDocs + "trip/2020/b.txt"); b != "beta" {
		t.Errorf("far trip/2020/b.txt = %q", b)
	}
	if _, ok := f.srv.File(filesDocs + "trip/a.txt"); ok {
		t.Error("the original stayed after the move")
	}
	for _, name := range far.Names(filesDocs) {
		if strings.Contains(name, ".carrel-") {
			t.Errorf("the name the copy was written under stayed: %q", name)
		}
	}
}

// A copy that breaks off leaves nothing under the folder's name there, and
// the original where it was.
func TestAFolderMoveThatBreaksOffLeavesNoHalfFolder(t *testing.T) {
	f, far, confirm := twoStorages(t)
	far.Refuse(http.MethodPut, http.StatusInsufficientStorage, "no room")
	f.h.do(postForm(confirm, f.sess, url.Values{}))
	if names := far.Names(filesDocs); len(names) != 0 {
		t.Errorf("the other storage holds %v after a move that broke off", names)
	}
	if a, _ := f.srv.File(filesDocs + "trip/a.txt"); a != "alpha" {
		t.Errorf("the original after a move that broke off: %q", a)
	}
	row := queueRow(t, decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Queue, "trip")
	if row.State != view.OpFailed {
		t.Errorf("row = %+v", row)
	}
}

// A file put into the folder while it is being copied is not in the copy:
// the original is not deleted, the copy is taken back, and the row says why.
func TestAFolderThatChangedWhileItWasCopiedIsNotDeleted(t *testing.T) {
	f, far, confirm := twoStorages(t)
	release := far.Stall(http.MethodPut)
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.h.do(postForm(confirm, f.sess, url.Values{}))
	}()
	// The copy has begun: its first file waits at the other storage.
	deadline := time.Now().Add(20 * time.Second)
	for far.CountRequests(http.MethodPut, filesDocs) == 0 {
		if time.Now().After(deadline) {
			release()
			t.Fatal("the copy never began")
		}
		time.Sleep(20 * time.Millisecond)
	}
	f.srv.SeedFile(filesDocs+"trip/late.txt", "put there meanwhile")
	release()
	<-done

	if got, _ := f.srv.File(filesDocs + "trip/late.txt"); got != "put there meanwhile" {
		t.Fatalf("the file put into the folder meanwhile is gone: %q", got)
	}
	if a, _ := f.srv.File(filesDocs + "trip/a.txt"); a != "alpha" {
		t.Errorf("the original was deleted: %q", a)
	}
	if names := far.Names(filesDocs); slices.Contains(names, "trip") || len(names) != 0 {
		t.Errorf("the other storage holds %v", names)
	}
	row := queueRow(t, decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))).Queue, "trip")
	if row.State != view.OpFailed || !strings.Contains(string(row.Reason), "changed while it was copied") {
		t.Errorf("row = %+v", row)
	}
}
