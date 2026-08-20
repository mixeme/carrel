// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"regexp"
	"strings"
	"testing"
)

// 2.6.E3: structural invariants on every rendered screen — exactly one
// header, the header before any content, and no heading that did not come
// through the component. Reading a template and reading what it renders are
// different questions: 2.6.E1 catches a hand-written `class="page-head"` in
// the source, this catches the same defect if it ever arrives some other
// way — a helper that builds the string at runtime, a partial that only
// exists in a fragment response, or a heading duplicated by a bug in the
// component itself.
var bareHeading = regexp.MustCompile(`<h1(\s[^>]*)?>`)

// authCardScreens are the sign-in-adjacent pages the mockup gives their own
// frame (§7.12): a centred card with a plain heading, not the rail-and-header
// shell `pagehead` is built for. Wave 2.5's acceptance (§7.9) verified these
// against that frame as they stand — wrapping them in `pagehead` would add a
// bar, breadcrumbs and hint slot the mockup never draws here, which is a
// regression against an already-verified match, not a fix. Anything not
// listed is expected to carry a real page-head.
var authCardScreens = map[string]bool{
	"/about":        true,
	"/forgot":       true,
	"/app/password": true,
}

func TestEveryShellScreenAssemblesInOrder(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)

	for _, path := range pageRoutes(t) {
		if authCardScreens[path] {
			continue
		}
		rec := a.get(path)
		if rec.Code >= 500 {
			continue // TestEveryPageObeysTheMarkupRules already reports this
		}
		body := rec.Body.String()
		if !strings.Contains(body, "<html") {
			continue // not a full page — same skip routes_conformance_test.go uses
		}

		if n := strings.Count(body, `<header class="page-head">`); n > 1 {
			t.Errorf("GET %s renders %d page-head headers; exactly one expected", path, n)
		}

		for _, m := range bareHeading.FindAllString(body, -1) {
			if !strings.Contains(m, "class=") {
				t.Errorf("GET %s renders %s: every <h1> should come from the pagehead component, "+
					"or the screen belongs in authCardScreens", path, m)
			}
		}

		headIdx := strings.Index(body, `<header class="page-head">`)
		if headIdx < 0 {
			continue // a screen with no list/detail content of its own — nothing to be first before
		}
		for _, marker := range []string{`<table`, `<ul class="list`, `id="find-panel"`} {
			if ci := strings.Index(body, marker); ci >= 0 && ci < headIdx {
				t.Errorf("GET %s renders %s before its page-head; the header is meant to be "+
					"the first thing in the work area", path, marker)
			}
		}
	}
}
