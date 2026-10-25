package contact

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the card on paper (print-3): the rubric of what Carrel keeps
// as the server sent it, how tags stand in a line.
const (
	paperKept   = "Kept as the server sent them"
	paperTagSep = ", "
)

// Paper draws the card on paper (?print=1, U-10), as frame «Карточка
// контакта на бумаге» (print-3) draws it: the name, where the card is, each
// field a row «name — value» — where the person works and as what first
// (Print.Lead), the tags in one line —, then what Carrel keeps as the server
// sent it, the foot. Nothing is pressed.
func Paper(p *view.ContactPage) w.Page {
	var rows []w.PagePart
	for _, f := range append(append([]view.Field(nil), p.Print.Lead...), p.Fields...) {
		rows = append(rows, paperField(f))
	}
	if len(p.Foreign) > 0 {
		rows = append(rows, w.PrintRubric{Text: paperKept})
		for _, f := range p.Foreign {
			rows = append(rows, paperField(f))
		}
	}
	return bind.Paper(p.Print, rows)
}

// paperField is a field on paper: its name, then its value — the tags'
// names one after another.
func paperField(f view.Field) w.PrintRow {
	value := string(f.Value)
	for i, t := range f.Tags {
		if i > 0 {
			value += paperTagSep
		}
		value += string(t.Label)
	}
	return w.PrintRow{Field: true, Items: []w.PrintRowPart{w.PrintCell{Name: string(f.Label)}, w.PrintCell{Name: value}}}
}
