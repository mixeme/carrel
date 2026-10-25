package bind

import (
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// Paper is a page on paper (?print=1, U-10) as the frames «… на бумаге»
// draw it (print-1…-7): the heading — Print.Heading, else the title —, what
// it was printed from, the screen's rows, the foot. Nothing on it is
// pressed.
func Paper(p view.Print, rows []w.PagePart) w.Page {
	heading := p.Heading
	if heading == "" {
		heading = p.Title
	}
	items := []w.PagePart{w.PrintTitle{Text: string(heading)}}
	if p.Sources != "" {
		items = append(items, w.PrintSources{Text: string(p.Sources)})
	}
	items = append(items, rows...)
	if p.Foot != "" {
		items = append(items, w.PrintFoot{Text: string(p.Foot)})
	}
	return w.Page{Items: items, Variant: w.PagePrint}
}
