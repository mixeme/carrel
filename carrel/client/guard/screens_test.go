package guard_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"carrel.dev/carrel/client/fixtures"
	"carrel.dev/carrel/client/guard"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/server/view"
	"carrel.dev/typecase"
	"carrel.dev/typecase/shape"
	w "carrel.dev/typecase/widget"
)

// The guards that need the library (U-15.2, 3 and 4) and the addresses of
// C-5 (U-15.2, 5), on the screens of screens.All — and, for their bites, on
// samples of this file that are not screens of the client.

// From carrel/client/guard up to the repository root.
const (
	mockupPath = "../../../docs/visual/carrel-ui-mockups.html"
	checkPy    = "../../../docs/spec/check.py"
	fixtureDir = "../../../docs/spec/fixtures"
	commonSpec = "../../../docs/spec/carrel-common.md"
)

// lib is what the checks read once: the mockup of Carrel and of Typecase,
// the mockup parsed by shape, the classes of Typecase's CSS, the maps of
// check.py — fixtures to routes, states to their frames.
type lib struct {
	mock, tcMock string
	shape        *shape.Mockup
	classes      map[string]bool
	routes       map[string]string // fixture directory → route of C-5
	states       map[string]string // «directory/state» → the frame the state draws
}

var (
	libOnce sync.Once
	libVal  lib
	libErr  error
)

func load(t *testing.T) lib {
	t.Helper()
	libOnce.Do(func() { libVal, libErr = loadLib() })
	if libErr != nil {
		t.Fatal(libErr)
	}
	return libVal
}

func loadLib() (lib, error) {
	var l lib
	raw, err := os.ReadFile(mockupPath)
	if err != nil {
		return l, err
	}
	l.mock = string(raw)
	dir, err := typecaseDir()
	if err != nil {
		return l, err
	}
	raw, err = os.ReadFile(filepath.Join(dir, "docs", "visual", "carrel-ui-mockups.html"))
	if err != nil {
		return l, err
	}
	l.tcMock = string(raw)
	if l.shape, err = shape.Parse(l.mock); err != nil {
		return l, err
	}
	css, err := typecase.CSS()
	if err != nil {
		return l, err
	}
	l.classes = map[string]bool{}
	for _, m := range regexp.MustCompile(`\.(-?[_a-zA-Z][_a-zA-Z0-9-]*)`).FindAllStringSubmatch(css, -1) {
		l.classes[m[1]] = true
	}
	// The names the mockup gives a node without a look of its own: the CSS
	// lists them in its comment «names-without-rule», and Typecase's lint
	// reads the list the same way.
	if m := regexp.MustCompile(`(?s)names-without-rule:(.*?)—`).FindStringSubmatch(css); m != nil {
		for _, c := range strings.Fields(m[1]) {
			l.classes[c] = true
		}
	} else {
		return l, fmt.Errorf("Typecase's CSS has no list names-without-rule")
	}
	raw, err = os.ReadFile(checkPy)
	if err != nil {
		return l, err
	}
	py := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if l.routes, err = pyMap(py, "FIXTURE_SCREEN"); err != nil {
		return l, err
	}
	if len(l.routes) == 0 {
		return l, fmt.Errorf("check.py: FIXTURE_SCREEN is empty")
	}
	if l.states, err = pyMap(py, "STATE_FRAME"); err != nil {
		return l, err
	}
	return l, nil
}

// typecaseDir is the directory of the module carrel.dev/typecase as the
// client module resolves it (the replace of go.mod).
func typecaseDir() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "carrel.dev/typecase")
	cmd.Dir = module
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list -m carrel.dev/typecase: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// pyMap reads the map name of docs/spec/check.py, a dict of strings to
// strings: FIXTURE_SCREEN — fixture directories to the routes of C-5 —, and
// STATE_FRAME — states of a fixture («login/refused») to the frame that draws
// them when it is not the route's (s-auth-13). The two are check.py's, and
// the client reads them, not a copy of them.
func pyMap(py, name string) (map[string]string, error) {
	i := strings.Index(py, "\n"+name+" = {")
	if i < 0 {
		return nil, fmt.Errorf("check.py: no %s", name)
	}
	// The dict ends at the first «}» outside a string and a comment (a
	// route has braces of its own: «/invite/{token}»); what it holds is read
	// without its comments.
	var dict strings.Builder
	quoted, comment, ended := false, false, false
	for _, c := range py[i+len(name)+5:] {
		switch {
		case comment:
			comment = c != '\n'
		case quoted:
			quoted = c != '"'
		case c == '"':
			quoted = true
		case c == '#':
			comment = true
			continue
		case c == '}':
			ended = true
		}
		if ended {
			break
		}
		if !comment {
			dict.WriteRune(c)
		}
	}
	if !ended {
		return nil, fmt.Errorf("check.py: %s does not end", name)
	}
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`"([^"]+)":\s*"([^"]+)"`).FindAllStringSubmatch(dict.String(), -1) {
		out[m[1]] = m[2]
	}
	return out, nil
}

