// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// interface-rebuild.md §3.2: "the first act of accepting any screen is to
// break it". Green tests prove nothing until it is shown that the test can go
// red, and on the day the gate is written there is no rebuilt screen to break —
// every one of the fifty-five would fail for the same reason, and a gate that
// fails everywhere says as little as one that passes everywhere.
//
// So the comparison is bitten here instead, on a frame and a screen written
// out in full: take the bar away, change a button's label, put the nodes in
// another order, and exactly the difference that was introduced has to come
// back. And the three things the gate is required *not* to notice — a repeat,
// an extra node, a node the mockups marked as belonging to another wave — have
// to stay unnoticed, because a gate that cries at those gets switched off.

const biteFrame = `<div class="m-shell">
  <div class="m-rail">
    <div class="m-nav">
      <a class="is-on">Contacts<span class="m-num">312</span></a>
      <a>Calendar<span class="m-num">14</span></a>
    </div>
  </div>
  <div class="m-main">
    <div class="m-head">
      <div class="m-h1">Combined address book</div>
      <div class="m-acts">
        <span class="m-btn is-primary">New contact</span>
        <span class="m-btn">Export</span>
      </div>
    </div>
    <div class="m-bar">
      <span class="m-in" placeholder="Filter the loaded cards"></span>
      <span class="m-seg"><span class="is-on">A-Z</span><span>Recently changed</span></span>
    </div>
    <div class="m-list">
      <div class="m-row m-row--contact"><span class="t">one</span></div>
    </div>
    <div class="m-sec" data-planned="§23.8"><div class="m-rubric">Source</div></div>
  </div>
</div>`

// biteScreen is the same shape as the app would answer with: different tags,
// a wrapper the frame does not draw, three rows where the frame drew one.
const biteScreen = `<div class="m-shell">
  <nav class="m-rail" data-app-rail>
    <div class="m-nav">
      <a class="is-on" href="/app/contacts">Contacts<span class="m-num">7</span></a>
      <a href="/app/calendar">Calendar<span class="m-num">2</span></a>
    </div>
  </nav>
  <div class="app-main contacts-layout m-main">
    <header class="m-head">
      <div><h1 class="m-h1">Combined address book</h1></div>
      <div class="m-acts no-print">
        <a class="m-btn is-primary" href="/new"><svg><use href="#i-plus"/></svg>New contact</a>
        <a class="m-btn" href="/export">Export</a>
      </div>
    </header>
    <div class="m-bar" data-bar>
      <label><input class="m-in" placeholder="Filter the loaded cards"></label>
      <div class="m-seg"><a class="is-on">A-Z</a><a>Recently changed</a></div>
      <span class="m-right is-2nd"><span>7 items</span></span>
    </div>
    <ul class="m-list">
      <li class="m-row m-row--contact"><span class="t">one</span></li>
      <li class="m-row m-row--contact"><span class="t">two</span></li>
      <li class="m-row m-row--contact"><span class="t">three</span></li>
    </ul>
  </div>
</div>`

func skelOf(t *testing.T, source string) []*skel {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return buildSkel(doc, nil)
}

func problemsBetween(t *testing.T, frame, screen string) []string {
	t.Helper()
	var out []string
	diffSkel("/app/contacts", skelOf(t, frame), skelOf(t, screen), &out)
	return out
}

