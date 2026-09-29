// Drafted by frame2go from frame «Вход» (s-auth-1) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Auth1 is frame «Вход» (s-auth-1).
func Auth1() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo60}},
						w.Tagline{
							Lead:     "One desk. Every server.",
							Text:     "Calendars, contacts, tasks, notes and files — one place.",
							Centered: true,
						},
						w.AuthStack{
							Items: []w.AuthStackPart{
								w.Form{
									Items: []w.FormPart{
										w.Field{
											Label: "Login",
											Parts: []w.FieldPart{w.Input{Value: "mix", Label: "Login", Focus: true}},
											Width: w.FieldW100,
										},
										w.Field{
											Label: "Password",
											Parts: []w.FieldPart{w.Input{Value: "••••••••••", Label: "Password"}},
											Width: w.FieldW100,
										},
									},
									Variant: w.FormColumnFlush,
								},
								w.Button{Label: "Sign in", Kind: w.ButtonPrimary},
							},
							Links: []w.Link{{Text: "Create an account"}, {Text: "Forgotten your password?"}},
						},
					},
					Variant: w.DialogNarrow,
				},
			},
		},
	}
}

