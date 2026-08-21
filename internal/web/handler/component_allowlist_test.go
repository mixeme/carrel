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

// 2.6.E1, extended when the component library moved to internal/web/component:
// an allow-list of where the primitives may appear, rather than a list of bad
// patterns to catch. Wave 2.5 passed over ten screens that wrote the old header
// markup by hand because nothing said "this string may only appear here" — a
// fallback CSS rule made them look right anyway.
//
// The list of classes to police is no longer written here. It is read out of
// the library's own stylesheets, so a component added tomorrow is guarded the
// day it is added, without anyone remembering to extend a test. That is the
// same reason the library keeps markup and styles side by side: a rule that
// has to be repeated somewhere else is a rule that will be forgotten.
var (
	libraryClassRule = regexp.MustCompile(`(?m)^\.(m-[a-z0-9-]+)`)
	tableTag         = regexp.MustCompile(`<table`)
)

// slotClasses are the library classes any screen may write: content the
// screen owns, sitting in a box the component sizes. .m-acts holds whatever
// actions this screen has, but how they line up is the header's business.
var slotClasses = map[string]string{
	"m-acts": "the screen's own actions, laid out by m-head",
	"m-bar3": "the collection stripe; the row names the colour, the component names the size",
}

// standaloneUse names, per class, the screens allowed to use a primitive on
// its own, away from the component it usually lives in — and only those. The
// permission is per screen rather than global on purpose: "some screens may
// write a heading by hand" is how ten of them drifted in wave 2.5.
//
// The sign-in-adjacent screens carry the mockup's own §7.12 frame — a centred
// card, not the rail-and-header shell — and take the title and subtitle
// typography without the header layout around them, exactly as §7.12 draws
// them inside .m-dialog.
var standaloneUse = map[string][]string{
	"m-h1":  {"setup.html"},
	"m-sub": {"setup.html"},
}

func mayWrite(name, class string) bool {
	if _, ok := slotClasses[class]; ok {
		return true
	}
	for _, allowed := range standaloneUse[class] {
		if allowed == name {
			return true
		}
	}
	return false
}

// tablesOutsideDatatable are the templates whose tables were never in scope
// for the datatable component of 2.6.A3 — import previews and the conflict
// diff are a different shape of table (a row per field, not a row per
// record), not "admin and files stop being two tables". Anything not listed
// here is expected to reach a `<table>` only through the component.
var tablesOutsideDatatable = map[string]bool{
	"calendar_import.html": true,
	"contacts_import.html": true,
	"notes_import.html":    true,
	"conflict.html":        true,
}

// libraryClasses reads every class the component library defines.
func libraryClasses(t *testing.T) []string {
	t.Helper()
	componentFS, err := fs.Sub(web.ComponentFS, "component")
	if err != nil {
		t.Fatalf("component FS: %v", err)
	}
	names, err := fs.Glob(componentFS, "css/*.css")
	if err != nil {
		t.Fatalf("glob component css: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("component library has no stylesheets")
	}
	seen := map[string]bool{}
	for _, name := range names {
		b, err := fs.ReadFile(componentFS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range libraryClassRule.FindAllStringSubmatch(string(b), -1) {
			seen[m[1]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func TestComponentClassesStayInComponentFiles(t *testing.T) {
	classes := libraryClasses(t)
	if len(classes) == 0 {
		t.Fatal("no library classes found; the gate would pass vacuously")
	}

	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	names, err := fs.Glob(templateFS, "*.html")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no templates found")
	}
	for _, name := range names {
		b, err := fs.ReadFile(templateFS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		body := string(b)
		for _, class := range classes {
			if mayWrite(name, class) {
				continue
			}
			// class="m-bar …" and class="… m-bar" both count: what matters is
			// that the screen wrote the library's class itself.
			if writesClass(body, class) {
				t.Errorf("%s writes class %q by hand; call the component instead "+
					"(or name it in slotClasses / standaloneUse with the reason it is "+
					"this screen's to write)", name, class)
			}
		}
		if name == "base.html" {
			continue
		}
		if tableTag.MatchString(body) && !tablesOutsideDatatable[name] {
			t.Errorf("%s opens a <table> by hand; call the datatable component instead "+
				"(or add it to tablesOutsideDatatable if it genuinely is not a data table)", name)
		}
	}
}

// writesClass reports whether body has the class inside a class="…" value.
func writesClass(body, class string) bool {
	for _, attr := range classAttr.FindAllStringSubmatch(body, -1) {
		for _, f := range strings.Fields(attr[1]) {
			if f == class {
				return true
			}
		}
	}
	return false
}

var classAttr = regexp.MustCompile(`class="([^"]*)"`)

// retiredBarNames are the extra names the action bar had before stage 1
// collapsed them onto .m-bar / .m-sel / .m-sep / .m-right / .m-acts.
// A screen that writes one of these is inventing a sixteenth name again.
var retiredBarNames = []string{
	"files-ops-bar", "files-ops-sep", "files-ops-right", "files-ops-count",
	"dialog-acts", "bar-count", "form-actions", "conflict-actions",
}

func TestRetiredBarNamesStayGone(t *testing.T) {
	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	names, err := fs.Glob(templateFS, "*.html")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	for _, name := range names {
		b, err := fs.ReadFile(templateFS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		body := string(b)
		for _, class := range retiredBarNames {
			if writesClass(body, class) {
				t.Errorf("%s still writes retired bar class %q; use m-bar / m-sel / m-sep / m-right / m-acts",
					name, class)
			}
		}
	}
}

// A screen that writes one of these is inventing an eleventh header family
// again. The leftovers — dup-head, note-read-title — are a card rubric and
// a 27px document title, not this header.
var retiredHeadNames = []string{
	"dialog-head", "dialog-sub",
	"detail-panel-head", "detail-panel-title", "detail-panel-sub",
	"detail-panel-actions", "detail-panel-intro", "detail-panel-photo",
	"person-head",
	"note-read-head", "note-read-sub", "note-read-actions",
	"note-read-meta", "note-read-bar",
}

func TestRetiredHeadNamesStayGone(t *testing.T) {
	assertRetiredClassesGone(t, retiredHeadNames, "m-head / m-h1 / m-sub / m-acts")
}

// A screen that writes one of these is inventing a fourteenth row family
// again. .src-row is the rail (stage 5); the files table is still a table
// (stage 0 leftover), not .m-row--file.
var retiredRowNames = []string{
	"list-row", "list-row--contact", "list-row--agenda", "list-row--task",
	"list-row--note", "list-row--find", "list-row--contact-find",
	"contact-row", "contact-row-link",
	"task-row", "task-title", "task-meta",
	"note-row", "note-link", "note-excerpt",
	"find-row", "find-row-time", "find-row-body", "find-row-match",
	"find-row-kind", "find-row-source", "find-rows",
	"agenda-event-row", "agenda-events", "agenda-time",
	"list-group", "list-rubric", "list-num",
	"list-title", "list-sub", "list-detail", "list-time", "list-end", "list-bar",
}

func TestRetiredRowNamesStayGone(t *testing.T) {
	assertRetiredClassesGone(t, retiredRowNames, "m-list / m-row / m-group / m-rubric")
}

func assertRetiredClassesGone(t *testing.T, classes []string, instead string) {
	t.Helper()
	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	names, err := fs.Glob(templateFS, "*.html")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	for _, name := range names {
		b, err := fs.ReadFile(templateFS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		body := string(b)
		for _, class := range classes {
			if writesClass(body, class) {
				t.Errorf("%s still writes retired class %q; use %s", name, class, instead)
			}
		}
	}
}
