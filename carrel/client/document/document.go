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
// (C-5.9): page.Static of Typecase, mounted by Files. The page follows the
// content policy of C-11 as it is — the theme's script and the stylesheet by
// address, nothing inline.
package document

import (
	"net/http"

	"carrel.dev/carrel/client/screens"
	"carrel.dev/typecase/page"
)

// Static is where the client serves Typecase's files, under the base path of
// the instance: Static/css/typecase.css, Static/fonts/…, Static/js/theme.js,
// Static/js/tc.js.
const Static = "/static/tc"

// Lang is the language of every page: the interface speaks English (U-13).
const Lang = "en"

// Assets are the addresses of the library's files for an instance served at
// base ("" or "/carrel", as CARREL_BASE_PATH normalised).
func Assets(base string) page.Assets { return page.At(base + Static) }

// Files serves the library's files at Static, for a mux that sees paths
// without the base path: mux.Handle(document.Static+"/", document.Files()).
func Files() http.Handler { return http.StripPrefix(Static, page.Static()) }

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
