// Package login is the sign-in page, /login, drawn by frame «Вход»
// (s-auth-1) of the mockup: the draft frame2go printed for it, with the
// frame's data become the view model's (U-5.1, U-6.14).
package login

import (
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
	var hidden []w.Hidden
	var fields []w.FormPart
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputHidden {
				hidden = append(hidden, w.Hidden{Name: r.Name, Value: string(r.Value)})
				continue
			}
			kind := w.InputText
			if r.Kind == view.InputPassword {
				kind = w.InputPassword
			}
			parts := []w.FieldPart{w.Input{
				Kind:     kind,
				Name:     r.Name,
				Value:    string(r.Value),
				Label:    string(r.Label),
				Focus:    r.Focus,
				Error:    r.Error != "",
				ReadOnly: r.ReadOnly,
				Required: r.Required,
			}}
			if r.Error != "" {
				parts = append(parts, w.FieldHint{Parts: []w.Inline{w.Text(r.Error)}, Variant: w.HintDanger})
			}
			fields = append(fields, w.Field{Label: string(r.Label), Parts: parts, Width: w.FieldW100})
		}
	}
	var links []w.Link
	if p.Register != nil {
		links = append(links, w.Link{Text: string(p.Register.Label), Href: p.Register.Href})
	}
	links = append(links, w.Link{Text: string(p.Forgot.Label), Href: p.Forgot.Href})
	var buttons []w.AuthStackPart
	for _, a := range p.Form.Actions {
		buttons = append(buttons, w.Button{Label: string(a.Label), Kind: w.ButtonPrimary, Form: formID})
	}
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo60}},
						w.Tagline{
							Lead:     lead(p.Lede),
							Text:     string(p.Tagline),
							Centered: true,
						},
						w.AuthStack{
							Items: append([]w.AuthStackPart{
								w.Form{
									Items:   fields,
									Variant: w.FormColumnFlush,
									Action:  p.Form.Action,
									Method:  p.Form.Method,
									Fields:  hidden,
									ID:      formID,
								},
							}, buttons...),
							Links: links,
						},
					},
					Variant: w.DialogNarrow,
				},
			},
		},
	}
}

// lead is the first line of the tagline, the pieces of Lede one after
// another: the tagline marks none of them (C-6.2 Runs).
func lead(r view.Runs) string {
	var s string
	for _, p := range r {
		s += string(p.Text)
	}
	return s
}
