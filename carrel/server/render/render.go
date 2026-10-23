// Package render is the open contract between the request layer and whatever
// draws its answers: the Renderer the server calls with a view model, the
// header names and view values of C-7.1, the target names of C-7.2, the
// out-of-band updates of C-7.3, the events of C-7.4, the names of the token
// (C-11) and of print, and the JSON renderer the server answers with until
// the client (carrel.dev/carrel/client) supplies HTML.
//
// It is a package of the server module, not of internal/, so that the client
// module can implement Renderer and name the targets without reaching into
// the server's internals (docs/architecture.md, «Модули»). It knows no markup:
// no HTML, no CSS, no widget names.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
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

// Targets is every target of C-7.2, in the order of its table. The list is
// closed: the client does not invent targets, the server does not add them.
func Targets() []string {
	return []string{TargetMain, TargetRail, TargetDetails, TargetNotices, TargetPoll, TargetQueue, TargetSheet, TargetTopbar}
}

// The names the client hands the library as data, so that no screen and no
// glue of the client writes them as literals (stage 4 of the client's plan):
// the field and the header of the session's token (C-11) — a form sent by a
// script carries the field and the header both —, and the parameter of a
// printed page (the owner's decision 4: the same route and view model, drawn
// on paper; C-5 names the addresses that take it).
const (
	CSRFField  = "_csrf"
	CSRFHeader = "X-CSRF-Token"
	PrintParam = "print"
	PrintValue = "1"
)

// ManifestPath is the web manifest of installing Carrel as an application
// (U-11, C-5.9), under the base path: the document links it (the library's
// task 91); its icons it names itself.
const ManifestPath = "/manifest.webmanifest"

// SearchSuggestPath is the address of the search line's suggestions
// (Typecase's note 118, its task 227): the document names it, and the live
// layer asks it ?q=… as the line gets the focus and as something is typed;
// the answer is SearchSuggest, a menu, not a page.
const SearchSuggestPath = "/app/search/suggest"

// ThemeColor is the colour the application opens on and the browser draws
// around it: the background of the brand (--bg). The manifest names it, and
// every page as its theme-color (U-11).
const ThemeColor = "#F5F1E8"

// Event names of C-7.4: the events of the progress stream — the bar changed,
// and the poll is over. The data of an event is the fragment of its target
// (FragmentPayload): poll carries the bar, poll-finished the rail of the
// page the poll stands on (C-7.3), or the bar when that page is gone. The
// list is closed (the lead's decision П1, 02.10.2026).
const (
	EventPoll         = "poll"
	EventPollFinished = "poll-finished"
)

// Events is every event of C-7.4, in the order of its table.
func Events() []string { return []string{EventPoll, EventPollFinished} }

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
	return json.NewEncoder(w).Encode(withEmptyLists(body))
}

// withEmptyLists is the rule of C-6: an empty list goes out as [], never as
// null. In Go a list nobody appended to is nil and marshals to null, so the
// same field would be [] on one screen and null on another by accident, and
// every client would have to check both. null keeps one meaning — a pointer
// that is not there: no range on this bar, no details panel, no retry.
//
// The answer is copied before it is filled in: a view model may share slices
// with the session's cache, and writing into those would change what another
// request reads.
func withEmptyLists(body any) any {
	v := reflect.ValueOf(body)
	if !v.IsValid() {
		return body
	}
	out := reflect.New(v.Type()).Elem()
	out.Set(v)
	fillEmptyLists(out)
	return out.Interface()
}

func fillEmptyLists(v reflect.Value) {
	switch v.Kind() {
	case reflect.Slice:
		if v.IsNil() {
			if v.CanSet() {
				v.Set(reflect.MakeSlice(v.Type(), 0, 0))
			}
			return
		}
		fresh := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		reflect.Copy(fresh, v)
		for i := 0; i < fresh.Len(); i++ {
			fillEmptyLists(fresh.Index(i))
		}
		if v.CanSet() {
			v.Set(fresh)
		}
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			fillEmptyLists(v.Index(i))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				fillEmptyLists(v.Field(i))
			}
		}
	case reflect.Pointer:
		if v.IsNil() {
			return // a pointer that is not there stays null: that is its meaning
		}
		fresh := reflect.New(v.Type().Elem())
		fresh.Elem().Set(v.Elem())
		fillEmptyLists(fresh.Elem())
		if v.CanSet() {
			v.Set(fresh)
		}
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		inner := reflect.New(v.Elem().Type()).Elem()
		inner.Set(v.Elem())
		fillEmptyLists(inner)
		if v.CanSet() {
			v.Set(inner)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			e := reflect.New(v.Type().Elem()).Elem()
			e.Set(v.MapIndex(k))
			fillEmptyLists(e)
			v.SetMapIndex(k, e)
		}
	}
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
