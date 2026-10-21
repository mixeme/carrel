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
//   - Fragment: the model has a screen and nothing goes out of band. Of a
//     page outside the shell, main, and of a sheet, sheet, are the screen's
//     root and the forms its buttons submit. Of a page in the shell, a zone
//     of C-7.2 the screen draws alone is the part of its tree that zone is
//     (shellZone): main — the page (tc-main), details — the panel (tc-side;
//     an empty answer when there is none, which closes it), rail — the
//     rail's parts, notices — the notices as the screens draw them
//     (bind.Notice), poll — the poll's bar (bind.Runbar; empty while none
//     stands), each with the page's forms; text/html. The rest — queue and
//     topbar, which no screen built draws alone, the out-of-band updates of
//     C-7.3 — is JSON until Typecase swaps zones.
//   - Event and JSON: JSON — the events of C-7.4 carry fragments of the
//     shell, and JSON is JSON.
package renderer

import (
	"io"
	"net/http"
	"reflect"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/client/zone"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
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
// page outside the shell and for sheet of a sheet, a zone the screen draws
// alone of a page in the shell, when nothing goes out of band; JSON
// otherwise (the package's doc).
func (r *Renderer) Fragment(w http.ResponseWriter, req *http.Request, status int, target string, body any, extra []render.OOB) error {
	scr, model, ok := r.screenOf(body)
	if ok && len(extra) == 0 {
		kind := zone.KindOf(model)
		out, drawn := "", false
		switch {
		case (target == render.TargetMain && kind == zone.PlainPage) || (target == render.TargetSheet && kind == zone.Sheet):
			out, drawn = scr.Draw(model).Render(), true
		case kind == zone.ShellPage:
			out, drawn = ShellZone(scr, model, target)
		}
		if drawn {
			return writeHTML(w, status, out+forms(scr, model))
		}
	}
	return r.json.Fragment(w, req, status, target, body, extra)
}

// forms are the forms a screen prints at the page's end, printed.
func forms(scr screens.Screen, model any) string {
	out := ""
	if scr.Forms != nil {
		for _, f := range scr.Forms(model) {
			out += f.Render()
		}
	}
	return out
}

// renders is a widget that prints itself.
type renders interface{ Render() string }

// ShellZone is the zone target of a page in the shell as its screen draws it
// (the package's doc), and whether the screen draws that zone alone: main,
// details and rail are the parts of the screen's shell; notices and poll are
// the model's zone (zone.Of) translated as the screens translate them.
func ShellZone(scr screens.Screen, model any, target string) (string, bool) {
	switch target {
	case render.TargetNotices:
		ns, err := zone.Of(model, target)
		if err != nil {
			return "", false
		}
		// The messages: a notice's technical block (MainDiag, under it on the
		// page) prints nothing alone in the library — the zone carries none.
		out := ""
		for _, n := range ns.([]view.Notice) {
			space := scr.Notices
			if scr.NoticesOf != nil {
				space = scr.NoticesOf(model)
			}
			out += bind.Notice(n, space).Render()
		}
		return out, true
	case render.TargetPoll:
		p, err := zone.Of(model, target)
		if err != nil {
			return "", false
		}
		if bar := bind.Runbar(p.(*view.PollBar)); bar != nil {
			return bar.Render(), true
		}
		return "", true
	}
	shell, ok := scr.Draw(model).(w.Shell)
	if !ok {
		return "", false
	}
	body, ok := shell.Body.(w.ShellBody)
	if !ok {
		return "", false
	}
	switch target {
	case render.TargetMain:
		return body.Main.Render(), true
	case render.TargetDetails:
		if body.Side == nil {
			return "", true
		}
		return body.Side.Render(), true
	case render.TargetRail:
		out := ""
		for _, part := range body.Rail {
			p, ok := part.(renders)
			if !ok {
				return "", false
			}
			out += p.Render()
		}
		return out, true
	}
	return "", false
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
