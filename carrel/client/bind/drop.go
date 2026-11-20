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
//
// The form's file field says what it takes — the kind's extensions, several
// files at once only where the kind takes them (notes) —, and the sheet
// refuses on its own what it does not take, in the server's words
// (Typecase's note 209, task 265): a file another kind takes, where to
// carry it (Elsewhere); any other, what is imported here (Refused);
// several where one is taken (OneOnly).
func ImportDrop(d view.ImportDrop, icon w.Icon) (w.Scrim, w.PostForm) {
	f := ActionForm(d.Action, d.Action.ID)
	for i, a := range d.Accept {
		if i > 0 {
			f.Accept += ","
		}
		f.Accept += string(a)
	}
	f.Many = d.Many
	return ImportSheet(d, f.ID, icon), f
}

// ImportSheet is the drop sheet of an import that names the form taking
// the file — a list's own (ImportDrop), the wizard's first step, whose
// dialog is the form (ImportSource.Drop) —: the server's words, and its
// refusals of a file the form's field does not take.
func ImportSheet(d view.ImportDrop, form string, icon w.Icon) w.Scrim {
	var note []w.Inline
	if d.Note != "" {
		note = []w.Inline{w.Text(d.Note)}
	}
	sheet := DropSheet(form, icon, string(d.Lead), Inlines(d.Line), note)
	empty := sheet.Content.(w.Empty)
	for _, e := range d.Elsewhere {
		empty.Refusals = append(empty.Refusals, w.Refusal{Of: string(e.Ext), Parts: Inlines(e.Says)})
	}
	if len(d.Refused) > 0 {
		empty.Refusals = append(empty.Refusals, w.Refusal{Of: w.RefuseOther, Parts: Inlines(d.Refused)})
	}
	if d.OneOnly != "" {
		empty.Refusals = append(empty.Refusals, w.Refusal{Of: w.RefuseSeveral, Parts: []w.Inline{w.Text(d.OneOnly)}})
	}
	sheet.Content = empty
	return sheet
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
