package tasks

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the list on paper (print-4): its columns.
const (
	paperDue  = "Due"
	paperTask = "Task"
)

// Paper draws the list on paper (?print=1, U-10), as frame «Задачи на
// бумаге» (print-4) draws it: the filter in the heading, the lists that
// answered, the row naming the columns, the filter's rubrics and its tasks
// — the day due monospaced on the left, the box (with its check when done)
// and the title in bold, the time due, where it lies and how much is done
// or how late it is under it —, the foot. Nothing is pressed.
func Paper(p *view.TasksPage) w.Page {
	rows := []w.PagePart{w.PrintRow{Head: true, Items: []w.PrintRowPart{
		w.Mono{Text: paperDue, Variant: w.MonoFace}, w.PrintCell{Name: paperTask},
	}}}
	for _, g := range p.Groups {
		rows = append(rows, w.PrintRubric{Text: string(g.Rubric)})
		for _, r := range g.Rows {
			tick := w.PrintTickOpen
			if r.Done {
				tick = w.PrintTickDone
			}
			rows = append(rows, w.PrintRow{Items: []w.PrintRowPart{
				w.Mono{Text: string(r.PaperDue), Variant: w.MonoFace},
				w.PrintCell{Name: string(r.Summary), Detail: string(r.PaperLine), Tick: tick},
			}})
		}
	}
	return bind.Paper(p.Print, rows)
}
