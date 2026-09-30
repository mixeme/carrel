// Drafted by frame2go from frame «Забыли пароль · письмо отправлено» (s-auth-8) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Auth8 is frame «Забыли пароль · письмо отправлено» (s-auth-8).
func Auth8() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogTitle{Text: "Check your inbox", Gap: w.DialogTitleGap8},
						w.Prose{
							Parts: []w.Inline{
								w.Text("If that account has a confirmed email, a letter is on its way. The link erases everything stored here after you confirm it, and then asks for a new password. It cannot bring your data back."),
							},
							Variant: w.ProseMuted,
						},
						w.FormFoot{Items: []w.FormFootPart{w.Button{Label: "Back to sign in"}}, Variant: w.FormFootFlush},
					},
					Variant: w.DialogPage520,
				},
			},
		},
	}
}
