package agenda

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// Paper draws the agenda on paper (?print=1, U-10), as frames «Агенда на
// бумаге» (print-1) and «Агенда одного календаря на бумаге» (print-7) draw
// it: the week in the heading, what answered by its connection, each day's
// rubric and its events — the time monospaced, the title in bold with where,
// its badges and in which calendar under it, the mark's colour said in words
// (C-10; print-1, Typecase's note 183); one calendar is named once, under
// the heading —, the foot with the days, when it was taken and how many
// sources the snapshot holds. Nothing on it is pressed.
func Paper(p *view.AgendaPage) w.Page {
	var rows []w.PagePart
	for _, d := range p.Days {
		rows = append(rows, w.PrintRubric{Text: string(d.Rubric)})
		for _, r := range d.Rows {
			var detail []string
			if r.Location != "" {
				detail = append(detail, string(r.Location))
			}
			for _, b := range r.Badges {
				detail = append(detail, string(b.Label))
			}
			if r.Stamp != "" {
				detail = append(detail, string(r.Stamp))
			}
			rows = append(rows, w.PrintRow{Items: []w.PrintRowPart{
				w.Mono{Text: string(r.Time), Variant: w.MonoFace},
				w.PrintCell{Name: string(r.Summary), Detail: joined(detail)},
			}})
		}
	}
	return bind.Paper(p.Print, rows)
}

// joined is the line under a row's title on paper: its parts one after
// another.
func joined(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += paperSep
		}
		out += p
	}
	return out
}

// paperSep joins where an event is and its calendar on paper.
const paperSep = " · "
