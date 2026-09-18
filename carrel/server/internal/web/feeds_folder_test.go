package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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
