// Package forgot is the forgotten password, /forgot, drawn by frame «Забыли
// пароль · логин или почта» (s-auth-5) of the mockup and by the frames of its
// states — what can be done (s-auth-6), no confirmed address (s-auth-7), the
// letter sent (s-auth-8), the administrator asked (s-auth-9): the drafts
// frame2go printed for them, with the frames' data become the view model's
// (U-5.1, U-6.14).
package forgot

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// formID names the form of the login or address for its buttons, which
// stand under it and outside it (Button.Form).
const formID = "forgot"

// Screen draws the forgotten password from its view model, in the state the
// model is in. Each state has its title (Title) and under it what it means
// (Lede, the login bold where the frame marks it), a refusal of the server
// over what follows (Notices), and «Back to sign in» (Back) in its foot.
//
//   - The form (its rows, s-auth-5): «Login or email» with the cursor
//     (Focus); «Continue» submits the form by name.
//   - What can be done (Options, s-auth-6): a card for each option — its
//     title, what it does and its one button, which submits the option's own
//     form, printed at the page's end (Forms); the wipe's button is the
//     primary one (Primary), asking the administrator is offered only with a
//     deposited key, and the server sends only the options there are.
//   - No confirmed address (s-auth-7), the letter sent (Sent, s-auth-8), the
//     administrator asked (Asked, s-auth-9): what happened and nothing to do
//     but go back. Nothing can be done without a confirmed address, and the
//     page says so.
func Screen(p *view.ForgotPage) w.Page {
	rows := bind.Rows(p.Form)
	lede := w.Prose{Parts: bind.Inlines(p.Lede), Variant: w.ProseMuted}
	variant := w.DialogPage520
	if len(rows) > 0 {
		lede.Variant, variant = w.ProseMutedClose, w.DialogPage440
	}
	items := []w.DialogPart{w.DialogTitle{Text: string(p.Title), Gap: w.DialogTitleGap8}, lede}
	for _, n := range p.Notices {
		items = append(items, bind.Msg(n, w.MsgFlushBottom12))
	}
	var foot []w.FormFootPart
	switch {
	case len(rows) > 0:
		var fields []w.FormPart
		for _, r := range rows {
			fields = append(fields, bind.Field(r, w.FieldW100))
		}
		form := bind.Form(p.Form, formID)
		form.Items, form.Variant = fields, w.FormColumnFlush
		items = append(items, form)
		for _, a := range p.Form.Actions {
			foot = append(foot, bind.Button(a, formID))
		}
	case len(p.Options) > 0:
		var cards []w.FormPart
		for _, o := range p.Options {
			parts := []w.CardPart{
				w.CardText{Kind: w.CardTerm, Parts: []w.CardTextPart{w.Text(o.Title)}},
				w.Prose{Parts: []w.Inline{w.Text(o.Body)}, Variant: w.ProseMutedClose},
			}
			for _, a := range o.Form.Actions {
				parts = append(parts, bind.Button(a, a.ID))
			}
			cards = append(cards, w.Card{Items: parts, Variant: w.CardSnug})
		}
		items = append(items, w.Form{Items: cards, Variant: w.FormColumnGap10})
	}
	foot = append(foot, bind.Button(p.Back, ""))
	items = append(items, w.FormFoot{Items: foot, Variant: w.FormFootFlush})
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items:   items,
					Variant: variant,
				},
			},
		},
	}
}

// Forms are the forms of the options' buttons, printed at the page's end: an
// option's form carries no field a person fills in — the login, what to do
// and the token ride hidden — so each of its buttons submits it as a
// PostForm by the button's action (Button.Form).
func Forms(p *view.ForgotPage) []w.PostForm {
	var out []w.PostForm
	for _, o := range p.Options {
		for _, a := range o.Form.Actions {
			out = append(out, bind.PostForm(o.Form, a.ID))
		}
	}
	return out
}
