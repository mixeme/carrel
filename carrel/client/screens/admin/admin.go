// Package admin is the administration of the instance, /admin/… (U-6.13):
// its screens share the top bar of the application, a rail of their own —
// «Back to app» apart, the sections under «Administration», the state of
// the instance in the foot — and the main column of each (s-admin-1, -7).
package admin

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
		NewKey: "n", SearchKey: "/",
		Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
		Keys:  bind.KeysWords{Label: "Keyboard shortcuts", Icon: "search", Key: "?"},
	}
	rail = bind.RailWords{None: "—", ReadOnly: "ro", Davloom: "davloom"}
)

// The rail's words (U-1.3): the rubric of the sections.
const administration = "Administration"

// back is the ID of the rail's way back to the application (C-7): it stands
// apart from the sections (s-admin-7).
const back = "admin.back"

// shell is an administration screen: the top bar, the rail, the main column.
func shell(t view.TopBar, r view.Rail, main []w.MainPart) w.Shell {
	tb := bind.Top(t, top)
	return w.Shell{Top: &tb, Body: w.ShellBody{Rail: railParts(r), Main: w.Main{Items: main}}}
}

// railParts is the rail of the administration (s-admin-7): «Back to app»
// alone, then «Administration» over its sections, the open one on, each
// with its count where it has one; at the foot the state of the instance —
// each a note with its dot (mail not going out in alarm, a check not run
// muted) and its line under it —, and the name and the build.
func railParts(r view.Rail) []w.RailPart {
	var parts []w.RailPart
	nav := w.Nav{}
	for _, n := range r.Nav {
		if n.ID == back {
			parts = append(parts, w.Nav{Items: []w.NavItem{bind.NavItem(n)}})
			continue
		}
		nav.Items = append(nav.Items, bind.NavItem(n))
		if n.On {
			nav.On = len(nav.Items)
		}
	}
	parts = append(parts, w.RailSec{Items: []w.RailSecPart{w.Rubric{Text: administration}, nav}})
	foot := bind.Foot(r, rail)
	var notes []w.RailFootPart
	for _, s := range r.Status {
		notes = append(notes, status(s))
	}
	foot.Items = append(notes, foot.Items...)
	return append(parts, foot)
}

// status is a note of the rail's foot (C-6.1 RailStatus): its tone as its
// dot, its title, its line muted under it.
func status(s view.RailStatus) w.RailStatus {
	tone := w.RailStatusRunning
	switch s.Tone {
	case view.StatusAlert:
		tone = w.RailStatusAlert
	case view.StatusOff:
		tone = w.RailStatusOff
	}
	return w.RailStatus{Tone: tone, Text: string(s.Title), Detail: w.Dim{Parts: bind.Inlines(s.Line), Size: w.Dim11}}
}

// Forms are the forms of an administration screen's actions that post with
// none of their own: the top bar's and the notices'.
func forms(t view.TopBar, ns []view.Notice) []w.PostForm {
	return append(bind.TopForms(t), bind.NoticeForms(ns)...)
}
