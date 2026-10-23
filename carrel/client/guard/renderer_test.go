package guard_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/fixtures"
	"carrel.dev/carrel/client/renderer"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/client/zone"
	"carrel.dev/carrel/server/render"
	"carrel.dev/typecase/widget"
)

// The renderer cmd/carrel hands the server (carrel.dev/carrel/client/renderer)
// on every fixture: what it draws is held by the frame comparison of
// screens_test.go (htmlFindings) — the same lib, the same frames — and where
// it falls back to JSON, the JSON is the server's own, byte for byte.

// byValue is the view model as the server hands it to the renderer: a value,
// not the pointer Screen.Model makes.
func byValue(model any) any { return reflect.ValueOf(model).Elem().Interface() }

// built tells whether a view model has a screen, or is an answer the
// renderer prints alone (the search line's suggestions).
func built(model any) bool {
	t := reflect.TypeOf(model)
	return slices.ContainsFunc(screens.All, func(scr screens.Screen) bool { return reflect.TypeOf(scr.Model()) == t }) ||
		slices.ContainsFunc(screens.Answers, func(a screens.Answer) bool { return reflect.TypeOf(a.Model()) == t })
}

// writing is one answer of a renderer.
type writing func(render.Renderer, http.ResponseWriter) error

// answer is what rd answers: status, type, body.
func answer(t *testing.T, rd render.Renderer, write writing) (int, string, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := write(rd, rec); err != nil {
		t.Fatal(err)
	}
	return rec.Code, rec.Header().Get("Content-Type"), rec.Body.String()
}

const htmlType = "text/html; charset=utf-8"

// formsOf are the forms a screen prints after its root, printed.
func formsOf(scr screens.Screen, model any) string {
	out := ""
	if scr.Forms != nil {
		for _, f := range scr.Forms(model) {
			out += f.Render()
		}
	}
	return out
}

// rootOf cuts the root out of a served page: after the sprite of icons,
// before the screen's forms and the library's script.
func rootOf(scr screens.Screen, model any, base, page string) (string, bool) {
	const open = "<body data-tc-root><div hidden>"
	i := strings.Index(page, open)
	if i < 0 {
		return "", false
	}
	j := strings.Index(page[i:], "</div>")
	if j < 0 {
		return "", false
	}
	rest := page[i+j+len("</div>"):]
	assets := document.Assets(base)
	tail := formsOf(scr, model) + `<script src="` + assets.JS + `"></script><script src="` + assets.Live + `"></script></body></html>`
	root, ok := strings.CutSuffix(rest, tail)
	return unzoned(root), ok && root != ""
}

// unzoned is a root as its frame draws it: without the empty zone of a
// panel not open (document.Zoned), which the frames, drawn without the
// live layer, do not have.
func unzoned(root string) string {
	return strings.ReplaceAll(root, document.EmptyDetails.Render(), "")
}

// served holds what the renderer served for a fixture against the fixture's
// frame (htmlFindings); a state that waits for the library (screens.Wait) by
// the classes of the library only.
func served(l lib, scr screens.Screen, model any, path, frame, at, html string) []string {
	if _, ok := waits(scr, key(path)); ok {
		return classFindings(l, at, html)
	}
	return htmlFindings(l, frame, at, html, formsOf(scr, model))
}

