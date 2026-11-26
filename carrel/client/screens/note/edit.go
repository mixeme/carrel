package note

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3) of the edit: the accessible name of the title
// and the text (the frame names both «Text»), the bar's buttons of the
// markup, «⋯», Auto-save, where the draft is kept and the key that saves, the
// focus; what the line of Save that sends files says before the library's
// script moves it (Typecase ac658b0); the dash of an empty line of text
// (s-notes-30: Tags of a new note).
const (
	text     = "Text"
	bold     = "B"
	italic   = "I"
	heading2 = "H2"
	list     = "• list"
	task     = "task"
	taskName = "Task"
	link     = "Link"
	more     = "More"
	autoSave = "Auto-save"
	draft    = "Draft in this window ·"
	saved    = "Saved"
	saveKey  = "Ctrl+S"
	focus    = "Focus"
	saving   = "Saving · "
	noBytes  = "0 of 0 MB"
	none     = "—"
)

// autoSaveName is the name the tick of Auto-save is kept under in the
// browser (Check.AutoSave; Typecase's note 229): one for every notebook — a
// person who wants their notes to save themselves wants it in each.
const autoSaveName = "notes"

// The names of the contract the edit places by (C-6.8, C-7.5): the title and
// the text of the document's form, Save of its bar, and the notebook among
// its fields.
const (
	titleName = "summary"
	textName  = "description"
	saveID    = "record.save"
	notebook  = "notebook"
)

// widths are the widths of the edit's fields by their names, as the frame
// lays them out (C-6.1: the layout is the screen's): the link after the
// tags, as wide as the notebook (s-notes-20; Typecase's task 172); Related
// to on a row of its own, as wide as the column, so that it grows without
// leaving gaps (the owner, 26.11.2026; Typecase's note 235).
var widths = map[string]w.FieldWidth{
	notebook:     w.FieldFlex2_280,
	"start.date": w.FieldFix150,
	"start.time": w.FieldFix130,
	"categories": w.FieldFlex3_320,
	"url":        w.FieldFlex2_280,
	"related":    w.FieldW100,
}

// ro is the badge of a notebook read only in the choice of the notebook.
var ro = w.Badge{Label: "ro", Place: w.Pushed}

// barItems are the items of the bar's Attach (Action.Arrow): the column's
// Attach has the same ones, and each that posts by a form of its own gets
// the bar's form of it, by barSuffix — a file input belongs to one form.
func barItems(arrow []view.MenuItem) []view.MenuItem {
	out := make([]view.MenuItem, len(arrow))
	for i, it := range arrow {
		if it.Method == "POST" && !it.Submits {
			it.ID += barSuffix
		}
		out[i] = it
	}
	return out
}

// The lists a file picked in the edit joins (FileField.List): the column's
// attachments, an edit's and a new note's (s-notes-20, -30).
const (
	listEdit = "note-attachments"
	listNew  = "note-new-attachments"
)

// listOf is the list of the edit's picked files.
func listOf(p *view.NotePage) string {
	if p.Source.UID == "" {
		return listNew
	}
	return listEdit
}

// picked is the row a picked file becomes in that list: its name, its size,
// Remove (s-notes-20).
var picked = w.DocAttach{
	Name:   "{name}",
	Size:   w.Dim{Parts: []w.Inline{w.Text(lead + "{size}")}},
	Remove: &w.Button{Label: "Remove", Kind: w.ButtonDanger, Small: true, Removes: true},
}

// editFiles are the items of an Attach of the edit: «From this computer» a
// field of the document, standing outside it, that goes with its Save
// (FileField.Form, List; Typecase's note 152, task 249); an item that sends
// the document to its own address — a new note's From Files — names it too
// (MenuItem.To with Form, task 247); the rest post by forms of their own.
func editFiles(arrow []view.MenuItem, form, list string) []w.MenuPart {
	out := bind.EditFiles(arrow, list, "size", picked)
	for i, part := range out {
		m, ok := part.(w.MenuItem)
		if !ok {
			continue
		}
		switch {
		case m.File != nil:
			m.File.Form = form
		case m.To != "":
			m.Form = form
		}
		out[i] = m
	}
	return out
}

// columnAttach is the column's Attach, nil where it has none.
func columnAttach(p *view.NotePage) *view.Action {
	for _, a := range p.Meta.Actions {
		if a.ID == attachID {
			return &a
		}
	}
	return nil
}

