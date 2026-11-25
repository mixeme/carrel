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
		if err := parseMultipart(r); err != nil {
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
		return parseMultipart(r)
	}
	return r.ParseForm()
}

// bodyCapKey holds the ceiling the middleware put on a request's body.
type bodyCapKey struct{}

// parseMultipart parses a multipart form whole, in memory: the body has a
// ceiling (bodyLimit), and the form is read within it. With less memory
// than the body may hold — a MiB for a photo of up to
// CARREL_PHOTO_MAX_UPLOAD_BYTES — Go writes the file past it to a temporary
// file, a copy of a person's photo on Carrel's disk (audit №4, А4-3). A body
// without a ceiling is never parsed so: its fields are read in front of the
// files (readUploadFields).
func parseMultipart(r *http.Request) error {
	limit, ok := r.Context().Value(bodyCapKey{}).(int64)
	if !ok {
		limit = conf.MaxFormBodyBytes
	}
	return r.ParseMultipartForm(limit)
}
