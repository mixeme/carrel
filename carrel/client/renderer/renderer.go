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
//     (bind.Notice), poll — the poll's bar as its page draws it, the zone
//     poll (document.PollZone; closed while none stands), each with the
//     page's forms; text/html. A poll's bar answered alone (the run's
//     retries, Stop, its fallback) is the zone poll too, with the forms of
//     its retries; a queue of transfers answered alone (an upload's
//     answer) the zone queue as the folder draws it. Out of band (C-7.3)
//     go a rail — drawn by the screen of the page it is of
//     (screens.RailOf) — and a page's main, each in its template
//     (page.Fragment). The rest — topbar, which no screen built draws
//     alone — is JSON.
//   - Event: the events of C-7.4 as a poll's bar answered alone, the rail
//     out of band, their lines the data lines; JSON is JSON.
package renderer

import (
	"io"
	"net/http"
	"reflect"
	"strings"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/client/zone"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
	"carrel.dev/typecase/page"
	w "carrel.dev/typecase/widget"
)

// Renderer answers the server's view models with the client's screens.
type Renderer struct {
	base    string
	json    render.JSONRenderer
	screens map[reflect.Type]screens.Screen
	answers map[reflect.Type]screens.Answer
}

var _ render.Renderer = (*Renderer)(nil)

// New is the renderer of an instance served at base ("" or "/carrel", as
// CARREL_BASE_PATH normalised): the addresses of the library's files in a
// page carry it (document.Assets).
func New(base string) *Renderer {
	r := &Renderer{base: base, screens: map[reflect.Type]screens.Screen{}, answers: map[reflect.Type]screens.Answer{}}
	for _, scr := range screens.All {
		r.screens[reflect.TypeOf(scr.Model()).Elem()] = scr
	}
	for _, a := range screens.Answers {
		r.answers[reflect.TypeOf(a.Model()).Elem()] = a
	}
	return r
}

// answerOf is what body answers alone (screens.Answer), printed.
func (r *Renderer) answerOf(body any) (string, bool) {
	t := reflect.TypeOf(body)
	if t == nil {
		return "", false
	}
	v := reflect.ValueOf(body)
	if t.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "", false
		}
		t, v = t.Elem(), v.Elem()
	}
	a, ok := r.answers[t]
	if !ok {
		return "", false
	}
	p := reflect.New(t)
	p.Elem().Set(v)
	return a.Draw(p.Interface()).Render(), true
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
	if part, ok := r.answerOf(body); ok {
		return writeHTML(w, status, part)
	}
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
	if p, ok := pollOf(body); ok && target == render.TargetPoll {
		if out, ok := r.pollAnswer(p, extra); ok {
			return writeHTML(w, status, out)
		}
		return r.json.Fragment(w, req, status, target, body, extra)
	}
	if q, ok := queueOf(body); ok && target == render.TargetQueue {
		if out, ok := r.queueAnswer(q, extra); ok {
			return writeHTML(w, status, out)
		}
		return r.json.Fragment(w, req, status, target, body, extra)
	}
	scr, model, ok := r.screenOf(body)
	if ok && len(extra) == 0 {
		kind := zone.KindOf(model)
		out, drawn := "", false
		switch {
		case target == render.TargetSheet && kind == zone.Sheet:
			// The sheet's scrim, named: it takes the place of the zone
			// whole, as the live layer puts a widget of its zone.
			out, drawn = SheetZone(scr.Draw(model))+forms(scr, model), true
		case target == render.TargetMain && kind == zone.PlainPage:
			out, drawn = scr.Draw(model).Render()+forms(scr, model), true
		case kind == zone.ShellPage:
			out, drawn = ShellZone(scr, model, target)
		}
		if drawn {
			return writeHTML(w, status, out)
		}
	}
	return r.json.Fragment(w, req, status, target, body, extra)
}

// SheetZone is a sheet answered alone: its scrim named as the zone sheet,
// without the screen it stands over (C-6 Under) — that is on the page.
func SheetZone(root screens.Root) string {
	switch root := document.Zoned(root).(type) {
	case w.Overlay:
		return root.Scrim.Render()
	case w.Shell:
		if over, ok := root.Body.(w.Overlay); ok {
			return over.Scrim.Render()
		}
	}
	return root.Render()
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
		return out + forms(scr, model), true
	case render.TargetPoll:
		p, err := zone.Of(model, target)
		if err != nil {
			return "", false
		}
		// A screen's own bar (the search's), else the lists'; its place
		// closed while none stands.
		bar := screens.PollBar(p.(*view.PollBar))
		if scr.Poll != nil {
			bar = document.EmptyPoll
			if own, ok := scr.Poll(model); ok {
				bar = own
			}
		}
		return document.PollZone(bar, p.(*view.PollBar)).Render() + forms(scr, model), true
	}
	shell, ok := document.Root(scr, model).(w.Shell)
	if !ok {
		return "", false
	}
	body, ok := shell.Body.(w.ShellBody)
	if !ok {
		return "", false
	}
	switch target {
	case render.TargetMain:
		// The page named as its zone, the forms its buttons submit inside
		// it: the answer takes the zone's place whole, the forms with it.
		main := body.Main
		if scr.Forms != nil {
			for _, f := range scr.Forms(model) {
				main.Items = append(main.Items, f)
			}
		}
		return main.Render(), true
	case render.TargetDetails:
		if body.Side == nil || body.Side.Closed {
			return "", true
		}
		return body.Side.Render() + forms(scr, model), true
	case render.TargetRail:
		out := ""
		for _, part := range body.Rail {
			p, ok := part.(renders)
			if !ok {
				return "", false
			}
			out += p.Render()
		}
		return out + forms(scr, model), true
	}
	return "", false
}

