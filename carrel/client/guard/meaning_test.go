package guard_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/client/guard"
	"carrel.dev/carrel/client/screens"
	w "carrel.dev/typecase/widget"
)

// meaningFindings is the meaning guard (U-8, U-15.2) on each screen, drawn on
// every fixture of its view model: its tree, whose buttons may name the
// forms the page prints at its end (Screen.Forms), and each of those forms.
func meaningFindings(l lib, all []screens.Screen) []string {
	var out []string
	for _, scr := range all {
		at := label(scr)
		for _, path := range l.fixtures(scr) {
			model, err := decode(scr, path)
			if err != nil {
				out = append(out, fmt.Sprintf("%s: %v", at, err))
				continue
			}
			fx := filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
			var forms []w.PostForm
			if scr.Forms != nil {
				forms = scr.Forms(model)
			}
			var ids []string
			for _, pf := range forms {
				if slices.Contains(ids, pf.ID) {
					out = append(out, fmt.Sprintf("%s on %s: two forms of the page are %q", at, fx, pf.ID))
				}
				ids = append(ids, pf.ID)
				for _, f := range guard.Meaning(pf) {
					out = append(out, fmt.Sprintf("%s on %s: the page's form %s", at, fx, f))
				}
			}
			for _, f := range guard.Meaning(scr.Draw(model), ids...) {
				out = append(out, fmt.Sprintf("%s on %s: %s", at, fx, f))
			}
		}
	}
	return out
}

func TestScreensHaveMeaning(t *testing.T) {
	none(t, "meaning", meaningFindings(load(t), screens.All))
}

// The bite on a screen: the sample of the sign-in page draws the frame and
// no meaning — a button that sends nothing, a link to nowhere, fields in no
// form — and each is named.
func TestMeaningBite(t *testing.T) {
	found := meaningFindings(load(t), []screens.Screen{signIn(nil)})
	for _, want := range []string{
		"(AuthStack).Items[1](Button): a button with nothing to follow or send",
		"(AuthStack).Links[0](Link): no Href",
		"(Field).Parts[0](Input): a field outside a form that sends",
	} {
		if !slices.ContainsFunc(found, func(f string) bool { return strings.Contains(f, want) }) {
			t.Errorf("the meaning guard did not name %q; it found:\n%s", want, strings.Join(found, "\n"))
		}
	}
	// The page's forms (Screen.Forms): a form that goes nowhere and two of
	// one name are named; a button that names one of them is not.
	posting := signIn(nil)
	posting.Forms = func(any) []w.PostForm {
		return []w.PostForm{{ID: "wipe"}, {ID: "wipe", Action: "/forgot"}}
	}
	found = meaningFindings(load(t), []screens.Screen{posting})
	for _, want := range []string{
		`the page's form PostForm: no Action`,
		`two forms of the page are "wipe"`,
	} {
		if !slices.ContainsFunc(found, func(f string) bool { return strings.Contains(f, want) }) {
			t.Errorf("the meaning guard did not name %q; it found:\n%s", want, strings.Join(found, "\n"))
		}
	}
}

// The meaning on trees alone.
func TestMeaningRules(t *testing.T) {
	auth := func(stack w.AuthStack) w.Page {
		return w.Page{Items: []w.PagePart{w.Auth{Dialog: w.Dialog{Items: []w.DialogPart{stack}, Variant: w.DialogNarrow}}}}
	}
	login := func(input w.Input, button w.Button, links ...w.Link) w.Page {
		return auth(w.AuthStack{
			Items: []w.AuthStackPart{
				w.Form{ID: "login", Action: "/login", Fields: []w.Hidden{{Name: "_csrf", Value: "x"}},
					Items: []w.FormPart{w.Field{Label: "Login", Parts: []w.FieldPart{input}}}},
				button,
			},
			Links: links,
		})
	}
	named := w.Input{Name: "login"}
	submit := w.Button{Label: "Sign in", Form: "login"}
	forgot := w.Link{Text: "Forgotten your password?", Href: "/forgot"}
	for name, c := range map[string]struct {
		tree any
		want []string // parts of the findings, one each; none — green
	}{
		"all with meaning":           {login(named, submit, forgot), nil},
		"a disabled button":          {login(named, w.Button{Label: "Sign in", State: w.ButtonDisabled}), nil},
		"a disabled field":           {login(w.Input{Disabled: true}, submit), nil},
		"a button that is a link":    {login(named, w.Button{Label: "Back", Href: "/login"}), nil},
		"a submit inside the form":   {w.Form{Action: "/x", Items: []w.FormPart{w.Field{Parts: []w.FieldPart{named, w.Button{Submit: true}}}}}, nil},
		"a PostForm named":           {w.Form{Items: []w.FormPart{w.Field{Parts: []w.FieldPart{w.Button{Form: "post"}}}}}, nil},
		"words, and words that lead": {w.FormFoot{Items: []w.FormFootPart{w.Span{Text: "or"}, w.Span{Text: "Sign in", Href: "/login"}}}, nil},
		"a button without meaning":   {login(named, w.Button{Label: "Sign in"}), []string{"Items[1](Button): a button with nothing"}},
		"a submit in no form":        {w.Form{Items: []w.FormPart{w.Field{Parts: []w.FieldPart{w.Button{Submit: true}}}}}, []string{"(Button): a submit button outside a form"}},
		"a form nobody has":          {login(named, w.Button{Label: "Sign in", Form: "logon"}), []string{`(Button): submits form "logon", which is not on the page`}},
		"a form without Action":      {auth(w.AuthStack{Items: []w.AuthStackPart{w.Form{ID: "login", Items: []w.FormPart{w.Field{Parts: []w.FieldPart{named}}}}, submit}}), []string{"(Input): a field outside a form", `(Button): submits form "login", which is not on the page`}},
		"a link without Href":        {login(named, submit, w.Link{Text: "Forgotten your password?"}), []string{"Links[0](Link): no Href"}},
		"a field without Name":       {login(w.Input{}, submit), []string{"(Input): a field without Name"}},
		"a cross without Href":       {w.Close{Label: "Close"}, []string{"Close: no Href"}},
		"a control the guard lacks":  {w.List{Rows: []w.ListRow{w.TimeRow{}}}, []string{"(TimeRow): TimeRow carries meaning the guard's contract does not know"}},
	} {
		found := guard.Meaning(c.tree, "post")
		if len(found) != len(c.want) {
			t.Errorf("%s: found %d, want %d:\n%s", name, len(found), len(c.want), strings.Join(found, "\n"))
			continue
		}
		for i, want := range c.want {
			if !strings.Contains(found[i], want) {
				t.Errorf("%s: found %q, want %q", name, found[i], want)
			}
		}
	}
}
