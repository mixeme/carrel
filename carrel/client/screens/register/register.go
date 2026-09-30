// Package register is the page of registration, /register, drawn by frame
// «Регистрация и подтверждение» (s-auth-2) of the mockup, and after the form
// is sent by frame «Письмо ушло» (s-auth-3): the drafts frame2go printed for
// them, with the frames' data become the view model's (U-5.1, U-6.14).
package register

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// formID names the registration's form for its buttons, which stand under
// it and outside it (Button.Form): the frame draws them in the form's foot.
const formID = "register"

// Screen draws the page of registration from its view model. Before the form
// is sent: «Create an account» (Title), what happens next (Lede), the warning
// about the password and a refusal of the server — a letter that did not go
// — over the fields (Notices, in their tones), then the form's rows — Login,
// Email, Password, Repeat password — with the cursor where the model says
// (Focus) and a reason under a field that came back with one; «Create
// account» submits the form by name, and beside it «Already have an account?
// Sign in» goes to sign-in (SignIn). After it is sent (Sent, s-auth-3): the
// mark, «Check your email», where the letter went — the address monospaced
// (Lede) — and «Back to sign in» (SignIn).
func Screen(p *view.RegisterPage) w.Page {
	if p.Sent {
		return w.Page{
			Items: []w.PagePart{
				w.Auth{
					Dialog: w.Dialog{
						Items: []w.DialogPart{
							w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo48}, Wide: true},
							w.DialogTitle{Text: string(p.Title), Gap: w.DialogTitleGap6},
							w.Prose{Parts: bind.Inlines(p.Lede), Variant: w.ProseMuted},
							w.FormFoot{Items: []w.FormFootPart{bind.Button(p.SignIn, "")}, Variant: w.FormFootFlush},
						},
						Variant: w.DialogNarrow,
					},
				},
			},
		}
	}
	items := []w.DialogPart{
		w.DialogTitle{Text: string(p.Title), Gap: w.DialogTitleGap4},
		w.DialogSub{Parts: bind.Inlines(p.Lede)},
	}
	for _, n := range p.Notices {
		items = append(items, bind.Msg(n, w.MsgFlushBottom12))
	}
	var fields []w.FormPart
	for _, r := range bind.Rows(p.Form) {
		fields = append(fields, bind.Field(r, w.FieldMD))
	}
	form := bind.Form(p.Form, formID)
	form.Items, form.Variant = fields, w.FormFlushAll
	var foot []w.FormFootPart
	for _, a := range p.Form.Actions {
		foot = append(foot, bind.Button(a, formID))
	}
	foot = append(foot, bind.Span(p.SignIn, w.FaceAccent12))
	items = append(items, form, w.FormFoot{Items: foot, Variant: w.FormFootFlush})
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items:   items,
					Variant: w.DialogPage440,
				},
			},
		},
	}
}
