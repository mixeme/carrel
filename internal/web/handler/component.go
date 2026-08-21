// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"reflect"
	"sort"
	"strings"
)

// The Go half of the component library in internal/web/component: the typed
// input each component takes, and the assembly of its stylesheet.
//
// The types came from wave 2.6.E2 and keep its rule: a component takes a
// concrete struct, not a map[string]any. Under a map a call site could
// misspell a field name or pass the wrong kind of value and the component
// rendered anyway — with an empty title, or nothing where a bool belonged.
// That is the same silent gap that let ten screens drift in wave 2.5, and it
// is the reason the library exists at all.

// componentTmplDir and componentCSSDir are where the library keeps the two
// halves of each component. They sit side by side on purpose: markup and
// styles apart is how one primitive ends up with sixteen names.
const (
	componentTmplDir = "tmpl"
	componentCSSDir  = "css"
)

// Head is the input to the m-head component.
//
// Subtitle is plain text and is escaped like any other field; SubtitleHTML is
// the handful of empty states that build a real link into the subtitle
// ("Connect a DAV account from Connections…") from constants, never from
// anything a visitor typed — development.md's rule that template.HTML never
// wraps user input holds exactly as it did when these were `(safeHTML …)`.
//
// Side, Plain and Doc are the same header in the three places the mockups
// already draw it: a details panel (h2, 19px), a dialog (no padding of its
// own — the dialog already has one), and the note-reading chrome (no title
// in the header; the 27px document title lives in the body). Code and Mono
// are tails the component wraps itself, so a screen never has to hand the
// template raw HTML for a filename or a date. Swatch is the collection
// stripe in the subtitle — the same .m-bar3 the combined views already put
// in the h1.
type Head struct {
	Title        string
	Subtitle     string
	SubtitleHTML template.HTML
	Hint         string
	Photo        string
	Bar          bool
	Muted        bool
	Side         bool
	Plain        bool
	Doc          bool
	Code         string
	Mono         string
	Swatch       string
	Crumbs       []fileCrumb
}

// Bar is the input to the m-bar component. Sel is the filled selection
// band (.m-sel): the same bar, in the card colour, with buttons on paper.
type Bar struct {
	Attrs template.HTML
	Sel   bool
}

// Right is the input to m-right: the trailing group of a bar (counter,
// density, "Clear selection"). Second marks it is-2nd — it retreats under
// ⋯ when the content column is narrow.
type Right struct {
	Second bool
}

// Sep is the input to m-sep, the 1px rule between groups on a bar.
type Sep struct {
	Second bool
}

// BarRange is the input to m-range: From/To (and its Show) as one bar
// item, so the whole range retreats together rather than leaving a
// hanging separator.
type BarRange struct {
	Second bool
	Attrs  template.HTML
}

// BarForm is the input to the m-barform component: the same bar built as a
// real GET form, which the agenda's From/To filter needs.
type BarForm struct {
	Action string
	Method string
}

// DataTable is the input to m-table. The table is always .m-table;
// Class is extra screen markers (install-check-table), Root is
// data-columns-root, Attrs is everything else the tag needs.
type DataTable struct {
	Class string
	Root  string
	Attrs template.HTML
}

// ImportExportMenu is the input to the ⋯ menu of 2.6.D1/D2.
type ImportExportMenu struct {
	Sources   []sourceRow
	ImportURL string
	ExportURL string
}

// List is the input to m-list, the column of rows. Attrs is the handful of
// identity attributes the screen owns (id, data-columns-root) — the list
// itself does not.
type List struct {
	Attrs template.HTML
}

// Row is the input to m-row. Kind is the mockup modifier (contact, agenda,
// task, note, find, file) and becomes m-row--Kind; a typo here is an
// unknown class, not a silent fallback. Class is JS/state markers the
// screen owns (is-contact, is-merged, is-task) — not a second name for
// the row.
type Row struct {
	Kind    string
	On      bool
	Done    bool
	Overdue bool
	Class   string
}

// Mod is the mockup modifier class (m-row--contact, …). Emitted as a
// complete token so the class attribute never interpolates Kind inside
// quotes, which would break the allow-list's class="…" scan.
func (r Row) Mod() string {
	switch r.Kind {
	case "contact":
		return "m-row--contact"
	case "agenda":
		return "m-row--agenda"
	case "task":
		return "m-row--task"
	case "note":
		return "m-row--note"
	case "find":
		return "m-row--find"
	case "file":
		return "m-row--file"
	default:
		return ""
	}
}

// Group is the input to m-group: the rubric above a run of rows, and
// the count on the right when the screen has one.
type Group struct {
	Label string
	Num   string
}