var (
	frameDiv    = regexp.MustCompile(`<div class="frame"([^>]*)>`)
	dataScreen  = regexp.MustCompile(`data-screen="([^"]+)"`)
	dataFrame   = regexp.MustCompile(`data-frame="([^"]+)"`)
	kindOfRoute = regexp.MustCompile(`/app/(contacts|calendar|tasks|notes)/`)
)

// canonFrame is the frame of a route as check.py's canon_frame finds it: the
// first frame, not a part, whose data-screen names the route — exactly
// first, then by the template of kinds.
func canonFrame(mock, route string) string {
	asKind := func(s string) string { return kindOfRoute.ReplaceAllString(s, "/app/{kind}/") }
	for _, exact := range []bool{true, false} {
		for _, m := range frameDiv.FindAllStringSubmatch(mock, -1) {
			attrs := m[1]
			if strings.Contains(attrs, "data-part=") {
				continue
			}
			ds := dataScreen.FindStringSubmatch(attrs)
			df := dataFrame.FindStringSubmatch(attrs)
			if ds == nil || df == nil {
				continue
			}
			for _, name := range strings.Fields(ds[1]) {
				if (exact && name == route) || (!exact && asKind(name) == asKind(route)) {
					return df[1]
				}
			}
		}
	}
	return ""
}

// frameBlock is the frame's text in a mockup, from its div to the next
// frame's.
func frameBlock(mock, id string) string {
	at := strings.Index(mock, `data-frame="`+id+`"`)
	if at < 0 {
		return ""
	}
	start := strings.LastIndex(mock[:at], `<div class="frame"`)
	end := strings.Index(mock[at:], `<div class="frame"`)
	if end < 0 {
		return mock[start:]
	}
	return mock[start : at+end]
}

// modelName is the name of a screen's view model: the key of the registry.
func modelName(scr screens.Screen) string {
	return typeName(scr.Model())
}

// typeName is the name of the type a view model points to.
func typeName(model any) string {
	return reflect.TypeOf(model).Elem().Name()
}

// label names a screen in a finding: its view model and its frames.
func label(scr screens.Screen) string {
	return modelName(scr) + " (" + strings.Join(scr.Frames, ", ") + ")"
}

// fixtures are the state files of every fixture directory whose view model
// (fixtures.Models) is the screen's, sorted: a screen is a screen of a view
// model, so the dead link's fixture on the route of the invitation
// (FIXTURE_SCREEN) is the dead link's, not the invitation's.
func (l lib) fixtures(scr screens.Screen) []string {
	name := modelName(scr)
	var out []string
	for dir, model := range fixtures.Models {
		if typeName(model()) != name {
			continue
		}
		states, _ := filepath.Glob(filepath.Join(fixtureDir, dir, "*.json"))
		out = append(out, states...)
	}
	slices.Sort(out)
	return out
}

// decode decodes a fixture into the screen's view model — no field the
// model lacks.
func decode(scr screens.Screen, path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	model := scr.Model()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(model); err != nil {
		return nil, fmt.Errorf("decode into %T: %v", model, err)
	}
	return model, nil
}

// draw decodes a fixture into the screen's view model and draws the screen.
func draw(scr screens.Screen, path string) (screens.Root, error) {
	model, err := decode(scr, path)
	if err != nil {
		return nil, err
	}
	return scr.Draw(model), nil
}

var classAttr = regexp.MustCompile(`class="([^"]*)"`)

// key is a fixture's state as check.py names it: «login/refused».
func key(path string) string {
	return filepath.Base(filepath.Dir(path)) + "/" + strings.TrimSuffix(filepath.Base(path), ".json")
}

// frameOf is the frame a fixture is held against, as check.py's check 8
// finds it: the frame STATE_FRAME gives the fixture's state — the refused
// sign-in, login/refused, is s-auth-13 —, else the frame of the fixture's
// route (FIXTURE_SCREEN, canonFrame). "" — check.py gives the fixture no
// frame.
func (l lib) frameOf(path string) string {
	if f, ok := l.states[key(path)]; ok {
		return f
	}
	if route, ok := l.routes[filepath.Base(filepath.Dir(path))]; ok {
		return canonFrame(l.mock, route)
	}
	return ""
}

// waits is the wait of a screen for a fixture's state («login/refused»),
// if it has one (screens.Wait).
func waits(scr screens.Screen, k string) (screens.Wait, bool) {
	for _, w := range scr.Waits {
		if w.Fixture != "" && w.Fixture == k {
			return w, true
		}
	}
	return screens.Wait{}, false
}

