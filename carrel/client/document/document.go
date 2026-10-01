// Package document is the page a screen stands in: Typecase's page.Document
// around the screen's root (U-5.7, «Документ страницы»; task 38 of Typecase).
// The head, the library's stylesheet, fonts and scripts, the sprite of icons
// and the body are the library's; the client names only the title, the
// language, where it serves the library's files and the root. It writes no
// markup of its own — the guards of the module's strings hold it (U-15.2, 2)
// — and the root is one of the library's roots, a closed interface
// (U-15.2, 1).
//
// The files are served from the binary at Static under the base path
// (C-5.9): page.Static of Typecase, mounted by Files, under the version of
// their contents, so a browser keeps them for good. The page follows the
// content policy of C-11 as it is — the theme's script and the stylesheet by
// address, nothing inline.
package document

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"

	"carrel.dev/carrel/client/screens"
	"carrel.dev/typecase"
	"carrel.dev/typecase/page"
)

// Static is where the client serves Typecase's files, under the base path of
// the instance: Static/{version}/css/typecase.css, Static/{version}/fonts/…,
// Static/{version}/js/theme.js, Static/{version}/js/tc.js — the version is
// Version.
const Static = "/static/tc"

// Immutable is the caching of a file served at Static: its address carries
// the version of its contents, so the address never names another file and
// a browser keeps it for a year without asking again. A new binary with
// other files serves them at another address.
const Immutable = "public, max-age=31536000, immutable"

// Lang is the language of every page: the interface speaks English (U-13).
const Lang = "en"

// Version is the version of the library's files in the binary: the first 12
// hex digits of SHA-256 over each file's name and contents — the stylesheet,
// both scripts, the fonts. The same files give the same version on every
// build; any change of any of them gives another.
func Version() string { return version() }

var version = sync.OnceValue(func() string {
	h := sha256.New()
	add := func(name string, b []byte) {
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	css, err := typecase.CSS()
	if err != nil {
		panic(err) // the library's own embedded files: a broken build
	}
	add("css/typecase.css", []byte(css))
	for name, read := range map[string]func() ([]byte, error){"js/theme.js": typecase.ThemeJS, "js/tc.js": typecase.JS} {
		b, err := read()
		if err != nil {
			panic(err)
		}
		add(name, b)
	}
	fonts := typecase.Fonts()
	err = fs.WalkDir(fonts, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fonts, path)
		if err != nil {
			return err
		}
		add(path, b)
		return nil
	})
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
})

// Assets are the addresses of the library's files for an instance served at
// base ("" or "/carrel", as CARREL_BASE_PATH normalised).
func Assets(base string) page.Assets { return page.At(base + Static + "/" + Version()) }

// Files serves the library's files at Static, for a mux that sees paths
// without the base path: mux.Handle("GET "+document.Static+"/",
// document.Files()). Only a file of this version is found, and only a file
// found is kept (Immutable): an address of another version, a directory, a
// name the library does not have — 404, and the caller's caching stays.
func Files() http.Handler {
	prefix := Static + "/" + Version()
	lib := http.StripPrefix(prefix, page.Static())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, prefix+"/")
		if !ok || rest == "" || strings.HasSuffix(rest, "/") {
			http.NotFound(w, r)
			return
		}
		if t, ok := fontTypes[path.Ext(rest)]; ok {
			w.Header().Set("Content-Type", t)
		}
		lib.ServeHTTP(&keep{ResponseWriter: w}, r)
	})
}

// fontTypes are the types of the fonts by their extension. The standard
// library knows none of them of itself — it asks the system's tables, and the
// image (Alpine, S-28) has none — and a font sniffed is
// application/octet-stream.
var fontTypes = map[string]string{
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}

// keep sets Immutable on an answer that found its file, as its status is
// written: a miss or an error is not kept.
type keep struct {
	http.ResponseWriter
	wrote bool
}

func (k *keep) WriteHeader(code int) {
	if !k.wrote {
		k.wrote = true
		if code == http.StatusOK || code == http.StatusNotModified {
			k.Header().Set("Cache-Control", Immutable)
		}
	}
	k.ResponseWriter.WriteHeader(code)
}

func (k *keep) Write(b []byte) (int, error) {
	if !k.wrote {
		k.WriteHeader(http.StatusOK)
	}
	return k.ResponseWriter.Write(b)
}

// Of is the document of a screen drawn from its view model, for an instance
// served at base: the screen's root, and at the page's end the forms its
// buttons outside a form submit (Screen.Forms).
func Of(scr screens.Screen, model any, base string) page.Document {
	doc := page.Document{
		Title:  scr.Title(model),
		Lang:   Lang,
		Assets: Assets(base),
		Root:   scr.Draw(model),
	}
	if scr.Forms != nil {
		doc.Forms = scr.Forms(model)
	}
	return doc
}
