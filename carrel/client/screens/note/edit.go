package note

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3) of the edit: the accessible name of the title
// and the text (the frame names both «Text»), the bar's buttons of the
// markup, «⋯», Auto-save, where the draft is kept and the key that saves, the
// focus.
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

// edit is the note in its edit (s-notes-20): the bar — Save, which submits
// the document, Cancel, the client's buttons of the markup, Attach, «⋯»,
// Auto-save, the draft, the key, the width, Focus —; the notices; the
// document, which is the form — the title, the fields (Notebook with the
// notebooks' marks, Date and Time monospaced, Tags, Related to as the record
// chosen), the text, the line of the properties Carrel does not know —; the
// column without the tags and related records, which are fields here.
func edit(p *view.NotePage) []w.MainPart {
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
	// ждёт библиотеку: U-5.5 #41 — the client's buttons of the markup, «⋯»,
	// Auto-save, the draft in this window, the width and Focus do nothing
	// without a script, and the bar's Attach has no menu there (Typecase's
	// task 104)
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
		items = append(items, w.Button{Folds: true, Label: string(attach.Label), Menu: true, Icon: bind.Icon(attach.Icon), Title: string(attach.Label)})
	}
	bar := w.Docbar{
		Items: items,
		More:  &w.Button{Icon: "dots", Title: more},
		End: []w.DocbarEnd{
			w.Check{Parts: []w.CheckPart{w.Text(autoSave)}, Tertiary: true},
			w.DraftNote{Text: draft, Secondary: true},
			w.Kbd{Keys: saveKey, Secondary: true},
			w.Seg{Options: []w.SegOption{{Label: fullWidth, On: true}, {Label: readWidth}}, Secondary: true},
			w.Button{Folds: true, Label: focus, Icon: "focus", Title: focus},
		},
	}
	main := []w.MainPart{bar}
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
	if p.Doc.Foreign != "" {
		doc.Items = append(doc.Items, w.DocNote{Parts: []w.Inline{w.Text(p.Doc.Foreign)}})
	}
	return append(main, w.DocLayout{Items: []w.DocLayoutPart{doc, w.Docmeta{Sections: column(p), Danger: danger(p.Meta.Actions)}}})
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
		f.Parts = append([]w.FieldPart{bind.Control(r)}, bind.Hint(r)...)
	}
	return f
}