// frameFindings is the frame comparison (U-15.2, 4; U-15.1) of each screen:
// on every fixture of its view model the screen has the make-up of the
// fixture's frame (shape.Check) — the frame check.py gives it, its state's by
// STATE_FRAME (which need not name the route in data-screen: the lead's
// decision of 01.10.2026), else its route's; one the screen names — and no
// class Typecase's CSS does not have. Each frame the screen names is drawn by
// a fixture, and is the same in both copies of the mockup. A state that
// waits for the library (screens.Wait, U-5.5) is drawn and its classes
// held, not its make-up; and one whose make-up already matches its frame
// waits for nothing — its row is to be closed.
func frameFindings(l lib, all []screens.Screen) []string {
	var out []string
	for _, scr := range all {
		at := label(scr)
		copies := map[string]bool{}
		sameCopies := func(frame string) {
			if copies[frame] {
				return
			}
			copies[frame] = true
			if a, b := frameBlock(l.mock, frame), frameBlock(l.tcMock, frame); a == "" || a != b {
				out = append(out, fmt.Sprintf("%s: frame %s differs between the mockup of Carrel and of Typecase", at, frame))
			}
		}
		states := l.fixtures(scr)
		if len(states) == 0 {
			out = append(out, at+": no fixture of its view model")
		}
		drawn := map[string]bool{}
		for _, path := range states {
			fx := filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
			route, ok := l.routes[filepath.Base(filepath.Dir(path))]
			if !ok {
				out = append(out, fmt.Sprintf("%s on %s: no route in FIXTURE_SCREEN of check.py", at, fx))
				continue
			}
			frame := l.frameOf(path)
			switch {
			case frame == "":
				out = append(out, fmt.Sprintf("%s on %s: no frame of the route %s", at, fx, route))
				continue
			case !slices.Contains(scr.Frames, frame):
				out = append(out, fmt.Sprintf("%s on %s: drawn by %s (check.py), which the screen does not name", at, fx, frame))
				continue
			}
			drawn[frame] = true
			sameCopies(frame)
			fx += " (" + frame + ")"
			root, err := draw(scr, path)
			if err != nil {
				out = append(out, fmt.Sprintf("%s on %s: %v", at, fx, err))
				continue
			}
			if w, ok := waits(scr, key(path)); ok {
				out = append(out, classFindings(l, at+" on "+fx, root.Render())...)
				if l.shape.Check(frame, root.Render()) == nil {
					out = append(out, fmt.Sprintf("%s on %s: waits (U-5.5 #%d) and matches its frame", at, fx, w.Row))
				}
				continue
			}
			out = append(out, htmlFindings(l, frame, at+" on "+fx, root.Render())...)
		}
		for _, f := range scr.Frames {
			if !drawn[f] {
				out = append(out, fmt.Sprintf("%s: no fixture of its view model is drawn by %s", at, f))
			}
		}
	}
	return out
}

// htmlFindings holds a screen's printed HTML against its frame: the frame's
// make-up (shape.Check) and no class Typecase's CSS does not have.
func htmlFindings(l lib, frame, at, html string) []string {
	var out []string
	if err := l.shape.Check(frame, html); err != nil {
		out = append(out, fmt.Sprintf("%s: %v", at, err))
	}
	return append(out, classFindings(l, at, html)...)
}

// classFindings holds a screen's printed HTML to the classes of Typecase's
// CSS.
func classFindings(l lib, at, html string) []string {
	var out []string
	for _, m := range classAttr.FindAllStringSubmatch(html, -1) {
		for _, c := range strings.Fields(m[1]) {
			if !l.classes[c] {
				out = append(out, fmt.Sprintf("%s: class %q is not a class of Typecase's CSS", at, c))
			}
		}
	}
	return out
}

// kinFindings is the kinship with the draft (U-15.2, 3): the screen drawn
// on each normal fixture of its view model, and on each fixture whose state
// has a frame of its own (STATE_FRAME), against the draft of that fixture's
// frame — each frame the screen names has its draft. A state that waits for
// the library (screens.Wait) is not held against a draft.
func kinFindings(l lib, drafts map[string]*guard.Node, all []screens.Screen) []string {
	var out []string
	for _, scr := range all {
		at := label(scr)
		for _, f := range scr.Frames {
			if d, ok := drafts[f]; !ok || d == nil {
				out = append(out, fmt.Sprintf("%s: no draft of %s", at, f))
			}
		}
		normal := 0
		for _, path := range l.fixtures(scr) {
			_, state := l.states[key(path)]
			if filepath.Base(path) == "normal.json" {
				normal++
			} else if !state {
				continue
			}
			if _, ok := waits(scr, key(path)); ok {
				continue
			}
			frame := l.frameOf(path)
			fx := filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path) + " (" + frame + ")"
			d, ok := drafts[frame]
			if !ok || d == nil {
				out = append(out, fmt.Sprintf("%s on %s: no draft", at, fx))
				continue
			}
			root, err := draw(scr, path)
			if err != nil {
				out = append(out, fmt.Sprintf("%s: %v", at, err))
				continue
			}
			if err := guard.Kin(d, guard.Skeleton(root)); err != nil {
				out = append(out, fmt.Sprintf("%s on %s: %v", at, fx, err))
			}
		}
		if normal == 0 {
			out = append(out, at+": no normal fixture")
		}
	}
	return out
}

