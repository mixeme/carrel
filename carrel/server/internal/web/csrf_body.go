package web

import (
	"net/http"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
)

// csrfTokenFromBody parses urlencoded or multipart form data within the route
// body limit and returns _csrf. X-CSRF-Token is handled separately so uploads
// need not be buffered when the header is present.
func csrfTokenFromBody(r *http.Request) (string, error) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(conf.MaxFormBodyBytes); err != nil {
			return "", err
		}
		return r.FormValue(csrfFormField), nil
	}
	if err := r.ParseForm(); err != nil {
		return "", err
	}
	return r.FormValue(csrfFormField), nil
}

// readForm reads a posted form whatever its encoding: the live layer sends
// the browser's FormData — multipart — with its token in X-CSRF-Token, and
// then nothing before the handler has read the body. ParseForm skips a
// multipart body, so a form so sent was read empty: the rail's tick became
// an empty selection (24.11.2026). A handler reads its form with this.
func readForm(r *http.Request) error {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if r.MultipartForm != nil {
			return nil
		}
		return r.ParseMultipartForm(conf.MaxFormBodyBytes)
	}
	return r.ParseForm()
}
