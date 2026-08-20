// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"io/fs"
	"regexp"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/web"
)

// 2.6.E1: an allow-list of where the component primitives may appear, rather
// than a list of bad patterns to catch. Wave 2.5 passed over ten screens that
// wrote the old header markup by hand because nothing said "this string may
// only appear here" — a fallback CSS rule made them look right anyway. This
// test is the same check the manual verification of 2.6.F ran by hand
// (`grep -c 'class="page-head"' …`), made to fail the build instead of
// depending on someone remembering to run it.
var (
	pageHeadClass = regexp.MustCompile(`class="page-head"`)
	pageBarClass  = regexp.MustCompile(`class="page-bar`)
	tableTag      = regexp.MustCompile(`<table`)
)

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

func TestComponentClassesStayInComponentFiles(t *testing.T) {
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
		if name == "base.html" {
			continue
		}
		b, err := fs.ReadFile(templateFS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		body := string(b)
		if pageHeadClass.MatchString(body) {
			t.Errorf("%s writes class=\"page-head\" by hand; call the pagehead component instead", name)
		}
		if pageBarClass.MatchString(body) {
			t.Errorf("%s writes class=\"page-bar\" by hand; call the pagebar component instead", name)
		}
		if tableTag.MatchString(body) && !tablesOutsideDatatable[name] {
			t.Errorf("%s opens a <table> by hand; call the datatable component instead "+
				"(or add it to tablesOutsideDatatable if it genuinely is not a data table)", name)
		}
	}
}
