package contactedit

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the card's drop sheet (s-io-22).
const (
	dropIcon w.Icon = "img"
	dropLead        = "Drop a photo"
	dropLine        = "It replaces the photo when you save"
)

// Drop is the card's drop sheet (s-io-22; Typecase's note 136): a picture
// dropped on the page stands in the photo's field as one chosen there,
// shown at once, and goes with Save. None where the photo cannot be chosen.
func Drop(p *view.ContactEditPage) (w.Scrim, bool) {
	for _, a := range p.Photo.Actions {
		if a.File != "" && a.Enabled {
			return bind.DropSheet(formOf(p), dropIcon, dropLead, []w.Inline{w.Text(dropLine)}, nil), true
		}
	}
	return w.Scrim{}, false
}
