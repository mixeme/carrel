package web

import (
	"net/http"
	"strings"
)

// The auto-save of an edit (Typecase's note 229): with Auto-save ticked the
// library's script sends the edit's form by itself after a pause in typing,
// to the form's own address, as Save sends it, with this header, and stays
// on the page. Any 2xx is «written», anything else «not written»: the typed
// text then stays a draft in the window. A redirect is followed, and the
// address it ends at is where the form goes from then on — so a new record
// is created by its first auto-save and edited by the next.
const autosaveHeader = "Tc-Autosave"

// autosaving tells whether a write is an edit's auto-save.
func autosaving(r *http.Request) bool { return r.Header.Get(autosaveHeader) == "1" }

// autosaveWriter answers an auto-save. The handlers answer it as they answer
// Save; what they say is turned into what the script reads:
//
//   - an edit sent to its record's page (done) — written: 204, no body;
//   - a new record sent to its edit (created, an address ending in /edit) —
//     written: the redirect stands, the script follows it and the form goes
//     there;
//   - anything else a page or a redirect would say — the form again with its
//     errors, a loss report, the conflict screen, a question — not written:
//     409, no body. The edit stays a draft in the window, and Save says what
//     the page would have said.
//
// Errors of HTTP (400, 404, 413…) stand as they are.
type autosaveWriter struct {
	http.ResponseWriter
	done    string // the routed address of the record's page (an edit)
	created bool   // a new record's form
	header  bool
	mute    bool
}

func (a *autosaveWriter) WriteHeader(code int) {
	if a.header {
		return
	}
	a.header = true
	h := a.Header()
	loc := h.Get("Location")
	switch {
	case code >= 400:
		a.ResponseWriter.WriteHeader(code)
		return
	case code == http.StatusSeeOther && !a.created && loc == a.done:
		code = http.StatusNoContent
	case code == http.StatusSeeOther && a.created && strings.HasSuffix(loc, "/edit"):
		a.ResponseWriter.WriteHeader(code)
		return
	default:
		code = http.StatusConflict
	}
	h.Del("Location")
	h.Del("Content-Type")
	h.Del("Content-Length")
	a.mute = true
	a.ResponseWriter.WriteHeader(code)
}

func (a *autosaveWriter) Write(b []byte) (int, error) {
	if !a.header {
		a.WriteHeader(http.StatusOK)
	}
	if a.mute {
		return len(b), nil
	}
	return a.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the connection's writer.
func (a *autosaveWriter) Unwrap() http.ResponseWriter { return a.ResponseWriter }
