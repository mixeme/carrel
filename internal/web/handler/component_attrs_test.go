// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"html/template"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/web"
)

// An attribute a screen hands a component is the last thing about that tag
// the screen still owns: `data-files-ops hidden` on the selection band,
// `data-columns-root` on a list, `data-files-list` on the table. It lands
// where html/template expects an attribute *name*, and that position is
// filtered by content type — template.HTML is refused there and comes out as
// the literal ZgotmplZ, taking the attribute with it.
//
// Nothing fails when it does. The class is still right, the page still
// renders, every gate stays green, and only the browser notices: the
// JavaScript that hangs off the attribute finds nothing. That is how the file
// browser lost its selection bar, its toolbar and its sortable table at once.
//
// Two gates, because there are two halves to get wrong: the type of the field
// in component.go, and the value the screen passes into it.

// TestComponentAttrsAreAttributeTyped reads the input structs rather than a
// list of them, so a component added tomorrow is covered on the day it is
// added.
func TestComponentAttrsAreAttributeTyped(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "component.go", nil, 0)
	if err != nil {
		t.Fatalf("parse component.go: %v", err)
	}

	seen := 0
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range st.Fields.List {
			for _, name := range field.Names {
				if name.Name != "Attrs" {
					continue
				}
				seen++
				sel, ok := field.Type.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "HTMLAttr" {
					t.Errorf("%s.Attrs is not template.HTMLAttr; it lands in an "+
						"attribute-name position, where anything else is written "+
						"out as ZgotmplZ and the attribute disappears", spec.Name.Name)
				}
			}
		}
		return true
	})
	if seen == 0 {
		t.Fatal("no Attrs field found in component.go; the gate would pass vacuously")
	}
}

// attrsUse finds the components that put an attribute on their own tag,
// whichever way they name the input ({{.Attrs}} or {{$t.Attrs}}).
var attrsUse = regexp.MustCompile(`\{\{\s*if\s+\$?[a-zA-Z]*\.Attrs\s*\}\}`)

// TestComponentAttrsReachTheTag renders each of those components and looks at
// what came out. The type check above says the field is declared right; this
// says the tag it is written into actually keeps it — the two are different
// questions, and it was the second one that broke.
func TestComponentAttrsReachTheTag(t *testing.T) {
	fsys := componentLibraryFS(t)
	names, err := fs.Glob(fsys, path.Join(componentTmplDir, "*.html"))
	if err != nil {
		t.Fatalf("glob component templates: %v", err)
	}
	set, err := template.New("gate").Funcs(templateFuncs).ParseFS(fsys, names...)
	if err != nil {
		t.Fatalf("parse component library: %v", err)
	}

	const marker = `data-gate-attr="1"`
	checked := 0
	for _, name := range names {
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, comp := range componentDefine.FindAllStringSubmatch(string(body), -1) {
			tmpl := set.Lookup(comp[1])
			if tmpl == nil || !attrsUse.MatchString(tmpl.Tree.Root.String()) {
				continue
			}
			checked++
			var out strings.Builder
			// A map, not the component's own struct: the gate is about the
			// tag, and this way it needs no table of which struct goes with
			// which component to keep up to date.
			in := map[string]any{"Attrs": template.HTMLAttr(marker)}
			if err := tmpl.Execute(&out, in); err != nil {
				t.Errorf("%s: %v", comp[1], err)
				continue
			}
			if !strings.Contains(out.String(), marker) {
				t.Errorf("%s renders %q: the attribute the screen passed is not on the tag",
					comp[1], out.String())
			}
			if strings.Contains(out.String(), "ZgotmplZ") {
				t.Errorf("%s renders %q: html/template refused the value in an "+
					"attribute-name position", comp[1], out.String())
			}
		}
	}
	if checked == 0 {
		t.Fatal("no component takes Attrs; the gate would pass vacuously")
	}
}

// TestScreensPassAttrsAsAttributes catches the other half in the screens
// themselves. `(safeHTML …)` is right everywhere else and wrong here, and the
// two read alike at a glance — which is why this says so in the one place a
// reader of the template will be looking.
func TestScreensPassAttrsAsAttributes(t *testing.T) {
	pages, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	names, err := fs.Glob(pages, "*.html")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	for _, name := range names {
		body, err := fs.ReadFile(pages, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, `"Attrs" (safeHTML`) {
				t.Errorf("%s:%d passes Attrs through safeHTML; an attribute name "+
					"needs safeAttr, or html/template drops it", name, i+1)
			}
		}
	}
}
