package guard_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

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

// fixtures are the state files of every fixture directory check.py maps to
// the route, sorted.
func (l lib) fixtures(route string) []string {
	var out []string
	for dir, r := range l.routes {
		if r != route {
			continue
		}
		states, _ := filepath.Glob(filepath.Join(fixtureDir, dir, "*.json"))
		out = append(out, states...)
	}
	slices.Sort(out)
	return out
}

// draw decodes a fixture into the screen's view model — no field the model
// lacks — and draws the screen.
func draw(scr screens.Screen, path string) (screens.Root, error) {
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
	return scr.Draw(model), nil
}

var classAttr = regexp.MustCompile(`class="([^"]*)"`)

// frameOf is the frame a fixture of the screen is held against: the frame
// STATE_FRAME of check.py gives the fixture's state — the refused sign-in,
// login/refused, is s-auth-13 —, else the screen's.
func (l lib) frameOf(scr screens.Screen, path string) string {
	key := filepath.Base(filepath.Dir(path)) + "/" + strings.TrimSuffix(filepath.Base(path), ".json")
	if f, ok := l.states[key]; ok {
		return f
	}
	return scr.Frame
}

// namesRoute tells whether the frame's data-screen names the route: a state's
// frame is a part of the route's screen.
func namesRoute(mock, frame, route string) bool {
	m := frameDiv.FindStringSubmatch(frameBlock(mock, frame))
	if m == nil {
		return false
	}
	ds := dataScreen.FindStringSubmatch(m[1])
	return ds != nil && slices.Contains(strings.Fields(ds[1]), route)
}

// frameFindings is the frame comparison (U-15.2, 4; U-15.1) of each screen:
// its frame is the route's frame, and on every fixture of its route the
// screen has the make-up of the fixture's frame (shape.Check) — the route's,
// or its state's by STATE_FRAME, a frame of the same route — and no class
// Typecase's CSS does not have. Each frame held is the same in both copies
// of the mockup.
func frameFindings(l lib, all []screens.Screen) []string {
	var out []string
	for _, scr := range all {
		at := scr.Route + " (" + scr.Frame + ")"
		if canon := canonFrame(l.mock, scr.Route); canon != scr.Frame {
			out = append(out, fmt.Sprintf("%s: the route's frame is %q", at, canon))
			continue
		}
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
		sameCopies(scr.Frame)
		states := l.fixtures(scr.Route)
		if len(states) == 0 {
			out = append(out, at+": no fixture in FIXTURE_SCREEN of check.py")
		}
		for _, path := range states {
			fx := filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
			frame := l.frameOf(scr, path)
			if frame != scr.Frame {
				if !namesRoute(l.mock, frame, scr.Route) {
					out = append(out, fmt.Sprintf("%s on %s: the state's frame %s does not name the route", at, fx, frame))
					continue
				}
				sameCopies(frame)
				fx += " (" + frame + ")"
			}
			root, err := draw(scr, path)
			if err != nil {
				out = append(out, fmt.Sprintf("%s on %s: %v", at, fx, err))
				continue
			}
			out = append(out, htmlFindings(l, frame, at+" on "+fx, root.Render())...)
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
// on each normal fixture of its route against the draft of its frame, and on
// each fixture whose state has a frame of its own (STATE_FRAME) against the
// draft of that frame — a state's frame has its draft too.
func kinFindings(l lib, drafts map[string]*guard.Node, all []screens.Screen) []string {
	var out []string
	for _, scr := range all {
		at := scr.Route + " (" + scr.Frame + ")"
		if d, ok := drafts[scr.Frame]; !ok || d == nil {
			out = append(out, at+": no draft")
			continue
		}
		normal := 0
		for _, path := range l.fixtures(scr.Route) {
			fx := filepath.Base(filepath.Dir(path))
			frame := l.frameOf(scr, path)
			switch {
			case frame != scr.Frame:
				fx += "/" + filepath.Base(path) + " (" + frame + ")"
			case filepath.Base(path) == "normal.json":
				normal++
			default:
				continue
			}
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

// frames are the frames the screens are held against: each screen's, and
// the frames of its fixtures' states.
func frames(l lib, all []screens.Screen) []string {
	var out []string
	add := func(f string) {
		if !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	for _, s := range all {
		add(s.Frame)
		for _, path := range l.fixtures(s.Route) {
			add(l.frameOf(s, path))
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
		built = append(built, s.Route)
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

// --- The bites: samples of the sign-in page (s-auth-1). They are not
// screens of the client — they draw the frame, not the meaning of the page —
// only trees to hold the guards against. ---

// signIn draws the sign-in frame from a LoginPage the way a screen would: the
// fields are a loop over the form's rows, the rest the frame's widgets.
// spoil changes the tree the way a bite needs.
func signIn(spoil func(*w.Page)) screens.Screen {
	return screens.Screen{
		Route: "/login",
		Frame: "s-auth-1",
		Model: func() any { return new(view.LoginPage) },
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
					w.Tagline{Lead: string(p.Lede), Text: string(p.Title), Centered: true},
					w.AuthStack{
						Items: []w.AuthStackPart{
							w.Form{Items: fields, Variant: w.FormColumnFlush},
							w.Button{Label: "Sign in", Kind: w.ButtonPrimary},
						},
						Links: []w.Link{{Text: string(p.Forgot.Label)}},
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
	root, err := draw(signIn(nil), l.fixtures("/login")[0])
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
	// The route's frame is the one the screen names.
	wrong := signIn(nil)
	wrong.Frame = "s-auth-2"
	if found := frameFindings(l, []screens.Screen{wrong}); len(found) == 0 {
		t.Error("a screen naming another frame than its route's stayed green")
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
	// A state's frame is a frame of the route.
	elsewhere := l
	elsewhere.states = map[string]string{"login/refused": "s-auth-2"}
	if found := frameFindings(elsewhere, []screens.Screen{signIn(nil)}); !slices.ContainsFunc(found, func(f string) bool { return strings.Contains(f, "does not name the route") }) {
		t.Errorf("a state's frame of another route stayed out of the findings: %q", found)
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
}

func TestKinshipBite(t *testing.T) {
	l := load(t)
	all := []screens.Screen{signIn(nil), wrapped, otherVariant, otherWidget, plainReason}
	d := drafts(t, l, all)
	if d["s-auth-13"] == nil {
		t.Error("no draft of the refused state's frame s-auth-13")
	}
	none(t, "the good sample", kinFindings(l, d, []screens.Screen{signIn(nil)}))
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
		[]string{"/login", "/app/contacts/{account}/{col}", "/nowhere", "/login", "/setup"})
	if err != nil {
		t.Fatal(err)
	}
	bite(t, found, []string{
		"client.md:10: address", // U-5.5 #1 names /setup, which is built too
		"client.md:11: address", // U-5.5 #2 names no address of C-5
		"client.md:12: address", // U-5.5 #3 names no address at all
		"common.md:0: address",  // /nowhere is not in C-5
		"common.md:0: address",  // /login built twice
	})
	if want := []string{"/about", "/app/{kind}/{account}/{col}/{uid}"}; !slices.Equal(left, want) {
		t.Errorf("left %q, want %q", left, want)
	}
}
