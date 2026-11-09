// Package invite is the page of an invitation, /invite/{token}, drawn by
// frame «Приглашение · свой логин и пароль» (s-auth-12) of the mockup: the
// draft frame2go printed for it, with the frame's data become the view
// model's (U-5.1, U-6.14).
package invite

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// formID names the invitation's form for its buttons, which stand under it
// and outside it (Button.Form): the frame draws them in the form's foot.
const formID = "invite"

// Screen draws the page of an invitation from its view model: «Join» and the
// instance's name (Title), under it who invited and what to do (Lede), the
// warning about the password over the fields (Notices), then the form. The
// fields are the form's rows: Login, the address an invitation by letter
// went to — shown switched off, as a field the server does not read
// (ReadOnly, C-6.8) —, Password, Repeat password; the cursor stands where
// the model says (Focus), and a field with an error — a login taken — is
// marked and says it under itself. The hidden rows (CSRF) ride with the
// form; «Create account» submits it by name, drawn primary as the model says
// (Primary). A dead link is not this page: it is DeadLinkPage's.
func Screen(p *view.InvitePage) w.Page {
	items := []w.DialogPart{
		w.DialogTitle{Text: string(p.Title), Gap: w.DialogTitleGapTight},
		w.DialogSub{Parts: bind.Inlines(p.Lede)},
	}
	for _, n := range p.Notices {
		items = append(items, bind.Msg(n, w.MsgFlushBelowWide))
	}
	var fields []w.FormPart
	for _, r := range bind.Rows(p.Form) {
		fields = append(fields, bind.Field(r, w.FieldMD))
	}
	var buttons []w.FormFootPart
	for _, a := range p.Form.Actions {
		buttons = append(buttons, bind.Button(a, formID))
	}
	form := bind.Form(p.Form, formID)
	form.Items, form.Variant = fields, w.FormFlushAll
	items = append(items, form, w.FormFoot{Items: buttons, Variant: w.FormFootFlush})
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
