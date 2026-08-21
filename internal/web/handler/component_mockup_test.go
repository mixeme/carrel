// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The mockups name 87 primitives and end with the rule that they move into
// production "by copying, not by redrawing". Until this gate, that was a
// sentence. Now it is a set comparison: every .m-* class in the mockups'
// stylesheet is either in the library or named here with why it is not —
// chrome of the signed-in shell, the desktop wrapper, or a primitive no
// production screen draws yet. A class that appears in neither list is a
// primitive that was redrawn under a new name, which is how sixteen names
// for one bar happened.
//
// Property values are not compared. A move and a fix are never the same
// commit: a narrow dialog is still 520 px, not the mockup's 420. The names
// are the contract this stage can keep.

var mockupClassRule = regexp.MustCompile(`\.m-([a-z0-9-]+)`)

// mockupClassesOutsideLibrary are .m-* classes the mockups draw that this
// library does not yet own. Each entry is a decision, not an omission.
var mockupClassesOutsideLibrary = map[string]string{
	// Signed-in shell — rebuilt with base.html in the interface-rebuild plan.
	"m-top":       "app chrome (app-top); interface-rebuild",
	"m-brand":     "app chrome (logotype in the top bar); interface-rebuild",
	"m-logo":      "app chrome (the SVG wordmark); interface-rebuild",
	"m-search":    "app chrome (the find field in the top bar); interface-rebuild",
	"m-top-right": "app chrome (New note / refresh / user); interface-rebuild",
	"m-user":      "app chrome (the signed-in name); interface-rebuild",
	"m-shell":     "app chrome (app-shell grid); interface-rebuild",
	"m-main":      "app chrome (app-main); interface-rebuild",
	"m-tabbar":    "phone bottom bar in carrel.css; interface-rebuild",
	"m-overlay":   "quick-note host (app-overlay); shell, not a screen primitive",
	"m-scrim":     "quick-note scrim (app-scrim); shell, not a screen primitive",
	"m-sheet":     "quick-note card (app-sheet); shell, not a screen primitive",

	// Desktop wrapper — this plan does not touch internal/desktop/.
	"m-win":      "native window frame; desktop wrapper",
	"m-titlebar": "native title bar; desktop wrapper",
	"m-winbtns":  "native window buttons; desktop wrapper",
	"m-menubar":  "native menu bar; desktop wrapper",
	"m-status":   "native status bar; desktop wrapper",
	"m-toast":    "desktop notification toast; desktop wrapper",
	"m-tray":     "desktop tray menu; desktop wrapper",

	// Drawn in the mockups, not yet a production screen.
	"m-skel":        "loading placeholder; no production screen draws it",
	"m-two":         "two-line cell; production nests spans inside the row",
	"m-photo":       "person photo lives in m-head as an img, not this class",
	"m-danger-zone": "destructive form foot; production uses m-formfoot + is-danger",
	"m-row--phone":  "phone modifier on .mock.is-phone, not a library component",
}

func TestMockupPrimitivesAreInTheLibrary(t *testing.T) {
	mockup := readMockupStylesheet(t)
	mockupSet := map[string]bool{}
	for _, m := range mockupClassRule.FindAllStringSubmatch(mockup, -1) {
		mockupSet["m-"+m[1]] = true
	}
	if len(mockupSet) < 80 {
		t.Fatalf("mockup stylesheet yielded %d .m-* classes; expected the closed set of ~87", len(mockupSet))
	}

	libSet := map[string]bool{}
	for _, c := range libraryClasses(t) {
		libSet[c] = true
	}

	var missing []string
	for c := range mockupSet {
		if libSet[c] {
			continue
		}
		if _, ok := mockupClassesOutsideLibrary[c]; ok {
			continue
		}
		missing = append(missing, c)
	}
	sort.Strings(missing)
	for _, c := range missing {
		t.Errorf("mockup class %q is in neither the component library nor mockupClassesOutsideLibrary — "+
			"copied into the library, or named here with why it is not", c)
	}

	var stale []string
	for c := range mockupClassesOutsideLibrary {
		if !mockupSet[c] {
			stale = append(stale, c)
		}
		if libSet[c] {
			t.Errorf("mockupClassesOutsideLibrary still lists %q, which is now in the library; remove the exception", c)
		}
	}
	sort.Strings(stale)
	for _, c := range stale {
		t.Errorf("mockupClassesOutsideLibrary lists %q, which the mockups no longer draw; remove the exception", c)
	}

	var extra []string
	for c := range libSet {
		if mockupSet[c] {
			continue
		}
		extra = append(extra, c)
	}
	sort.Strings(extra)
	for _, c := range extra {
		t.Errorf("library class %q is not in the mockups — a new name for something the mockups already named, "+
			"which is how sixteen names for one bar happened", c)
	}
}

func readMockupStylesheet(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "docs", "visual", "carrel-ui-mockups.html")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mockups: %v", err)
	}
	s := string(b)
	end := strings.Index(s, "</style>")
	if end < 0 {
		t.Fatal("mockups file has no </style>; the gate would scan the frames")
	}
	return s[:end]
}