// JSON is JSON.
func (r *Renderer) JSON(w http.ResponseWriter, status int, body any) error {
	return r.json.JSON(w, status, body)
}

// Event is an event of C-7.4: a poll's bar as the zone poll with what goes
// out of band (pollAnswer), its lines the data lines of the event; JSON
// for what the screens do not draw alone.
func (r *Renderer) Event(w io.Writer, event, target string, body any, extra []render.OOB) error {
	if p, ok := pollOf(body); ok && target == render.TargetPoll {
		if out, ok := r.pollAnswer(p, extra); ok {
			return writeEvent(w, event, out)
		}
	}
	return r.json.Event(w, event, target, body, extra)
}

// pollOf is body as a poll's bar, by value or by pointer.
func pollOf(body any) (*view.PollBar, bool) {
	switch p := body.(type) {
	case *view.PollBar:
		return p, p != nil
	case view.PollBar:
		return &p, true
	}
	return nil, false
}

// pollAnswer is a poll's bar answered alone: the zone poll as its page
// draws it (screens.PollBar), the forms of its retries after it, and out of
// band what changed with it (outOfBand); false when something out of band
// is not one the screens draw alone.
func (r *Renderer) pollAnswer(p *view.PollBar, extra []render.OOB) (string, bool) {
	f := page.Fragment{Body: []page.Part{document.PollZone(screens.PollBar(p), p)}, Forms: bind.PollForms(p)}
	more, ok := r.outOfBand(extra)
	f.Extra = more
	return f.Render(), ok
}

// queueAnswer is a queue of transfers answered alone (an upload's answer,
// C-7.3): the zone queue as the folder draws it (screens.QueueOf), the forms
// of its answers about a name taken after it, and out of band the folder
// the file went to (outOfBand).
func (r *Renderer) queueAnswer(q view.TransferQueue, extra []render.OOB) (string, bool) {
	queue, forms := screens.QueueOf(q)
	f := page.Fragment{Body: []page.Part{document.QueueZone(queue)}, Forms: forms}
	more, ok := r.outOfBand(extra)
	f.Extra = more
	return f.Render(), ok
}

// outOfBand are the zones that change with an answer (C-7.3), each in its
// template: a rail, drawn by the screen of the page it is of
// (screens.RailOf); a page, the main of its screen (ShellZone). False when
// one is not what the screens draw alone.
func (r *Renderer) outOfBand(extra []render.OOB) ([]page.ZonePart, bool) {
	var out []page.ZonePart
	for _, x := range extra {
		z := page.ZonePart{Zone: x.Target}
		switch x.Target {
		case render.TargetRail:
			rail, ok := x.Body.(view.Rail)
			if !ok {
				return nil, false
			}
			parts, form := screens.RailOf(rail)
			for _, part := range parts {
				p, ok := part.(page.Part)
				if !ok {
					return nil, false
				}
				z.Body = append(z.Body, p)
			}
			if form != nil {
				z.Forms = []w.PostForm{*form}
			}
		case render.TargetMain:
			scr, model, ok := r.screenOf(x.Body)
			if !ok || zone.KindOf(model) != zone.ShellPage {
				return nil, false
			}
			main, ok := ShellZone(scr, model, render.TargetMain)
			if !ok {
				return nil, false
			}
			z.Body = []page.Part{page.Printed(main)}
		default:
			return nil, false
		}
		out = append(out, z)
	}
	return out, true
}

// queueOf is body as a queue of transfers, by value or by pointer.
func queueOf(body any) (view.TransferQueue, bool) {
	switch q := body.(type) {
	case *view.TransferQueue:
		if q == nil {
			return view.TransferQueue{}, false
		}
		return *q, true
	case view.TransferQueue:
		return q, true
	}
	return view.TransferQueue{}, false
}

// writeEvent writes one server-sent event: its name, and each line of what
// it carries as a data line — the browser joins them back with newlines.
func writeEvent(out io.Writer, event, data string) error {
	if _, err := io.WriteString(out, "event: "+event+"\n"); err != nil {
		return err
	}
	for _, line := range strings.Split(data, "\n") {
		if _, err := io.WriteString(out, "data: "+line+"\n"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, "\n")
	return err
}

func writeHTML(w http.ResponseWriter, status int, html string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := io.WriteString(w, html)
	return err
}
