// Package renderer is the client's side of the server's render contract
// (carrel.dev/carrel/server/render): the Renderer cmd/carrel hands the
// server, which answers a view model with the screen of its type
// (screens.All) and with JSON where there is none yet.
//
// The fallback is by the type of the view model, never by the address: a
// screen is a screen of a view model, and the server keeps answering JSON
// for every model whose screen is not built, as it did before the client
// was wired (render.JSONRenderer). The rules:
//
//   - Page: the model has a screen — the screen's document (document.Of:
//     Typecase's page around the root, the forms at its end), text/html,
//     with the status the server gives; otherwise JSON.
//   - Fragment: the model has a screen, the target is main of a page
//     outside the shell or sheet of a sheet, and nothing goes out of band —
//     the screen's root and the forms its buttons submit, text/html;
//     otherwise JSON. The zones proper — rail, details, notices, poll,
//     queue, topbar cut out of a page in the shell (zone), and the
//     out-of-band updates of C-7.3 — come with the shell and Typecase's
//     swapping of zones; until then they are JSON.
//   - Event and JSON: JSON — the events of C-7.4 carry fragments of the
//     shell, and JSON is JSON.
package renderer

import (
	"io"
	"net/http"
	"reflect"

	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/client/zone"
	"carrel.dev/carrel/server/render"
)

// Renderer answers the server's view models with the client's screens.
type Renderer struct {
	base    string
	json    render.JSONRenderer
	screens map[reflect.Type]screens.Screen
}

var _ render.Renderer = (*Renderer)(nil)

// New is the renderer of an instance served at base ("" or "/carrel", as
// CARREL_BASE_PATH normalised): the addresses of the library's files in a
// page carry it (document.Assets).
func New(base string) *Renderer {
	r := &Renderer{base: base, screens: map[reflect.Type]screens.Screen{}}
	for _, scr := range screens.All {
		r.screens[reflect.TypeOf(scr.Model()).Elem()] = scr
	}
	return r
}

// screenOf is the screen of body's type and the model to draw it from — a
// pointer, as Screen.Model makes it: a body handed by value is copied.
func (r *Renderer) screenOf(body any) (screens.Screen, any, bool) {
	v := reflect.ValueOf(body)
	if !v.IsValid() {
		return screens.Screen{}, nil, false
	}
	t := v.Type()
	byValue := t.Kind() != reflect.Pointer
	if !byValue {
		if v.IsNil() {
			return screens.Screen{}, nil, false
		}
		t = t.Elem()
	}
	scr, ok := r.screens[t]
	if !ok {
		return screens.Screen{}, nil, false
	}
	if byValue {
		p := reflect.New(t)
		p.Elem().Set(v)
		body = p.Interface()
	}
	return scr, body, true
}

// Page answers a page: the screen's document, or JSON while the model has no
// screen.
func (r *Renderer) Page(w http.ResponseWriter, req *http.Request, status int, body any) error {
	scr, model, ok := r.screenOf(body)
	if !ok {
		return r.json.Page(w, req, status, body)
	}
	page, err := document.Of(scr, model, r.base).Render()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return err
	}
	return writeHTML(w, status, page)
}

// Fragment answers a fragment: the screen's root and its forms for main of a
// page outside the shell and for sheet of a sheet, when nothing goes out of
// band; JSON otherwise (the package's doc).
func (r *Renderer) Fragment(w http.ResponseWriter, req *http.Request, status int, target string, body any, extra []render.OOB) error {
	scr, model, ok := r.screenOf(body)
	if ok && len(extra) == 0 {
		kind := zone.KindOf(model)
		if (target == render.TargetMain && kind == zone.PlainPage) || (target == render.TargetSheet && kind == zone.Sheet) {
			out := scr.Draw(model).Render()
			if scr.Forms != nil {
				for _, f := range scr.Forms(model) {
					out += f.Render()
				}
			}
			return writeHTML(w, status, out)
		}
	}
	return r.json.Fragment(w, req, status, target, body, extra)
}

// JSON is JSON.
func (r *Renderer) JSON(w http.ResponseWriter, status int, body any) error {
	return r.json.JSON(w, status, body)
}

// Event is JSON until the shell: the events of C-7.4 carry its zones.
func (r *Renderer) Event(w io.Writer, event, target string, body any) error {
	return r.json.Event(w, event, target, body)
}

func writeHTML(w http.ResponseWriter, status int, html string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := io.WriteString(w, html)
	return err
}
