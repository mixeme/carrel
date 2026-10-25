package folder

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the folder on paper (print-6): its columns, its rubrics, the
// size of a folder.
const (
	paperName    = "Name"
	paperSize    = "Size"
	paperChanged = "Changed"
	paperFolders = "Folders"
	paperFiles   = "Files"
	paperNone    = "—"
)

// Paper draws the folder on paper (?print=1, U-10), as frame «Список
// файлов на бумаге» (print-6) draws it: its path in the heading, the
// storage under it, the row naming the columns, the folders first and the
// files after, each its name, its size and when it changed, the foot.
// Nothing is pressed.
func Paper(p *view.FilesPage) w.Page {
	rows := []w.PagePart{w.PrintRow{Columns: true, Head: true, Items: []w.PrintRowPart{
		w.PrintCell{Name: paperName}, w.PrintNum{Text: paperSize}, w.PrintNum{Text: paperChanged},
	}}}
	for _, dirs := range []bool{true, false} {
		rubric := paperFiles
		if dirs {
			rubric = paperFolders
		}
		first := true
		for _, r := range p.Rows {
			if r.IsDir != dirs {
				continue
			}
			if first {
				rows, first = append(rows, w.PrintRubric{Text: rubric}), false
			}
			size := string(r.Size)
			if r.IsDir || size == "" {
				size = paperNone
			}
			rows = append(rows, w.PrintRow{Columns: true, Items: []w.PrintRowPart{
				w.PrintCell{Name: string(r.Name)}, w.PrintNum{Text: size}, w.PrintNum{Text: string(r.Modified)},
			}})
		}
	}
	return bind.Paper(p.Print, rows)
}
