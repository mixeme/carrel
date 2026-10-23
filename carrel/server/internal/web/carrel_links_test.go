package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// What one Carrel publishes by link another takes, the way a person pastes
// the link into Connect account: a calendar and a book are subscribed to as
// feeds (their file, /p/{token}/download), a folder as a shared folder (its
// mount, /p/{token}/dav/) — the link names no kind, and the server asks it.
// Each is then read where it belongs: the event on the agenda (as published:
// «Busy»), the card in the book, the file in the folder.
func TestAnotherCarrelsLinksAreTaken(t *testing.T) {
	owner := publishFixture(t)
	published := func(f davFixture, href string) string {
		t.Helper()
		sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
		// The sheet's tick «Allow .ics / .vcf download and subscription».
		got := decode[view.PublishSheet](t, f.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), f.sess, url.Values{"lives": {"never"}, "download": {"on"}})))
		return string(got.Link)
	}
	docs := pubDocs(t)
	links := map[string]string{
		"calendar": published(owner, owner.publishHref("calendar", searchWork, "")),
		"book":     published(owner, owner.publishHref("contacts", searchBook, "")),
		"folder":   publishFile(t, docs, "share", true),
	}
	servers := map[string]*httptest.Server{
		"calendar": httptest.NewServer(owner.h.srv.Handler()),
		"folder":   httptest.NewServer(docs.h.srv.Handler()),
	}
	servers["book"] = servers["calendar"]
	for _, s := range servers {
		t.Cleanup(s.Close)
	}

	for _, kind := range []string{"calendar", "book", "folder"} {
		reader := feedHarness(t)
		addr := servers[kind].URL + links[kind]
		page := decode[view.ConnectionsPage](t, reader.h.do(postForm("/app/settings/connections", reader.sess, url.Values{"server": {addr}})))
		folder := strings.HasSuffix(page.Add.Action, "&kind=folder")
		if folder != (kind == "folder") {
			t.Errorf("%s: Connect account offers %q", kind, page.Add.Action)
			continue
		}
		if rec := reader.h.do(postForm(page.Add.Action, reader.sess, url.Values{"url": {addr}, "name": {kind}})); rec.Code != http.StatusSeeOther {
			t.Errorf("%s: subscribing answered %d", kind, rec.Code)
			continue
		}
		key, _ := reader.sess.DataKey()
		blob, _ := reader.h.users.Catalog(reader.sess.AccountID(), key)
		if len(blob.Subscriptions) != 1 {
			t.Errorf("%s: subscriptions %+v", kind, blob.Subscriptions)
			continue
		}
		id := blob.Subscriptions[0].ID
		read := map[string]struct{ path, want string }{
			"calendar": {"/app/calendar/" + id + "/feed?from=2026-08-10", "Busy"},
			"book":     {"/app/contacts/" + id + "/feed", "Ada Lovelace"},
			"folder":   {"/app/files/" + id + "/feed/", "readme.txt"},
		}[kind]
		if rec := reader.h.do(get(read.path, reader.sess)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), read.want) {
			t.Errorf("%s: %s answered %d without %q", kind, read.path, rec.Code, read.want)
		}
	}
}
