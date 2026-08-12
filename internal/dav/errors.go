// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	// ErrSSRF is returned when a URL resolves to a blocked address (§24.2).
	ErrSSRF = errors.New("dav: outbound connection blocked")
	// ErrTooManyRedirects is returned when a redirect chain exceeds the limit.
	ErrTooManyRedirects = errors.New("dav: too many redirects")
	// ErrResponseTooLarge is returned when a response body exceeds the limit.
	ErrResponseTooLarge = errors.New("dav: response too large")
)

// HTTPError is a non-2xx DAV response.
type HTTPError struct {
	Code int
	Err  error
}

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("dav: HTTP %d: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("dav: HTTP %d", e.Code)
}

func (e *HTTPError) Unwrap() error { return e.Err }

// StatusCode returns the HTTP status carried by err, or zero when err is not a
// DAV response error.
func StatusCode(err error) int {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Code
	}
	return 0
}

// IsPreconditionFailed reports whether err is a 412: the version named in
// If-Match is no longer the current one (§9).
func IsPreconditionFailed(err error) bool {
	return StatusCode(err) == http.StatusPreconditionFailed
}
