package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A file of a published folder opens its own page inside the link (Пс21;
// the caption of s-publish-12): the row's Open is that page, its Href the
// stream; the page is a file link's — name, type and size, the text in the
// page —, Download the file's stream. A path out of the folder is not
// found.
func TestPublishedFolderFileHasItsPage(t *testing.T) {
	f := pubDocs(t)
	link := publishFile(t, f, "share", true)
	folder := publicPage(t, f, link)
	var row view.FileRow
	for _, r := range folder.Folder.Rows {
		if r.Name == "readme.txt" {
			row = r
		}
	}
	if !strings.HasPrefix(row.Href, link+"/file/") || row.Open != link+"?file=readme.txt" {
		t.Fatalf("the file's row = %+v", row)
	}
	page := publicPage(t, f, row.Open)
	if page.Title != "readme.txt" || page.File == nil || page.File.Preview != view.PreviewText || page.File.Body != "hello from share" ||
		len(page.Actions) != 1 || page.Actions[0].Href != row.Href || page.Folder != nil {
		t.Errorf("the file's page = %+v %+v %+v", page.Title, page.File, page.Actions)
	}
	if got := f.h.do(anon(page.Actions[0].Href)); got.Code != http.StatusOK || got.Body.String() != "hello from share" {
		t.Errorf("its Download: %d %q", got.Code, got.Body.String())
	}
	deep := publicPage(t, f, link+"?file="+url.QueryEscape("sub/deep.txt"))
	if deep.Title != "deep.txt" || deep.File == nil || deep.File.Body != "deep inside" {
		t.Errorf("a file deeper in = %+v %+v", deep.Title, deep.File)
	}
	for _, out := range []string{"../secret.txt", "sub", "missing.txt"} {
		rec := f.h.do(anon(link + "?file=" + url.QueryEscape(out)))
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "the neighbour") {
			t.Errorf("%s leads out of the folder", out)
		}
		if out == "../secret.txt" && rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want not found", out, rec.Code)
		}
	}
}
