// Package confirm is the page of an address confirmed, /confirm-email/{token},
// drawn by frame «Адрес подтверждён» (rest-2) of the mockup — the route has
// no frame of a whole screen, its state confirmed is this part: the draft
// frame2go printed for it, with the frame's data become the view model's
// (U-5.1, U-6.14).
package confirm

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// Screen draws the page of an address confirmed from its view model: the
// mark, the page's name (Title), what happened (Lede) and one door on — «Sign
// in» (Next). A dead link is not this page: it is DeadLinkPage's, so the page
// is only ever the confirmed one (OK).
func Screen(p *view.ConfirmEmailPage) w.Page {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo48}, Wide: true},
						w.DialogTitle{Text: string(p.Title), Gap: w.DialogTitleGapNormal},
						w.Prose{Parts: bind.Inlines(p.Lede), Variant: w.ProseMuted},
						w.FormFoot{Items: []w.FormFootPart{bind.Button(p.Next, "")}, Variant: w.FormFootFlush},
					},
					Variant: w.DialogNarrow,
				},
			},
		},
	}
}
