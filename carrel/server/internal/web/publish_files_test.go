package web_test

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// pubDocs seeds Docs with a folder to publish, a neighbour that must stay
// out of reach, and files inside.
func pubDocs(t *testing.T) davFixture {
	t.Helper()
	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs + "share/")
	f.srv.SeedFolder(filesDocs + "share/sub/")
	f.srv.SeedFile(filesDocs+"share/readme.txt", "hello from share")
	f.srv.SeedFile(filesDocs+"share/sub/deep.txt", "deep inside")
	f.srv.SeedFile(filesDocs+"secret.txt", "the neighbour")
	return f
}

func publishFile(t *testing.T, f davFixture, rel string, dir bool) string {
	t.Helper()
	q := url.Values{"k": {"files"}, "a": {f.id}, "c": {filesDocs}, "f": {rel}}
	if dir {
		q.Set("dir", "1")
	}
	return publishLink(t, f, "/app/publish?"+q.Encode(), false)
}

func dav(method, path, depth string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	if depth != "" {
		r.Header.Set("Depth", depth)
	}
	return r
}

// A folder link: its page lists it and goes down, never up or sideways; a
// file in it streams; the folder downloads as a zip of everything in it.
func TestPublishedFolder(t *testing.T) {
	f := pubDocs(t)
	link := publishFile(t, f, "share", true)
	page := publicPage(t, f, link)
	if page.Folder == nil || len(page.Folder.Rows) != 2 || page.Folder.DAV != text(link+"/dav/") {
		t.Fatalf("folder page = %+v", page.Folder)
	}
	var file, sub string
	for _, r := range page.Folder.Rows {
		if r.IsDir {
			sub = r.Href
		} else {
			file = r.Href
		}
	}
	if got := f.h.do(anon(file)); got.Code != http.StatusOK || got.Body.String() != "hello from share" || !strings.Contains(got.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("file: %d %q", got.Code, got.Body.String())
	}
	// A folder over the ceiling says so (Пс22), as the owner's own does.
	f.h.cfg.FilesMaxEntries = 1
	cut := publicPage(t, f, link)
	f.h.cfg.FilesMaxEntries = 2000
	if len(cut.Folder.Rows) != 1 || len(cut.Notices) != 1 || cut.Notices[0].Title != "Showing 1 of 2" {
		t.Errorf("a folder cut at its ceiling: %d rows, notices %+v", len(cut.Folder.Rows), cut.Notices)
	}
	deeper := publicPage(t, f, sub)
	if deeper.Folder == nil || len(deeper.Folder.Rows) != 1 || len(deeper.Folder.Crumbs) != 2 {
		t.Errorf("sub folder = %+v", deeper.Folder)
	}
	for _, escape := range []string{link + "/file/../secret.txt", link + "/file/%2E%2E/secret.txt", link + "?in=..", link + "/file/sub/../../secret.txt"} {
		if rec := f.h.do(anon(escape)); rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "the neighbour") {
			t.Errorf("%s reached the neighbour", escape)
		}
	}

	zipped := f.h.do(anon(link + "/download"))
	if zipped.Code != http.StatusOK || zipped.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("zip: %d %v", zipped.Code, zipped.Header())
	}
	zr, err := zip.NewReader(bytes.NewReader(zipped.Body.Bytes()), int64(zipped.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, zf := range zr.File {
		rc, _ := zf.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		names[zf.Name] = string(b)
	}
	if names["readme.txt"] != "hello from share" || names["sub/deep.txt"] != "deep inside" || len(names) != 3 {
		t.Errorf("zip holds %v", names)
	}
}

// One file: its page and its download; nothing else of the storage.
func TestPublishedFile(t *testing.T) {
	f := pubDocs(t)
	link := publishFile(t, f, "share/readme.txt", false)
	page := publicPage(t, f, link)
	if page.File == nil || page.File.Name != "readme.txt" || page.Folder != nil {
		t.Fatalf("file page = %+v", page)
	}
	if got := f.h.do(anon(link + "/download")); got.Code != http.StatusOK || got.Body.String() != "hello from share" {
		t.Errorf("download: %d %q", got.Code, got.Body.String())
	}
	for _, p := range []string{link + "/file/readme.txt", link + "/dav/"} {
		if got := f.h.do(anon(p)); got.Code != http.StatusNotFound {
			t.Errorf("%s on a file link: %d", p, got.Code)
		}
	}
	// Publishing what is not there makes nothing.
	q := url.Values{"k": {"files"}, "a": {f.id}, "c": {filesDocs}, "f": {"share/missing.txt"}}
	sheet := decode[view.PublishSheet](t, f.h.do(get("/app/publish?"+q.Encode(), f.sess)))
	if rec := f.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), f.sess, url.Values{})); rec.Code != http.StatusNotFound {
		t.Errorf("a missing file was published: %d", rec.Code)
	}
}

