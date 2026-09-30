// Package login is the sign-in page, /login, drawn by frame «Вход»
// (s-auth-1) of the mockup: the draft frame2go printed for it, with the
// frame's data become the view model's (U-5.1, U-6.14).
package login

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// formID names the sign-in form for «Sign in», which stands under it and
// outside it (Button.Form): the frame draws the button in the column, not in
// the form.
const formID = "login"

// Screen draws the sign-in page from its view model. The mark stands in place
// of the title — Title is the document's (U-6.14) — and under it the tagline,
// Lede and Tagline. The form sends Login and Password with its hidden fields
// (CSRF) to where the model says; «Sign in» submits it by name, and Enter in a
// field does the same. The cursor stands in the field the model says (Focus:
// Login on an empty sign-in, Password after a refusal); a field with an error
// is marked and says it under itself — «Login or password is wrong.» under
// Password after a refusal (s-auth-13). Under the button, «Create an account»
// while registration is open (Register), then «Forgotten your password?».
func Screen(p *view.LoginPage) w.Page {
	var fields []w.FormPart
	for _, r := range bind.Rows(p.Form) {
		fields = append(fields, bind.Field(r, w.FieldW100))
	}
	var links []w.Link
	if p.Register != nil {
		links = append(links, bind.Link(*p.Register))
	}
	links = append(links, bind.Link(p.Forgot))
	form := bind.Form(p.Form, formID)
	form.Items, form.Variant = fields, w.FormColumnFlush
	items := []w.AuthStackPart{form}
	for _, a := range p.Form.Actions {
		items = append(items, bind.Button(a, formID))
	}
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo60}},
						w.Tagline{
							Lead:     bind.Words(p.Lede),
							Text:     string(p.Tagline),
							Centered: true,
						},
						w.AuthStack{
							Items: items,
							Links: links,
						},
					},
					Variant: w.DialogNarrow,
				},
			},
		},
	}
}
