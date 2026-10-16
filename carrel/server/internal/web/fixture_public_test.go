package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// C-14.2: the public pages a link opens (U-6.15, Пс1), one state for each
// kind of page the frames draw — s-publish-6 to -12. The links are given
// out through the sheet, as a person does; the page is asked without a
// session.
func TestFixturePublicPages(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t))
	app.h.users.SetClock(mockupNow(t))
	// «Личный» for 10–31 August is a link of the set already (s-publish-1).
	calendar := ""
	for issued, name := range app.byAccount {
		if name == "link-lichnyj-token" {
			calendar = issued
		}
	}
	if calendar == "" {
		t.Fatal("the set has no link to «Личный»")
	}
	give := func(name string, q url.Values) string {
		rec := app.h.do(postForm("/app/publish", app.sess, q))
		if rec.Code != 200 {
			t.Fatalf("publish %v: %d %s", q, rec.Code, rec.Body.String())
		}
		link := string(decode[view.PublishSheet](t, rec).Link)
		if !strings.Contains(link, "/p/") {
			t.Fatalf("publish %v: no link", q)
		}
		token := strings.TrimPrefix(link[strings.Index(link, "/p/"):], "/p/")
		app.byAccount[token] = "link-" + name + "-token"
		return token
	}
	record := func(name, k, slug, uid string) string {
		q := url.Values{"k": {k}, "a": {app.realAccount(t, slug)}, "c": {app.realPath(t, slug)}}
		if uid != "" {
			q.Set("uid", uid)
		}
		return give(name, q)
	}
	file := func(name, rel string, dir bool) string {
		q := url.Values{"k": {"files"}, "a": {app.realAccount(t, "Documents")}, "c": {app.realPath(t, "Documents")}, "f": {rel}}
		if dir {
			q.Set("dir", "1")
		}
		return give(name, q)
	}
	for _, one := range []struct{ state, token string }{
		{"normal", calendar}, // s-publish-6
		{"addressbook", record("komanda", "contacts", "komanda", "")},                        // s-publish-7
		{"note", record("zametka", "notes", "zapisi", "5c81-77aa-0d64")},                     // s-publish-8
		{"image", file("sync-board", "carrel/attachments/2026-08-10-sync-board.png", false)}, // s-publish-9
		{"text", file("checklist", "carrel/attachments/release-checklist.md", false)},        // s-publish-10
		{"download", file("panorama", "carrel/attachments/2026-07/панорама.tif", false)},     // s-publish-11
		{"folder", file("attachments", "carrel/attachments", true)},                          // s-publish-12
	} {
		t.Run(one.state, func(t *testing.T) {
			app.compare(t, "p-token/"+one.state+".json", app.h.do(get("/p/"+one.token, nil)))
		})
	}
}
