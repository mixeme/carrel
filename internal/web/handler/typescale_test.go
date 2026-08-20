// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/web"
)

// The mockups are not a picture to copy but a closed system of values, and
// this is that system: every size the design uses, and no others. Lifted from
// the .m-* rules of carrel-ui-mockups.html, which is the only place it is
// defined; if the design gains a size, it is added here in the same commit
// that uses it, and that is the point — a new number has to be a decision
// somebody made rather than a number somebody typed.
var typeScale = map[string]bool{
	"9.5": true, "10": true, "10.5": true, "11": true, "11.5": true,
	"12": true, "12.5": true, "13": true, "13.5": true,
	"16": true, "17": true, "19": true, "20": true, "22": true,
	"24": true, "26": true, "27": true,
}

// Sizes that set a glyph rather than text. An icon drawn with a font size is
// not part of the reading scale, and holding it to one would be a rule about
// nothing. Each entry names the selector so the exception cannot spread by
// accident.
var glyphSizes = map[string]string{
	".app-rail-toggle":                 "18px, the hamburger on a narrow screen",
	".app-bottom-nav .bottom-nav-icon": "18px, the icons of the bottom bar",
}

var (
	fontSizeDecl = regexp.MustCompile(`font-size:\s*([0-9.]+)(px|rem|em)`)
	selectorLine = regexp.MustCompile(`^([^@\s].*)\{\s*$`)
)

// TestStylesheetKeepsTheTypeScale is the check that the layout transfer needed
// and did not have. Comparing a screen with its frame is leaky — two passes
// proved it, missing ten screens between them — because it only looks at the
// screens somebody thought to open. A declaration cannot hide: every screen is
// drawn from this one file, so holding the file to the system holds every
// screen at once, including the ones with no frame of their own.
func TestStylesheetKeepsTheTypeScale(t *testing.T) {
	sheet := readStylesheet(t)

	var selector string
	for n, line := range strings.Split(sheet, "\n") {
		if m := selectorLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			selector = strings.TrimSpace(m[1])
		}
		m := fontSizeDecl.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		value, unit := m[1], m[2]

		// The system is written in pixels. A rem here is not a smaller
		// mistake than a wrong number: it is the pre-redesign scale, and it
		// is how the file came to hold 42 sizes nobody had chosen.
		if unit != "px" {
			t.Errorf("carrel.css:%d  %s sets font-size in %s; the design system is in px", n+1, selector, unit)
			continue
		}
		if typeScale[trimZero(value)] {
			continue
		}
		if why, ok := glyphSizes[selector]; ok {
			if trimZero(value) == strings.SplitN(why, "px", 2)[0] {
				continue
			}
		}
		t.Errorf("carrel.css:%d  %s uses %spx, which is not in the type scale of the mockups.\n"+
			"    Either take the nearest size the design already has, or add this one to typeScale "+
			"in the same commit — with a frame that uses it.", n+1, selector, value)
	}
}

// The two families are the whole typographic palette. A third one arriving by
// way of a browser default is the failure this catches: it does not look like
// a mistake on screen, it looks like a slightly different page.
func TestStylesheetKeepsTheTwoFamilies(t *testing.T) {
	sheet := readStylesheet(t)
	decl := regexp.MustCompile(`font-family:\s*([^;]+);`)
	allowed := map[string]bool{
		"var(--f-sans)": true, "var(--f-serif)": true, "var(--f-mono)": true,
		"inherit": true,
	}
	var selector string
	inFontFace := false
	for n, line := range strings.Split(sheet, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@font-face") {
			inFontFace = true
		} else if trimmed == "}" {
			inFontFace = false
		}
		if m := selectorLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			selector = strings.TrimSpace(m[1])
		}
		m := decl.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		value := strings.TrimSpace(m[1])
		// :root defines the three stacks themselves.
		if inFontFace || strings.HasPrefix(selector, ":root") || allowed[value] {
			continue
		}
		t.Errorf("carrel.css:%d  %s sets font-family: %s. Use one of the three tokens; a stack "+
			"written out here is a fourth family the design does not have.", n+1, selector, value)
	}
}

// trackingScale is the mockup's closed set of letter-spacing values (2.6.G4).
// 2.6.E4 named tracking as the third leg of the type-scale audit alongside
// size and family; only the first two ever got a test.
var trackingScale = map[string]bool{
	"0": true, "0.03": true, "0.04": true, "0.06": true,
	"0.1": true, "0.12": true, "0.14": true, "0.16": true, "0.18": true,
}

// TestStylesheetKeepsTheTracking is TestStylesheetKeepsTheTypeScale's shape
// applied to letter-spacing instead of font-size — the gate 2.6.E4 promised
// and never wrote. No known declaration fails it today; the point is the
// next one that would.
func TestStylesheetKeepsTheTracking(t *testing.T) {
	sheet := readStylesheet(t)
	decl := regexp.MustCompile(`letter-spacing:\s*([0-9.]+)(px|em|rem)?`)

	var selector string
	for n, line := range strings.Split(sheet, "\n") {
		if m := selectorLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			selector = strings.TrimSpace(m[1])
		}
		m := decl.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		value, unit := m[1], m[2]

		if value != "0" && unit != "em" {
			t.Errorf("carrel.css:%d  %s sets letter-spacing in %q; the design system's tracking is in em "+
				"(0 is unitless everywhere, mockups included)", n+1, selector, unit)
			continue
		}
		if trackingScale[trimZero(value)] {
			continue
		}
		t.Errorf("carrel.css:%d  %s uses %s%s tracking, which is not in the mockup's set.\n"+
			"    Either take the nearest value the design already has, or add this one to trackingScale "+
			"in the same commit — with a frame that uses it.", n+1, selector, value, unit)
	}
}

func readStylesheet(t *testing.T) string {
	t.Helper()
	staticFS, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		t.Fatalf("static FS: %v", err)
	}
	b, err := fs.ReadFile(staticFS, "carrel.css")
	if err != nil {
		t.Fatalf("read carrel.css: %v", err)
	}
	return string(b)
}

func trimZero(v string) string {
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return v
}
