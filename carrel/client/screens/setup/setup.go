// Drafted by frame2go from frame «Первый запуск» (s-auth-4) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Auth4 is frame «Первый запуск» (s-auth-4).
func Auth4() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogTitle{Text: "Set up Carrel", Gap: w.DialogTitleGap4},
						w.DialogSub{
							Parts: []w.Inline{w.Text("The account you create here is the first one, and it can manage everyone else.")},
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
									Parts: []w.FieldPart{w.Input{Placeholder: "a-z 0-9 . _ -", Label: "Login", Mono: true, Focus: true}},
									Width: w.FieldMD,
								},
								w.Field{
									Label:      "Email",
									LabelAside: w.Dim{Parts: []w.Inline{w.Text("(optional)")}},
									Parts:      []w.FieldPart{w.Input{Placeholder: "—", Label: "Email"}},
									Width:      w.FieldMD,
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
							Items:   []w.FormFootPart{w.Button{Label: "Create administrator", Kind: w.ButtonPrimary}},
							Variant: w.FormFootFlush,
						},
					},
					Variant: w.DialogPage440,
				},
			},
		},
	}
}
