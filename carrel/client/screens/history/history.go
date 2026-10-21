// Package history is the history of an account's password by its permanent
// link, /password-history/{token}, drawn by frame «История пароля»
// (s-auth-15) of the mockup: the draft frame2go printed for it, with the
// frame's data become the view model's (U-5.1, U-6.14). No sign-in: the page
// says only what was done to the password, when and by whom.
package history

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The page's words (U-1.3): the columns.
const (
	colWhen = "When"
	colWhat = "What"
	colWho  = "Who"
)

// Screen draws the history from its view model: whose (Title), the zone its
// times are in (Lede), and a row a thing done — the moment monospaced, what,
// by whom —, newest first; an empty history says so (Empty).
func Screen(p *view.PasswordHistoryPage) w.Page {
	items := []w.DialogPart{
		w.DialogTitle{Text: string(p.Title), Gap: w.DialogTitleGap8},
		w.Prose{Parts: bind.Inlines(p.Lede), Variant: w.ProseMutedClose},
	}
	if len(p.Rows) > 0 {
		t := w.Table{Cols: []w.TableCol{{Label: colWhen}, {Label: colWhat}, {Label: colWho}}}
		for _, r := range p.Rows {
			t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{
				{Parts: []w.CellPart{w.Text(r.When)}, Mono: true},
				{Parts: []w.CellPart{w.Text(r.What)}},
				{Parts: []w.CellPart{w.Text(r.Who)}},
			}})
		}
		items = append(items, t)
	}
	if p.Empty != "" {
		items = append(items, w.Prose{Parts: []w.Inline{w.Text(p.Empty)}, Variant: w.ProseMuted})
	}
	return w.Page{Items: []w.PagePart{w.Auth{Dialog: w.Dialog{Items: items, Variant: w.DialogPage520}}}}
}
