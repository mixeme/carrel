package document_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/screens"
	w "carrel.dev/typecase/widget"
)

// sample is a screen of one message, its title from its model.
var sample = screens.Screen{
	Model: func() any { return new(string) },
	Draw: func(model any) screens.Root {
		return w.Page{Items: []w.PagePart{w.Msg{Parts: []w.MsgPart{w.Text(*model.(*string))}}}}
	},
	Title: func(model any) string { return *model.(*string) },
}

// posting is the sample with a button outside a form, and the form it
// submits at the page's end.
var posting = screens.Screen{
	Model: sample.Model,
	Draw: func(model any) screens.Root {
		return w.Page{Items: []w.PagePart{w.Msg{Parts: []w.MsgPart{w.Button{Label: *model.(*string), Form: "wipe"}}}}}
	},
	Title: sample.Title,
	Forms: func(model any) []w.PostForm {
		return []w.PostForm{{ID: "wipe", Action: "/forgot", Fields: []w.Hidden{{Name: "_csrf", Value: "x"}}}}
	},
}

// inline is what the content policy of C-11 refuses: a script or a style
// written into the page, a handler in an attribute.
var inline = regexp.MustCompile(`<script(\s[^>]*)?>[^<]|<script>|<style|\sstyle=|\son[a-z]+=`)

func check(t *testing.T, at string, scr screens.Screen, model any, base string) {
	t.Helper()
	doc := document.Of(scr, model, base)
	html, err := doc.Render()
	if err != nil {
		t.Fatalf("%s: %v", at, err)
	}
	for _, want := range []string{
		`<html lang="en"`,
		`<script src="` + base + `/static/tc/` + document.Version() + `/js/theme.js"></script>`,
		`<link rel="stylesheet" href="` + base + `/static/tc/` + document.Version() + `/css/typecase.css">`,
		`<script src="` + base + `/static/tc/` + document.Version() + `/js/tc.js"></script></body></html>`,
		scr.Draw(model).Render(),
	} {
		if !strings.Contains(html, want) {
			t.Errorf("%s: the page has no %s", at, want)
		}
	}
	if m := inline.FindString(html); m != "" {
		t.Errorf("%s: the page has something inline: %q", at, m)
	}
}

func TestDocument(t *testing.T) {
	title := "Contacts <&>"
	check(t, "the sample", sample, &title, "")
	check(t, "the sample under a base path", sample, &title, "/carrel")
	html, err := document.Of(sample, &title, "").Render()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<title>Contacts &lt;&amp;&gt;</title>") {
		t.Error("the title is not the model's, escaped")
	}
	// The forms of the screen are printed after its root.
	html, err = document.Of(posting, &title, "").Render()
	if err != nil {
		t.Fatal(err)
	}
	root := posting.Draw(&title).Render()
	form := posting.Forms(&title)[0].Render()
	if i, j := strings.Index(html, root), strings.Index(html, form); i < 0 || j < i+len(root) {
		t.Errorf("the page does not print the screen's form after its root:\n%s", html)
	}
	for _, scr := range screens.All {
		if scr.Title == nil {
			t.Errorf("%T: the screen has no Title", scr.Model())
			continue
		}
		check(t, fmt.Sprintf("%T", scr.Model()), scr, scr.Model(), "")
	}
}

// The files the page names are served where it names them, and a browser
// keeps them: their addresses carry the version of their contents
// (document.Version), so an address never names two different files.
func TestFiles(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET "+document.Static+"/", document.Files())
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	a := document.Assets("")
	font := strings.TrimSuffix(a.CSS, "css/typecase.css") + "fonts/PublicSans.ttf"
	for path, kind := range map[string]string{a.CSS: "text/css", a.Theme: "javascript", a.JS: "javascript", font: "font/ttf"} {
		if !strings.HasPrefix(path, document.Static+"/"+document.Version()+"/") {
			t.Errorf("%s: not under the version of the files %s", path, document.Version())
		}
		rec := get(path)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), kind) || rec.Body.Len() == 0 {
			t.Errorf("%s: %d %q, %d bytes", path, rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
		}
		if got := rec.Header().Get("Cache-Control"); got != document.Immutable {
			t.Errorf("%s: Cache-Control %q, want %q", path, got, document.Immutable)
		}
	}
	// What is not a file of this version is not found, and not kept: an old
	// version's address, the files without a version, a directory's listing,
	// a file the library does not have.
	for _, path := range []string{
		document.Static + "/000000000000/css/typecase.css",
		document.Static + "/css/typecase.css",
		document.Static + "/" + document.Version() + "/fonts/",
		document.Static + "/" + document.Version() + "/",
		document.Static + "/" + document.Version() + "/css/other.css",
	} {
		rec := get(path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, rec.Code)
		}
		if got := rec.Header().Get("Cache-Control"); got == document.Immutable {
			t.Errorf("%s: a miss is kept: Cache-Control %q", path, got)
		}
	}
	if v := document.Version(); len(v) != 12 || strings.Trim(v, "0123456789abcdef") != "" {
		t.Errorf("version %q: want 12 hex digits", v)
	}
}

// The version is taken over every script the library serves — the theme's,
// its own and its live layer (Typecase b381931, L-1), which page.Static
// serves at the same version whether a page names it or not —, each as
// served; and the same files give the same version every time it is taken.
func TestVersionCoversEveryScript(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET "+document.Static+"/", document.Files())
	a := document.Assets("")
	prefix := document.Static + "/" + document.Version() + "/"
	scripts := document.Scripts()
	for _, path := range []string{a.Theme, a.JS, a.Live} {
		name := strings.TrimPrefix(path, prefix)
		read, ok := scripts[name]
		if !ok {
			t.Errorf("%s: served, and not in the version", name)
			continue
		}
		b, err := read()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil || rec.Code != http.StatusOK || rec.Body.String() != string(b) {
			t.Errorf("%s: the version reads other contents than are served (%v, %d)", name, err, rec.Code)
		}
	}
	for range 20 {
		if v := document.VersionOf(); v != document.Version() {
			t.Fatalf("the version is %s once and %s another time", document.Version(), v)
		}
	}
}

// No page loads the live layer yet: the client takes it with its stage 4,
// after Typecase's tasks 84 and 85 (plan, «Typecase сборки»).
func TestNoPageLoadsTheLiveLayer(t *testing.T) {
	for _, scr := range screens.All {
		html, err := document.Of(scr, scr.Model(), "").Render()
		if err != nil {
			t.Fatal(err)
		}
		if live := document.Assets("").Live; strings.Contains(html, live) {
			t.Errorf("%T: the page loads %s", scr.Model(), live)
		}
	}
}
