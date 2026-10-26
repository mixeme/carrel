package guard

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
)

// Kinship with the draft (U-15.2, 3; U-5.1). A screen starts as the draft
// frame2go -self prints for its frame, and the difference between the two is
// the data: the frame's literals became the view model's fields, the frame's
// rows a loop. What must not differ is the skeleton — which widgets, in what
// nesting, with which variants. The guard prints the draft again, builds it,
// and holds the skeleton of the screen's tree, drawn on a fixture, against
// the draft's.
//
// Both trees are values of Typecase's widget types, walked by reflection, so
// nothing is read from source and nothing is interpreted: the draft is run as
// Go, the screen too.
//
// The skeleton of a widget (a struct of the widget package) is:
//
//   - its type;
//   - its variants — every field of an integer type named in the widget
//     package (ButtonKind, FieldWidth, DialogVariant…), with its value, zero
//     included;
//   - its parts — every field that holds widgets: a widget struct, a pointer
//     or an interface holding one (nil is no part), a slice of them (empty
//     is no part).
//
// The meaning of HTML (Р-25 of Typecase; U-5.1) is data too: where a form
// goes, its hidden fields, the options of a select, the type of an input.
// The frame draws none of it, the draft has none of it, and a screen must
// have it. So the widgets and enums that carry only the meaning — meaning
// below, the ones Typecase's package shape drops from a screen as well — are
// not in the skeleton.
//
// A button with its menu (Drop, TopDrop) is its button: its skeleton is its
// trigger's, as shape reads it (menus below).
//
// Leaf values — strings, bools, plain numbers, slices of strings — are the
// data and are not compared; so is a field of a named string type (an icon's
// name). What they draw is held by the frame comparison (U-15.1), not here.
//
// A list of parts is compared by runs: neighbours of one widget type are one
// run, the screen's run may be longer or shorter than the draft's (a loop
// over the rows the frame draws three of), and each of its widgets must have
// the skeleton of one of the draft run's widgets. The runs themselves — how
// many, of which types, in what order — must be the draft's.

// meaning are the types of the widget package that carry the meaning of HTML
// and draw nothing of the frame's: a hidden field (shape drops
// input[type=hidden]), an option of a select (option), a PostForm (tc-post),
// the template of a row «Add …» (template is not drawn), the type of an
// input (an attribute, not a class), a choice of a line of input (datalist:
// shape drops it since Typecase's note 43).
var meaning = map[string]bool{"Hidden": true, "PickOption": true, "PostForm": true, "Template": true, "InputKind": true, "ListOption": true}

// Node is the skeleton of one widget.
type Node struct {
	Type     string             `json:"type"`
	Variants map[string]int64   `json:"variants,omitempty"`
	Parts    map[string][]*Node `json:"parts,omitempty"`
}

// Skeleton is the skeleton of the widget tree v. The widget package is v's
// own: the package of its dynamic type. A v that is not a widget is nil.
func Skeleton(v any) *Node {
	rv := reflect.ValueOf(v)
	rv = deref(rv)
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return nil
	}
	return skeleton(rv, rv.Type().PkgPath())
}

