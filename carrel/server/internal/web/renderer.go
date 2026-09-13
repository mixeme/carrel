package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ViewMode distinguishes a full page from a fragment (C-7.1). The client sends
// Carrel-View: fragment; absence of the header means a page navigation.
const (
	HeaderView   = "Carrel-View"
	HeaderTarget = "Carrel-Target"
	ViewPage     = "page"
	ViewFragment = "fragment"
)

// Target names from C-7.2.
const (
	TargetMain    = "main"
	TargetRail    = "rail"
	TargetDetails = "details"
	TargetNotices = "notices"
	TargetPoll    = "poll"
	TargetQueue   = "queue"
	TargetSheet   = "sheet"
	TargetTopbar  = "topbar"
)

// Renderer writes view models to the response. JSONRenderer is used until a
// client supplies HTML (architecture.md).
type Renderer interface {
	Page(w http.ResponseWriter, r *http.Request, status int, body any) error
	Fragment(w http.ResponseWriter, r *http.Request, status int, target string, body any, extra []OOB) error
	JSON(w http.ResponseWriter, status int, body any) error
	// Event writes one server-sent event (C-7.4): its name, and the fragment
	// for the target as the data line.
	Event(w io.Writer, event, target string, body any) error
}

// OOB is an out-of-band fragment update allowed by C-7.3.
type OOB struct {
	Target string `json:"target"`
	Body   any    `json:"body"`
}

// FragmentPayload is the JSON envelope for fragment responses.
type FragmentPayload struct {
	Target string `json:"target"`
	Body   any    `json:"body"`
	Extra  []OOB  `json:"extra,omitempty"`
}

// JSONRenderer encodes view models as application/json.
type JSONRenderer struct{}

func (JSONRenderer) Page(w http.ResponseWriter, r *http.Request, status int, body any) error {
	return writeJSON(w, status, body)
}

func (JSONRenderer) Fragment(w http.ResponseWriter, r *http.Request, status int, target string, body any, extra []OOB) error {
	return writeJSON(w, status, FragmentPayload{Target: target, Body: body, Extra: extra})
}

func (JSONRenderer) JSON(w http.ResponseWriter, status int, body any) error {
	return writeJSON(w, status, body)
}

// Event puts the fragment envelope on one data line: encoding/json writes no
// newline inside it.
func (JSONRenderer) Event(w io.Writer, event, target string, body any) error {
	data, err := json.Marshal(FragmentPayload{Target: target, Body: body})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	return err
}

func writeJSON(w http.ResponseWriter, status int, body any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return nil
	}
	return json.NewEncoder(w).Encode(body)
}

// WantsFragment reports whether the request asks for a fragment rather than a
// full page (C-7.1).
func WantsFragment(r *http.Request) bool {
	return r.Header.Get(HeaderView) == ViewFragment
}

// FragmentTarget returns the primary swap target; default main.
func FragmentTarget(r *http.Request) string {
	if t := r.Header.Get(HeaderTarget); t != "" {
		return t
	}
	return TargetMain
}
