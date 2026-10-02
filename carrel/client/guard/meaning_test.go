package guard_test

import (
	"fmt"
	"path/filepath"
	"regexp"
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
// A finding at a control that waits for the library (screens.Wait.Control,
// U-5.5) is excused; a wait that excuses nothing is a finding.
func meaningFindings(l lib, all []screens.Screen) []string {
	var out []string
	used := map[waitKey]bool{}
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
				if i := excuse(scr, f); i >= 0 {
					used[waitKey{at, i}] = true
					continue
				}
				out = append(out, fmt.Sprintf("%s on %s: %s", at, fx, f))
			}
		}
		for i, wt := range scr.Waits {
			if wt.Control != "" && !used[waitKey{at, i}] {
				out = append(out, fmt.Sprintf("%s: the control %s waits (U-5.5 #%d) and has its meaning on every fixture", at, wt.Control, wt.Row))
			}
		}
	}
	return out
}

type waitKey struct {
	screen string
	wait   int
}

// indexes are the places in a path of the meaning guard: «Items[3]».
var indexes = regexp.MustCompile(`\[\d+\]`)

// excuse is the wait of a screen that excuses a finding of the meaning guard
// — a control waiting for the library (screens.Wait.Control) at the end of
// the finding's path, the indexes left out —, or -1.
func excuse(scr screens.Screen, finding string) int {
	path, _, ok := strings.Cut(finding, ": ")
	if !ok {
		return -1
	}
	path = indexes.ReplaceAllString(path, "")
	for i, wt := range scr.Waits {
		if wt.Control != "" && strings.HasSuffix(path, wt.Control) {
			return i
		}
	}
	return -1
}

func TestScreensHaveMeaning(t *testing.T) {
	none(t, "meaning", meaningFindings(load(t), screens.All))
}

