package note

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

// Drop is the note's drop sheet (Typecase's note 136). On the page
// (s-io-20) a file dropped is attached to the note, as one picked with
// Attach «From this computer»; in the edit (s-io-21) it joins the
// attachments the same way and goes with Save. None where Attach picks no
// file.
func Drop(p *view.NotePage) (w.Scrim, bool) {
	if p.Doc.Mode != view.NoteEditing {
		// The page (s-io-20): the server's words (NotePage.Drop), the
		// form of Attach «From this computer» in the column.
		if p.Drop == nil {
			return w.Scrim{}, false
		}
		for _, a := range p.Meta.Actions {
			if a.ID == attachID {
				return bind.AttachDrop(*p.Drop, a.Arrow, dropIcon)
			}
		}
		return w.Scrim{}, false
	}
	for _, a := range p.Doc.Bar {
		if a.ID == attachID && bind.PicksFile(a.Arrow) {
			return bind.DropSheet(formOf(p), dropIcon, dropLead, []w.Inline{w.Text(dropLine)}, nil), true
		}
	}
	return w.Scrim{}, false
}