// frames are the frames the screens are held against: those each screen
// names, and the frames of its fixtures.
func frames(l lib, all []screens.Screen) []string {
	var out []string
	add := func(f string) {
		if f != "" && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	for _, s := range all {
		for _, f := range s.Frames {
			add(f)
		}
		for _, path := range l.fixtures(s) {
			add(l.frameOf(path))
		}
	}
	return out
}

func drafts(t *testing.T, l lib, all []screens.Screen) map[string]*guard.Node {
	t.Helper()
	d, err := guard.Drafts(module, frames(l, all))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func none(t *testing.T, what string, found []string) {
	t.Helper()
	for _, f := range found {
		t.Errorf("%s: %s", what, f)
	}
}

// --- The real screens. ---

func TestScreensMatchTheirFrames(t *testing.T) {
	none(t, "frame", frameFindings(load(t), screens.All))
}

func TestScreensAreKinOfTheirDrafts(t *testing.T) {
	l := load(t)
	none(t, "kinship", kinFindings(l, drafts(t, l, screens.All), screens.All))
}

func TestAddressesBuiltOrWaiting(t *testing.T) {
	var built []string
	for _, s := range screens.All {
		built = append(built, modelName(s))
	}
	found, left, err := guard.CheckAddresses(commonSpec, clientSpec, built)
	if err != nil {
		t.Fatal(err)
	}
	green(t, "addresses", found)
	// Every address built or waiting is acceptance (U-15.4, plan stage 6):
	// until then the addresses left are the work, named, not a failure.
	if os.Getenv("CARREL_ACCEPTANCE") != "" {
		for _, a := range left {
			t.Errorf("%s: neither built nor in the list «ждёт библиотеку»", a)
		}
	} else if len(left) > 0 {
		t.Logf("%d addresses of C-5 have no screen yet (CARREL_ACCEPTANCE=1 makes them red)", len(left))
	}
}

// A screen's fixtures are its view model's: the dead link's fixture stands
// on the invitation's route in FIXTURE_SCREEN, and is the dead link's.
func TestFixturesAreTheViewModels(t *testing.T) {
	l := load(t)
	names := func(model func() any) []string {
		var out []string
		for _, p := range l.fixtures(screens.Screen{Model: model}) {
			out = append(out, key(p))
		}
		return out
	}
	if got := names(func() any { return new(view.DeadLinkPage) }); !slices.Equal(got, []string{"dead-link/normal"}) {
		t.Errorf("the dead link's fixtures: %q", got)
	}
	for _, k := range names(func() any { return new(view.InvitePage) }) {
		if !strings.HasPrefix(k, "invite/") {
			t.Errorf("the invitation's fixtures hold %s", k)
		}
	}
	if l.routes["dead-link"] != l.routes["invite"] {
		t.Errorf("the sample moved: FIXTURE_SCREEN puts dead-link on %q, invite on %q", l.routes["dead-link"], l.routes["invite"])
	}
}

// --- The bites: samples of the sign-in page (s-auth-1). They are not
// screens of the client — they draw the frame, not the meaning of the page —
// only trees to hold the guards against. ---

// signIn draws the sign-in frame from a LoginPage the way a screen would: the
// fields are a loop over the form's rows, the rest the frame's widgets.
// spoil changes the tree the way a bite needs.
func signIn(spoil func(*w.Page)) screens.Screen {
	return screens.Screen{
		Frames: []string{"s-auth-1", "s-auth-13"},
		Model:  func() any { return new(view.LoginPage) },
		Draw: func(model any) screens.Root {
			p := model.(*view.LoginPage)
			var fields []w.FormPart
			for _, set := range p.Form.Sets {
				for _, r := range set.Rows {
					if r.Kind == view.InputHidden {
						continue
					}
					parts := []w.FieldPart{w.Input{Name: r.Name, Value: string(r.Value), Label: string(r.Label), Focus: r.Focus, Error: r.Error != ""}}
					if r.Error != "" {
						parts = append(parts, w.FieldHint{Parts: []w.Inline{w.Text(r.Error)}, Variant: w.HintDanger})
					}
					fields = append(fields, w.Field{Label: string(r.Label), Parts: parts, Width: w.FieldW100})
				}
			}
			page := w.Page{Items: []w.PagePart{w.Auth{Dialog: w.Dialog{
				Items: []w.DialogPart{
					w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo60}},
					w.Tagline{Lead: runsText(p.Lede), Text: string(p.Title), Centered: true},
					w.AuthStack{
						Items: []w.AuthStackPart{
							w.Form{Items: fields, Variant: w.FormColumnFlush},
							w.Button{Label: "Sign in", Kind: w.ButtonPrimary},
						},
						// The frame's two links — «Create an account» while
						// registration is open, as s-auth-1 and -13 draw it.
						Links: []w.Link{{Text: "Create an account"}, {Text: string(p.Forgot.Label)}},
					},
				},
				Variant: w.DialogNarrow,
			}}}}
			if spoil != nil {
				spoil(&page)
			}
			return page
		},
	}
}

// runsText is a sentence of the model, its pieces one after another.
func runsText(r view.Runs) string {
	var s string
	for _, p := range r {
		s += string(p.Text)
	}
	return s
}

