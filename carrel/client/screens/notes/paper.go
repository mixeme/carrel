package notes

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the list on paper (print-8): its columns, what a note in
// several notebooks says of them.
const (
	paperDate      = "Date"
	paperNote      = "Note"
	paperIn        = "in "
	paperNotebooks = " notebooks"
)

// Paper draws the list on paper (?print=1, U-10), as frame «Список заметок
// на бумаге» (print-8; Typecase's note 159) draws it: the heading, the
// notebooks that answered, the row naming the columns, the rubrics of the
// order on the screen and their notes — the date monospaced on the left,
// the title in bold, under it the excerpt and the tags as the columns show
// them and where the note is (one notebook its name, several how many) —,
// the foot. Nothing is pressed.
func Paper(p *view.NotesPage) w.Page {
	on := map[string]bool{"excerpt": true}
	if c := p.Bar.Columns; c != nil {
		on = map[string]bool{}
		for _, it := range c.Items {
			on[it.ID] = it.On
		}
	}
	rows := []w.PagePart{w.PrintRow{Head: true, Items: []w.PrintRowPart{
		w.Mono{Text: paperDate, Variant: w.MonoFace}, w.PrintCell{Name: paperNote},
	}}}
	for _, g := range p.Groups {
		if g.Rubric != "" {
			rows = append(rows, w.PrintRubric{Text: string(g.Rubric)})
		}
		for _, r := range g.Rows {
			var detail []string
			if on["excerpt"] && r.Excerpt != "" {
				detail = append(detail, string(r.Excerpt))
			}
			if on["tags"] && len(r.Tags) > 0 {
				var tags []string
				for _, t := range r.Tags {
					tags = append(tags, string(t.Label))
				}
				detail = append(detail, paperJoin(tags, joined))
			}
			where := string(r.Stamp)
			if len(r.Sources) > 1 {
				where = paperIn + strconv.Itoa(len(r.Sources)) + paperNotebooks
			}
			if where != "" {
				detail = append(detail, where)
			}
			rows = append(rows, w.PrintRow{Items: []w.PrintRowPart{
				w.Mono{Text: string(r.Date), Variant: w.MonoFace},
				w.PrintCell{Name: string(r.Title), Detail: paperJoin(detail, sep)},
			}})
		}
	}
	return bind.Paper(p.Print, rows)
}

// paperJoin is the words with sep between them.
func paperJoin(words []string, sep string) string {
	out := ""
	for i, word := range words {
		if i > 0 {
			out += sep
		}
		out += word
	}
	return out
}