// The drive: OPTIONS says what it answers, PROPFIND lists with our own
// hrefs, GET reads a file, and every method that would write is 405.
func TestPublishedFolderMountsReadOnly(t *testing.T) {
	f := pubDocs(t)
	link := publishFile(t, f, "share", true)
	root := link + "/dav/"
	opt := f.h.do(dav(http.MethodOptions, root, ""))
	if opt.Code != http.StatusOK || opt.Header().Get("DAV") != "1" || !strings.Contains(opt.Header().Get("Allow"), "PROPFIND") {
		t.Errorf("OPTIONS: %d %v", opt.Code, opt.Header())
	}
	pf := f.h.do(dav("PROPFIND", root, "1"))
	body := pf.Body.String()
	if pf.Code != http.StatusMultiStatus || strings.Count(body, "<D:response>") != 3 || !strings.Contains(body, root+"readme.txt") ||
		!strings.Contains(body, root+"sub/") || strings.Contains(body, "secret") || strings.Contains(body, filesDocs) {
		t.Fatalf("PROPFIND %d:\n%s", pf.Code, body)
	}
	if one := f.h.do(dav("PROPFIND", root, "0")); strings.Count(one.Body.String(), "<D:response>") != 1 {
		t.Errorf("Depth 0:\n%s", one.Body.String())
	}
	if got := f.h.do(dav(http.MethodGet, root+"sub/deep.txt", "")); got.Code != http.StatusOK || got.Body.String() != "deep inside" {
		t.Errorf("GET: %d %q", got.Code, got.Body.String())
	}
	before := f.srv.CountRequests(http.MethodPut, "") + f.srv.CountRequests(http.MethodDelete, "") + f.srv.CountRequests("MKCOL", "") + f.srv.CountRequests("MOVE", "")
	for _, m := range []string{http.MethodPut, http.MethodDelete, "MKCOL", "MOVE", "COPY", "PROPPATCH", "LOCK", http.MethodPost} {
		if rec := f.h.do(dav(m, root+"readme.txt", "")); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: %d", m, rec.Code)
		}
	}
	after := f.srv.CountRequests(http.MethodPut, "") + f.srv.CountRequests(http.MethodDelete, "") + f.srv.CountRequests("MKCOL", "") + f.srv.CountRequests("MOVE", "")
	if after != before {
		t.Error("a write reached the storage")
	}
}

// The owner's decision on the limit: misses spend the address's bucket, a
// live link spends its own — a mount outlives 30 requests a minute, a
// guesser does not.
func TestPublishedLinkLimits(t *testing.T) {
	f := pubDocs(t)
	link := publishFile(t, f, "share", true)
	for i := 0; i < 60; i++ {
		if rec := f.h.do(dav("PROPFIND", link+"/dav/", "1")); rec.Code != http.StatusMultiStatus {
			t.Fatalf("request %d of a live mount: %d", i+1, rec.Code)
		}
	}
	limited := false
	for i := 0; i < 40; i++ {
		if f.h.do(anon("/p/"+strings.Repeat("B", 43))).Code == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Error("guessing is not limited")
	}
}

// Publish is in a row's menu, a folder's too; the bar over a folder draws
// none (s-files-1).
func TestFilesOfferPublish(t *testing.T) {
	f := pubDocs(t)
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	for _, r := range page.Rows {
		if r.Name == "share" {
			actionHref(t, menuActions(r.Menu), "publish")
		}
	}
}
