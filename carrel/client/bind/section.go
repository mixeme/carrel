package bind

import (
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// BackRail is the rail of a part of Carrel that is not the application's —
// the settings (s-publish-1), the administration (s-admin-7) —: the way
// back to the application alone (the row of the ID back), then the rubric
// over the part's pages, the open one on, each with its count where it has
// one — a page without an icon is davloom's, its emblem (davloom, the
// sprite's symbol) —; at the foot the state of things, a note each (Status),
// and the name and the build.
func BackRail(r view.Rail, back, rubric, davloom string) []w.RailPart {
	var parts []w.RailPart
	nav := w.Nav{}
	for _, n := range r.Nav {
		it := NavItem(n)
		if n.ID == back {
			// «Sections» beside it: on a phone the bottom bar holds «← App»,
			// the current section and the button that opens the rail with
			// all of them (Typecase's note 222, task 274).
			parts = append(parts, w.Nav{Items: []w.NavItem{it}, Sections: true})
			continue
		}
		if n.Icon == "" {
			it.Emblem = &w.Emblem{Symbol: davloom}
		}
		nav.Items = append(nav.Items, it)
		if n.On {
			nav.On = len(nav.Items)
		}
	}
	parts = append(parts, w.RailSec{Items: []w.RailSecPart{w.Rubric{Text: rubric}, nav}})
	foot := w.RailFoot{About: &w.About{Name: string(r.About.Label), Build: w.Mono{Text: string(r.About.Build)}, Href: r.About.Href}}
	for _, s := range r.Status {
		foot.Items = append(foot.Items, Status(s))
	}
	return append(parts, foot)
}

// Status is a note of the rail's foot (C-6.1 RailStatus): its tone as its
// dot — what runs in the accent, what needs care in alarm, what is off or
// not checked muted —, its title, its line muted under it, a word with a
// hint marked (verified, s-publish-1).
func Status(s view.RailStatus) w.RailStatus {
	tone := w.RailStatusRunning
	switch s.Tone {
	case view.StatusAlert:
		tone = w.RailStatusAlert
	case view.StatusOff:
		tone = w.RailStatusOff
	}
	var line []w.Inline
	for _, run := range s.Line {
		if run.Hint != "" {
			line = append(line, w.Hint{Text: string(run.Text), Title: string(run.Hint)})
			continue
		}
		line = append(line, Inlines(view.Runs{run})...)
	}
	return w.RailStatus{Tone: tone, Text: string(s.Title), Detail: w.Dim{Parts: line, Size: w.Dim11}}
}
