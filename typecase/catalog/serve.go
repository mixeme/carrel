package catalog

import (
	"net/http"

	"carrel.dev/typecase"
)

// sampleSVG is the placeholder image the catalog hands to widgets that take a
// picture (tc-avatar, tc-thumb, tc-crop). It is drawn here rather than stored
// as a binary so the repository keeps no asset the library itself does not
// ship (В-12).
const sampleSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 240" width="240" height="240" role="img" aria-label="Sample">` +
	`<rect width="240" height="240" fill="#e8dfcb"/>` +
	`<circle cx="86" cy="84" r="30" fill="#c9bfa6"/>` +
	`<path d="M0 240 L84 128 L146 196 L188 156 L240 208 L240 240 Z" fill="#4a6b52"/>` +
	`</svg>`

// Mux builds the whole catalog site: both pages, the stylesheet and scripts,
// and the fonts, icons and sample image the markup links to. Passing a css
// override replaces the library bundle; an empty string uses it.
//
// Everything the pages reference is served here. An unknown path answers 404
// rather than the page itself: a catch-all returns HTML for a font request,
// the face fails to parse, and the catalog silently measures fallback
// typefaces (В-12).
func Mux(css string) (*http.ServeMux, error) {
	if css == "" {
		bundle, err := typecase.CSS()
		if err != nil {
			return nil, err
		}
		css = bundle
	}
	themeJS, err := typecase.ThemeJS()
	if err != nil {
		return nil, err
	}
	js, err := typecase.JS()
	if err != nil {
		return nil, err
	}
	entries := Entries()
	withJS := PageHTML("Typecase catalog", entries, true)
	withoutJS := PageHTML("Typecase catalog (no behavior)", entries, false)

	mux := http.NewServeMux()
	page := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" && r.URL.Path != "/static" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(body))
		}
	}
	mux.HandleFunc("/", page(withJS))
	mux.HandleFunc("/static", page(withoutJS))

	text := func(mime string, body []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", mime)
			_, _ = w.Write(body)
		}
	}
	mux.HandleFunc("/typecase.css", text("text/css; charset=utf-8", []byte(css)))
	mux.HandleFunc("/catalog.css", text("text/css; charset=utf-8", []byte(CSS())))
	mux.HandleFunc("/theme.js", text("application/javascript; charset=utf-8", themeJS))
	mux.HandleFunc("/tc.js", text("application/javascript; charset=utf-8", js))
	mux.HandleFunc("/img/sample.svg", text("image/svg+xml; charset=utf-8", []byte(sampleSVG)))

	mux.Handle("/fonts/", http.FileServerFS(typecase.Fonts()))
	mux.Handle("/icons/", http.FileServerFS(typecase.Icons()))

	return mux, nil
}
