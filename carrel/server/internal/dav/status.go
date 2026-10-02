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
	// Method and Path are the request the server answered so — what the
	// menu of refusals says was asked (states-16); empty when not known.
	Method, Path string
	// Resent is true when the guard had sent this request once already and
	// the connection died before the answer. The refusal then means the
	// first attempt landed, not that someone else took the name.
	Resent bool
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

// WasResent reports whether err came from a request the guard had already
// sent once, its first answer lost to a closed connection (SV-9). A caller
// that made something new reads a refusal differently then: "it is already
// there" is its own write arriving twice, not a name someone else took.
func WasResent(err error) bool {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Resent
	}
	return false
}

// headerResent is how the guard tells the rest of the package that a
// response answered a second attempt. It is set inside the process and
// never trusted from a server: the guard deletes it from every first
// answer before anyone looks.
const headerResent = "X-Carrel-Resent"

func httpStatusError(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	msg := sanitizeRemoteText(b)
	if msg == "" {
		msg = resp.Status
	}
	se := &StatusError{Code: resp.StatusCode, Msg: msg, Resent: resp.Header.Get(headerResent) != ""}
	if resp.Request != nil && resp.Request.URL != nil {
		se.Method, se.Path = resp.Request.Method, resp.Request.URL.Path
	}
	return se
}
