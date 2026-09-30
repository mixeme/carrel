package web_test

import (
	"html"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The primary button and the monospaced field (Action.Primary, FormRow.Mono):
// a form a frame draws has its primary button where the frame draws
// «is-primary» on a tc-btn, and a monospaced field where it draws «is-mono»
// on a tc-in — a login at the first start (s-auth-4), a server's address,
// a number, a date, a temporary password. Read off the frame by the label:
// the words on a button, the label of a field. A label the frame draws both
// ways says nothing and is left out. check.py (check 9) holds every fixture
// against its frame the same way; these hold the forms the fixtures do not
// carry — sheets, dialogs, the steps of the pages outside the shell.

var (
	marksButton = regexp.MustCompile(`(?s)<(button|a) class="(tc-btn(?: [^"]*)?)"[^>]*>(.*?)</(?:button|a)>`)
	marksSVG    = regexp.MustCompile(`(?s)<svg.*?</svg>`)
	marksAside  = regexp.MustCompile(`(?s)<span class="tc-dim">.*?</span>`)
)

// frameMarks reads a frame, its caption left out: each button's words to
// whether it is drawn primary, each field's label to whether it is drawn
// monospaced — every way a label is drawn.
func frameMarks(t *testing.T, id string) (buttons, fields map[string]map[bool]bool) {
	t.Helper()
	frame, _, _ := strings.Cut(mockupFrame(t, id), "<figcaption")
	buttons, fields = map[string]map[bool]bool{}, map[string]map[bool]bool{}
	add := func(m map[string]map[bool]bool, label string, on bool) {
		if m[label] == nil {
			m[label] = map[bool]bool{}
		}
		m[label][on] = true
	}
	for _, m := range marksButton.FindAllStringSubmatch(frame, -1) {
		if label := frameText(marksSVG.ReplaceAllString(m[3], "")); label != "" {
			add(buttons, label, strings.Contains(" "+m[2]+" ", " is-primary "))
		}
	}
	for _, at := range caretTag.FindAllStringIndex(frame, -1) {
		tag := frame[at[0]:at[1]]
		c := caretClass.FindStringSubmatch(tag)
		if c == nil || !strings.Contains(" "+c[1]+" ", " tc-in ") {
			continue
		}
		label := ""
		before := frame[:at[0]]
		if i := strings.LastIndex(before, `<div class="tc-field`); i >= 0 {
			if seg := before[i:]; strings.Count(seg, "<div") > strings.Count(seg, "</div>") {
				if l := caretLabel.FindStringSubmatch(seg); l != nil {
					label = frameText(marksAside.ReplaceAllString(l[1], ""))
				}
			}
		}
		if label == "" {
			if a := caretAria.FindStringSubmatch(tag); a != nil {
				label = html.UnescapeString(a[1])
			}
		}
		if label != "" {
			add(fields, label, strings.Contains(" "+c[1]+" ", " is-mono "))
		}
	}
	return buttons, fields
}

// oneWay is how a label is drawn when the frame draws it one way only.
func oneWay(ways map[bool]bool) (on, ok bool) {
	if len(ways) != 1 {
		return false, false
	}
	for on := range ways {
		return on, true
	}
	return false, false
}

// sameMarks holds the buttons and the fields of a form against the frame:
// each button the frame draws is primary exactly when drawn so, each field
// monospaced exactly when drawn so. A form of which the frame draws nothing
// fails: the test that asks has named the wrong frame.
func sameMarks(t *testing.T, id, what string, f view.Form) {
	t.Helper()
	sameActionMarks(t, id, what, f.Actions, formRows(f))
}

// sameActionMarks is sameMarks for the buttons and the fields of a page
// that are not one form: the doors of the pages outside the shell, the cards
// of s-auth-6.
func sameActionMarks(t *testing.T, id, what string, actions []view.Action, rows []view.FormRow) {
	t.Helper()
	buttons, fields := frameMarks(t, id)
	seen := 0
	for _, a := range actions {
		want, ok := oneWay(buttons[said(a.Label)])
		if !ok {
			continue
		}
		seen++
		if a.Primary != want {
			t.Errorf("%s: «%s» (%s) primary %v, want (%s) %v", what, a.Label, a.ID, a.Primary, id, want)
		}
	}
	for _, r := range rows {
		want, ok := oneWay(fields[string(r.Label)])
		if !ok || r.Kind == view.InputHidden {
			continue
		}
		seen++
		if r.Mono != want {
			t.Errorf("%s: field «%s» (%s) monospaced %v, want (%s) %v", what, r.Label, r.Name, r.Mono, id, want)
		}
	}
	if seen == 0 {
		t.Errorf("%s: nothing of it is drawn by %s: read the frame again", what, id)
	}
}

// formRows are the rows of a form, set after set, with the rows an «Add»
// inserts (Template): they are drawn as the rows they become.
func formRows(f view.Form) []view.FormRow {
	var out []view.FormRow
	for _, set := range f.Sets {
		for _, r := range set.Rows {
			out = append(out, r)
			out = append(out, r.Template...)
		}
	}
	return out
}
