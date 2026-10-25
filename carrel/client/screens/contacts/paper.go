package contacts

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the list on paper (print-2): its columns, what a card in
// several books says of them.
const (
	paperName  = "Name"
	paperPhone = "Phone"
	paperEmail = "Email"
	paperIn    = "in "
	paperBooks = " books"
	paperSep   = " · "
)

// Paper draws the list on paper (?print=1, U-10), as frame «Список
// контактов на бумаге» (print-2) draws it: the heading, the books that
// answered, the row naming the columns, each letter's rubric and its cards
// — the name in bold with where the person works and where the card is
// under it (one book its name, several how many), the phone and the address
// —, the foot. No photographs (the owner, 26.10, Р-80); nothing is pressed.
func Paper(p *view.ContactsListPage) w.Page {
	rows := []w.PagePart{w.PrintRow{Columns: true, Head: true, Items: []w.PrintRowPart{
		w.PrintCell{Name: paperName}, w.PrintNum{Text: paperPhone}, w.PrintNum{Text: paperEmail},
	}}}
	for _, g := range p.Groups {
		rows = append(rows, w.PrintRubric{Text: string(g.Rubric)})
		for _, r := range g.Rows {
			where := string(r.Stamp)
			if len(r.Sources) > 1 {
				where = paperIn + strconv.Itoa(len(r.Sources)) + paperBooks
			}
			detail := where
			if r.Org != "" {
				detail = string(r.Org)
				if where != "" {
					detail += paperSep + where
				}
			}
			rows = append(rows, w.PrintRow{Columns: true, Items: []w.PrintRowPart{
				w.PrintCell{Name: string(r.Name), Detail: detail},
				w.PrintNum{Text: string(r.Phone)}, w.PrintNum{Text: string(r.Email)},
			}})
		}
	}
	return bind.Paper(p.Print, rows)
}
