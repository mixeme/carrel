package admin

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of Add users (U-1.3): the accessible names of the links shown
// to copy, what Copy says once it has, the mark of a password shown once
// and the ID of its block, the rubric of the list.
const (
	inviteLink   = "Invitation link"
	registerLink = "Registration address"
	copied       = "Copied"
	onlyNow      = "visible only now"
	secretID     = "admin-issued-secret"
	pending      = "Pending invitations"
)

// Invites is Add users, /admin/invites (AdminInvitesPage, s-admin-1; its
// fragments -17, -18, -19): three cards, three ways — an invitation (a
// link, or a letter), a temporary password handed over in person, the
// registration open to visitors — and the invitations still pending.
func Invites(p *view.AdminInvitesPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	main = append(main, w.MainCards{Cards: []w.Card{inviteCard(p.Invite), temporaryCard(p.Temporary), selfRegCard(p.SelfReg)}})
	if len(p.Rows) > 0 {
		main = append(main, w.ListGroup{Title: w.Rubric{Text: pending, Form: w.RubricInline}, Count: &w.Num{Value: len(p.Rows)}}, invitesTable(p.Rows))
	}
	return shell(p.Top, p.Rail, main)
}

// card is the head of a card of Add users: its title, the words after it
// (Aside, plain) and its badge; the warning under it (s-admin-1).
func card(c view.AddUserCard) []w.CardPart {
	r := w.Rubric{Text: string(c.Title)}
	if c.Aside != "" {
		r.Items = append(r.Items, w.Plain{Text: string(c.Aside)})
	}
	if c.Badge != nil {
		r.Items = append(r.Items, bind.Badge(*c.Badge))
	}
	parts := []w.CardPart{r}
	if c.Lede != "" {
		parts = append(parts, w.CardText{Kind: w.CardLede, Parts: []w.CardTextPart{w.Text(c.Lede)}})
	}
	if c.Warning != nil {
		parts = append(parts, bind.Msg(*c.Warning, w.MsgFlushBottom12))
	}
	return parts
}

// formCard is a card that is its form: where it sends and what it carries.
func formCard(c view.AddUserCard, parts []w.CardPart) w.Card {
	return w.Card{Items: parts, Action: c.Form.Action, Method: c.Form.Method, Fields: bind.Hidden(c.Form)}
}

// submit is a button of a card's form: it sends the card, to its own
// address where that is not the card's (Send invitation); switched off, it
// says why.
func submit(a view.Action, f view.Form) w.Button {
	b := bind.Button(a, "")
	if a.Enabled {
		b.Form, b.Href, b.Submit = "", "", true
		if a.Href != f.Action {
			b.To = a.Href
		}
	}
	return b
}

// linkRow is a whole address shown to be copied (Link, Copy): the link
// just made, the address of the registration (s-admin-1).
func linkRow(link view.Text, cp *view.Action, label string) w.CardRow {
	in := w.Input{Value: string(link), Label: label, Mono: true, ReadOnly: true, Variant: w.Input11Fill}
	// Copy puts the address on the clipboard; without a script it is the
	// link itself.
	b := w.Button{Label: "Copy", Small: true, Href: string(link), Copy: string(link), Copied: copied}
	if cp != nil {
		b.Label = string(cp.Label)
	}
	return w.CardRow{Kind: w.CardTop10, Gap: 6, Items: []w.CardRowPart{in, b}}
}

// field is a row of a card's form by its name, and the width its frame
// gives it; a tick is a checkbox.
func field(f view.Form, name string, width w.FieldWidth) (w.CardFieldsPart, bool) {
	for _, r := range bind.Rows(f) {
		if r.Name == name {
			return bind.Field(r, width), true
		}
	}
	return nil, false
}

// action is the form's action of the ID.
func action(f view.Form, id string) (view.Action, bool) {
	for _, a := range f.Actions {
		if a.ID == id {
			return a, true
		}
	}
	return view.Action{}, false
}

// inviteCard is the invitation (s-admin-1, -18): the role and Create link;
// the link made, to copy; under a rule, the address and Send invitation —
// off with its reason while no letter can go.
func inviteCard(c view.AddUserCard) w.Card {
	parts := card(c)
	first := w.CardFields{Wide: true}
	if f, ok := field(c.Form, "role", w.FieldSM); ok {
		first.Items = append(first.Items, f)
	}
	if a, ok := action(c.Form, "admin.invite.create"); ok {
		first.Items = append(first.Items, submit(a, c.Form))
	}
	parts = append(parts, first)
	if c.Link != "" {
		parts = append(parts, linkRow(c.Link, c.Copy, inviteLink))
	}
	second := w.CardFields{Wide: true, Rule: true}
	if f, ok := field(c.Form, "email", w.FieldMD); ok {
		second.Items = append(second.Items, f)
	}
	if a, ok := action(c.Form, "admin.invite.send"); ok {
		second.Items = append(second.Items, submit(a, c.Form))
	}
	return formCard(c, append(parts, second))
}

