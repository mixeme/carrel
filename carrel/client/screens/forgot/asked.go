// Drafted by frame2go from frame «Забыли пароль · запрос администратору» (s-auth-9) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Auth9 is frame «Забыли пароль · запрос администратору» (s-auth-9).
func Auth9() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogTitle{Text: "The administrator has been asked", Gap: w.DialogTitleGap8},
						w.Prose{
							Parts: []w.Inline{
								w.Text("A recovery request is recorded for this account. The administrator still has to use the instance master password. Until then, the sign-in screen will not accept a new password."),
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
