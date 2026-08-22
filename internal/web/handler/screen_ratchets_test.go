// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// interface-rebuild.md §3.2 — the three gates that measure the distance left
// to go, rather than the distance already covered.
//
// None of them can be green today: the rebuild has not started, and a gate
// that only goes green at the end would be switched off long before it did.
// So each is a ratchet. It holds the measurement of the day it was written,
// refuses to let the number rise, and refuses just as firmly to let the number
// fall without the record being updated — a ratchet that can be left behind is
// a ratchet that will be. An entry that survives to the end of the plan has to
// be named with its reason rather than inherited in silence.

// ------------------------------------------------- 1. assembled, not written

// blockContainer is the set §3.2 names: the tags that build a layout. A screen
// that opens one of these is building a layout, and the whole claim of the
// rebuild is that a screen no longer does — it calls components and fills
// slots. Inline content of a slot (span, a, input, svg, label) is fine, and so
// is <form>, which carries the screen's own method and action: the form is a
// slot by the design of m-form.
var blockContainer = regexp.MustCompile(`(?i)<(div|section|table|ul|ol|nav|aside|header|footer)\b`)

// containersLeft is one screen's own layout tags, counted on the day the gate
// was written. A rebuilt screen takes its line to zero and drops out of the
// map; anything else moving is a number to be re-recorded on purpose.
//
// The claim the plan makes about editing rather than rewriting is visible
// right here: after four stages of the component library, a list template
// still carries ten containers of its own, and every one of them looks
// harmless on its own.
const containersInScreenTemplates = 552

var containersLeft = map[string]int{
	"about.html": 1, "admin.html": 8, "admin_audit.html": 3, "admin_dav.html": 12,
	"admin_escrow.html": 17, "admin_install.html": 3, "admin_invites.html": 16,
	"admin_settings.html": 17, "agenda.html": 11, "base.html": 49,
	"calendar_import.html": 6, "calendar_import_report.html": 10,
	"collection_form.html": 15, "conflict.html": 3, "contact.html": 39,
	"contact_crop.html": 3, "contact_panel.html": 5, "contacts.html": 10,
	"contacts_import.html": 6, "contacts_import_report.html": 10, "contacts_page.html": 1,
	"duplicate_merge.html": 17, "duplicates.html": 3, "email_confirmed.html": 1,
	"event.html": 37, "event_panel.html": 9, "files.html": 26, "files_published.html": 3,
	"folder_picker.html": 5, "folder_picker_children.html": 1, "forgot.html": 1,
	"invite.html": 7, "invite_invalid.html": 1, "login.html": 5, "note.html": 35,
	"note_panel.html": 7, "note_quick.html": 10, "notes.html": 11,
	"notes_import.html": 13, "notes_import_report.html": 10, "password.html": 5,
	"person.html": 8, "register.html": 6, "register_sent.html": 1, "search.html": 3,
	"settings_account.html": 10, "settings_appearance.html": 7,
	"settings_attachments.html": 4, "settings_connections.html": 18, "setup.html": 7,
	"task.html": 22, "task_panel.html": 5, "tasks.html": 9,
}

func TestScreenTemplateIsAssemblyOnly(t *testing.T) {
	total := 0
	recorded := map[string]bool{}
	for _, name := range screenTemplateNames(t) {
		body := templateAction.ReplaceAllString(readTemplate(t, name), " ")
		n := len(blockContainer.FindAllString(body, -1))
		total += n
		want, listed := containersLeft[name]
		recorded[name] = true
		switch {
		case n == 0 && listed:
			t.Errorf("%s no longer opens a container of its own — remove its line from "+
				"containersLeft and take %d off containersInScreenTemplates", name, want)
		case n > 0 && !listed:
			t.Errorf("%s opens %d containers of its own (div/section/table/ul/ol/nav/aside/"+
				"header/footer). A screen calls components and fills slots; a container of its "+
				"own means it was not assembled from the library", name, n)
		case listed && n != want:
			t.Errorf("%s opens %d containers of its own, and containersLeft says %d. "+
				"Fewer is the point — record it, and move containersInScreenTemplates by the "+
				"same amount, so the ratchet cannot be left behind", name, n, want)
		}
	}
	for name := range containersLeft {
		if !recorded[name] {
			t.Errorf("containersLeft still counts %s, which is not a template any more", name)
		}
	}
	if total != containersInScreenTemplates {
		t.Errorf("screen templates open %d containers of their own; containersInScreenTemplates "+
			"says %d. The number goes one way", total, containersInScreenTemplates)
	}
}