// edit is the note in its edit (s-notes-20; a new note's — s-notes-30): the
// bar of an edit (Docbar.Edit: without a script only its Save and Cancel
// stand, Typecase's task 104) — Save, which submits
// the document, Cancel, the client's buttons of the markup, Attach with its
// arrow (Typecase's task 126), «⋯», Auto-save, the draft — none for a new
// note with nothing written, which is not kept (Р-41 of Typecase) —, the key,
// the width, Focus —; the line of Save that
// sends files (Runbar.Progress), hidden until the library's live layer moves
// it; the notices; the document, which is the form — the title, the fields (Notebook with the
// notebooks' marks, Date and Time monospaced, Tags, Related to as the record
// chosen), the text, the line of the properties Carrel does not know —; the
// column without the tags and related records, which are fields here.
func edit(p *view.NotePage) []w.MainPart {
	formEdit := formOf(p)
	var items []w.DocbarPart
	var attach *view.Action
	for _, a := range p.Doc.Bar {
		switch {
		case a.ID == saveID:
			save := bind.SubmitForm(a, formEdit)
			save.Key = saveKey // U-7: Save in the editor
			items = append(items, save)
		case a.ID == attachID:
			attach = &a
		default:
			items = append(items, bind.Button(a, ""))
		}
	}
	// The notebook it is saved in, right of Cancel (Typecase 434ac87, note
	// 72): no longer a field under the title.
	for _, r := range p.Doc.Fields {
		if r.Name == notebook {
			items = append(items, bind.SavedIn(r, ro, formEdit))
		}
	}
	// The client's buttons of the markup, «⋯», Auto-save, the draft in this
	// window, the width and Focus are the page's script's: an edit's bar draws
	// none of them without it (Docbar.Edit). With it the markup marks the text
	// (Marks), Auto-save sends the form after a pause in typing with
	// Tc-Autosave (AutoSave), the draft is kept in the window and says when
	// it was saved (Saved), Focus hides the rail and the column (Focuses;
	// Typecase's note 229).
	items = append(items,
		w.BarSep{},
		w.Button{Label: bold, Bold: true, Marks: w.MarkBold},
		w.Button{Label: italic, Variant: w.ButtonItalic, Marks: w.MarkItalic},
		w.Button{Label: heading2, Tertiary: true, Marks: w.MarkHeading},
		w.Button{Label: list, Tertiary: true, Marks: w.MarkList},
		w.Button{Folds: true, Label: task, Icon: "task", Title: taskName, Tertiary: true, Marks: w.MarkTick},
		w.Button{Icon: "link", Title: link, Tertiary: true, Marks: w.MarkLink},
	)
	if attach != nil {
		items = append(items, w.Drop{
			Trigger: w.Button{Folds: true, Label: string(attach.Label), Menu: true, Icon: bind.Icon(attach.Icon), Title: string(attach.Label)},
			Menu:    w.Menu{Items: editFiles(barItems(attach.Arrow), formEdit, listOf(p)), Width: w.MenuWidth212, Pop: w.MenuUnder},
		})
	}
	end := []w.DocbarEnd{w.Check{Parts: []w.CheckPart{w.Text(autoSave)}, Tertiary: true, AutoSave: autoSaveName}}
	if !blank(p) {
		end = append(end, w.DraftNote{Text: draft, Saved: saved, Secondary: true})
	}
	end = append(end,
		w.Kbd{Keys: saveKey, Secondary: true},
		widthSeg(p.Look, false),
		w.Button{Folds: true, Label: focus, Icon: "focus", Title: focus, Focuses: true},
	)
	bar := w.Docbar{Items: items, More: &w.Button{Icon: "dots", Title: more}, End: end, Edit: true}
	main := []w.MainPart{bar, w.Runbar{Progress: formEdit, Parts: []w.RunbarPart{w.Text(saving), w.Mono{Text: noBytes}}}}
	main = append(main, notices(p)...)
	doc := bind.DocForm(p.Doc.Form, formEdit)
	// The files Attach picks in the bar and the column stand outside the
	// document and go with its Save (Doc.Multipart, Typecase's note 152).
	doc.Multipart = attach != nil || columnAttach(p) != nil
	var title, body w.Area
	// A picture pasted into the title or the text joins the attachments, as
	// one dropped on the page (s-io-21; Typecase's task 88).
	paste := ""
	if _, drops := Drop(p); drops {
		paste = formEdit
	}
	if f := p.Doc.Form; f != nil {
		for _, r := range bind.Rows(*f) {
			a := bind.Area(r)
			a.Label = text
			a.Attach = paste
			switch r.Name {
			case titleName:
				a.Variant = w.AreaTitle
				title = a
			case textName:
				a.Variant = w.AreaBody
				body = a
			}
		}
	}
	fields := w.DocFields{}
	for _, r := range p.Doc.Fields {
		if r.Name == notebook {
			continue // in the bar
		}
		fields.Items = append(fields.Items, field(r))
	}
	doc.Items = []w.DocPart{title, fields, body}
	if len(p.Doc.Foreign) > 0 {
		doc.Items = append(doc.Items, w.DocNote{Parts: bind.Inlines(p.Doc.Foreign)})
	}
	return append(main, w.DocLayout{View: layoutOf(p.Look), Items: []w.DocLayoutPart{doc, w.Docmeta{Sections: column(p), Danger: danger(p.Meta.Actions)}}})
}

// blank tells whether the edit is a new note's with nothing written — no
// title and no text: such a note is not kept, and its bar says nothing of a
// draft (Р-41 of Typecase, s-notes-30).
func blank(p *view.NotePage) bool {
	if p.Source.UID != "" || p.Doc.Form == nil {
		return false
	}
	for _, r := range bind.Rows(*p.Doc.Form) {
		if (r.Name == titleName || r.Name == textName) && r.Value != "" {
			return false
		}
	}
	return true
}

// field is a field of the edit by its name's width: the notebook a choice
// with the notebooks' marks (Marks), Related to the record chosen (Related),
// the rest the control of its kind.
func field(r view.FormRow) w.Field {
	f := w.Field{Label: string(r.Label), Width: widths[r.Name]}
	switch {
	case r.Kind == view.InputSelect && r.Suggest != "":
		f.Parts = append([]w.FieldPart{bind.Related(r)}, bind.Hint(r)...)
	default:
		c := bind.Control(r)
		// An empty line shows the dash where the model gives no pattern of
		// its own: the link's is «optional» (s-notes-20).
		if in, ok := c.(w.Input); ok && r.Kind == view.InputText && in.Placeholder == "" && in.Value == "" {
			in.Placeholder = none
			c = in
		}
		f.Parts = append([]w.FieldPart{c}, bind.Hint(r)...)
	}
	return f
}
