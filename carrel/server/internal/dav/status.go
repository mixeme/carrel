package dav

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

// StatusError is a remote HTTP status from DAV (S-5).
type StatusError struct {
	Code int
	Msg  string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("dav: HTTP %d: %s", e.Code, e.Msg)
}

// IsStatus reports whether err is a StatusError with the given code.
func IsStatus(err error, code int) bool {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Code == code
	}
	return false
}

// StatusCode returns the HTTP status from err, or 0.
func StatusCode(err error) int {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Code
	}
	return 0
}

func httpStatusError(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	msg := sanitizeRemoteText(b)
	if msg == "" {
		msg = resp.Status
	}
	return &StatusError{Code: resp.StatusCode, Msg: msg}
}
