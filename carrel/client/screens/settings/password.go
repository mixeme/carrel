package settings

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// passwordFormID is the ID of the form of a new password: its button
// stands in the dialog's foot.
const passwordFormID = "password"

// Password is /app/password (PasswordPage, account-3): signed in with a
// temporary password, the line in alarm on top; the current password, the
// new one twice, what changes — the wrapping, not the key —; Change
// password and Back to settings, none when there is nowhere to go back to.
func Password(p *view.PasswordPage) w.Shell {
	d := w.Dialog{Variant: w.Dialog520, Items: []w.DialogPart{w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}}}
	if p.Warning.Body != "" || len(p.Warning.Runs) > 0 {
		m := bind.Msg(p.Warning, w.MsgFlushSides)
		d.Items = append(d.Items, m)
	}
	form := bind.Form(p.Form, passwordFormID)
	form.Variant = w.FormFlush
	for _, r := range bind.Rows(p.Form) {
		form.Items = append(form.Items, bind.Field(r, w.FieldLG))
	}
	if p.Note != "" {
		form.Items = append(form.Items, hintField(p.Note))
	}
	d.Items = append(d.Items, form)
	foot := w.FormFoot{Variant: w.FormFootFlush}
	for _, a := range p.Form.Actions {
		foot.Items = append(foot.Items, bind.SubmitForm(a, passwordFormID))
	}
	if b := p.Back; b != nil {
		foot.Items = append(foot.Items, bind.Button(*b, ""))
	}
	d.Items = append(d.Items, foot)
	main := append(bind.Notices(p.Notices, w.MsgSpacePage), d)
	if len(p.Rail.Nav) == 0 && len(p.Rail.Sections) == 0 {
		// The forced change: no rail — nothing further before it
		// (account-17).
		tb := bind.Top(p.Top, top)
		return w.Shell{Top: &tb, Body: w.ShellBody{Main: w.Main{Items: main}}}
	}
	return shell(p.Top, p.Rail, main)
}

// PasswordForms are the forms of the page that post with none of their
// own: the top bar's.
func PasswordForms(p *view.PasswordPage) []w.PostForm {
	return forms(p.Top, p.Notices)
}
