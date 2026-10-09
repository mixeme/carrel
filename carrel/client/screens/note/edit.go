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
	saveKey  = "Ctrl+S"
	focus    = "Focus"
	saving   = "Saving · "
	noBytes  = "0 of 0 MB"
	none     = "—"
)

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
// lays them out (C-6.1: the layout is the screen's).
var widths = map[string]w.FieldWidth{
	notebook:     w.FieldFlex2_280,
	"start.date": w.FieldFix150,
	"start.time": w.FieldFix130,
	"categories": w.FieldFlex3_320,
	"related":    w.FieldFlex3_340,
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
			items = append(items, bind.SubmitForm(a, formEdit))
		case a.ID == attachID:
			attach = &a
		default:
			items = append(items, bind.Button(a, ""))
		}
	}
	// The client's buttons of the markup, «⋯», Auto-save, the draft in this
	// window, the width and Focus are the page's script's: an edit's bar draws
	// none of them without it (Docbar.Edit).
	items = append(items,
		w.BarSep{},
		w.Button{Label: bold, Bold: true},
		w.Button{Label: italic, Variant: w.ButtonItalic},
		w.Button{Label: heading2, Tertiary: true},
		w.Button{Label: list, Tertiary: true},
		w.Button{Folds: true, Label: task, Icon: "task", Title: taskName, Tertiary: true},
		w.Button{Icon: "link", Title: link, Tertiary: true},
	)
	if attach != nil {
		// ждёт библиотеку: U-5.5 #43 — the frame draws the file a field of
		// the edit that goes with Save (FileField.List); it goes by its own
		// form, as the contract has it, until Save takes files
		arrow := barItems(attach.Arrow)
		menu := append(bind.MenuItems(arrow, ""), bind.Uploads(arrow, upload)...)
		for _, it := range menu {
			// A new note's file sends the document it is typed in (Submits,
			// FileField.To): the bar stands outside it.
			if m, ok := it.(w.MenuItem); ok && m.File != nil && m.File.To != "" {
				m.File.Form = formEdit
			}
		}
		// ждёт библиотеку: U-5.5 #36 — a new note's From Files and «Upload»
		// send the form they stand in (MenuItem.To), and the bar stands
		// outside the document
		items = append(items, w.Drop{
			Trigger: w.Button{Folds: true, Label: string(attach.Label), Menu: true, Icon: bind.Icon(attach.Icon), Title: string(attach.Label)},
			Menu:    w.Menu{Items: menu, Width: w.MenuWidth212, Pop: w.MenuUnder},
		})
	}
	end := []w.DocbarEnd{w.Check{Parts: []w.CheckPart{w.Text(autoSave)}, Tertiary: true}}
	if !blank(p) {
		end = append(end, w.DraftNote{Text: draft, Secondary: true})
	}
	end = append(end,
		w.Kbd{Keys: saveKey, Secondary: true},
		w.Seg{Options: []w.SegOption{{Label: fullWidth, On: true}, {Label: readWidth}}, Secondary: true},
		w.Button{Folds: true, Label: focus, Icon: "focus", Title: focus},
	)
	bar := w.Docbar{Items: items, More: &w.Button{Icon: "dots", Title: more}, End: end, Edit: true}
	main := []w.MainPart{bar, w.Runbar{Progress: formEdit, Parts: []w.RunbarPart{w.Text(saving), w.Mono{Text: noBytes}}}}
	main = append(main, notices(p)...)
	doc := bind.DocForm(p.Doc.Form, formEdit)
	var title, body w.Area
	if f := p.Doc.Form; f != nil {
		for _, r := range bind.Rows(*f) {
			a := bind.Area(r)
			a.Label = text
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
		fields.Items = append(fields.Items, field(r))
	}
	doc.Items = []w.DocPart{title, fields, body}
	if len(p.Doc.Foreign) > 0 {
		doc.Items = append(doc.Items, w.DocNote{Parts: bind.Inlines(p.Doc.Foreign)})
	}
	return append(main, w.DocLayout{Items: []w.DocLayoutPart{doc, w.Docmeta{Sections: column(p), Danger: danger(p.Meta.Actions)}}})
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
	case r.Name == notebook:
		d := bind.Marks(r, ro)
		d.Menu.Width, d.Menu.Pop = w.MenuWidth290, w.MenuUnder
		f.Parts = []w.FieldPart{d}
	case r.Kind == view.InputSelect && r.Suggest != "":
		f.Parts = append([]w.FieldPart{bind.Related(r)}, bind.Hint(r)...)
	default:
		c := bind.Control(r)
		if in, ok := c.(w.Input); ok && r.Kind == view.InputText && in.Placeholder == "" && in.Value == "" {
			in.Placeholder = none
			c = in
		}
		f.Parts = append([]w.FieldPart{c}, bind.Hint(r)...)
	}
	return f
}