// The spoiled samples.
var (
	// A wrapper the frame does not draw: the fields in a group of fields.
	wrapped = signIn(func(p *w.Page) {
		auth := p.Items[0].(w.Auth)
		a := auth.Dialog.Items[2].(w.AuthStack)
		form := a.Items[0].(w.Form)
		var inner []w.FieldSetPart
		for _, it := range form.Items {
			inner = append(inner, it.(w.Field))
		}
		form.Items = []w.FormPart{w.FieldSet{Items: inner}}
		a.Items[0] = form
		auth.Dialog.Items[2] = a
		p.Items[0] = auth
	})
	// Another variant of the dialog: the page of the sign-in flow.
	otherVariant = signIn(func(p *w.Page) {
		auth := p.Items[0].(w.Auth)
		auth.Dialog.Variant = w.DialogPage440
		p.Items[0] = auth
	})
	// A widget less: no button where the frame has «Sign in».
	otherWidget = signIn(func(p *w.Page) {
		auth := p.Items[0].(w.Auth)
		a := auth.Dialog.Items[2].(w.AuthStack)
		a.Items = a.Items[:1]
		auth.Dialog.Items[2] = a
		p.Items[0] = auth
	})
	// The refused state drawn as the empty sign-in: no reason under the
	// field, which is not marked. On s-auth-1 alone it is green.
	noReason = signIn(func(p *w.Page) {
		hints(p, func(f *w.Field) {
			in := f.Parts[0].(w.Input)
			in.Error = false
			f.Parts = []w.FieldPart{in}
		})
	})
	// The reason in another hint than the state's frame draws: the kinship
	// with the draft of s-auth-1 never sees it, the normal fixture has no
	// reason.
	plainReason = signIn(func(p *w.Page) {
		hints(p, func(f *w.Field) {
			h := f.Parts[1].(w.FieldHint)
			h.Variant = w.HintPlain
			f.Parts[1] = h
		})
	})
)

// hints changes each field of the sample's form that says a reason.
func hints(p *w.Page, change func(*w.Field)) {
	auth := p.Items[0].(w.Auth)
	a := auth.Dialog.Items[2].(w.AuthStack)
	form := a.Items[0].(w.Form)
	items := slices.Clone(form.Items)
	for i, it := range items {
		if f := it.(w.Field); len(f.Parts) > 1 {
			change(&f)
			items[i] = f
		}
	}
	form.Items = items
	a.Items = slices.Clone(a.Items)
	a.Items[0] = form
	auth.Dialog.Items[2] = a
	p.Items[0] = auth
}

