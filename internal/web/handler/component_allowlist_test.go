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
	// Form controls are stamped onto native elements the screen owns — a
	// button's href, an input's name, a field's label. The class names the
	// primitive so height and the pick arrow are the system's.
	"m-form":      "the screen's <form> (method, action, enctype, hx-*); the class is the layout",
	"m-f":         "one labelled cell; contents (label, input, hint, extra button) are always the screen's",
	"m-fset":      "a labelled group of fields; the legend and the fields are the screen's",
	"m-formfoot":  "the action row under a form; which buttons sit there is the screen's",
	"m-seg":       "the joined segment; the options (links, buttons, their is-on) are the screen's",
	"m-btn":       "a button or button-link; label, href, type and icons are the screen's",
	"m-in":        "a field; name, type and value are the screen's. select.m-in draws the pick arrow",
	"m-check":     "a labelled checkbox or radio; the input and the words are the screen's",
	"m-hint":      "the hint under a field; the words are the screen's",
	"m-lbl":       "a field label sitting outside m-f > label",
	"m-rail":      "the screen's <nav> or <aside> (aria-label, data-app-rail / data-section-rail); the class is the column",
	"m-nav":       "the stack of section links; which hrefs sit there is the screen's",
	"m-rail-sec":  "one account group; the rows inside are the screen's",
	"m-rail-foot": "the Apply / New collection stripe; which buttons sit there is the screen's",
	"m-box":       "the tick square on a source row; checked / part / off are the screen's",
	"m-meta":      "the source-row tail (count, ro); the words are the screen's",
	"m-rubric":    "small-caps label; the words are the screen's (list group, rail section, form sheet, card, panel section)",
	"m-side":      "the details column (<aside> id, hidden, aria-label); the class is the column",
	"m-fields":    "the label/value grid; which dt/dd sit there is the screen's",
	"m-sec":       "a subsection under a panel header; the rubric and the body are the screen's",
	"m-dialog":    "the centred dialog frame; wide/narrow and the contents are the screen's",
	"m-card":      "a bordered card; the rubric and the body are the screen's",
	"m-msg":       "a banner; the words and the role (alert/accent) are the screen's",
	"m-empty":     "an empty-state block; the copy and the CTA are the screen's",
	"m-poll":      "the poll bar; the summary and Stop are the screen's",
	"m-prog":      "the poll bar's meter; fill is data-fill, owned by the screen",
	"m-menu":      "a popover; which rows sit there is the screen's",
	"m-badge":     "an inline badge; the words and is-linked/is-alert/is-local are the screen's",
	"m-tag":       "a category chip; the words and is-on are the screen's",
	"m-tick":      "the 13px task tick; checked/label are the screen's",
	"m-av":        "the list-row avatar; initials or photo are the screen's",
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
// again. The leftover — note-read-title — is a 27px document title, not this header.
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
// again. The files table is still a table (stage 0 leftover), not
// .m-row--file.
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

var retiredFormNames = []string{
	"app-btn", "form-field", "form-foot", "form-hint", "form-label",
	"form-fieldset", "bar-sort", "theme-segment", "theme-segment-btn",
	"dialog-foot", "dup-buttons",
}

func TestRetiredFormNamesStayGone(t *testing.T) {
	assertRetiredClassesGone(t, retiredFormNames, "m-form / m-f / m-in / m-btn / m-seg / m-formfoot")
}

// A screen that writes one of these is inventing a second name for the
// rail again. The drawer chrome (app-rail-head / -toggle / -scrim / -mount)
// stays: it is the phone panel, not the column.
var retiredRailNames = []string{
	"src-row", "src-bar", "src-label", "src-box", "src-check",
	"src-sec", "src-rubric", "src-list", "src-meta",
	"section-rail", "app-nav", "app-rail",
	"rail-foot", "settings-rail", "settings-rubric", "settings-rail-foot",
	"admin-rail",
}

func TestRetiredRailNamesStayGone(t *testing.T) {
	assertRetiredClassesGone(t, retiredRailNames, "m-rail / m-nav / m-src / m-rail-sec / m-rail-foot")
}

var retiredPanelNames = []string{
	"app-details", "detail-panel", "detail-fields", "detail-section",
	"dialog-host", "app-dialog", "dialog-card", "dialog-rubric",
	"dup-head", "dup-group", "dup-badge", "dup-fields",
	"find-poll", "find-prog",
	"app-create-menu", "dots-menu",
	"badge", "tag", "task-tick", "list-avatar",
	"message", "warning", "page-hint",
	"settings-account-card", "files-props",
}

func TestRetiredPanelNamesStayGone(t *testing.T) {
	assertRetiredClassesGone(t, retiredPanelNames, "m-side / m-fields / m-sec / m-dialog / m-card / m-msg / m-empty / m-poll / m-menu / m-badge / m-tag / m-tick / m-av")
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