// The bite on a screen: the sample of the sign-in page draws the frame and
// no meaning — a button that sends nothing, a link to nowhere, fields in no
// form — and each is named.
func TestMeaningBite(t *testing.T) {
	// A control that waits for the library (U-5.5) is excused where it
	// stands, and only there; a wait at a control with its meaning is red.
	waiting := signIn(nil)
	waiting.Waits = []screens.Wait{{Row: 1, Control: ".Items(AuthStack).Items(Button)"}, {Row: 2, Control: ".Links(Link)"}, {Row: 3, Control: ".Items(Dialog)"}}
	found := meaningFindings(load(t), []screens.Screen{waiting})
	for _, f := range found {
		if strings.Contains(f, "(AuthStack).Items[1](Button)") || strings.Contains(f, "(AuthStack).Links[0](Link)") {
			t.Errorf("a waiting control stayed in the findings: %s", f)
		}
	}
	if !slices.ContainsFunc(found, func(f string) bool { return strings.Contains(f, "(Field).Parts[0](Input): a field outside") }) {
		t.Errorf("a wait excused another control: %q", found)
	}
	if !slices.ContainsFunc(found, func(f string) bool { return strings.Contains(f, ".Items(Dialog) waits (U-5.5 #3) and has its meaning") }) {
		t.Errorf("a wait that excuses nothing stayed green: %q", found)
	}
	found = meaningFindings(load(t), []screens.Screen{signIn(nil)})
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
	picked := w.TickedOn
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
		"a control the guard lacks":  {w.List{Rows: []w.ListRow{w.ExcerptRow{}}}, []string{"(ExcerptRow): ExcerptRow carries meaning the guard's contract does not know"}},
		"rows that lead":             {w.List{Rows: []w.ListRow{w.FaceRow{Href: "/c"}, w.FindRow{Href: "/e"}, w.LinkedRow{Href: "/t"}, w.TimeRow{Href: "/v"}}}, nil},
		"rows that lead nowhere":     {w.List{Rows: []w.ListRow{w.FaceRow{}, w.FindRow{}, w.LinkedRow{}, w.TimeRow{}}}, []string{"(FaceRow): no Href", "(FindRow): no Href", "(LinkedRow): no Href", "(TimeRow): no Href"}},
		// A task's row (Typecase's task 95): its title leads to the record,
		// its tick posts the form of Mark done.
		"a task's row that leads": {w.List{Rows: []w.ListRow{w.DueRow{Href: "/t", Tick: w.Tick{Label: "Done", Send: w.Send{Form: "post"}}}}}, nil},
		"a task's row that leads nowhere": {w.List{Rows: []w.ListRow{w.DueRow{Tick: w.Tick{Label: "Done"}}}},
			[]string{"(DueRow): no Href", "(Tick): a tick box that sends nothing"}},
		// A row of the columns a person chose (s-tasks-13, s-contacts-23):
		// it leads to the record, its tick posts as a task's row's.
		"a row of chosen columns that leads":         {w.List{Rows: []w.ListRow{w.ColumnRow{Href: "/t", Tick: &w.Tick{Label: "Done", Send: w.Send{Form: "post"}}}}}, nil},
		"a row of chosen columns that leads nowhere": {w.List{Rows: []w.ListRow{w.ColumnRow{}}}, []string{"(ColumnRow): no Href"}},
		// Typecase's tasks 60, 63, 67, 68 (the bump to 29413d6).
		"tags that go and tags that are words": {w.RailSec{Tags: []w.Tag{{Label: "работа", Href: "/t"}, {Label: "all"}}}, nil},
		"a closed menu's button opens it":      {w.Acts{Items: []w.ActsPart{w.Drop{Trigger: w.Button{Icon: "dots"}, Menu: w.Menu{Items: []w.MenuPart{w.MenuItem{Label: "Move", Href: "/m"}}}}}}, nil},
		"an open menu's button does not":       {w.Acts{Items: []w.ActsPart{w.Drop{Open: true, Trigger: w.Button{Icon: "dots"}, Menu: w.Menu{Items: []w.MenuPart{w.MenuItem{Label: "Move", Href: "/m"}}}}}}, []string{"(Button): a button with nothing"}},
		"a closed menu with nothing in it":     {w.Acts{Items: []w.ActsPart{w.Drop{Trigger: w.Button{Icon: "dots"}}}}, []string{"(Button): a button with nothing"}},
		"the account's menu":                   {w.Top{Actions: []w.TopAction{w.TopDrop{Trigger: w.UserMenu{Name: "mix"}, Menu: w.Menu{Items: []w.MenuPart{w.MenuItem{Label: "Sign out", Form: "post"}}}}}}, nil},
		"the account's button without a menu":  {w.Top{Actions: []w.TopAction{w.UserMenu{Name: "mix"}}}, []string{"(UserMenu): no Href"}},
		"an item that posts a form nobody has": {w.Menu{Items: []w.MenuPart{w.MenuItem{Label: "Sign out", Form: "logout"}}}, []string{`(MenuItem): submits form "logout", which is not on the page`}},
		"a bar's ticks that are a form":        {w.Bar{Items: []w.BarPart{w.CheckForm{Action: "/c", Checks: []w.Check{{Name: "compact", Value: "1"}}, Apply: w.Button{Submit: true}}}}, nil},
		"a bar's ticks that are no form":       {w.Bar{Items: []w.BarPart{w.CheckForm{Checks: []w.Check{{Name: "compact", Value: "1"}}, Apply: w.Button{Submit: true}}}}, []string{"(CheckForm): no Action", "(Check): a field outside", "(Button): a submit button outside"}},
		"the columns' menu, a form":            {w.Menu{Action: "/cols", Items: []w.MenuPart{w.MenuItem{Label: "Tags", Name: "col", Value: "tags", Tick: &w.TickBox{}}, w.MenuFoot{Items: []w.Button{{Label: "Apply", Submit: true}}}}}, nil},
		"a row's pick that sends nothing":      {w.PhoneRow{Pick: &w.TickBox{}}, []string{"(TickBox): a tick box that sends nothing"}},
		"a table's pick":                       {w.TableRow{Cells: []w.TableCell{{Pick: &picked, Send: w.Send{Form: "post", Name: "uid", Value: "a"}}, {Parts: []w.CellPart{w.Text("x")}}}}, nil},
		"a table's pick that sends nothing":    {w.TableRow{Cells: []w.TableCell{{Pick: &picked}}}, []string{"(TableCell): a tick box that sends nothing"}},
		"the build line":                       {w.RailFoot{About: &w.About{Name: "Carrel"}}, []string{"(About): no Href"}},
		"a tick of the rail's form":            {w.RailSec{Items: []w.RailSecPart{w.TickRow{Send: w.Send{Form: "post", Name: "toggle", Value: "/x"}}}}, nil},
		"a tick of the rail of return":         {w.RailSec{Items: []w.RailSecPart{w.TickRow{}}}, nil},
		"a tick that names no form":            {w.RailSec{Items: []w.RailSecPart{w.TickRow{Send: w.Send{Name: "toggle"}}}}, []string{"(TickRow): a tick box with Send.Name outside a form"}},
		"a switch of links":                    {w.Bar{Items: []w.BarPart{w.Seg{Options: []w.SegOption{{Label: "A–Z", Href: "/a"}, {Label: "Changed", Href: "/c"}}}}}, nil},
		"a switch with an option of no link":   {w.Bar{Items: []w.BarPart{w.Seg{Options: []w.SegOption{{Label: "A–Z", Href: "/a"}, {Label: "Changed"}}}}}, []string{"(Seg): a field without Name"}},
		"a range that sends":                   {w.Bar{Items: []w.BarPart{w.Range{Action: "/t", From: w.Input{Name: "from"}, To: w.Input{Name: "to"}, Show: w.Button{Submit: true}}}}, nil},
		"a range that is no form":              {w.Bar{Items: []w.BarPart{w.Range{From: w.Input{Name: "from"}, To: w.Input{Name: "to"}, Show: w.Button{Submit: true}}}}, []string{"(Range): no Action", "(Input): a field outside", "(Input): a field outside", "(Button): a submit button outside"}},
		"a value that goes, and one not":       {w.Fields{Items: []w.FieldsItem{{Term: "Phone", Href: "tel:+7"}, {Term: "Note"}}}, nil},
		// The calendar (Typecase's task 73): a choice whose menu is the radio
		// items of the form it stands in — the time zone —, its menu's line
		// that finds an option.
		"a closed menu's pick opens it": {w.Form{Action: "/e", Items: []w.FormPart{w.Field{Parts: []w.FieldPart{w.Drop{Trigger: w.Pick{Value: "UTC"},
			Menu: w.Menu{Items: []w.MenuPart{w.Input{Kind: w.InputSearch, Placeholder: "Find a zone"}, w.MenuItem{Label: "UTC", Name: "tz", Value: "UTC", On: true}}}}}}}}, nil},
		"an open menu's pick does not": {w.Form{Action: "/e", Items: []w.FormPart{w.Field{Parts: []w.FieldPart{w.Drop{Open: true, Trigger: w.Pick{Value: "UTC"},
			Menu: w.Menu{Items: []w.MenuPart{w.MenuItem{Label: "UTC", Name: "tz", Value: "UTC"}}}}}}}}, []string{"(Pick): a field without Name"}},
		"a menu's line that finds is no field": {w.Menu{Items: []w.MenuPart{w.Input{Kind: w.InputSearch, Placeholder: "Find a zone"}}}, nil},
		"a line of text in a menu is":          {w.Menu{Items: []w.MenuPart{w.Input{Placeholder: "Find a zone"}}}, []string{"(Input): a field without Name"}},
		"a search line outside a menu is":      {w.Form{Action: "/e", Items: []w.FormPart{w.Field{Parts: []w.FieldPart{w.Input{Kind: w.InputSearch}}}}}, []string{"(Input): a field without Name"}},
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
