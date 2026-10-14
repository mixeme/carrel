package guard_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/screens"
	w "carrel.dev/typecase/widget"
)

// Every page a screen prints, on every fixture of its view model, holds the
// names its parts give one another (Typecase's note 68, page.Check): a
// button's form, a label's field, a drop zone's form, a zone a link asks
// for, an id given once. Such a page renders and looks right; what breaks
// is what is pressed.
func TestPagesHoldTheirNames(t *testing.T) {
	none(t, "names", nameFindings(load(t), screens.All))
}

// The open card of the agenda, a task and a note takes a dropped file into
// its Attach (Side.Drop): the check above holds the name only where a page
// prints it, so some page must.
func TestAnOpenCardTakesADroppedFile(t *testing.T) {
	l := load(t)
	dropped := map[string]bool{}
	for _, scr := range screens.All {
		for _, path := range l.fixtures(scr) {
			model, err := decode(scr, path)
			if err != nil {
				t.Fatal(err)
			}
			html, err := document.Of(scr, model, "/").Render()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(html, `data-tc-drop="`) {
				dropped[label(scr)] = true
			}
		}
	}
	if len(dropped) < 3 {
		t.Errorf("screens whose open card takes a dropped file: %v, want the agenda's, the tasks' and the notes'", dropped)
	}
}

// The check bites: a sign-in whose Sign in submits a form the page does not
// print is told.
func TestNamesBite(t *testing.T) {
	l := load(t)
	astray := signIn(func(p *w.Page) {
		stack := p.Items[0].(w.Auth).Dialog.Items[2].(w.AuthStack)
		b := stack.Items[1].(w.Button)
		b.Form = "nowhere"
		stack.Items[1] = b
		auth := p.Items[0].(w.Auth)
		auth.Dialog.Items[2] = stack
		p.Items[0] = auth
	})
	astray.Title = func(any) string { return "Sign in" }
	if found := nameFindings(l, []screens.Screen{astray}); len(found) == 0 {
		t.Error("a button that submits a form the page has not is not told")
	}
}

func nameFindings(l lib, all []screens.Screen) []string {
	var out []string
	for _, scr := range all {
		at := label(scr)
		for _, path := range l.fixtures(scr) {
			fx := filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
			model, err := decode(scr, path)
			if err != nil {
				out = append(out, fmt.Sprintf("%s on %s: %v", at, fx, err))
				continue
			}
			problems, err := document.Of(scr, model, "/").Check()
			if err != nil {
				out = append(out, fmt.Sprintf("%s on %s: %v", at, fx, err))
				continue
			}
			for _, p := range problems {
				out = append(out, fmt.Sprintf("%s on %s: %s", at, fx, p))
			}
		}
	}
	return out
}
