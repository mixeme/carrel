package agenda

import (
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// Paper draws the agenda on paper (?print=1, U-10), as frame «Агенда на
// бумаге» (print-1) draws it: the week in the heading, what answered by its
// connection, each day's rubric and its events — the time monospaced, the
// title in bold with where and in which calendar under it, the mark's
// colour said in words (C-10) —, the foot with the days, when it was taken
// and how many sources the snapshot holds. Nothing on it is pressed.
func Paper(p *view.AgendaPage) w.Page {
	heading := p.Print.Heading
	if heading == "" {
		heading = p.Print.Title
	}
	items := []w.PagePart{w.PrintTitle{Text: string(heading)}}
	if p.Print.Sources != "" {
		items = append(items, w.PrintSources{Text: string(p.Print.Sources)})
	}
	for _, d := range p.Days {
		items = append(items, w.PrintRubric{Text: string(d.Rubric)})
		for _, r := range d.Rows {
			detail := string(r.Stamp)
			if r.Location != "" {
				detail = string(r.Location) + paperSep + detail
			}
			items = append(items, w.PrintRow{Items: []w.PrintRowPart{
				w.Mono{Text: string(r.Time), Variant: w.MonoFace},
				w.PrintCell{Name: string(r.Summary), Detail: detail},
			}})
		}
	}
	if p.Print.Foot != "" {
		items = append(items, w.PrintFoot{Text: string(p.Print.Foot)})
	}
	return w.Page{Items: items, Variant: w.PagePrint}
}

// paperSep joins where an event is and its calendar on paper.
const paperSep = " · "