func TestFrameComparisonBite(t *testing.T) {
	l := load(t)
	none(t, "the good sample", frameFindings(l, []screens.Screen{signIn(nil)}))
	for name, s := range map[string]screens.Screen{"wrapped": wrapped, "other variant": otherVariant, "no button": otherWidget} {
		if found := frameFindings(l, []screens.Screen{s}); len(found) == 0 {
			t.Errorf("%s: the frame comparison stayed green", name)
		} else {
			t.Logf("%s: %s", name, found[0])
		}
	}
	// A class that is not the library's. No widget prints one, and a screen
	// draws a root of the library, so the sample is the page printed and a
	// class put in by hand.
	root, err := draw(signIn(nil), l.fixtures(signIn(nil))[0])
	if err != nil {
		t.Fatal(err)
	}
	own := strings.Replace(root.Render(), `class="tc-auth__stack"`, `class="tc-auth__stack tc-mine"`, 1)
	found := htmlFindings(l, "s-auth-1", "own class", own)
	if !slices.ContainsFunc(found, func(f string) bool { return strings.HasSuffix(f, `class "tc-mine" is not a class of Typecase's CSS`) }) {
		t.Errorf("a class of the screen's own stayed out of the findings: %q", found)
	}
	// The two copies of the mockup part on the screen's frame.
	parted := l
	parted.tcMock = strings.Replace(l.tcMock, `<span class="tc-auth__link">Create an account</span>`, `<span class="tc-auth__link">Sign up</span>`, 1)
	if parted.tcMock == l.tcMock {
		t.Fatal("the sample of parted copies changed nothing: s-auth-1 moved")
	}
	if found := frameFindings(parted, []screens.Screen{signIn(nil)}); len(found) == 0 {
		t.Error("a frame that differs between the copies of the mockup stayed green")
	}
	// The frames are those check.py gives the fixtures of the view model:
	// another frame than the route's is red twice — the fixture drawn by a
	// frame the screen does not name, a frame named and drawn by nothing —,
	// and a state's frame left out is red on the state.
	has := func(found []string, parts ...string) bool {
		return slices.ContainsFunc(found, func(f string) bool {
			for _, p := range parts {
				if !strings.Contains(f, p) {
					return false
				}
			}
			return true
		})
	}
	wrong := signIn(nil)
	wrong.Frames = []string{"s-auth-2", "s-auth-13"}
	if found := frameFindings(l, []screens.Screen{wrong}); !has(found, "login/normal.json: drawn by s-auth-1") || !has(found, "is drawn by s-auth-2") {
		t.Errorf("a screen naming another frame than its route's: %q", found)
	}
	short := signIn(nil)
	short.Frames = []string{"s-auth-1"}
	if found := frameFindings(l, []screens.Screen{short}); !has(found, "login/refused.json: drawn by s-auth-13") {
		t.Errorf("a screen leaving out its state's frame: %q", found)
	}
	// A view model with no fixture is held against nothing, and says so.
	bare := signIn(nil)
	bare.Model = func() any { return new(view.PasswordResetSheet) }
	if found := frameFindings(l, []screens.Screen{bare}); !has(found, "PasswordResetSheet", "no fixture of its view model") {
		t.Errorf("a screen of a view model without fixtures: %q", found)
	}
	// A state is held against its own frame (STATE_FRAME of check.py): the
	// refused sign-in drawn as the empty one is red, on login/refused only.
	refused := frameFindings(l, []screens.Screen{noReason})
	if len(refused) == 0 {
		t.Error("the refused state drawn as the empty sign-in stayed green")
	}
	for _, f := range refused {
		if !strings.Contains(f, "login/refused.json (s-auth-13)") {
			t.Errorf("a finding not of the refused state: %s", f)
		}
	}
	// A state's frame is STATE_FRAME's, whether or not it names the route
	// (the lead's decision of 01.10.2026): the screen names it, and the state
	// is held against it.
	elsewhere := l
	elsewhere.states = map[string]string{"login/refused": "s-auth-2"}
	if found := frameFindings(elsewhere, []screens.Screen{signIn(nil)}); !has(found, "login/refused.json: drawn by s-auth-2 (check.py)") {
		t.Errorf("a state's frame STATE_FRAME gives stayed out of what the screen must name: %q", found)
	}
	other := signIn(nil)
	other.Frames = []string{"s-auth-1", "s-auth-2"}
	if found := frameFindings(elsewhere, []screens.Screen{other}); !has(found, "login/refused.json (s-auth-2)") {
		t.Errorf("a state drawn as another frame than STATE_FRAME's stayed green: %q", found)
	}
	// The two copies of the mockup part on a state's frame.
	partedState := l
	partedState.tcMock = strings.Replace(l.tcMock, `Login or password is wrong.</span>`, `Wrong.</span>`, 1)
	if partedState.tcMock == l.tcMock {
		t.Fatal("the sample of parted copies changed nothing: s-auth-13 moved")
	}
	if found := frameFindings(partedState, []screens.Screen{signIn(nil)}); !slices.ContainsFunc(found, func(f string) bool { return strings.Contains(f, "frame s-auth-13 differs") }) {
		t.Errorf("a state's frame that differs between the copies stayed out of the findings: %q", found)
	}
	// A state that waits for the library (U-5.5) is drawn and not held
	// against its frame; one that matches its frame waits for nothing.
	waiting := noReason
	waiting.Waits = []screens.Wait{{Row: 1, Fixture: "login/refused"}}
	none(t, "a waiting state", frameFindings(l, []screens.Screen{waiting}))
	idle := signIn(nil)
	idle.Waits = []screens.Wait{{Row: 1, Fixture: "login/normal"}}
	if found := frameFindings(l, []screens.Screen{idle}); !has(found, "login/normal.json (s-auth-1): waits (U-5.5 #1) and matches its frame") {
		t.Errorf("a state that waits and matches its frame stayed green: %q", found)
	}
}

func TestKinshipBite(t *testing.T) {
	l := load(t)
	all := []screens.Screen{signIn(nil), wrapped, otherVariant, otherWidget, plainReason}
	d := drafts(t, l, all)
	if d["s-auth-13"] == nil {
		t.Error("no draft of the refused state's frame s-auth-13")
	}
	none(t, "the good sample", kinFindings(l, d, []screens.Screen{signIn(nil)}))
	waiting := plainReason
	waiting.Waits = []screens.Wait{{Row: 1, Fixture: "login/refused"}}
	none(t, "a waiting state", kinFindings(l, d, []screens.Screen{waiting}))
	for name, s := range map[string]screens.Screen{"wrapped": wrapped, "other variant": otherVariant, "no button": otherWidget, "the reason in another hint": plainReason} {
		if found := kinFindings(l, d, []screens.Screen{s}); len(found) == 0 {
			t.Errorf("%s: kinship stayed green", name)
		} else {
			t.Logf("%s: %s", name, found[0])
		}
	}
}

