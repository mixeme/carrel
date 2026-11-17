package event

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// dropIcon is the icon of the page's drop sheet (s-io-20).
const dropIcon w.Icon = "clip"

// Drop is the page's drop sheet (s-io-20; Typecase's note 136): a file
// dropped on the page is attached to the event as one picked with Attach
// «From this computer»; the words are the server's (EventPage.Drop).
func Drop(p *view.EventPage) (w.Scrim, bool) {
	if p.Drop == nil {
		return w.Scrim{}, false
	}
	for _, a := range p.Actions {
		if a.ID == attachID {
			return bind.AttachDrop(*p.Drop, a.Arrow, dropIcon)
		}
	}
	return w.Scrim{}, false
}
