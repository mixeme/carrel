package screens

import (
	"fmt"
	"net/http"
	"strings"

	"carrel.dev/typecase"
)

const sampleSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 240" width="240" height="240" role="img" aria-label="Sample">` +
	`<rect width="240" height="240" fill="#e8dfcb"/>` +
	`<circle cx="86" cy="84" r="30" fill="#c9bfa6"/>` +
	`<path d="M0 240 L84 128 L146 196 L188 156 L240 208 L240 240 Z" fill="#4a6b52"/>` +
	`</svg>`

// Mux builds the assembled-screens site: index, one page per screen, styles,
// scripts, fonts, icons and the sample image the markup links to.
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

	mux := http.NewServeMux()
	text := func(mime string, body []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", mime)
			_, _ = w.Write(body)
		}
	}
	mux.HandleFunc("/typecase.css", text("text/css; charset=utf-8", []byte(css)))
	mux.HandleFunc("/screens.css", text("text/css; charset=utf-8", []byte(CSS())))
	mux.HandleFunc("/theme.js", text("application/javascript; charset=utf-8", themeJS))
	mux.HandleFunc("/tc.js", text("application/javascript; charset=utf-8", js))
	mux.HandleFunc("/img/sample.svg", text("image/svg+xml; charset=utf-8", []byte(sampleSVG)))
	mux.Handle("/fonts/", http.FileServerFS(typecase.Fonts()))
	mux.Handle("/icons/", http.FileServerFS(typecase.Icons()))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(PageHTML("Typecase screens", "", All(), r.URL.Query().Get("theme"))))
	})
	for _, s := range All() {
		sc := s
		path := "/" + sc.ID
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != path {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(PageHTML(sc.Title, sc.ID, All(), r.URL.Query().Get("theme"))))
		})
	}
	return mux, nil
}

func themeQuery(theme string) string {
	if theme == "dark" || theme == "light" {
		return "?theme=" + theme
	}
	return ""
}

// PageHTML renders a full screen page.
func PageHTML(title, current string, screens []Screen, theme string) string {
	q := themeQuery(theme)
	var nav strings.Builder
	for _, s := range screens {
		cls := ""
		if s.ID == current {
			cls = ` class="is-current"`
		}
		nav.WriteString(`<a href="/` + s.ID + q + `"` + cls + `>` + s.Title + `</a>`)
	}
	lightCls, darkCls := "", ""
	switch theme {
	case "dark":
		darkCls = ` class="is-current"`
	case "light":
		lightCls = ` class="is-current"`
	}
	toolbar := `<nav class="screens-toolbar" aria-label="Theme">` +
		`<a href="?theme=light"` + lightCls + `>Light</a>` +
		`<a href="?theme=dark"` + darkCls + `>Dark</a>` +
		`</nav>`
	body := ""
	if current != "" {
		if sc := ByID(current); sc != nil {
			body = sc.Body()
		}
	} else {
		body = `<p>Pick a screen above.</p>`
	}
	themeJS := `<script src="/theme.js"></script>`
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-tc-root>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
%s
<link rel="stylesheet" href="/typecase.css">
<link rel="stylesheet" href="/screens.css">
</head>
<body data-tc-root>
<header class="screens-nav">%s</header>
%s
<h1>%s</h1>
<div class="screens-stage">%s</div>
<script src="/tc.js"></script>
</body>
</html>`, title, themeJS, nav.String(), toolbar, title, body)
}
