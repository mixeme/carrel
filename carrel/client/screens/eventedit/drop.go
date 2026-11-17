package eventedit

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the edit's drop sheet (s-io-21).
const (
	dropIcon w.Icon = "clip"
	dropLead        = "Drop to attach"
	dropLine        = "Attached when you save"
)

// Drop is the edit's drop sheet (s-io-21; Typecase's note 136): a file
// dropped on the page joins the attachments as one picked «From this
// computer» and goes with Save. None where Attach picks no file.
func Drop(p *view.EventEditPage) (w.Scrim, bool) {
	for _, a := range p.Form.Actions {
		if a.ID == attachID && bind.PicksFile(a.Arrow) {
			return bind.DropSheet(formID(p), dropIcon, dropLead, []w.Inline{w.Text(dropLine)}, nil), true
		}
	}
	return w.Scrim{}, false
}
