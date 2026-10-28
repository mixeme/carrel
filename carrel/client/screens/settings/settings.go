// Package settings is the person's settings, /app/settings/… (U-6.12): its
// pages share the top bar of the application, the settings' rail — «Back
// to app» alone, «Settings» over its pages, the state of things at the foot
// (bind.BackRail) — and the main column of each (appearance-1, account-1,
// s-profile-1, …).
package settings

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

var top = bind.TopWords{
	Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
	Search: "Search everything…",
	NewKey: "n", SearchKey: "/",
	Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
	Keys:  bind.KeysWords{Label: "Keyboard shortcuts", Icon: "search", Key: "?", Sheet: "keys"},
}

// The words of the settings (U-1.3): the rubric of the rail, the emblem of
// davloom.
const (
	rubric  = "Settings"
	davloom = "davloom"
)

// back is the ID of the rail's way back to the application (C-7).
const back = "settings.back"

// shell is a settings page: the top bar, the settings' rail, the main
// column.
func shell(t view.TopBar, r view.Rail, main []w.MainPart) w.Shell {
	tb := bind.Top(t, top)
	return w.Shell{Top: &tb, Body: w.ShellBody{Rail: bind.BackRail(r, back, rubric, davloom), Main: w.Main{Items: main}}}
}

// head is a page's head: its title and the line under it.
func head(title, sub view.Text) w.Head {
	h := w.Head{Title: &w.HeadTitle{Text: string(title)}}
	if sub != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(sub)}}}
	}
	return h
}

// formActs are the head's Discard and Save of a page that is one form: the
// first a link, the second submitting the form of the ID.
func formActs(f view.Form, id string) *w.Acts {
	var acts []w.ActsPart
	for _, a := range f.Actions {
		if a.Method == "GET" {
			acts = append(acts, bind.Button(a, ""))
			continue
		}
		acts = append(acts, bind.SubmitForm(a, id))
	}
	return &w.Acts{Items: acts}
}

// segField is a choice of a few options as segments (InputRadio), its words
// under it.
func segField(r view.FormRow) w.Field {
	seg := w.Seg{Name: r.Name, Variant: w.SegStart}
	for _, o := range r.Options {
		seg.Options = append(seg.Options, w.SegOption{Label: string(o.Label), On: o.Selected, Value: o.Value})
	}
	return w.Field{Parts: append([]w.FieldPart{seg}, bind.Column(bind.Hint(r))...)}
}

// hintField is what a group of fields means, on a line of its own, in the
// page's reading column (Typecase's note 141).
func hintField(t view.Text) w.Field {
	return w.Field{Parts: []w.FieldPart{w.FieldHint{Parts: []w.Inline{w.Text(t)}, Variant: w.HintColumn}}, Place: w.Grown}
}

// forms are the forms of a settings page's actions that post with none of
// their own: the top bar's and the notices'.
func forms(t view.TopBar, ns []view.Notice) []w.PostForm {
	return append(bind.TopForms(t), bind.NoticeForms(ns)...)
}
