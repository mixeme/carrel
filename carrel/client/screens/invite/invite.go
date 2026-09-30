// Package invite is the page of an invitation, /invite/{token}, drawn by
// frame «Приглашение · свой логин и пароль» (s-auth-12) of the mockup: the
// draft frame2go printed for it, with the frame's data become the view
// model's (U-5.1, U-6.14).
package invite

import (
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
// form; «Create account» submits it by name. A dead link is not this page:
// it is DeadLinkPage's.
func Screen(p *view.InvitePage) w.Page {
	items := []w.DialogPart{
		w.DialogTitle{Text: string(p.Title), Gap: w.DialogTitleGap4},
		w.DialogSub{Parts: []w.Inline{w.Text(p.Lede)}},
	}
	for _, n := range p.Notices {
		items = append(items, w.Msg{
			Parts: []w.MsgPart{w.Text(n.Body)},
			Tone:  tone(n.Role),
			Space: w.MsgFlushBottom12,
		})
	}
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
				Disabled: r.ReadOnly,
				Required: r.Required,
			}}
			if r.Error != "" {
				parts = append(parts, w.FieldHint{Parts: []w.Inline{w.Text(r.Error)}, Variant: w.HintDanger})
			}
			fields = append(fields, w.Field{Label: string(r.Label), Parts: parts, Width: w.FieldMD})
		}
	}
	var buttons []w.FormFootPart
	for _, a := range p.Form.Actions {
		buttons = append(buttons, w.Button{Label: string(a.Label), Kind: w.ButtonPrimary, Form: formID})
	}
	items = append(items,
		w.Form{
			Items:   fields,
			Variant: w.FormFlushAll,
			Action:  p.Form.Action,
			Method:  p.Form.Method,
			Fields:  hidden,
			ID:      formID,
		},
		w.FormFoot{Items: buttons, Variant: w.FormFootFlush},
	)
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

// tone is the look of a notice by its role: an alert — the password that
// nobody can recover — is the frame's.
func tone(r view.NoticeRole) w.MsgTone {
	switch r {
	case view.NoticeAlert:
		return w.MsgAlert
	case view.NoticeAccent:
		return w.MsgAccent
	}
	return w.MsgPlain
}
