package guard_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/guard"
	"carrel.dev/carrel/client/screens"
	w "carrel.dev/typecase/widget"
)

// The sheet of the keys (U-7; Typecase's note 134) has no screen of its own:
// the document puts it, closed, on a page of the shell whose account's menu
// opens it. Opened, it is the sheet of account-6; its cross and Close are
// the script's (a page's own sheet), the rest is held as anywhere.
func TestKeysSheet(t *testing.T) {
	l := load(t)
	open := document.Keys
	open.Closed = false
	none(t, "the keys' sheet against account-6", sheetFindings(l, "account-6", "the keys' sheet", open.Render()))
	none(t, "the keys' sheet's classes", classFindings(l, "the keys' sheet", open.Render()))
	none(t, "the keys' sheet's meaning", guard.Meaning(w.Shell{Dialogs: []w.Scrim{document.Keys}}))
}

// Every page of the shell whose account's menu has «Keyboard shortcuts»
// carries the sheet it opens, once; a page without the item carries none.
func TestKeysSheetOnThePage(t *testing.T) {
	l := load(t)
	with, without := 0, 0
	for _, scr := range screens.All {
		for _, path := range l.fixtures(scr) {
			model, err := decode(scr, path)
			if err != nil {
				t.Fatalf("%s on %s: %v", label(scr), key(path), err)
			}
			html := document.Root(scr, model).Render()
			item := strings.Contains(html, `href="#keys"`)
			sheets := strings.Count(html, document.Keys.Render())
			switch {
			case item && sheets != 1:
				t.Errorf("%s on %s: the item of the keys and %d sheets of them", label(scr), key(path), sheets)
			case !item && sheets != 0:
				t.Errorf("%s on %s: a sheet of the keys nothing opens", label(scr), key(path))
			case item:
				with++
			default:
				without++
			}
		}
	}
	if with == 0 || without == 0 {
		t.Fatalf("pages with the keys' item %d, without %d: the samples moved", with, without)
	}
}
