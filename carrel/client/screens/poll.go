package screens

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/screens/agenda"
	"carrel.dev/carrel/client/screens/contacts"
	"carrel.dev/carrel/client/screens/notes"
	"carrel.dev/carrel/client/screens/search"
	"carrel.dev/carrel/client/screens/tasks"
	"carrel.dev/carrel/client/screens/timeline"
	"carrel.dev/carrel/server/view"
	"carrel.dev/typecase/widget"
)

// PollBar is a poll's bar drawn alone, as the page it stands on draws it
// (the stream of a run and its fallback bring it, C-7.4): a search's pass —
// it says itself short too (PollBar.Short) — as the search draws it, the
// others as the lists do; closed when none stands, the place the next
// answer goes (Typecase's note 124).
func PollBar(p *view.PollBar) widget.Runbar {
	if p != nil && p.Short != "" {
		if bar, ok := search.PollBar(p); ok {
			return bar
		}
	}
	if bar := bind.Runbar(p); bar != nil {
		return *bar
	}
	return widget.Runbar{Closed: true}
}

// RailOf is a rail drawn alone by the screen of the page it is of, and the
// form its ticks send: the end of a poll brings the rail of the page the
// poll stood on (C-7.4). The rail says whose it is — a person's timeline by
// its way back (nav.back), a list by its section open (NavRow.On), the
// search by none.
func RailOf(r view.Rail) ([]widget.RailPart, *widget.PostForm) {
	for _, n := range r.Nav {
		if n.ID == "nav.back" {
			return timeline.Rail(r)
		}
	}
	for _, n := range r.Nav {
		if !n.On {
			continue
		}
		switch n.ID {
		case "nav.contacts":
			return contacts.Rail(r)
		case "nav.calendar":
			return agenda.Rail(r)
		case "nav.tasks":
			return tasks.Rail(r)
		case "nav.notes":
			return notes.Rail(r)
		}
	}
	return search.Rail(r)
}