// Kinship on trees alone: runs, variants, meaning.
func TestKinRules(t *testing.T) {
	row := func(label string) w.Field {
		return w.Field{Label: label, Width: w.FieldW100, Parts: []w.FieldPart{w.Input{Value: label}}}
	}
	form := func(items ...w.FormPart) w.Form { return w.Form{Items: items, Variant: w.FormColumnFlush} }
	draft := guard.Skeleton(form(row("a"), row("b"), row("c")))
	for name, c := range map[string]struct {
		screen any
		kin    bool
	}{
		"a longer run":              {form(row("1"), row("2"), row("3"), row("4"), row("5")), true},
		"a shorter run":             {form(row("1")), true},
		"other data":                {w.Form{Items: []w.FormPart{w.Field{Label: "x", Width: w.FieldW100, Parts: []w.FieldPart{w.Input{Value: "y", Focus: true, Name: "n"}}}}, Variant: w.FormColumnFlush}, true},
		"the meaning of HTML":       {w.Form{Action: "/login", Fields: []w.Hidden{{Name: "_csrf", Value: "x"}}, Items: []w.FormPart{w.Field{Label: "p", Width: w.FieldW100, Parts: []w.FieldPart{w.Input{Kind: w.InputPassword, Name: "password", Required: true}}}}, Variant: w.FormColumnFlush}, true},
		"no rows":                   {form(), false},
		"a variant of the form":     {w.Form{Items: []w.FormPart{row("1")}, Variant: w.FormColumnGap10}, false},
		"a variant of a row":        {form(row("1"), w.Field{Label: "x", Width: w.FieldMD, Parts: []w.FieldPart{w.Input{}}}), false},
		"a variant deep in a row":   {form(w.Field{Label: "x", Width: w.FieldW100, Parts: []w.FieldPart{w.Input{Variant: w.InputChip}}}), false},
		"another widget in the run": {form(row("1"), w.Check{}), false},
		"a wrapper":                 {w.Form{Items: []w.FormPart{w.FieldSet{Items: []w.FieldSetPart{row("1")}}}, Variant: w.FormColumnFlush}, false},
		"a part more":               {form(w.Field{Label: "x", Width: w.FieldW100, Parts: []w.FieldPart{w.Input{}, w.FieldHint{}}}), false},
		"another root":              {w.Dialog{}, false},
	} {
		err := guard.Kin(draft, guard.Skeleton(c.screen))
		if (err == nil) != c.kin {
			t.Errorf("%s: kin %v, want %v (%v)", name, err == nil, c.kin, err)
		}
	}
}

func TestAddressesBite(t *testing.T) {
	found, left, err := guard.CheckAddresses("testdata/addresses/common.md", "testdata/addresses/client.md",
		[]string{"LoginPage", "ContactsListPage", "NowherePage", "LoginPage", "SetupPage", "InvitePage", "PasswordResetSheet"})
	if err != nil {
		t.Fatal(err)
	}
	bite(t, found, []string{
		"client.md:10: address", // U-5.5 #1 names /setup, built by SetupPage
		"client.md:11: address", // U-5.5 #2 names no address of C-5
		"client.md:12: address", // U-5.5 #3 names no address at all
		"client.md:14: address", // U-5.5 #5 names /admin/reset, built: `Issued` is no view model
		"common.md:0: address",  // NowherePage: no page or sheet of C-5 answers with it
		"common.md:0: address",  // LoginPage built twice
		"common.md:17: address", // /app/nothing: a page with no view model
	})
	// /invite/{token} waits for DeadLinkPage, the answer to a dead secret;
	// /app/{kind} answers as the row below, which waits for AgendaPage —
	// and is itself waiting in U-5.5 #4, by one of its kinds.
	if want := []string{"/invite/{token}", "/about", "/app/{kind}", "/app/{kind}/{account}/{col}/{uid}", "/app/nothing"}; !slices.Equal(left, want) {
		t.Errorf("left %q, want %q", left, want)
	}
}

// A button with its menu (Drop, TopDrop) is its button for kinship, as for
// shape (the lead's decision of 01.10.2026): its trigger is held against the
// draft's button, its menu is not.
func TestKinMenus(t *testing.T) {
	menu := w.Menu{Items: []w.MenuPart{w.MenuItem{Label: "Move", Href: "/move"}, w.MenuSep{}}, Pop: w.MenuUnder}
	acts := func(items ...w.ActsPart) w.Acts { return w.Acts{Items: items} }
	top := func(items ...w.TopAction) w.Top { return w.Top{Actions: items} }
	dots := w.Button{Icon: "dots", Title: "More"}
	for name, c := range map[string]struct {
		draft, screen any
		kin           bool
	}{
		"a closed menu where the draft has its button": {acts(dots), acts(w.Drop{Trigger: dots, Menu: menu}), true},
		"the draft's menu and the screen's button":     {acts(w.Drop{Trigger: dots, Menu: menu}), acts(dots), true},
		"a menu of the top bar":                        {top(w.UserMenu{Name: "mix"}), top(w.TopDrop{Trigger: w.UserMenu{Name: "x", Href: "/a"}, Menu: menu}), true},
		"a menu around another button":                 {acts(dots), acts(w.Drop{Trigger: w.Button{Icon: "dots", Kind: w.ButtonPrimary}, Menu: menu}), false},
		"a menu around another widget":                 {top(w.Button{Label: "New"}), top(w.TopDrop{Trigger: w.UserMenu{Name: "x"}, Menu: menu}), false},
	} {
		err := guard.Kin(guard.Skeleton(c.draft), guard.Skeleton(c.screen))
		if (err == nil) != c.kin {
			t.Errorf("%s: kin %v, want %v (%v)", name, err == nil, c.kin, err)
		}
	}
}

