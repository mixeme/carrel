// Drafted by frame2go from frame «Ссылка больше не работает» (rest-1) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Rest1 is frame «Ссылка больше не работает» (rest-1).
func Rest1() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo48}, Wide: true},
						w.DialogTitle{Text: "This link does not work", Gap: w.DialogTitleGap6},
						w.Prose{
							Parts: []w.Inline{
								w.Text("It may have expired, been used already, or been withdrawn. Ask whoever sent it for a new one."),
							},
							Variant: w.ProseMuted,
						},
						w.FormFoot{Items: []w.FormFootPart{w.Button{Label: "Sign in", Kind: w.ButtonPrimary}}, Variant: w.FormFootFlush},
					},
					Variant: w.DialogNarrow,
				},
			},
		},
	}
}