func TestFrameDiffBites(t *testing.T) {
	t.Run("a screen built from the frame is silent", func(t *testing.T) {
		if problems := problemsBetween(t, biteFrame, biteScreen); len(problems) != 0 {
			t.Errorf("the gate complains about a screen that matches:\n  %s",
				strings.Join(problems, "\n  "))
		}
	})

	// §3.2's own example: take the m-bar call out of the assembled screen.
	t.Run("a missing component is named", func(t *testing.T) {
		without := cutBlock(t, biteScreen, `<div class="m-bar" data-bar>`, `</div>
    <ul class="m-list">`)
		problems := problemsBetween(t, biteFrame, without)
		if len(problems) != 1 || !strings.Contains(problems[0], ".m-bar is missing") {
			t.Errorf("removing the bar produced %v, wanted one report naming .m-bar", problems)
		}
	})

	// The difference §2.6.E handed to a person: not "there is no Export" but
	// "Import where the frame says Export".
	t.Run("a control with the wrong words is caught", func(t *testing.T) {
		renamed := strings.Replace(biteScreen, `href="/export">Export</a>`, `href="/export">Import</a>`, 1)
		problems := problemsBetween(t, biteFrame, renamed)
		if len(problems) != 1 || !strings.Contains(problems[0], `reads "Import"`) {
			t.Errorf("renaming Export to Import produced %v, wanted one report of the text", problems)
		}
	})

	t.Run("a rail link with the wrong words is caught", func(t *testing.T) {
		renamed := strings.Replace(biteScreen, `>Calendar<span`, `>All calendars<span`, 1)
		problems := problemsBetween(t, biteFrame, renamed)
		if len(problems) != 1 || !strings.Contains(problems[0], `reads "All calendars"`) {
			t.Errorf("renaming a rail link produced %v, wanted one report of the text", problems)
		}
	})

	t.Run("order is compared", func(t *testing.T) {
		swapped := strings.Replace(biteScreen,
			`<a class="m-btn is-primary" href="/new"><svg><use href="#i-plus"/></svg>New contact</a>
        <a class="m-btn" href="/export">Export</a>`,
			`<a class="m-btn" href="/export">Export</a>
        <a class="m-btn is-primary" href="/new"><svg><use href="#i-plus"/></svg>New contact</a>`, 1)
		if problems := problemsBetween(t, biteFrame, swapped); len(problems) == 0 {
			t.Error("the primary action moved after the plain one and the gate said nothing")
		}
	})

	t.Run("a placeholder is part of the control", func(t *testing.T) {
		changed := strings.Replace(biteScreen, `placeholder="Filter the loaded cards"`, `placeholder="Search"`, 1)
		problems := problemsBetween(t, biteFrame, changed)
		if len(problems) != 1 || !strings.Contains(problems[0], `reads "Search"`) {
			t.Errorf("changing the filter placeholder produced %v, wanted one report", problems)
		}
	})

	// And the three the gate must stay quiet about.
	t.Run("repetition is not a difference", func(t *testing.T) {
		frame := skelOf(t, biteFrame)
		screen := skelOf(t, biteScreen)
		var out []string
		diffSkel("/app/contacts", frame, screen, &out)
		if len(out) != 0 {
			t.Errorf("one row in the frame against three in the screen produced %v", out)
		}
	})

	t.Run("a node the screen has and the frame does not is not a difference", func(t *testing.T) {
		// biteScreen already carries an .m-right the frame never draws.
		if problems := problemsBetween(t, biteFrame, biteScreen); len(problems) != 0 {
			t.Errorf("an extra .m-right was reported: %v", problems)
		}
	})

	t.Run("a node marked for another wave is not demanded", func(t *testing.T) {
		// biteFrame's .m-sec carries data-planned="§23.8" and the screen has
		// nothing like it; that is the whole point of the mark.
		if problems := problemsBetween(t, biteFrame, biteScreen); len(problems) != 0 {
			t.Errorf("a data-planned node was demanded: %v", problems)
		}
		demanded := strings.Replace(biteFrame, ` data-planned="§23.8"`, "", 1)
		problems := problemsBetween(t, demanded, biteScreen)
		if len(problems) != 1 || !strings.Contains(problems[0], ".m-sec is missing") {
			t.Errorf("with the mark taken off, the same node produced %v, wanted one report of .m-sec",
				problems)
		}
	})

	// The advice half of the message is what a weak model acts on (§12 О4),
	// so it is checked too rather than assumed.
	t.Run("the report says how to build what is missing", func(t *testing.T) {
		advice := adviceFor(t, "/app/notes > .m-bar is missing")
		if !strings.Contains(advice, `{{template "m-bar"`) || !strings.Contains(advice, "README.md") {
			t.Errorf("advice for a missing .m-bar was %q; it has to name the call and the README", advice)
		}
	})
}

// cutBlock removes everything from the start of open through to the start of
// until, so a whole component call can be taken out of the sample the way a
// careless edit would take it out of a template.
func cutBlock(t *testing.T, source, open, until string) string {
	t.Helper()
	i := strings.Index(source, open)
	j := strings.Index(source, until)
	if i < 0 || j < 0 || j < i {
		t.Fatalf("cutBlock: %q .. %q not found in order", open, until)
	}
	return source[:i] + source[j+len(`</div>
    `):]
}
