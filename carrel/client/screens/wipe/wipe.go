// Drafted by frame2go from frame «Забыли пароль · подтверждение очистки» (s-auth-10) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Auth10 is frame «Забыли пароль · подтверждение очистки» (s-auth-10).
func Auth10() interface{ Render() string } {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogTitle{Text: "Erase everything and start again?", Gap: w.DialogTitleGap8},
						w.Prose{
							Parts: []w.Inline{
								w.Text("This is not a password recovery. The key to your data comes from the password you forgot, so nothing stored under it can be read again — by you, by us, by anyone.\n\nConfirming erases what Carrel keeps encrypted for "),
								w.Strong("mix"),
								w.Text(": saved connections and their passwords, subscriptions, published links, devices, external backup triggers and the deposited copy of the key. Address books, calendars, tasks and notes on the servers are not touched — you connect to them again afterwards.\n\nThe account itself stays: same login, role and confirmed address, the same first day of the week and clock. Only the password is new, and you set it on the next screen."),
							},
							Variant: w.ProseMuted,
						},
						w.FormFoot{
							Items: []w.FormFootPart{
								w.Button{Label: "Erase and set a new password", Kind: w.ButtonDangerSolid},
								w.Button{Label: "Cancel"},
							},
							Variant: w.FormFootFlush,
						},
					},
					Variant: w.DialogPage520,
				},
			},
		},
	}
}
