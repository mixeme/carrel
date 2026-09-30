// Drafted by frame2go from frame «Забыли пароль · что можно сделать» (s-auth-6) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Auth6 is frame «Забыли пароль · что можно сделать» (s-auth-6).
func Auth6() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogTitle{Text: "What we can do", Gap: w.DialogTitleGap8},
						w.Prose{
							Parts: []w.Inline{
								w.Text("Account "),
								w.Strong("mix"),
								w.Text(" has a confirmed email. A silent password reset does not exist: a new password would be a new key, and everything stored under the old one would be lost anyway."),
							},
							Variant: w.ProseMuted,
						},
						w.Form{
							Items: []w.FormPart{
								w.Card{
									Items: []w.CardPart{
										w.CardText{Kind: w.CardTerm, Parts: []w.CardTextPart{w.Text("Wipe the account")}},
										w.Prose{
											Parts: []w.Inline{
												w.Text("We send a letter to the confirmed address. The link in it is not a reset: after you confirm, everything stored here is erased — connections become unreadable and are deleted — and the same link then lets you set a new password. Address books, calendars, tasks and notes on the servers stay where they are."),
											},
											Variant: w.ProseMutedClose,
										},
										w.Button{Label: "Send a confirmation letter", Kind: w.ButtonPrimary},
									},
									Variant: w.CardSnug,
								},
								w.Card{
									Items: []w.CardPart{
										w.CardText{Kind: w.CardTerm, Parts: []w.CardTextPart{w.Text("Ask the administrator")}},
										w.Prose{
											Parts: []w.Inline{
												w.Text("This account deposited its encryption key. The administrator can recover it with the instance master password, and your data is kept."),
											},
											Variant: w.ProseMutedClose,
										},
										w.Button{Label: "Ask the administrator"},
									},
									Variant: w.CardSnug,
								},
							},
							Variant: w.FormColumnGap10,
						},
						w.FormFoot{Items: []w.FormFootPart{w.Button{Label: "Back to sign in"}}, Variant: w.FormFootFlush},
					},
					Variant: w.DialogPage520,
				},
			},
		},
	}
}
