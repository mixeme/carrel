package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// C-2.1's named exception for files: a folder is not a feed. Somebody's
// published folder — here another Carrel's /p/{token}/dav/ — is connected
// like any WebDAV address: discovery finds no principal and takes the
// address as the file collection (S-6), and it reads like one.
func TestPublishedFolderConnectsAsAFileCollection(t *testing.T) {
	owner := pubDocs(t)
	link := publishFile(t, owner, "share", true)
	other := httptest.NewServer(owner.h.srv.Handler())
	t.Cleanup(other.Close)

	reader := feedHarness(t)
	rec := reader.h.do(postForm("/app/settings/connections", reader.sess, url.Values{"server": {other.URL + link + "/dav/"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("connect: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := reader.sess.DataKey()
	blob, _ := reader.h.users.Catalog(reader.sess.AccountID(), key)
	root := link + "/dav/"
	found := false
	for _, c := range blob.Collections {
		if !c.Has(view.KindFile) {
			continue
		}
		if !c.ReadOnly {
			t.Errorf("%s connects as writable", c.Path)
		}
		found = found || c.Path == root
	}
	if !found {
		t.Fatalf("the published folder is not a collection: %+v", blob.Collections)
	}
}

// The owner's decision (18.09.2026): somebody's folder is subscribed to like
// a feed — from the Files rail, one read-only collection under
// "Subscriptions", read live, never written, not published onwards, and
// unsubscribed rather than disconnected.
func TestSubscribeToAFolder(t *testing.T) {
	owner := pubDocs(t)
	link := publishFile(t, owner, "share", true)
	other := httptest.NewServer(owner.h.srv.Handler())
	t.Cleanup(other.Close)

	reader := feedHarness(t)
	rail := decode[view.FilesPage](t, reader.h.do(get("/app/files", reader.sess))).Rail
	form := actionHref(t, rail.Foot, "feed.subscribe")
	page := decode[view.ConnectionsPage](t, reader.h.do(get(form, reader.sess)))
	check := actionHref(t, page.Add.Actions, "feed.check")
	checked := decode[view.ConnectionsPage](t, reader.h.do(postForm(check, reader.sess, url.Values{"url": {other.URL + link}})))
	if !hasNotice(checked.Notices, "The folder answers") {
		t.Fatalf("check = %+v", checked.Notices)
	}
	rec := reader.h.do(postForm(actionHref(t, page.Add.Actions, "feed.subscribe"), reader.sess, url.Values{"url": {other.URL + link}, "name": {"Shared by Ann"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := reader.sess.DataKey()
	blob, _ := reader.h.users.Catalog(reader.sess.AccountID(), key)
	if len(blob.Subscriptions) != 1 || blob.Subscriptions[0].Kinds[0] != view.KindFile || !strings.HasSuffix(blob.Subscriptions[0].URL, "/dav/") {
		t.Fatalf("subscription = %+v", blob.Subscriptions)
	}
	if len(blob.Connections) != 1 {
		t.Error("a shared folder became a connection")
	}
	feed := blob.Subscriptions[0]

	files := decode[view.FilesPage](t, reader.h.do(get("/app/files", reader.sess)))
	sec, row := railRow(t, files.Rail, "Shared by Ann")
	if sec.Account != "Subscriptions" || !row.ReadOnly {
		t.Errorf("rail = %+v %+v", sec, row)
	}
	folder := decode[view.FilesPage](t, reader.h.do(get(row.Href, reader.sess)))
	var readme view.FileRow
	for _, r := range folder.Rows {
		if r.Name == "readme.txt" {
			readme = r
		}
	}
	if readme.Href == "" {
		t.Fatalf("folder rows = %+v", folder.Rows)
	}
	if got := reader.h.do(get(readme.Href, reader.sess)); got.Code != http.StatusOK || got.Body.String() != "hello from share" {
		t.Errorf("reading a file of the folder: %d %q", got.Code, got.Body.String())
	}
	for _, a := range folder.Bar.Actions {
		if (a.ID == "file.upload" || a.ID == "folder.new" || a.ID == "publish") && a.Enabled {
			t.Errorf("a subscribed folder offers %+v", a)
		}
	}
	reader.h.do(multipartPost(row.Href+"upload/", reader.sess, map[string]string{"x.txt": "nope"}))
	if n := owner.srv.CountRequests(http.MethodPut, ""); n != 0 {
		t.Errorf("a write reached the owner's storage %d times", n)
	}
	if rec := reader.h.do(get("/app/publish?"+url.Values{"k": {"files"}, "a": {feed.ID}, "c": {"feed"}, "f": {""}, "dir": {"1"}}.Encode(), reader.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("somebody's folder published onwards: %d", rec.Code)
	}

	cards := decode[view.ConnectionsPage](t, reader.h.do(get("/app/settings/connections", reader.sess)))
	if rec := reader.h.do(postForm(actionHref(t, cards.Subscriptions[0].Actions, "feed.unsubscribe"), reader.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("unsubscribe: %d", rec.Code)
	}
	if _, ok := owner.srv.File(filesDocs + "share/readme.txt"); !ok {
		t.Error("unsubscribing touched the owner's folder")
	}
}
