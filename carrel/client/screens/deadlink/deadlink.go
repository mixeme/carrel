// Package deadlink is the page of a dead secret link — any of
// /invite/{token}, /forgot/{token}, /confirm-email/{token}, /p/{token} whose
// secret is wrong, used, withdrawn, expired or never was — drawn by frame
// «Ссылка больше не работает» (rest-1) of the mockup: the draft frame2go
// printed for it, with the frame's data become the view model's (U-5.1,
// U-6.14).
package deadlink

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// Screen draws the page of a dead link from its view model: the mark, «This
// link does not work» (Title), why that happens (Lede) and the one door —
// «Sign in» (SignIn). One page for every kind of link: what the link was, and
// whether it ever was, the page does not say (C-11), and neither does the
// model.
func Screen(p *view.DeadLinkPage) w.Page {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo48}, Wide: true},
						w.DialogTitle{Text: string(p.Title), Gap: w.DialogTitleGap6},
						w.Prose{Parts: bind.Inlines(p.Lede), Variant: w.ProseMuted},
						w.FormFoot{Items: []w.FormFootPart{bind.Button(p.SignIn, "")}, Variant: w.FormFootFlush},
					},
					Variant: w.DialogNarrow,
				},
			},
		},
	}
}