// The choices of a line of input without a script (TagAdd.Options, a
// datalist) are the meaning of HTML, not the frame's: no frame draws them,
// no draft has them, and shape drops them (Typecase's note 43, task 146) —
// a screen that has them is the draft's kin all the same.
func TestKinListOptions(t *testing.T) {
	line := func(options ...w.ListOption) w.TagInput {
		return w.TagInput{Parts: []w.TagInputPart{w.Tag{Label: "a"}}, Add: &w.TagAdd{Name: "related.add", Options: options}}
	}
	if err := guard.Kin(guard.Skeleton(line()), guard.Skeleton(line(w.ListOption{Value: "u1", Label: "One"}, w.ListOption{Value: "u2"}))); err != nil {
		t.Errorf("a line with its choices is not kin of the draft's line: %v", err)
	}
	// The line itself is still held: a field without it is another field.
	if err := guard.Kin(guard.Skeleton(line()), guard.Skeleton(w.TagInput{Parts: []w.TagInputPart{w.Tag{Label: "a"}}})); err == nil {
		t.Error("a tag field without its line is kin of a draft that has one")
	}
}

// The rows of the list «ждёт библиотеку» that name a state or a control
// (U-5.5, the owner's decision of 01.10.2026), and every wait of every
// screen against them: a wait names a row that names its fixture or its
// screen, and every state a row names waits on the screen of its fixture.
func waitFindings(l lib, rows map[int]guard.WaitRow, all []screens.Screen) []string {
	var out []string
	named := map[string]bool{}
	for _, scr := range all {
		at := label(scr)
		mine := map[string]bool{}
		for _, path := range l.fixtures(scr) {
			mine[key(path)] = true
		}
		for _, wt := range scr.Waits {
			r, ok := rows[wt.Row]
			switch {
			case !ok:
				out = append(out, fmt.Sprintf("%s: a wait of U-5.5 #%d, which names no state or control", at, wt.Row))
			case (wt.Fixture == "") == (wt.Control == ""):
				out = append(out, fmt.Sprintf("%s: the wait of U-5.5 #%d names neither a state nor a control, or both", at, wt.Row))
			case wt.Fixture != "" && !mine[wt.Fixture]:
				out = append(out, fmt.Sprintf("%s: the wait of U-5.5 #%d names %s, no fixture of its view model", at, wt.Row, wt.Fixture))
			case wt.Fixture != "" && !slices.Contains(r.Fixtures, wt.Fixture):
				out = append(out, fmt.Sprintf("%s: U-5.5 #%d does not name %s", at, wt.Row, wt.Fixture))
			case wt.Control != "" && !slices.Contains(r.Models, modelName(scr)):
				out = append(out, fmt.Sprintf("%s: U-5.5 #%d does not name %s", at, wt.Row, modelName(scr)))
			}
			if wt.Fixture != "" {
				named[wt.Fixture] = true
			}
		}
	}
	for _, r := range rows {
		for _, f := range r.Fixtures {
			if !named[f] {
				out = append(out, fmt.Sprintf("U-5.5 #%d: %s waits on no screen", r.Num, f))
			}
		}
	}
	slices.Sort(out)
	return out
}

func TestWaitsAreRows(t *testing.T) {
	rows, err := guard.WaitRows(clientSpec)
	if err != nil {
		t.Fatal(err)
	}
	none(t, "waits", waitFindings(load(t), rows, screens.All))
}

func TestWaitsBite(t *testing.T) {
	l := load(t)
	rows, err := guard.WaitRows("testdata/addresses/client.md")
	if err != nil {
		t.Fatal(err)
	}
	if r := rows[8]; !slices.Equal(r.Fixtures, []string{"login/refused"}) || !slices.Equal(r.Models, []string{"LoginPage"}) {
		t.Fatalf("U-5.5 #8 of the sample: %+v", r)
	}
	good := signIn(nil)
	good.Waits = []screens.Wait{{Row: 8, Fixture: "login/refused"}, {Row: 9, Control: ".Links(Link)"}}
	none(t, "good waits", waitFindings(l, rows, []screens.Screen{good}))
	bad := signIn(nil)
	bad.Waits = []screens.Wait{{Row: 1, Control: ".Links(Link)"}, {Row: 8, Fixture: "login/normal"}, {Row: 9, Fixture: "register/sent"}, {Row: 10, Control: ".Links(Link)"}, {Row: 8}}
	want := []string{
		"LoginPage (s-auth-1, s-auth-13): U-5.5 #8 does not name login/normal",
		"LoginPage (s-auth-1, s-auth-13): U-5.5 #10 does not name LoginPage",
		"LoginPage (s-auth-1, s-auth-13): a wait of U-5.5 #1, which names no state or control",
		"LoginPage (s-auth-1, s-auth-13): the wait of U-5.5 #8 names neither a state nor a control, or both",
		"LoginPage (s-auth-1, s-auth-13): the wait of U-5.5 #9 names register/sent, no fixture of its view model",
		"U-5.5 #8: login/refused waits on no screen",
	}
	got := waitFindings(l, rows, []screens.Screen{bad})
	t.Log(strings.Join(got, "\n"))
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("waits: missing %q in\n%s", w, strings.Join(got, "\n"))
		}
	}
}
