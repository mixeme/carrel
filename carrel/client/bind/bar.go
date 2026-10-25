package bind

import (
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The bar of a list (C-6.4 ListBar): its filter, its tick that is a form,
// its choice of columns. The names a form sends a choice by that the
// contract names in its comments and not in a field (col, default) and the
// handle's icon come in with the screen's words (ColumnWords), as the rail's
// do (RailWords).

// Filter is the bar's filter (ListBar.FilterForm) as a filter that sends
// its query: the query's row under its name, what the person typed
// (ListBar.Filter), the list's other choices as its hidden fields (task,
// sort, tag), the screen's words its placeholder and accessible name.
// Without the form the filter sends nothing (Action is empty).
func Filter(b view.ListBar, words string) w.Filter {
	f := w.Filter{
		Query:   w.Query{Value: string(b.Filter), Placeholder: words, Label: words},
		Waiting: b.Filter == "",
	}
	if b.FilterForm == nil {
		return f
	}
	f.Action, f.Hidden = b.FilterForm.Action, Hidden(*b.FilterForm)
	for _, r := range Rows(*b.FilterForm) {
		f.Query.Name = r.Name
	}
	// The cross while it holds a word (Typecase's note 129 p. 2).
	f.Clear = b.FilterForm.Clear
	return f
}

// CheckForm is a tick of the bar that is a form (ListBar.Compact, GET, «Compact
// rows»): each option of its rows a check, ticked where it is selected, its
// hidden rows sent with it; apply is the screen's word for the button that
// sends it without a script.
func CheckForm(f view.Form, apply string) w.CheckForm {
	c := w.CheckForm{Action: f.Action, Hidden: Hidden(f), Apply: w.Button{Label: apply, Small: true, Submit: true}}
	for _, r := range Rows(f) {
		for _, o := range r.Options {
			c.Checks = append(c.Checks, w.Check{On: o.Selected, Name: r.Name, Value: o.Value, Parts: []w.CheckPart{w.Text(o.Label)}})
		}
	}
	return c
}

// Checked tells whether a tick of the bar that is a form (CheckForm) is
// ticked: «Compact rows» on.
func Checked(f *view.Form) bool {
	if f == nil {
		return false
	}
	for _, r := range Rows(*f) {
		for _, o := range r.Options {
			if o.Selected {
				return true
			}
		}
	}
	return false
}

// ColumnWords are what the choice of columns takes from the screen: the
// name its ticked items are sent under (col), the value «Reset to defaults»
// sends (default) — both the contract's (C-6.4 Columns), named in no field —
// and the icon of an item's handle.
type ColumnWords struct {
	Name  string
	Reset string
	Drag  w.Icon
}

// Columns is the bar's button Columns with its menu (C-6.4 Columns), closed:
// the button's words and icon; the menu a form that posts (Form) with its
// hidden rows, under the heading of the button's words, an item for each
// column in its order — its handle, ticked where it is shown, its name, the
// one that stays always (Fixed) with its muted words (Aside) —, and at its
// foot the form's actions: the primary one sends the ticks (Apply), the other
// sends the reset under the items' name. The heading, the faint handle of
// the fixed item and its muted words pushed to the end are how every frame
// of the menu draws it (components-42, s-contacts-25); where the menu pops
// and how wide it is are the screen's (Menu.Pop, Menu.Width).
func Columns(c view.Columns, words ColumnWords) w.Drop {
	items := []w.MenuPart{w.MenuHeading{Rubric: w.Rubric{Text: string(c.Label), Form: w.RubricItem}, Form: w.MenuHeadingRoomy}}
	for _, it := range c.Items {
		m := w.MenuItem{Icon: words.Drag, Faint: it.Fixed, Tick: &w.TickBox{}, Label: string(it.Label), Name: words.Name, Value: it.ID, Fixed: it.Fixed}
		if !it.On {
			m.Tick.Ticked = w.TickedOff
		}
		if it.Aside != "" {
			m.Trail = w.Dim{Parts: []w.Inline{w.Text(it.Aside)}, Size: w.Dim10_5, Place: w.Pushed}
		}
		items = append(items, m)
	}
	var foot []w.Button
	for _, a := range c.Form.Actions {
		b := Submit(a)
		b.Small = true
		if !a.Primary {
			b.Name, b.Value = words.Name, words.Reset
		}
		foot = append(foot, b)
	}
	items = append(items, w.MenuFoot{Items: foot})
	return w.Drop{
		Trigger: w.Button{Label: string(c.Label), Icon: Icon(c.Icon)},
		Menu:    w.Menu{Items: items, Action: c.Form.Action, Hidden: Hidden(c.Form)},
	}
}
