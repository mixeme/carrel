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

// 2.6.G12: a class written into a template and never once defined in
// carrel.css is exactly what happened to .session-list/.session-row — the
// SESSIONS block fell back to the browser's bare <ul> markers and margins,
// with nothing on screen to say a rule was missing. This is 2.6.E1's
// allow-list philosophy turned around: instead of naming where a handful of
// component primitives may appear, it names every template class that is
// permitted to carry no styling of its own, so a new one has to be a
// decision, not a typo nobody noticed.
//
// Two real shapes of "allowed to be unstyled" apply here, not one:
//   - a JS/semantic marker matched only through a compound selector this
//     naive class-selector scan cannot see (e.g. ".find-row.is-merged"), or
//     matched from JavaScript via querySelector rather than CSS at all;
//   - a second class on a node whose look is already carried by a sibling
//     class or an ancestor rule, present for meaning or for JS to find, not
//     for its own declaration.
var unstyledClassAllowList = map[string]string{
	// Kind/state markers read by carrel.js (querySelector), not matched by
	// their own CSS rule — visual state comes from the base row class.
	"is-contact":  "JS marker only (base.html row-kind branch), not its own rule",
	"is-file":     "JS marker only (files.html), not its own rule",
	"is-merged":   "JS marker only (base.html, collapsed duplicate rows), not its own rule",
	"detail-link": "JS hook for the details panel; the cell (.t) carries the look",

	// Layout wrapper classes carried for meaning/specificity; the visual
	// rule lives on a sibling class already present on the same element.
	"calendar-layout":  "wrapper alongside .app-layout, which carries the rule",
	"contacts-layout":  "wrapper alongside .app-layout, which carries the rule",
	"person-main":      "wrapper alongside .app-main, which carries the rule",
	"attachments":      "modifier on .other-props, which carries the rule",
	"source-block":     "modifier on .detail-section, which carries the rule",
	"contact-readonly": "state modifier read by carrel.js; layout is unaffected by it",
	"danger-form":      "modifier on .form; only the destructive action inside needs marking",
	"notice":           "modifier on .message; only .message.error diverges from the base look",

	// Structural containers with no visual rule of their own — real content
	// (rows, panels) supplies all the visible styling; these exist to be
	// selected as a whole (a query root, a swap target), not to be drawn.
	"dup-actions":  "structural wrapper for a form's own layout, not a drawn box",
	"find-results": "structural wrapper found by findresults' own children",

	// Small, single-purpose helper spans with genuinely no visual treatment
	// distinct from their surrounding context.
	"files-up-row":       "row-kind marker; .m-row carries the box",
	"folder-picker-hint": "plain hint text; .hint's rule already applies via cascade",
	"folder-picker-warn": "plain warning text; .message.notice's rule already applies",
	"note-edit-form":     "form-identity marker for carrel.js, not a drawn box",
	"note-meta-block":    "structural grouping only; children carry their own rules",
	"note-meta-danger":   "modifier read by carrel.js to gate an action, not to draw",
	"note-sidebar-sec":   "structural grouping only; children carry their own rules",
	"note-text-field":    "field-identity marker for carrel.js, not a drawn box",
	"note-title-field":   "field-identity marker for carrel.js, not a drawn box",

	// The column-picker toggle is an .app-btn first; this second class is
	// only how carrel.js finds the one that opens the columns menu.
	"column-picker-toggle": "JS hook only; .app-btn carries the look",
}

// extractClassAttrs finds every class="..." value in a template, treating
// {{...}} spans as opaque so a quote inside a template action — {{if eq
// .Kind "task"}} — does not read as the end of the attribute.
func extractClassAttrs(body string) []string {
	var out []string
	for i := 0; ; {
		idx := strings.Index(body[i:], `class="`)
		if idx < 0 {
			return out
		}
		start := i + idx + len(`class="`)
		j := start
		for j < len(body) {
			if strings.HasPrefix(body[j:], "{{") {
				end := strings.Index(body[j:], "}}")
				if end < 0 {
					break
				}
				j += end + 2
				continue
			}
			if body[j] == '"' {
				break
			}
			j++
		}
		out = append(out, body[start:j])
		if j >= len(body) {
			return out
		}
		i = j + 1
	}
}

var (
	templateActionSpan = regexp.MustCompile(`\{\{[^}]*\}\}`)
	classToken         = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	cssClassSelector   = regexp.MustCompile(`\.([A-Za-z][A-Za-z0-9_-]*)`)
)

func TestEveryTemplateClassIsStyled(t *testing.T) {
	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	names, err := fs.Glob(templateFS, "*.html")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}

	usedIn := map[string][]string{}
	for _, name := range names {
		b, err := fs.ReadFile(templateFS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, attr := range extractClassAttrs(string(b)) {
			cleaned := templateActionSpan.ReplaceAllString(attr, " ")
			for _, tok := range strings.Fields(cleaned) {
				// A token still ending in "-" is the literal prefix of a
				// dynamically built class (is-{{.Kind}}), not a class on
				// its own; the concrete classes it produces are checked
				// wherever they appear as their own literal, e.g. is-task.
				if !classToken.MatchString(tok) || strings.HasSuffix(tok, "-") {
					continue
				}
				usedIn[tok] = append(usedIn[tok], name)
			}
		}
	}

	sheet := readStylesheet(t)
	styled := map[string]bool{}
	for _, m := range cssClassSelector.FindAllStringSubmatch(sheet, -1) {
		styled[m[1]] = true
	}

	var missing []string
	for cls := range usedIn {
		if styled[cls] {
			continue
		}
		if _, allowed := unstyledClassAllowList[cls]; allowed {
			continue
		}
		missing = append(missing, cls)
	}
	sort.Strings(missing)
	for _, cls := range missing {
		t.Errorf("class %q (in %v) has no rule in the component library or carrel.css and is "+
			"not in unstyledClassAllowList — "+
			"either style it or add it to the allow-list with why it needs none", cls, usedIn[cls])
	}
}
