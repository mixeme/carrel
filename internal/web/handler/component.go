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
type Head struct {
	Title        string
	Subtitle     string
	SubtitleHTML template.HTML
	Hint         string
	Photo        string
	WrapClass    string
	Bar          bool
	Muted        bool
	Crumbs       []fileCrumb
}

// Bar is the input to the m-bar component.
type Bar struct {
	Attrs template.HTML
}

// BarForm is the input to the m-barform component: the same bar built as a
// real GET form, which the agenda's From/To filter needs.
type BarForm struct {
	Action string
	Method string
}

// DataTable is the input to the datatable component. Not yet a library
// component: every call site passes its own Class ("data", "files-table"),
// so the primitive still has no single name. It moves with the table stage.
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

func buildHead(pairs ...any) (Head, error)       { return typedDict[Head](pairs...) }
func buildBar(pairs ...any) (Bar, error)         { return typedDict[Bar](pairs...) }
func buildBarForm(pairs ...any) (BarForm, error) { return typedDict[BarForm](pairs...) }

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