// temporaryCard is the temporary password (s-admin-1, -19): the warning,
// the role, the login, the address and the password side by side, Create
// user after them; a refusal stands under its field, the fields aligned by
// their tops and the button keeping their line. Once the user is made, the
// card is the password shown once (s-admin-17).
func temporaryCard(c view.AddUserCard) w.Card {
	if c.Issued != nil {
		return issuedCard(c, *c.Issued)
	}
	parts := card(c)
	fs := w.CardFields{Wide: true}
	widths := map[string]w.FieldWidth{"role": w.FieldXS, "login": w.FieldSM, "email": w.FieldMD, "password": w.FieldMD}
	for _, r := range bind.Rows(c.Form) {
		if r.Error != "" {
			fs.Top = true
		}
		fs.Items = append(fs.Items, bind.Field(r, widths[r.Name]))
	}
	if a, ok := action(c.Form, "admin.user.create"); ok {
		b := submit(a, c.Form)
		if fs.Top {
			// The button in a field with no label: on the inputs' line.
			fs.Items = append(fs.Items, w.Field{Label: " ", Parts: []w.FieldPart{b}})
		} else {
			fs.Items = append(fs.Items, b)
		}
	}
	return formCard(c, append(parts, fs))
}

// issuedCard is the password of the user just made, shown once (s-admin-17):
// the card's title marked «visible only now», what was done in words, the
// login and the password in one block, Copy taking them both and Done.
func issuedCard(c view.AddUserCard, iss view.IssuedPassword) w.Card {
	msg := bind.Msg(view.Notice{Runs: iss.Message}, w.MsgFlushBottom12)
	secret := "login     " + string(iss.Login) + "\npassword  " + string(iss.Password)
	cp := w.Button{Label: string(iss.Copy.Label), Icon: "dup", Kind: w.ButtonPrimary, CopyFrom: secretID, Copied: copied}
	return w.Card{Variant: w.CardChosen, Items: []w.CardPart{
		w.Rubric{Text: string(c.Title), Items: []w.RubricPart{w.Mono{Text: onlyNow, Variant: w.MonoTightDanger}}},
		msg,
		w.Diag{Text: secret, Variant: w.DiagInk, ID: secretID},
		w.CardRow{Kind: w.CardTop8, Gap: 8, Items: []w.CardRowPart{cp, bind.Button(iss.Done, "")}},
	}}
}

// selfRegCard is the registration open to visitors (s-admin-1, -18): the
// lede, the tick, the address of the form to copy while it is open, Save —
// off with its reason while mail does not work.
func selfRegCard(c view.AddUserCard) w.Card {
	parts := card(c)
	for _, r := range bind.Rows(c.Form) {
		if r.Kind == view.InputCheck {
			parts = append(parts, bind.Check(r))
		}
	}
	if c.Link != "" {
		parts = append(parts, linkRow(c.Link, c.Copy, registerLink))
	}
	var acts []w.CardRowPart
	for _, a := range c.Form.Actions {
		acts = append(acts, submit(a, c.Form))
	}
	if len(acts) > 0 {
		parts = append(parts, w.CardRow{Kind: w.CardTop10, Items: acts})
	}
	return formCard(c, parts)
}

// invitesTable is the invitations pending (s-admin-1, -11): the address (a
// dash for a link), how it went, its state, the letter's fate — in alarm
// where it did not go —, when it lapses; Resend and Copy link, and «⋯» with
// Extend and Revoke.
func invitesTable(rows []view.InviteRow) w.Table {
	t := w.Table{Cols: []w.TableCol{{Label: "Address"}, {Label: "Delivery"}, {Label: "Status", Rank: w.Rank3}, {Label: "Mail", Rank: w.Rank4}, {Label: "Expires"}, {}}}
	for _, r := range rows {
		mono := func(s view.Text) w.TableCell { return w.TableCell{Parts: []w.CellPart{w.Text(s)}, Mono: true} }
		addr := mono(r.Address)
		if r.Address == noCopy {
			addr.Mono = false
		}
		sent := mono(r.Sent)
		if r.Sent == noCopy {
			sent.Mono = false
		}
		if r.SentFailed {
			sent.Tone = w.CellDanger
		}
		acts := w.TableCell{Acts: true}
		for _, a := range r.Actions {
			b := bind.Button(a, inviteFormID(a, r))
			b.Small = true
			if a.Method == "GET" && a.Enabled {
				// Copy link puts the whole address on the clipboard; without
				// a script it is the link itself.
				b.Copy, b.Copied = string(r.Link), copied
			}
			acts.Parts = append(acts.Parts, b)
		}
		acts.Parts = append(acts.Parts, w.Drop{
			Trigger: w.Button{Icon: "dots", Small: true, Variant: w.ButtonPadX3, Title: more},
			Menu:    w.Menu{Items: bind.MenuItems(inviteMenu(r), ""), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		})
		t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{
			addr, mono(r.Delivery), {Parts: []w.CellPart{w.Text(r.Status)}}, sent, mono(view.Text(r.Expires)), acts,
		}})
	}
	return t
}

// inviteFormID is the ID of the form of an invitation's action that posts:
// the same actions stand in every row, the address they post to tells them
// apart.
func inviteFormID(a view.Action, r view.InviteRow) string {
	return a.ID + "/" + string(r.Address) + "/" + a.Href
}

// inviteMenu is an invitation's «⋯», each item's ID made its own.
func inviteMenu(r view.InviteRow) []view.MenuItem {
	out := make([]view.MenuItem, len(r.Menu))
	for i, it := range r.Menu {
		it.ID = inviteFormID(it.Action, r)
		out[i] = it
	}
	return out
}

// InvitesForms are the forms of the page's actions that post: each
// invitation's Resend and the items of its «⋯», and the top bar's.
func InvitesForms(p *view.AdminInvitesPage) []w.PostForm {
	out := forms(p.Top, p.Notices)
	for _, r := range p.Rows {
		for _, a := range r.Actions {
			if a.Method == "POST" && a.Enabled {
				out = append(out, bind.ActionForm(a, inviteFormID(a, r)))
			}
		}
		out = append(out, bind.MenuForms(inviteMenu(r))...)
	}
	return out
}