// Form is the input to m-form: a real <form> laid out as the mockups'
// field row. Class is a screen-owned marker (contact-form, note-edit-form);
// Attrs is everything the tag needs besides method/action/class (enctype,
// hx-*, autocomplete). Tight drops the top padding (.is-tight).
type Form struct {
	Method string
	Action string
	Class  string
	Attrs  template.HTML
	Tight  bool
}

// Field is the input to m-f, one labelled cell of a form. Width is a
// content-width token (xs/sm/md/lg/xl/full) emitted as a complete class
// so the allow-list never sees interpolation inside quotes. OwnLine
// skips the empty label spacer — the cell occupies the row alone.
type Field struct {
	Width   string
	OwnLine bool
	Class   string
}

// WidthClass is the mockup width token (w-xs … w-full).
func (f Field) WidthClass() string {
	switch f.Width {
	case "xs":
		return "w-xs"
	case "sm":
		return "w-sm"
	case "md":
		return "w-md"
	case "lg":
		return "w-lg"
	case "xl":
		return "w-xl"
	case "full":
		return "w-full"
	default:
		return ""
	}
}

// FormFoot is the input to m-formfoot, the action row under a form.
type FormFoot struct {
	Class string
}

// Seg is the input to m-seg, the joined segment control. Menu is the
// compact full-width variant in the user dropdown; Second retreats
// under ⋯ with the rest of the bar.
type Seg struct {
	Menu   bool
	Second bool
	Class  string
	Attrs  template.HTML
}

// FieldSet is the input to m-fset, a labelled group of fields.
type FieldSet struct {
	Class string
}

// Rail is the input to m-rail, the left column. Nav emits a <nav> (section
// links, settings, administration); the default is an <aside> (sources).
// App and Sec are the two nodes of one grid column: the section nav the
// drawer opens, and the source list it borrows. Extra is a screen-owned
// marker (note-sidebar). Note marks the note-reading sidebar for JS.
type Rail struct {
	Nav   bool
	Label string
	App   bool
	Sec   bool
	Note  bool
	Extra string
}

// Nav is the input to m-nav, the stack of section links inside the rail.
type Nav struct{}

// Src is the input to m-src, one collection (or combined-view) row.
// Href makes it an <a>; Item makes it an <li>; otherwise it is a <div>.
// The same Href/Item must be passed to m-srcend, or the tags will not match.
type Src struct {
	Href  string
	Item  bool
	Title string
	All   bool
	On    bool
	Off   bool
	Error bool
	Ext   bool
	Root  bool
	Extra string
}

// Mods is the state classes (is-all, is-on, …) as a leading-space suffix
// so the class attribute never interpolates a bool inside quotes.
func (s Src) Mods() string {
	var b strings.Builder
	if s.All {
		b.WriteString(" is-all")
	}
	if s.On {
		b.WriteString(" is-on")
	}
	if s.Off {
		b.WriteString(" is-off")
	}
	if s.Error {
		b.WriteString(" is-error")
	}
	if s.Ext {
		b.WriteString(" is-ext")
	}
	if s.Root {
		b.WriteString(" is-root")
	}
	if s.Extra != "" {
		b.WriteByte(' ')
		b.WriteString(s.Extra)
	}
	return b.String()
}

// RailSec is the input to m-rail-sec, one account group in the source
// list. Label is the rubric above the rows.
type RailSec struct {
	Label string
}

// RailFoot is the input to m-rail-foot, the Apply / New collection
// stripe at the bottom of the rail.
type RailFoot struct {
	Extra string
}

// Side is the input to m-side, the details column. Attrs is id / hidden
// / aria-label the screen owns; Extra is a screen marker.
type Side struct {
	Label string
	Attrs template.HTML
	Extra string
}

// Dialog is the input to m-dialog, the centred page frame. Wide and
// Narrow are the mockup width tokens; Narrow keeps the 520px production
// already had, not the mockup's 420.
type Dialog struct {
	Wide   bool
	Narrow bool
}

// Card is the input to m-card. Accent is the highlighted import card;
// Class is a state marker (is-ignored) the screen owns.
type Card struct {
	Accent bool
	Class  string
}

// Menu is the input to m-menu, the popover. Pop is the mockup is-pop
// (floats, does not inflate the layout); Right aligns it to the trigger.
type Menu struct {
	Pop   bool
	Right bool
	Attrs template.HTML
}

