// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"regexp"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// 2.6.E3: structural invariants on every rendered screen — exactly one
// header, the header before any content, and no heading that did not come
// through the component. Reading a template and reading what it renders are
// different questions: 2.6.E1 catches a hand-written `class="m-head"` in
// the source, this catches the same defect if it ever arrives some other
// way — a helper that builds the string at runtime, a partial that only
// exists in a fragment response, or a heading duplicated by a bug in the
// component itself.
var bareHeading = regexp.MustCompile(`<h1(\s[^>]*)?>`)

// headerlessScreens are the routes a real m-head is not expected on, each
// for its own stated reason — an enumeration of what is allowed, the same
// shape as 2.6.E1's allow-list, not a list of patterns to avoid. Anything not
// listed here is expected to carry exactly one.
var headerlessScreens = map[string]string{
	// The sign-in-adjacent pages the mockup gives their own frame (§7.12): a
	// centred card with a plain heading, not the rail-and-header shell
	// `m-head` is built for. Wave 2.5's acceptance (§7.9) verified these
	// against that frame as they stand — wrapping them in `m-head` would
	// add a bar, breadcrumbs and hint slot the mockup never draws here,
	// which is a regression against an already-verified match, not a fix.
	"/about":        "auth-card frame (§7.12)",
	"/forgot":       "auth-card frame (§7.12)",
	"/app/password": "auth-card frame (§7.12)",
	// The shared folder-tree dialog content (files_tree.go, folder_picker.html):
	// a picker fragment reused inside Move/Copy dialogs and stand-alone, not a
	// screen of its own with a title to give.
	"/app/files/picker": "dialog fragment, not a screen",
}

// seedMergedViewAccount adds one DAV account with an address book, a
// calendar and a file collection (2.6.G2). The merged ($d.Mode) branch of
// contacts/calendar/tasks/notes — the branch every one of these screens
// actually ships on — is only reached once at least one collection of the
// right kind exists; an empty instance such as newApp(t, nil) never falls
// into it at all, which is exactly how a duplicated header there went
// unnoticed. s.Fanout is left nil, so the merged view's own early-out
// ("Cross-source polling is not configured") renders the full page
// synchronously — no fake DAV server needs to answer anything.
func seedMergedViewAccount(t *testing.T, a *app) {
	t.Helper()
	sess := a.session()
	acc := account.Account{
		ID: "g2-fixture", Label: "Fixture", BaseURL: "https://dav.example.invalid/",
		Username: "fixture", Password: "fixture", Enabled: true,
		Collections: []discovery.Collection{
			{Path: "/dav/addressbooks/fixture/book/", DisplayName: "Book", Kind: discovery.KindAddressBook},
			{Path: "/dav/calendars/fixture/cal/", DisplayName: "Calendar", Kind: discovery.KindCalendar},
			{Path: "/dav/files/fixture/", DisplayName: "Files", Kind: discovery.KindFiles},
		},
	}
	actor := store.Actor{ID: sess.UserID, Login: sess.Login}
	if err := a.Store.PutDAVAccount(actor, sess.UserID, sess.DEK(), acc); err != nil {
		t.Fatalf("seed DAV account: %v", err)
	}
}

func TestEveryShellScreenAssemblesInOrder(t *testing.T) {
	t.Run("empty instance", func(t *testing.T) {
		a := newApp(t, nil)
		a.setupAdmin("root", "root@example.org", testPassword)
		checkShellScreensAssembleInOrder(t, a)
	})
	t.Run("populated instance", func(t *testing.T) {
		a := newApp(t, nil)
		a.setupAdmin("root", "root@example.org", testPassword)
		seedMergedViewAccount(t, a)
		checkShellScreensAssembleInOrder(t, a)
	})
}

func checkShellScreensAssembleInOrder(t *testing.T, a *app) {
	t.Helper()
	for _, path := range pageRoutes(t) {
		if _, ok := headerlessScreens[path]; ok {
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

		if n := strings.Count(body, `<header class="m-head">`); n != 1 {
			t.Errorf("GET %s renders %d m-head headers; exactly one expected "+
				"(list it in headerlessScreens if none is genuinely correct here)", path, n)
		}

		for _, m := range bareHeading.FindAllString(body, -1) {
			if !strings.Contains(m, "class=") {
				t.Errorf("GET %s renders %s: every <h1> should come from the m-head component, "+
					"or the screen belongs in headerlessScreens", path, m)
			}
		}

		headIdx := strings.Index(body, `<header class="m-head">`)
		if headIdx < 0 {
			continue // a screen with no list/detail content of its own — nothing to be first before
		}
		for _, marker := range []string{`<table`, `<ul class="list`, `id="find-panel"`} {
			if ci := strings.Index(body, marker); ci >= 0 && ci < headIdx {
				t.Errorf("GET %s renders %s before its m-head; the header is meant to be "+
					"the first thing in the work area", path, marker)
			}
		}
	}
}
