// Drafted by frame2go from frame «Приглашение · по ссылке» (s-auth-14) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Auth14 is frame «Приглашение · по ссылке» (s-auth-14).
func Auth14() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogTitle{Text: "Join carrel.example", Gap: w.DialogTitleGap4},
						w.DialogSub{Parts: []w.Inline{w.Text("mix invited you. Choose a login and a password.")}},
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
									Parts: []w.FieldPart{w.Input{Value: "anna", Label: "Login", Focus: true}},
									Width: w.FieldMD,
								},
								w.Field{
									Label: "Email",
									Parts: []w.FieldPart{
										w.Input{Placeholder: "you@example.org", Label: "Email"},
										w.FieldHint{Parts: []w.Inline{w.Text("Confirmed by a letter.")}},
									},
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
							Items:   []w.FormFootPart{w.Button{Label: "Create account", Kind: w.ButtonPrimary}},
							Variant: w.FormFootFlush,
						},
					},
					Variant: w.DialogPage440,
				},
			},
		},
	}
}
