package bind

import (
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// A page's drop sheet (Typecase's notes 60 and 136): closed until a file is
// dragged over the page, then over all of it, saying what the drop does; the
// file dropped goes into the file field of the form the sheet names, as one
// picked there — the form is sent, or the field shows it until Save. The
// document puts it among the page's own sheets (Shell.Dialogs), as the keys'
// sheet: the frames draw it alone (s-io-1, -20…-23), not on each screen.

// DropSheet is the sheet: an icon, the first line in bold, a line in the
// large face and, when there is one, a note in the small one that wraps
// (s-io-1); form is the ID of the form that takes the file.
func DropSheet(form string, icon w.Icon, lead string, line, note []w.Inline) w.Scrim {
	parts := []w.EmptyPart{w.Aside{Face: w.FaceLarge, Parts: line}}
	if len(note) > 0 {
		parts = append(parts, w.Dim{Parts: note, Size: w.Dim11_5, Variant: w.DimWrap})
	}
	return w.Scrim{
		Content: w.Empty{Icon: icon, Lead: lead, Parts: parts, Variant: w.EmptyDrop},
		Pad:     w.ScrimPad38,
		Closed:  true,
		Drop:    form,
	}
}

// ImportDrop is the drop sheet of a list (ListBar.Drop, s-io-1) and the form
// it sends: the wizard's address and the token, the file under the
// action's name for it — the preview opens on the collection of the list.
// The sheet's words are the server's; icon is the screen's.
func ImportDrop(d view.ImportDrop, icon w.Icon) (w.Scrim, w.PostForm) {
	f := ActionForm(d.Action, d.Action.ID)
	var note []w.Inline
	if d.Note != "" {
		note = []w.Inline{w.Text(d.Note)}
	}
	return DropSheet(f.ID, icon, string(d.Lead), Inlines(d.Line), note), f
}

// PicksFile says whether a menu has an item that picks a file from this
// computer and may be used now: what a dropped file stands in for.
func PicksFile(items []view.MenuItem) bool {
	for _, it := range items {
		if it.File != "" && it.Enabled {
			return true
		}
	}
	return false
}

// AttachDrop is the drop sheet of a record's page (s-io-20; AttachDrop): the
// server's words — «Drop to attach», to the record by its title, where a file
// over the record's limit goes — and the form of Attach «From this
// computer» (UploadForm of its arrow), which sends the file. None where
// Attach picks no file. icon is the screen's.
func AttachDrop(d view.AttachDrop, arrow []view.MenuItem, icon w.Icon) (w.Scrim, bool) {
	form := UploadForm(arrow)
	if form == "" {
		return w.Scrim{}, false
	}
	return DropSheet(form, icon, string(d.Lead), Inlines(d.Line), Inlines(d.Note)), true
}
