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
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
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
		// The library's script, then its live layer (the client's stage 4).
		`<script src="` + base + `/static/tc/` + document.Version() + `/js/tc.js"></script><script src="` + base + `/static/tc/` + document.Version() + `/js/tc-live.js"></script></body></html>`,
		// The root with the zones of C-7.2 named.
		document.Zoned(scr.Draw(model)).Render(),
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

// Every page can be installed as an application (U-11): it links the
// manifest the server serves under the base path (C-5.9) and names the
// colour around it, the manifest's (render.ThemeColor).
func TestEveryPageNamesTheManifest(t *testing.T) {
	for _, base := range []string{"", "/carrel"} {
		for _, scr := range screens.All {
			html, err := document.Of(scr, scr.Model(), base).Render()
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{`<link rel="manifest" href="` + base + render.ManifestPath + `">`,
				`<meta name="theme-color" content="` + render.ThemeColor + `">`} {
				if !strings.Contains(html, want) {
					t.Errorf("%T (base %q): the page does not say %s", scr.Model(), base, want)
				}
			}
		}
	}
}

// Every page loads the live layer (the client's stage 4; Typecase's tasks 84,
// 85) and names, as data, the headers of a request for a part of the page
// and the field of the token — the server's names (render), C-7.1, C-11; a
// page of the shell names main as the zone whose answer is an address, a
// page without main names none (the library's names guard).
func TestEveryPageLoadsTheLiveLayer(t *testing.T) {
	for _, scr := range screens.All {
		html, err := document.Of(scr, scr.Model(), "").Render()
		if err != nil {
			t.Fatal(err)
		}
		if live := document.Assets("").Live; !strings.Contains(html, live) {
			t.Errorf("%T: the page does not load %s", scr.Model(), live)
		}
		for _, want := range []string{`data-tc-view-header="Carrel-View"`, `data-tc-view-value="fragment"`, `data-tc-target-header="Carrel-Target"`,
			`data-tc-token-field="_csrf"`, `data-tc-token-header="X-CSRF-Token"`} {
			if !strings.Contains(html, want) {
				t.Errorf("%T: the page does not say %s", scr.Model(), want)
				break
			}
		}
		main := strings.Contains(html, `data-tc-zone="main"`)
		if address := strings.Contains(html, `data-tc-address-zone="main"`); address != main {
			t.Errorf("%T: the address zone is main %v, the page has main %v", scr.Model(), address, main)
		}
	}
}

// The theme the server chose for this browser goes to the document (note 75
// of Typecase, task 186 p. 1): a page of the shell carries its Look, a page
// outside it none.
func TestThemeComesFromTheLook(t *testing.T) {
	page := &view.ContactsListPage{}
	page.Look.Theme = "dark"
	if look, ok := document.LookOf(page); !ok || look.Theme != "dark" {
		t.Errorf("look of a shell page = %+v %v", look, ok)
	}
	if _, ok := document.LookOf(&view.LoginPage{}); ok {
		t.Error("a page outside the shell has a look")
	}
	if _, ok := document.LookOf((*view.ContactsListPage)(nil)); ok {
		t.Error("a nil page has a look")
	}
}

// A page of the shell has one place a sheet is answered into (Typecase's
// note 124): over a page with none open, the closed scrim at the shell's end;
// under a sheet drawn with the page, that sheet's scrim.
func TestEveryShellPageHasOneSheetPlace(t *testing.T) {
	for _, scr := range screens.All {
		root := document.Zoned(scr.Draw(scr.Model()))
		shell, ok := root.(w.Shell)
		if !ok {
			continue
		}
		html := shell.Render()
		if n := strings.Count(html, `data-tc-zone="sheet"`); n != 1 {
			t.Errorf("%T: %d places of a sheet", scr.Model(), n)
		}
		if _, plain := shell.Body.(w.ShellBody); plain && (shell.Sheet == nil || !shell.Sheet.Closed) {
			t.Errorf("%T: the sheet's place over the shell = %+v, want it closed", scr.Model(), shell.Sheet)
		}
	}
}
