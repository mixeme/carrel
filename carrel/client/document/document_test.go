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
		`<script src="` + base + `/static/tc/js/theme.js"></script>`,
		`<link rel="stylesheet" href="` + base + `/static/tc/css/typecase.css">`,
		`<script src="` + base + `/static/tc/js/tc.js"></script></body></html>`,
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

// The files the page names are served where it names them.
func TestFiles(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(document.Static+"/", document.Files())
	a := document.Assets("")
	for path, kind := range map[string]string{a.CSS: "text/css", a.Theme: "javascript", a.JS: "javascript"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), kind) || rec.Body.Len() == 0 {
			t.Errorf("%s: %d %q, %d bytes", path, rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
		}
	}
}
