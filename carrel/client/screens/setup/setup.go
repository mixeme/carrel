// Package setup is the first run, /setup, drawn by frame «Первый запуск»
// (s-auth-4) of the mockup: the draft frame2go printed for it, with the
// frame's data become the view model's (U-5.1, U-6.14).
package setup

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// formID names the first run's form for its button, which stands under it
// and outside it (Button.Form): the frame draws it in the form's foot.
const formID = "setup"

// Screen draws the first run from its view model: «Set up Carrel» (Title),
// what the account is (Lede), the warning about the password over the fields
// (Notices), then the form's rows — Login monospaced with its pattern in the
// empty field (Mono, Placeholder) and the cursor (Focus), Email with
// «(optional)» after its label (LabelAside) and a dash in the empty field,
// Password, Repeat password; a reason under a field that came back with
// one. «Create administrator» submits the form by name. There is no default
// password.
func Screen(p *view.SetupPage) w.Page {
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
