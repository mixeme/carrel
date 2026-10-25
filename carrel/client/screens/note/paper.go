package note

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the note on paper (print-5): the rubric of its files, their
// columns, where a file written into the note lies.
const (
	paperAttach = "Attachments"
	paperName   = "Name"
	paperSize   = "Size"
	paperWhere  = "Where"
	paperInNote = "in the note"
)

// Paper draws the note on paper (?print=1, U-10), as frame «Заметка на
// бумаге» (print-5) draws it, in a reader's width: its title, where it lies
// and when it was written, the text read — a heading of the text a heading,
// a task a box, struck when done —, its files with the row naming the
// columns and where each lies, the foot. Nothing is pressed.
func Paper(p *view.NotePage) w.Page {
	rows := paperBlocks(p.Doc.Rendered)
	if len(p.Meta.Attach) > 0 {
		rows = append(rows, w.PrintRubric{Text: paperAttach}, w.PrintRow{Columns: true, Head: true, Items: []w.PrintRowPart{
			w.PrintCell{Name: paperName}, w.PrintNum{Text: paperSize}, w.PrintNum{Text: paperWhere},
		}})
		for _, a := range p.Meta.Attach {
			where := string(a.Place)
			if a.Inline {
				where = paperInNote
			}
			rows = append(rows, w.PrintRow{Columns: true, Items: []w.PrintRowPart{
				w.PrintCell{Name: string(a.Name)}, w.PrintNum{Text: string(a.Size)}, w.PrintNum{Text: where},
			}})
		}
	}
	return bind.Paper(p.Print, rows)
}

// paperBlocks are the blocks of the text on paper: a heading, a paragraph
// or a line of a list by its words, a task by its box.
func paperBlocks(blocks []view.Block) []w.PagePart {
	var out []w.PagePart
	for _, b := range blocks {
		switch b.Kind {
		case view.BlockHeading:
			out = append(out, w.PrintText{Text: plain(b), Heading: true})
		case view.BlockList:
			out = append(out, paperBlocks(b.Children)...)
		case view.BlockTask:
			tick := w.PrintTickOpen
			if b.Checked {
				tick = w.PrintTickDone
			}
			out = append(out, w.PrintText{Text: plain(b), Tick: tick})
		default:
			out = append(out, w.PrintText{Text: plain(b)})
		}
	}
	return out
}

// plain is the words of a block, its children's after its own.
func plain(b view.Block) string {
	out := string(b.Text)
	for _, c := range b.Children {
		out += plain(c)
	}
	return out
}