// deref follows pointers and interfaces to the value they hold; the zero
// Value when one of them is nil.
func deref(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func isWidget(v reflect.Value, pkg string) bool {
	return v.IsValid() && v.Kind() == reflect.Struct && v.Type().PkgPath() == pkg && !meaning[v.Type().Name()] && !unseen(v)
}

// unseen says whether a widget is a field the page does not show
// (Field.Hidden: the numbers «Custom…» of a reminder opens, s-agenda-19): the
// frames of the edits draw no such field, and Typecase's shape does not see
// it either (Typecase's note 64, task 161).
func unseen(v reflect.Value) bool {
	if v.Type().Name() != "Field" {
		return false
	}
	h := v.FieldByName("Hidden")
	return h.IsValid() && h.Kind() == reflect.Bool && h.Bool()
}

// menus are the widgets of a button with its menu (Typecase's Drop and
// TopDrop). The frames draw such a button where it stands and the menu's
// contents in a frame of their own beside the screen, and shape reads the
// pair as its button (Typecase's task 49); so does kinship: the skeleton of a
// button with its menu is its trigger's, and the menu is not in it (the lead's
// decision of 01.10.2026, U-15.2, 3).
var menus = map[string]bool{"Drop": true, "TopDrop": true}

// bands are the widgets that stand for their parts in their place: the band
// of notices (Typecase's Notices, display: contents) is its messages and
// their technical blocks, as the frames draw them in the page or in a band
// alike (Typecase's note 127 p. 3) — Typecase's shape reads it so too
// (note 133 p. 1).
var bands = map[string]bool{"Notices": true}

func skeleton(v reflect.Value, pkg string) *Node {
	t := v.Type()
	if menus[t.Name()] {
		if tr := deref(v.FieldByName("Trigger")); isWidget(tr, pkg) {
			return skeleton(tr, pkg)
		}
	}
	n := &Node{Type: t.Name()}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := v.Field(i)
		switch f.Type.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if f.Type.PkgPath() == pkg && f.Type.Name() != "" && !meaning[f.Type.Name()] {
				if n.Variants == nil {
					n.Variants = map[string]int64{}
				}
				if fv.CanInt() {
					n.Variants[f.Name] = fv.Int()
				} else {
					n.Variants[f.Name] = int64(fv.Uint())
				}
			}
		case reflect.Struct, reflect.Pointer, reflect.Interface:
			if w := deref(fv); isWidget(w, pkg) {
				n.part(f.Name, skeleton(w, pkg))
			}
		case reflect.Slice, reflect.Array:
			for j := range fv.Len() {
				w := deref(fv.Index(j))
				if !isWidget(w, pkg) {
					continue
				}
				if bands[w.Type().Name()] {
					items := w.FieldByName("Items")
					for k := range items.Len() {
						if it := deref(items.Index(k)); isWidget(it, pkg) {
							n.part(f.Name, skeleton(it, pkg))
						}
					}
					continue
				}
				n.part(f.Name, skeleton(w, pkg))
			}
		}
	}
	return n
}

func (n *Node) part(field string, c *Node) {
	if n.Parts == nil {
		n.Parts = map[string][]*Node{}
	}
	n.Parts[field] = append(n.Parts[field], c)
}

// Kin compares the skeleton of a screen with the skeleton of its draft and
// names the first place where they part; nil when the screen is the draft's
// kin.
func Kin(draft, screen *Node) error {
	if draft == nil || screen == nil {
		if draft == screen {
			return nil
		}
		return fmt.Errorf("one of the trees is not a widget")
	}
	if d := kin(draft, screen, draft.Type); d != "" {
		return fmt.Errorf("%s", d)
	}
	return nil
}

func kin(d, s *Node, path string) string {
	if d.Type != s.Type {
		return fmt.Sprintf("%s: the screen has %s where the draft has %s", path, s.Type, d.Type)
	}
	for _, name := range keys(d.Variants, s.Variants) {
		dv, dok := d.Variants[name]
		sv, sok := s.Variants[name]
		if dok != sok || dv != sv {
			return fmt.Sprintf("%s.%s: variant %d, the draft's %d", path, name, sv, dv)
		}
	}
	for _, name := range keys(d.Parts, s.Parts) {
		if msg := kinRuns(runs(d.Parts[name]), runs(s.Parts[name]), path+"."+name); msg != "" {
			return msg
		}
	}
	return ""
}

