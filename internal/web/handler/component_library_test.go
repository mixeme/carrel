// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/web"
)

// A component nobody can find gets written again under a new name. That is
// not a guess: it is how one toolbar band ended up with sixteen names and one
// header with twenty-eight. So the library is required to be complete in all
// three of its halves at once — markup, styles, and the row in README.md that
// says the component exists and what it is for.
//
// The check is deliberately dull: it does not read CSS properties or judge
// markup. It only refuses to let a component exist in one place and not the
// others, because that is the state every duplicate started from.

var (
	componentDefine = regexp.MustCompile(`\{\{define "(m-[a-z0-9-]+)"\}\}`)
	readmeCode      = regexp.MustCompile("`(m-[a-z0-9-]+)`")
)

// pairedSuffix marks the closing half of a two-part component. m-head opens a
// tag and m-headend closes it; only the opening half needs its own row in the
// table, and the table says so.
const pairedSuffix = "end"

func componentLibraryFS(t *testing.T) fs.FS {
	t.Helper()
	fsys, err := fs.Sub(web.ComponentFS, "component")
	if err != nil {
		t.Fatalf("component FS: %v", err)
	}
	return fsys
}

func TestComponentLibraryIsComplete(t *testing.T) {
	fsys := componentLibraryFS(t)

	tmpls, err := fs.Glob(fsys, "tmpl/*.html")
	if err != nil {
		t.Fatalf("glob component templates: %v", err)
	}
	if len(tmpls) == 0 {
		t.Fatal("component library has no templates; the gate would pass vacuously")
	}

	readme, err := fs.ReadFile(fsys, "README.md")
	if err != nil {
		t.Fatalf("read component README: %v", err)
	}
	documented := map[string]bool{}
	for _, m := range readmeCode.FindAllStringSubmatch(string(readme), -1) {
		documented[m[1]] = true
	}

	// Every {{define}} in the library is a component; every component needs a
	// row saying it exists.
	var defined []string
	for _, name := range tmpls {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range componentDefine.FindAllStringSubmatch(string(b), -1) {
			defined = append(defined, m[1])
		}
	}
	sort.Strings(defined)
	for _, comp := range defined {
		if strings.HasSuffix(comp, pairedSuffix) {
			continue
		}
		if !documented[comp] {
			t.Errorf("component %q has no row in internal/web/component/README.md; "+
				"a component nobody can find is written again under a new name", comp)
		}
	}

	// Every class the library styles must belong to a component that exists,
	// and every component that draws a class must have styles for it.
	styled := map[string]bool{}
	for _, class := range libraryClasses(t) {
		styled[class] = true
	}
	drawn := map[string][]string{}
	for _, name := range tmpls {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, attr := range classAttr.FindAllStringSubmatch(string(b), -1) {
			// A class attribute is half template: class="m-sub{{if …}} …".
			// Strip the actions first, or the token read is "m-sub{{if".
			cleaned := templateActionSpan.ReplaceAllString(attr[1], " ")
			for _, f := range strings.Fields(cleaned) {
				if strings.HasPrefix(f, "m-") {
					drawn[f] = append(drawn[f], name)
				}
			}
		}
	}
	for class, where := range drawn {
		if !styled[class] {
			t.Errorf("the library draws %q (in %v) but styles it nowhere in component/css; "+
				"markup and styles live side by side so neither can go missing quietly",
				class, where)
		}
	}
}

// TestComponentStylesheetAssembles is the load-time contract the server relies
// on: the same assembly main.go performs at startup, so a stylesheet that
// cannot be built fails a test rather than the process.
func TestComponentStylesheetAssembles(t *testing.T) {
	sheet, err := LoadStylesheet(componentLibraryFS(t))
	if err != nil {
		t.Fatalf("LoadStylesheet: %v", err)
	}
	if len(sheet.Body) == 0 {
		t.Fatal("assembled stylesheet is empty")
	}
	if sheet.ETag == "" {
		t.Fatal("assembled stylesheet has no ETag")
	}
	// The order is the order of the names, which is why they carry a numeric
	// prefix; a cascade that depends on glob order is a cascade nobody can
	// reason about.
	if head, bar, row, form := strings.Index(string(sheet.Body), "10-head.css"),
		strings.Index(string(sheet.Body), "20-bar.css"),
		strings.Index(string(sheet.Body), "30-row.css"),
		strings.Index(string(sheet.Body), "40-form.css"); head < 0 || bar < 0 || row < 0 || form < 0 || head > bar || bar > row || row > form {
		t.Errorf("stylesheets are not concatenated in name order (head at %d, bar at %d, row at %d, form at %d)", head, bar, row, form)
	}
}