func buildHead(pairs ...any) (Head, error)   { return typedDict[Head](pairs...) }
func buildForm(pairs ...any) (Form, error)   { return typedDict[Form](pairs...) }
func buildField(pairs ...any) (Field, error) { return typedDict[Field](pairs...) }
func buildFormFoot(pairs ...any) (FormFoot, error) {
	return typedDict[FormFoot](pairs...)
}
func buildSeg(pairs ...any) (Seg, error) { return typedDict[Seg](pairs...) }
func buildFieldSet(pairs ...any) (FieldSet, error) {
	return typedDict[FieldSet](pairs...)
}
func buildRail(pairs ...any) (Rail, error)       { return typedDict[Rail](pairs...) }
func buildNav(pairs ...any) (Nav, error)         { return typedDict[Nav](pairs...) }
func buildSrc(pairs ...any) (Src, error)         { return typedDict[Src](pairs...) }
func buildRailSec(pairs ...any) (RailSec, error) { return typedDict[RailSec](pairs...) }
func buildRailFoot(pairs ...any) (RailFoot, error) {
	return typedDict[RailFoot](pairs...)
}
func buildSide(pairs ...any) (Side, error)       { return typedDict[Side](pairs...) }
func buildDialog(pairs ...any) (Dialog, error)   { return typedDict[Dialog](pairs...) }
func buildCard(pairs ...any) (Card, error)       { return typedDict[Card](pairs...) }
func buildMenu(pairs ...any) (Menu, error)       { return typedDict[Menu](pairs...) }
func buildList(pairs ...any) (List, error)       { return typedDict[List](pairs...) }
func buildRow(pairs ...any) (Row, error)         { return typedDict[Row](pairs...) }
func buildGroup(pairs ...any) (Group, error)     { return typedDict[Group](pairs...) }
func buildBar(pairs ...any) (Bar, error)         { return typedDict[Bar](pairs...) }
func buildBarForm(pairs ...any) (BarForm, error) { return typedDict[BarForm](pairs...) }
func buildRight(pairs ...any) (Right, error)     { return typedDict[Right](pairs...) }
func buildSep(pairs ...any) (Sep, error)         { return typedDict[Sep](pairs...) }
func buildBarRange(pairs ...any) (BarRange, error) {
	return typedDict[BarRange](pairs...)
}

func buildDataTable(pairs ...any) (DataTable, error) { return typedDict[DataTable](pairs...) }
func buildImportExportMenu(pairs ...any) (ImportExportMenu, error) {
	return typedDict[ImportExportMenu](pairs...)
}

// typedDict builds a T from alternating name/value arguments the same way
// wave 2.6.A's `dict` built a map (2.6.E2). It is generic so every component
// gets the same rule from one implementation rather than a hand-rolled switch
// per struct.
func typedDict[T any](pairs ...any) (T, error) {
	var out T
	if len(pairs)%2 != 0 {
		return out, fmt.Errorf("%T: odd number of arguments", out)
	}
	v := reflect.ValueOf(&out).Elem()
	for i := 0; i < len(pairs); i += 2 {
		name, ok := pairs[i].(string)
		if !ok {
			return out, fmt.Errorf("%T: key %v is not a string", out, pairs[i])
		}
		field := v.FieldByName(name)
		if !field.IsValid() || !field.CanSet() {
			return out, fmt.Errorf("%T has no field %q", out, name)
		}
		val := reflect.ValueOf(pairs[i+1])
		if !val.IsValid() || !val.Type().AssignableTo(field.Type()) {
			return out, fmt.Errorf("%T.%s: %#v is not assignable to %s", out, name, pairs[i+1], field.Type())
		}
		field.Set(val)
	}
	return out, nil
}

// Stylesheet is the assembled component stylesheet: every file in the
// library's css/ directory, in one response.
type Stylesheet struct {
	Body []byte
	// ETag is the hash of Body, so a rebuilt binary invalidates the cached
	// copy and an unchanged one does not.
	ETag string
}

// LoadStylesheet concatenates the library's stylesheets in the order of their
// names — which is why those names carry a numeric prefix. The result is
// assembled at startup rather than committed as a built file: a committed
// build drifts from its sources and nothing notices, which is the failure
// this whole library exists to stop.
func LoadStylesheet(fsys fs.FS) (*Stylesheet, error) {
	names, err := fs.Glob(fsys, path.Join(componentCSSDir, "*.css"))
	if err != nil {
		return nil, fmt.Errorf("handler: list component css: %w", err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("handler: component library has no stylesheets")
	}
	sort.Strings(names)
	var buf bytes.Buffer
	for _, name := range names {
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("handler: read %s: %w", name, err)
		}
		// The file each rule came from, so a browser's inspector points at a
		// source that exists rather than at a line in a concatenation.
		fmt.Fprintf(&buf, "/* %s */\n", path.Base(name))
		buf.Write(body)
		if !bytes.HasSuffix(body, []byte("\n")) {
			buf.WriteByte('\n')
		}
		buf.WriteByte('\n')
	}
	sum := sha256.Sum256(buf.Bytes())
	return &Stylesheet{Body: buf.Bytes(), ETag: `"` + hex.EncodeToString(sum[:16]) + `"`}, nil
}

// ComponentCSS serves the assembled library stylesheet.
func (s *Server) ComponentCSS(w http.ResponseWriter, r *http.Request) {
	if s.Components == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("ETag", s.Components.ETag)
	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, s.Components.ETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(s.Components.Body)
}