// Each fixture of a built screen's view model answers a page: the screen's
// document, text/html, with the server's status, whose root — the screen as
// served — passes the frame comparison against the fixture's frame; under a
// base path the library's files are named under it. A fragment of main of a
// page outside the shell, and of sheet of a sheet, is the root and the
// forms, and passes it too.
func TestRendererDrawsBuiltScreens(t *testing.T) {
	l := load(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	n := 0
	for _, scr := range screens.All {
		for _, path := range l.fixtures(scr) {
			at := label(scr) + " on " + key(path)
			frame := l.frameOf(path)
			if frame == "" {
				t.Errorf("%s: no frame", at)
				continue
			}
			model, err := decode(scr, path)
			if err != nil {
				t.Fatalf("%s: %v", at, err)
			}
			n++
			for _, base := range []string{"", "/carrel"} {
				code, ctype, body := answer(t, renderer.New(base), func(rd render.Renderer, w http.ResponseWriter) error {
					return rd.Page(w, req, http.StatusTeapot, byValue(model))
				})
				if code != http.StatusTeapot || ctype != htmlType {
					t.Errorf("%s (base %q): page %d %q", at, base, code, ctype)
					continue
				}
				want, err := document.Of(scr, model, base).Render()
				if err != nil {
					t.Fatal(err)
				}
				if body != want {
					t.Errorf("%s (base %q): the page is not the screen's document", at, base)
					continue
				}
				if !strings.Contains(body, `href="`+base+document.Static+"/"+document.Version()+`/css/typecase.css"`) {
					t.Errorf("%s (base %q): the stylesheet is not under the base path", at, base)
				}
				root, ok := rootOf(scr, model, base, body)
				if !ok {
					t.Errorf("%s (base %q): no root between the sprite and the forms", at, base)
					continue
				}
				for _, f := range served(l, scr, model, path, frame, fmt.Sprintf("%s (%s, page)", at, frame), root) {
					t.Error(f)
				}
			}
			target := render.TargetMain
			switch zone.KindOf(model) {
			case zone.PlainPage:
			case zone.Sheet:
				target = render.TargetSheet
			default:
				continue
			}
			code, ctype, body := answer(t, renderer.New(""), func(rd render.Renderer, w http.ResponseWriter) error {
				return rd.Fragment(w, req, http.StatusOK, target, byValue(model), nil)
			})
			forms := formsOf(scr, model)
			if target == render.TargetSheet {
				// The scrim alone, named as the zone: the part of the page
				// the page's comparison saw.
				scrim := renderer.SheetZone(scr.Draw(model))
				if code != http.StatusOK || ctype != htmlType || body != scrim+forms {
					t.Errorf("%s: fragment of %s %d %q, not the scrim and the forms", at, target, code, ctype)
					continue
				}
				page, err := document.Of(scr, model, "").Render()
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(page, scrim) {
					t.Errorf("%s: the fragment of %s is not a part of the page", at, target)
				}
				continue
			}
			root := scr.Draw(model).Render()
			if code != http.StatusOK || ctype != htmlType || body != root+forms {
				t.Errorf("%s: fragment of %s %d %q, not the root and the forms", at, target, code, ctype)
				continue
			}
			for _, f := range served(l, scr, model, path, frame, fmt.Sprintf("%s (%s, fragment)", at, frame), strings.TrimSuffix(body, forms)) {
				t.Error(f)
			}
		}
	}
	if n == 0 {
		t.Fatal("no fixture of a built screen")
	}
}

// Each fixture of a view model without a screen answers exactly what the
// server's JSON renderer answers: the page, its fragment of main, of sheet.
func TestRendererFallsBackToJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rd := renderer.New("")
	n := 0
	for dir, newModel := range fixtures.Models {
		if built(newModel()) {
			continue
		}
		states, _ := filepath.Glob(filepath.Join(fixtureDir, dir, "*.json"))
		for _, path := range states {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			model := newModel()
			if err := json.Unmarshal(raw, model); err != nil {
				t.Fatalf("%s: %v", key(path), err)
			}
			n++
			for name, write := range map[string]writing{
				"page": func(rd render.Renderer, w http.ResponseWriter) error {
					return rd.Page(w, req, http.StatusOK, byValue(model))
				},
				"main": func(rd render.Renderer, w http.ResponseWriter) error {
					return rd.Fragment(w, req, http.StatusOK, render.TargetMain, byValue(model), nil)
				},
				"sheet": func(rd render.Renderer, w http.ResponseWriter) error {
					return rd.Fragment(w, req, http.StatusOK, render.TargetSheet, byValue(model), nil)
				},
			} {
				c1, t1, b1 := answer(t, rd, write)
				c2, t2, b2 := answer(t, render.JSONRenderer{}, write)
				if c1 != c2 || t1 != t2 || b1 != b2 || !strings.HasPrefix(t1, "application/json") {
					t.Errorf("%s, %s: %d %q, want the JSON renderer's %d %q", key(path), name, c1, t1, c2, t2)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no fixture of a view model without a screen")
	}
}

// A built screen falls back to JSON where the answer is not its screen: a
// zone a page outside the shell does not have, a zone of the shell no screen
// draws alone (queue, topbar), an update out of band, JSON itself, an event.
func TestRendererFallsBackOnBuiltScreens(t *testing.T) {
	l := load(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rd := renderer.New("")
	for _, scr := range screens.All {
		states := l.fixtures(scr)
		if len(states) == 0 {
			t.Errorf("%s: no fixture", label(scr))
			continue
		}
		model, err := decode(scr, states[0])
		if err != nil {
			t.Fatal(err)
		}
		at := label(scr)
		cases := map[string]writing{
			"main with notices out of band": func(rd render.Renderer, w http.ResponseWriter) error {
				return rd.Fragment(w, req, http.StatusOK, render.TargetMain, byValue(model), []render.OOB{{Target: render.TargetNotices, Body: []string{}}})
			},
			"json": func(rd render.Renderer, w http.ResponseWriter) error {
				return rd.JSON(w, http.StatusOK, byValue(model))
			},
		}
		// A page outside the shell has no rail; a page in the shell has a
		// queue and a top bar no screen draws alone. A sheet that is a page
		// has no sheet of its own to answer: the sheet is the screen
		// (TestRendererDrawsBuiltScreens).
		if zone.KindOf(model) != zone.Sheet {
			cases["sheet"] = func(rd render.Renderer, w http.ResponseWriter) error {
				return rd.Fragment(w, req, http.StatusOK, render.TargetSheet, byValue(model), nil)
			}
		}
		others := []string{render.TargetRail}
		if zone.KindOf(model) == zone.ShellPage {
			others = []string{render.TargetQueue, render.TargetTopbar}
		}
		for _, target := range others {
			cases[target] = func(rd render.Renderer, w http.ResponseWriter) error {
				return rd.Fragment(w, req, http.StatusOK, target, byValue(model), nil)
			}
		}
		for name, write := range cases {
			c1, t1, b1 := answer(t, rd, write)
			c2, t2, b2 := answer(t, render.JSONRenderer{}, write)
			if c1 != c2 || t1 != t2 || b1 != b2 {
				t.Errorf("%s, %s: %d %q, want the JSON renderer's %d %q", at, name, c1, t1, c2, t2)
			}
		}
		var got, want bytes.Buffer
		if err := rd.Event(&got, "poll", render.TargetMain, byValue(model)); err != nil {
			t.Fatal(err)
		}
		if err := (render.JSONRenderer{}).Event(&want, "poll", render.TargetMain, byValue(model)); err != nil {
			t.Fatal(err)
		}
		if got.String() != want.String() {
			t.Errorf("%s: the event is not the JSON renderer's", at)
		}
	}
}

// Each fixture of a built screen in the shell answers the zones of C-7.2 its
// screen draws alone as HTML (renderer.ShellZone): main — the page as the
// page shows it, a part of the page that passed the frame comparison —,
// details — the panel, or nothing while it is closed —, rail, notices and
// poll (nothing while no bar stands), each with the page's forms after it
// (main with them inside, the one element that takes the zone's place), and
// each a part of the page served; their classes are the library's.
func TestRendererDrawsShellZones(t *testing.T) {
	l := load(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	n := 0
	for _, scr := range screens.All {
		for _, path := range l.fixtures(scr) {
			model, err := decode(scr, path)
			if err != nil {
				t.Fatal(err)
			}
			if zone.KindOf(model) != zone.ShellPage {
				continue
			}
			if _, shell := scr.Draw(model).(widget.Shell); !shell {
				continue // a page on paper (?print=1, U-10): no zones
			}
			at := label(scr) + " on " + key(path)
			_, _, page := answer(t, renderer.New(""), func(rd render.Renderer, w http.ResponseWriter) error {
				return rd.Page(w, req, http.StatusOK, byValue(model))
			})
			forms := formsOf(scr, model)
			for _, target := range []string{render.TargetMain, render.TargetDetails, render.TargetRail, render.TargetNotices, render.TargetPoll} {
				want, drawn := renderer.ShellZone(scr, model, target)
				if !drawn {
					if target != render.TargetPoll {
						t.Errorf("%s: the screen does not draw %s alone", at, target)
					}
					continue
				}
				n++
				code, ctype, body := answer(t, renderer.New(""), func(rd render.Renderer, w http.ResponseWriter) error {
					return rd.Fragment(w, req, http.StatusOK, target, byValue(model), nil)
				})
				if code != http.StatusOK || ctype != htmlType || body != want {
					t.Errorf("%s, %s: %d %q, not the zone and the forms", at, target, code, ctype)
					continue
				}
				part := strings.TrimSuffix(want, forms)
				if target == render.TargetMain {
					// main carries the forms inside: the answer is the one
					// element that takes the zone's place whole
					shell, _ := document.Zoned(scr.Draw(model)).(widget.Shell)
					body, _ := shell.Body.(widget.ShellBody)
					part = body.Main.Render()
					if bare := strings.TrimSuffix(part, "</div>"); want != bare+forms+"</div>" {
						t.Errorf("%s, %s: not the zone with the forms inside", at, target)
					}
				}
				if part != "" && !strings.Contains(page, part) {
					t.Errorf("%s, %s: the zone is not a part of the page", at, target)
				}
				for _, f := range classFindings(l, at+", "+target, want) {
					t.Error(f)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no zone of a built screen in the shell")
	}
}
