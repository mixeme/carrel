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