func kinRuns(dr, sr [][]*Node, path string) string {
	n := min(len(dr), len(sr))
	for i := range n {
		p := fmt.Sprintf("%s[%d]", path, i)
		if dr[i][0].Type != sr[i][0].Type {
			return fmt.Sprintf("%s: the screen has %s where the draft has %s", p, sr[i][0].Type, dr[i][0].Type)
		}
		for _, s := range sr[i] {
			first := ""
			for _, d := range dr[i] {
				msg := kin(d, s, p+"("+d.Type+")")
				if msg == "" {
					first = ""
					break
				}
				if first == "" {
					first = msg
				}
			}
			if first != "" {
				return first
			}
		}
	}
	if len(dr) > n {
		return fmt.Sprintf("%s[%d]: the screen has no %s where the draft has it", path, n, dr[n][0].Type)
	}
	if len(sr) > n {
		return fmt.Sprintf("%s[%d]: the screen has %s the draft does not", path, n, sr[n][0].Type)
	}
	return ""
}

// runs groups a list of parts into runs of neighbours of one widget type.
func runs(ns []*Node) [][]*Node {
	var out [][]*Node
	for _, n := range ns {
		if last := len(out) - 1; last >= 0 && out[last][0].Type == n.Type {
			out[last] = append(out[last], n)
		} else {
			out = append(out, []*Node{n})
		}
	}
	return out
}

func keys[V any](a, b map[string]V) []string {
	var out []string
	for k := range a {
		out = append(out, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// draftFunc is the function a -self draft declares for its frame.
var draftFunc = regexp.MustCompile(`(?m)^func (\w+)\(\) interface\{ Render\(\) string \} \{`)

// Drafts prints the -self draft of each frame with Typecase's frame2go — the
// tool of the client module (go.mod), run with the module of Typecase as
// its root — builds the drafts as they are printed, and returns their
// skeletons by frame. module is the client module's directory. The drafts are
// built in a directory of guard/testdata made for the call and removed after
// it: testdata is never built by ./..., and a draft is not a package of the
// client.
func Drafts(module string, frames []string) (map[string]*Node, error) {
	if len(frames) == 0 {
		return map[string]*Node{}, nil
	}
	root, err := goOut(module, "list", "-m", "-f", "{{.Dir}}", "carrel.dev/typecase")
	if err != nil {
		return nil, err
	}
	root = strings.TrimSpace(root)
	dir, err := os.MkdirTemp(filepath.Join(module, "guard", "testdata"), "draft-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	base := "carrel.dev/carrel/client/guard/testdata/" + filepath.Base(dir)
	var imports, calls strings.Builder
	for i, frame := range frames {
		src, err := goOut(module, "tool", "frame2go", "-self", "-root", root, frame)
		if err != nil {
			return nil, err
		}
		m := draftFunc.FindStringSubmatch(src)
		if m == nil {
			return nil, fmt.Errorf("the draft of %s declares no screen function:\n%s", frame, src)
		}
		pkg := fmt.Sprintf("d%d", i)
		if err := os.MkdirAll(filepath.Join(dir, pkg), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dir, pkg, "draft.go"), []byte(src), 0o644); err != nil {
			return nil, err
		}
		fmt.Fprintf(&imports, "\t%s %q\n", pkg, base+"/"+pkg)
		fmt.Fprintf(&calls, "\t\t%q: guard.Skeleton(%s.%s()),\n", frame, pkg, m[1])
	}
	probe := "package main\n\nimport (\n\t\"encoding/json\"\n\t\"os\"\n\n\t\"carrel.dev/carrel/client/guard\"\n" + imports.String() +
		")\n\nfunc main() {\n\tjson.NewEncoder(os.Stdout).Encode(map[string]*guard.Node{\n" + calls.String() + "\t})\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(probe), 0o644); err != nil {
		return nil, err
	}
	out, err := goOut(module, "run", base)
	if err != nil {
		return nil, err
	}
	var skels map[string]*Node
	if err := json.Unmarshal([]byte(out), &skels); err != nil {
		return nil, fmt.Errorf("the drafts' skeletons: %v", err)
	}
	return skels, nil
}

// goOut runs the go command in dir and returns its standard output.
func goOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}
