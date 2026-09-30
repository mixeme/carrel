// Drafted by frame2go from frame «Забыли пароль · логин или почта» (s-auth-5) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Auth5 is frame «Забыли пароль · логин или почта» (s-auth-5).
func Auth5() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogTitle{Text: "Forgotten your password?", Gap: w.DialogTitleGap8},
						w.Prose{
							Parts: []w.Inline{
								w.Text("Enter the login or the confirmed email of the account. There is no way to set a new password and keep the data stored here: the encryption key is derived from the password, and the server never has that key."),
							},
							Variant: w.ProseMutedClose,
						},
						w.Form{
							Items: []w.FormPart{
								w.Field{
									Label: "Login or email",
									Parts: []w.FieldPart{w.Input{Value: "mix", Label: "Login or email", Focus: true}},
									Width: w.FieldW100,
								},
							},
							Variant: w.FormColumnFlush,
						},
						w.FormFoot{
							Items:   []w.FormFootPart{w.Button{Label: "Continue", Kind: w.ButtonPrimary}, w.Button{Label: "Back to sign in"}},
							Variant: w.FormFootFlush,
						},
					},
					Variant: w.DialogPage440,
				},
			},
		},
	}
}
