package settings

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the account (U-1.3): the rubrics the model does not carry,
// the count of sessions, the mark of this one, the columns.
const (
	rubricPassword = "Password"
	rubricSessions = "Sessions"
	rubricEscrow   = "Key escrow"
	activeWord     = " active"
	thisOne        = "this one"
	colWhere       = "Where"
	colStarted     = "Started"
	colLastSeen    = "Last seen"
)

// Account is /app/settings/account (AccountPage, account-1; its states
// account-7…-10, -13, s-mail-9): the login and the day it was made; the
// name and the address, each its own form with its button beside the
// field, the address's state under it; the password and what changing it
// does; the sessions, this one marked, the others each with End, and End
// every other session; key escrow and what it means; the danger zone.
func Account(p *view.AccountPage) w.Shell {
	f := w.Form{}
	for i, set := range p.Profile.Sets {
		// The state of the address in the rubric (Typecase's note 90): its
		// badge and the words beside it.
		r := w.Rubric{Text: string(set.Legend)}
		for _, b := range set.Badges {
			r.Items = append(r.Items, bind.Badge(b))
		}
		if set.LegendAside != "" {
			r.Items = append(r.Items, w.Plain{Text: string(set.LegendAside)})
		}
		fs := w.FieldSet{Rubric: r, Method: "POST", Fields: bind.Hidden(p.Profile)}
		if i == 0 {
			fs.Variant = w.FieldSetLead
		}
		for _, r := range set.Rows {
			if r.Kind == view.InputHidden {
				continue
			}
			fs.Items = append(fs.Items, profileField(p, r, &fs))
		}
		f.Items = append(f.Items, fs)
	}
	pass := w.FieldSet{Rubric: w.Rubric{Text: rubricPassword}}
	if p.PasswordNote != "" {
		pass.Items = append(pass.Items, hintField(p.PasswordNote))
	}
	for _, a := range p.Actions {
		if a.ID == "account.password" {
			pass.Items = append(pass.Items, w.Field{Parts: []w.FieldPart{bind.Button(a, "")}})
		}
	}
	f.Items = append(f.Items, pass, sessionsSet(p), escrowSet(p.Escrow))
	main := []w.MainPart{head(p.Title, p.Subtitle)}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	main = append(main, f)
	for _, a := range p.Danger {
		main = append(main, w.Danger{Button: bind.Button(a, bind.TickID(a)), Note: string(p.DangerNote)})
	}
	return shell(p.Top, p.Rail, main)
}

// profileField is the name or the address: the field filling its line, its
// button beside it sending the group's own form (FormRow.Extra), what it is
// for under it; after the address's Change, Send confirmation or Send again
// where a link is to go — the field wider for the two (s-mail-9, account-9,
// -10).
func profileField(p *view.AccountPage, r view.FormRow, fs *w.FieldSet) w.Field {
	in := bind.Input(r)
	in.Variant = w.InputFill
	line := w.Line{Items: []w.LinePart{in}, Gap: w.LineGap8}
	if a := r.Extra; a != nil {
		fs.Action = a.Href
		b := bind.Button(*a, "")
		if a.Enabled {
			b.Form, b.Href, b.Submit = "", "", true
		}
		line.Items = append(line.Items, b)
	}
	width := w.FieldLG
	if a := p.Email.Send; r.Kind == view.InputEmail && a != nil {
		b := bind.Button(*a, "")
		if a.Enabled {
			b.Form, b.Href, b.Submit, b.To = "", "", true, a.Href
		}
		line.Items = append(line.Items, b)
		width = w.FieldXL
	}
	parts := append([]w.FieldPart{line}, bind.Hint(r)...)
	return w.Field{Label: string(r.Label), Parts: parts, Width: width}
}

// sessionsSet is the sessions (account-1): how many, a row each — where,
// when it started, when last seen —, this one marked and its End off with
// why; End every other session under them.
func sessionsSet(p *view.AccountPage) w.FieldSet {
	fs := w.FieldSet{Rubric: w.Rubric{Text: rubricSessions, Items: []w.RubricPart{w.Plain{Text: strconv.Itoa(len(p.Sessions)) + activeWord}}}}
	t := w.Table{Cols: []w.TableCol{{Label: colWhere}, {Label: colStarted}, {Label: colLastSeen, Rank: w.Rank3}, {Width: w.Col120, Rank: w.Rank4}},
		Variant: w.TableInset, Place: w.Grown}
	for _, s := range p.Sessions {
		where := w.TableCell{Parts: []w.CellPart{w.Text(s.Agent)}}
		if s.Current {
			where.Parts = []w.CellPart{w.Text(string(s.Agent) + " "), w.Badge{Label: thisOne}}
		}
		// End of each, this one's off with its reason: it is ended by
		// signing out (account-1, note 101).
		b := bind.Button(s.End, bind.TickID(s.End))
		b.Kind, b.Small = w.ButtonDanger, true
		end := w.TableCell{Parts: []w.CellPart{b}, Acts: true}
		t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{where, {Parts: []w.CellPart{w.Text(s.Started)}, Mono: true},
			{Parts: []w.CellPart{w.Text(s.LastSeen)}, Mono: true}, end}})
	}
	fs.Items = append(fs.Items, t)
	for _, a := range p.Actions {
		if a.ID == "account.session.others" {
			b := bind.Button(a, bind.TickID(a))
			b.Kind = w.ButtonDanger
			fs.Items = append(fs.Items, w.Field{Parts: []w.FieldPart{b}})
		}
	}
	return fs
}

// escrowSet is key escrow on the account (account-1, -7, -13): its badge
// where a copy is deposited, what it means, and what can be done.
func escrowSet(e view.EscrowStatus) w.FieldSet {
	r := w.Rubric{Text: rubricEscrow}
	if e.Badge != nil {
		r.Items = append(r.Items, bind.Badge(*e.Badge))
	}
	fs := w.FieldSet{Rubric: r}
	if e.Note != "" {
		fs.Items = append(fs.Items, hintField(e.Note))
	}
	for _, a := range e.Actions {
		fs.Items = append(fs.Items, w.Field{Parts: []w.FieldPart{bind.Button(a, bind.TickID(a))}})
	}
	return fs
}

// AccountForms are the forms of the page's actions that post with none of
// their own: each session's End, End every other session, the escrow's and
// the danger zone's where they post, and the top bar's.
func AccountForms(p *view.AccountPage) []w.PostForm {
	out := forms(p.Top, p.Notices)
	post := func(a view.Action) {
		if a.Method == "POST" && a.Enabled {
			out = append(out, bind.ActionForm(a, bind.TickID(a)))
		}
	}
	for _, s := range p.Sessions {
		if !s.Current {
			post(s.End)
		}
	}
	for _, a := range p.Actions {
		post(a)
	}
	for _, a := range p.Escrow.Actions {
		post(a)
	}
	for _, a := range p.Danger {
		post(a)
	}
	return out
}
