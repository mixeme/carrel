// Drafted by frame2go from frame «Письмо ушло» (s-auth-3) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Auth3 is frame «Письмо ушло» (s-auth-3).
func Auth3() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo48}, Wide: true},
						w.DialogTitle{Text: "Check your email", Gap: w.DialogTitleGap6},
						w.Prose{
							Parts: []w.Inline{
								w.Text("The letter went to "),
								w.Mono{Text: "v@example.org"},
								w.Text(". The account cannot be used until you open the link in it.\n\nSending again with the same login and address, while the account is unconfirmed, simply replaces the letter and the password."),
							},
							Variant: w.ProseMuted,
						},
						w.FormFoot{Items: []w.FormFootPart{w.Button{Label: "Back to sign in"}}, Variant: w.FormFootFlush},
					},
					Variant: w.DialogNarrow,
				},
			},
		},
	}
}