// ------------------------------------------- 2. no shared class outside the library

// A class in carrel.css that more than one template writes is a component by
// definition — it is one look with one name, used in several places, which is
// the entire description of a component. Wave 2.5 grew 368 of those. The
// library exists so that the number goes to zero; this gate is how anyone can
// tell whether it is going there.
//
// Each line is a class and where it still lives. Most of these fall out with
// the screen that writes them; the utility names (mono, no-print, hint) fall
// out when the shell and the library take them over in О2.
const sharedClassesOutsideTheLibrary = 51

var sharedClassesLeft = map[string]string{
	"actions":             "hand-written action row; m-acts / m-formfoot",
	"admin-panel":         "the administration section body; a panel, not a screen class",
	"agenda-print-root":   "print scope marker on five screens",
	"app-create-item":     "a row of the create menu; m-menu owns the popover, not its rows",
	"app-layout":          "the shell grid; О2 replaces it with the frame's m-shell",
	"app-main":            "the work column; О2 replaces it with m-main",
	"attachment-list":     "the attachments block of a panel; §23.10's own list",
	"auth-brand":          "the sign-in card's logotype block (§7.12)",
	"auth-card":           "the §7.12 frame: a centred card, not the rail-and-header shell",
	"auth-tagline":        "the tagline under the sign-in logotype",
	"collection-dot":      "colour dot on a connections row; m-bar3 is the stripe",
	"contact-photo":       "the photo on three contact screens",
	"contacts-header":     "a record screen's header wrap",
	"contacts-print-root": "print scope marker",
	"detail-panel-body":   "the body of the four panels; m-side owns the column",
	"diagnostic":          "the server-said dump; m-diag in the library, this is the wrap",
	"dup-members":         "the rows of a duplicate group",
	"field-value":         "a read-only value beside a label",
	"files-layout":        "files' own shell grid",
	"folder-picker":       "the picker dialog body",
	"folder-picker-title": "the picker's title, read by carrel.js",
	"hint":                "the oldest utility of them all: small muted text on 39 screens",
	"import-preview":      "the import wizard's preview table wrap",
	"import-report-stat":  "one number of an import report",
	"import-stats":        "the row of numbers above an import preview",
	"inline-form":         "a form that sits inside a row rather than on its own",
	"is-active":           "state marker; a modifier with no component of its own",
	"is-disabled":         "state marker; a modifier with no component of its own",
	"is-error":            "state marker on import rows",
	"is-on":               "state marker written outside a library component",
	"is-primary":          "the button modifier, styled in carrel.css instead of the library",
	"is-wide":             "the wide dialog modifier, styled outside m-dialog",
	"link":                "a plain text link, styled per screen",
	"list-filter":         "the filter field wrap on four list screens (B1 rebuilds it)",
	"mono":                "monospace utility on 14 screens",
	"no-print":            "print utility on 16 screens",
	"note-body":           "the note text on the screen and in the panel",
	"optional":            "a marker on optional form fields",
	"other-props":         "«Kept as the server sent them» on five screens",
	"print-footer":        "the printed footer line",
	"print-only":          "print utility",
	"print-photo":         "photo in printed contacts",
	"related-list":        "the Related block of six panels and cards",
	"settings-panel":      "the settings section body",
	"source-label":        "the account · collection line on four record screens",
	"sr-only":             "second name for visually-hidden; one of them is redundant",
	"subsection":          "an administration subsection",
	"subsection-h2":       "its heading",
	"subsection-h3":       "its smaller heading",
	"tz-label":            "the source time zone of an event (§23.8)",
	"visually-hidden":     "screen-reader-only text",
}

var (
	cssRuleClass  = regexp.MustCompile(`\.([A-Za-z][A-Za-z0-9_-]*)`)
	plainClassTok = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
)

