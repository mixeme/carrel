// Drafted by frame2go from frame «Регистрация и подтверждение» (s-auth-2) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Auth2 is frame «Регистрация и подтверждение» (s-auth-2).
func Auth2() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogTitle{Text: "Create an account", Gap: w.DialogTitleGap4},
						w.DialogSub{
							Parts: []w.Inline{
								w.Text("A confirmation link will be sent to the address you give; the account cannot be used until you open it."),
							},
						},
						w.Msg{
							Parts: []w.MsgPart{
								w.Text("Your password is the only key to your own data. Nobody — including this server — can recover it for you."),
							},
							Tone:  w.MsgAlert,
							Space: w.MsgFlushBottom12,
						},
						w.Form{
							Items: []w.FormPart{
								w.Field{
									Label: "Login",
									Parts: []w.FieldPart{w.Input{Value: "viktor", Label: "Login", Focus: true}},
									Width: w.FieldMD,
								},
								w.Field{
									Label: "Email",
									Parts: []w.FieldPart{w.Input{Value: "v@example.org", Label: "Email"}},
									Width: w.FieldMD,
								},
								w.Field{
									Label: "Password",
									Parts: []w.FieldPart{w.Input{Value: "••••••••••••", Label: "Password"}},
									Width: w.FieldMD,
								},
								w.Field{
									Label: "Repeat password",
									Parts: []w.FieldPart{w.Input{Value: "••••••••••••", Label: "Repeat password"}},
									Width: w.FieldMD,
								},
							},
							Variant: w.FormFlushAll,
						},
						w.FormFoot{
							Items: []w.FormFootPart{
								w.Button{Label: "Create account", Kind: w.ButtonPrimary},
								w.Span{Face: w.FaceAccent12, Text: "Already have an account? Sign in"},
							},
							Variant: w.FormFootFlush,
						},
					},
					Variant: w.DialogPage440,
				},
			},
		},
	}
}
