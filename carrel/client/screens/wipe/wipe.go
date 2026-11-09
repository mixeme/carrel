// Package wipe is the link of the forgotten password, /forgot/{token}, in its
// two steps: the wipe's confirmation, drawn by frame «Забыли пароль ·
// подтверждение очистки» (s-auth-10) of the mockup, and the new password,
// drawn by «Забыли пароль · новый пароль» (s-auth-11): the drafts frame2go
// printed for them, with the frames' data become the view model's (U-5.1,
// U-6.14). A dead link is not this page: it is DeadLinkPage's.
package wipe

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// formID names the form of the new password for its button, which stands
// under it and outside it (Button.Form).
const formID = "wipe"

// Screen draws the link of the forgotten password from its view model, at
// the step the model is at (Step).
//
//   - The confirmation (WipeConfirm, s-auth-10): the question (Title), what
//     is erased and what stays, the login bold (Lede); the destructive button
//     repeats the consequence and submits the step's form — no field a
//     person fills in, so a PostForm at the page's end (Forms) —, and
//     «Cancel» goes to sign-in (Cancel). No word to type.
//   - The new password (WipePassword, s-auth-11): «Set a new password», the
//     account's login monospaced in what happened (Lede), the warning about
//     the password over the fields (Notices), Password with the cursor
//     (Focus) and Repeat password; the button submits the form by name. No
//     login and no address are asked: the account stays.
func Screen(p *view.ForgotWipePage) w.Page {
	if p.Step == view.WipeConfirm {
		var foot []w.FormFootPart
		for _, a := range p.Form.Actions {
			foot = append(foot, bind.Button(a, a.ID))
		}
		if p.Cancel != nil {
			foot = append(foot, bind.Button(*p.Cancel, ""))
		}
		return w.Page{
			Items: []w.PagePart{
				w.Auth{
					Dialog: w.Dialog{
						Items: []w.DialogPart{
							w.DialogTitle{Text: string(p.Title), Gap: w.DialogTitleGapNormal},
							w.Prose{Parts: bind.Inlines(p.Lede), Variant: w.ProseMuted},
							w.FormFoot{Items: foot, Variant: w.FormFootFlush},
						},
						Variant: w.DialogPage520,
					},
				},
			},
		}
	}
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

// Forms are the form of the confirmation, printed at the page's end: it
// carries only the token, hidden, and its button submits it by the action's
// ID (Button.Form). The new password's form is drawn in place and needs none.
func Forms(p *view.ForgotWipePage) []w.PostForm {
	if p.Step != view.WipeConfirm {
		return nil
	}
	var out []w.PostForm
	for _, a := range p.Form.Actions {
		out = append(out, bind.PostForm(p.Form, a.ID))
	}
	return out
}