func TestNoSharedClassOutsideTheLibrary(t *testing.T) {
	sheet, err := os.ReadFile(strings.Join([]string{"..", "static", "carrel.css"}, string(os.PathSeparator)))
	if err != nil {
		t.Fatalf("read carrel.css: %v", err)
	}
	styled := map[string]bool{}
	for _, m := range cssRuleClass.FindAllStringSubmatch(string(sheet), -1) {
		styled[m[1]] = true
	}
	if len(styled) < 50 {
		t.Fatalf("only %d classes found in carrel.css; the scan is wrong, not the stylesheet", len(styled))
	}

	usedIn := map[string][]string{}
	for _, name := range screenTemplateNames(t) {
		body := templateAction.ReplaceAllString(readTemplate(t, name), " ")
		seen := map[string]bool{}
		for _, attr := range classAttr.FindAllStringSubmatch(body, -1) {
			for _, tok := range strings.Fields(attr[1]) {
				if !plainClassTok.MatchString(tok) || strings.HasPrefix(tok, "m-") || seen[tok] {
					continue
				}
				seen[tok] = true
				usedIn[tok] = append(usedIn[tok], name)
			}
		}
	}

	shared := map[string]bool{}
	for class, where := range usedIn {
		if len(where) > 1 && styled[class] {
			shared[class] = true
		}
	}

	for class := range shared {
		if _, known := sharedClassesLeft[class]; !known {
			sort.Strings(usedIn[class])
			t.Errorf("carrel.css styles %q and %d templates write it (%v). One look with one name "+
				"in several places is a component: move it into internal/web/component, or reduce it "+
				"to the one screen it belongs to. This is how 368 of them grew",
				class, len(usedIn[class]), usedIn[class])
		}
	}
	for class, why := range sharedClassesLeft {
		if !shared[class] {
			t.Errorf("sharedClassesLeft still lists %q (%s), which is no longer shared; "+
				"remove the line and take one off sharedClassesOutsideTheLibrary", class, why)
		}
	}
	if len(sharedClassesLeft) != sharedClassesOutsideTheLibrary {
		t.Errorf("sharedClassesLeft holds %d classes and sharedClassesOutsideTheLibrary says %d; "+
			"the two move down together", len(sharedClassesLeft), sharedClassesOutsideTheLibrary)
	}
}

// ------------------------------------------ 3. the script binds to data, not to looks

// A class is the name of a look and changes when the look does; a binding
// point must not. carrel.js already reaches for data-* nearly everywhere —
// these are what is left, and each one is a place where renaming a component
// would break behaviour with nothing to say so.
var componentSelectorsLeft = map[string]string{
	".m-row":         "the row a filter hides and a keyboard walks; wants data-row",
	".m-row.is-on":   "the open row; wants a data-* state, not a look modifier",
	".m-row.is-task": "the task rows the Open/Done/All filter counts (B3)",
	".m-menu":        "the popover inside ⋯ and the create menu; wants data-menu",
	".m-tile":        "the file tile a click opens; wants data-file-row alone",
	".m-tree":        "the folder tree of the picker; wants data-tree",
}

// componentSelector finds a component class used as a selector. The comment on
// carrel.js line 511 says why the bar is *not* found this way, and a gate that
// counted the comment would be reporting the fix as the defect.
var componentSelector = regexp.MustCompile(`['"` + "`" + `][^'"` + "`" + `
]*?(\.m-[a-z0-9-]+(?:\.[a-z0-9-]+)*)`)

func TestScriptNeverBindsToComponentClasses(t *testing.T) {
	src, err := os.ReadFile(strings.Join([]string{"..", "static", "carrel.js"}, string(os.PathSeparator)))
	if err != nil {
		t.Fatalf("read carrel.js: %v", err)
	}
	found := map[string]bool{}
	for _, m := range componentSelector.FindAllStringSubmatch(string(src), -1) {
		found[m[1]] = true
	}

	for sel := range found {
		if _, known := componentSelectorsLeft[sel]; !known {
			t.Errorf("carrel.js binds to %s. A class is the name of a look and changes when the "+
				"look does; a binding point must not. Give the node a data-* of its own and ask "+
				"for that", sel)
		}
	}
	for sel, why := range componentSelectorsLeft {
		if !found[sel] {
			t.Errorf("componentSelectorsLeft still lists %s (%s), which carrel.js no longer uses; "+
				"remove the line", sel, why)
		}
	}
	t.Logf("component-class selectors left in carrel.js: %d", len(found))
}
