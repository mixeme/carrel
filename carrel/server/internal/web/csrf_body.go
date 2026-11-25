package web

import (
	"maps"
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
//
// An upload's body (isFileUpload) has no ceiling, and the middleware has
// read its fields in front of the first file (readUploadFields): those are
// the form, and nothing parses the rest — ParseMultipartForm would write
// the files past a MiB to a temporary file on disk (audit №4, А4-1).
func readForm(r *http.Request) error {
	if up, ok := r.Context().Value(uploadKey{}).(*uploadBody); ok {
		r.PostForm = up.fields
		r.Form = maps.Clone(up.fields)
		for k, vs := range r.URL.Query() {
			r.Form[k] = append(r.Form[k], vs...)
		}
		return nil
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if r.MultipartForm != nil {
			return nil
		}
		return r.ParseMultipartForm(conf.MaxFormBodyBytes)
	}
	return r.ParseForm()
}
