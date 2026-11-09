package admin

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of key escrow (U-1.3): the rubric of the password shown once
// and of the list, the list's columns, the ID of the password's block.
const (
	tempPassword = "Temporary password"
	opensWhom    = "Whom the master password opens"
	colAccount   = "Account"
	colDeposited = "Deposited"
	colRecovery  = "Last recovery"
	recoveredID  = "admin-escrow-secret"
)

// escrowWidths are the widths the frame gives the fields, by their names
// (s-admin-6, -31).
var escrowWidths = map[string]w.FieldWidth{"current": w.FieldLG, "master": w.FieldLG, "master2": w.FieldLG, "login": w.FieldMD, "temp": w.FieldMD}

// Escrow is key escrow, /admin/escrow (AdminEscrowPage, s-admin-6; an
// account recovered, -30; the scheme off, -31): what escrow is, in alarm;
// the password a recovery gave, once; the policy, the master password and
// the recovery — each its own form —; whom the master password opens. Off,
// the master password and Turn on key escrow alone.
func Escrow(p *view.AdminEscrowPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	main := []w.MainPart{h}
	main = append(main, bind.ColumnMsgs(bind.Notices(p.Notices, w.MsgSpacePage))...)
	if p.Warning.Body != "" {
		m := bind.Msg(p.Warning, w.MsgSpacePage)
		m.Icon, m.Column = "shield", true
		main = append(main, m)
	}
	if !p.Enabled {
		f := bind.Form(p.Form, "")
		for _, fl := range escrowFields(p.Form) {
			f.Items = append(f.Items, fl)
		}
		return shell(p.Top, p.Rail, append(main, f))
	}
	if r := p.Result; r != nil {
		m := bind.Msg(view.Notice{Runs: r.Message}, w.MsgSpacePage)
		m.Column = true
		main = append(main, m, recoveredCard(*r))
	}
	sets := w.Form{}
	for i, f := range []view.Form{p.Form, p.Master, p.Recover} {
		fs := w.FieldSet{Action: f.Action, Method: f.Method, Fields: bind.Hidden(f)}
		for _, fl := range escrowFields(f) {
			fs.Items = append(fs.Items, fl)
		}
		for _, set := range f.Sets {
			if set.Legend != "" {
				fs.Rubric = w.Rubric{Text: string(set.Legend)}
			}
		}
		if i == 0 {
			fs.Variant = w.FieldSetLead
		}
		sets.Items = append(sets.Items, fs)
	}
	main = append(main, sets)
	if len(p.Covered) > 0 {
		main = append(main, w.ListGroup{Title: w.Rubric{Text: opensWhom, Form: w.RubricInline}, Count: &w.Num{Value: p.Opens}}, coveredTable(p.Covered))
	}
	return shell(p.Top, p.Rail, main)
}

// escrowFields are a form's rows as the frame lays them out: the tick and
// its words under it; the fields side by side, the button after them, and
// what they mean after the button, on a line of its own (s-admin-6, -31).
func escrowFields(f view.Form) []w.Field {
	var out, hints []w.Field
	for _, r := range bind.Rows(f) {
		if r.Kind == view.InputCheck {
			out = append(out, w.Field{Parts: []w.FieldPart{bind.Check(r)}, Gap: 7, Place: w.Grown})
			if r.Hint != "" {
				out = append(out, hintField(r.Hint))
			}
			continue
		}
		in := r
		in.Hint = ""
		out = append(out, bind.ColumnField(in, escrowWidths[r.Name]))
		if r.Hint != "" {
			hints = append(hints, hintField(r.Hint))
		}
	}
	for _, a := range f.Actions {
		b := submit(a, f)
		if a.Danger {
			// It opens someone's data: in the danger's colour, not filled.
			b.Kind = w.ButtonDanger
		}
		out = append(out, w.Field{Parts: []w.FieldPart{b}})
	}
	return append(out, hints...)
}

// hintField is what a field means, on a line of its own.
func hintField(t view.Text) w.Field {
	return w.Field{Parts: []w.FieldPart{w.FieldHint{Parts: []w.Inline{w.Text(t)}, Variant: w.HintColumn}}, Place: w.Grown}
}

// recoveredCard is the password a recovery gave, shown once (s-admin-30):
// marked «visible only now», the login and the password in one block, Copy
// taking them both, Done, and whether a letter went.
func recoveredCard(r view.IssuedPassword) w.Pad {
	secret := "login     " + string(r.Login) + "\npassword  " + string(r.Password)
	row := w.CardRow{Kind: w.CardTop8, Gap: w.LineGapNormal, Items: []w.CardRowPart{
		w.Button{Label: string(r.Copy.Label), Icon: "dup", Kind: w.ButtonPrimary, CopyFrom: recoveredID, Copied: copied},
		bind.Button(r.Done, ""),
	}}
	if r.Note != "" {
		row.Items = append(row.Items, w.Dim{Parts: []w.Inline{w.Text(r.Note)}, Size: w.Dim11_5, Variant: w.DimWrap})
	}
	return w.Pad{Items: []w.PadPart{w.Card{Variant: w.CardChosenFlushGrow, Items: []w.CardPart{
		w.Rubric{Text: tempPassword, Items: []w.RubricPart{w.Mono{Text: onlyNow, Variant: w.MonoTightDanger}}},
		w.Diag{Text: secret, Variant: w.DiagInk, ID: recoveredID},
		row,
	}}}}
}

// coveredTable is whom the master password opens (s-admin-6): an account a
// row, when its copy was deposited and when it was last recovered; the
// words without a moment — «no», «never» — muted. Deposited is the main
// column of a card on a phone (note 156).
func coveredTable(rows []view.EscrowCoverRow) w.Table {
	t := w.Table{Cols: []w.TableCol{{Label: colAccount}, {Label: colDeposited, Main: true}, {Label: colRecovery, Rank: w.Rank3}}}
	for _, r := range rows {
		dep := w.TableCell{Parts: []w.CellPart{w.Text(r.Deposited)}, Mono: true}
		if !r.Covered {
			dep.Tone = w.CellDusty
		}
		last := w.TableCell{Parts: []w.CellPart{w.Text(r.LastRecovery)}, Mono: true}
		if r.Covered && !r.Recovered {
			last.Tone = w.CellDusty
		}
		t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{{Parts: []w.CellPart{w.Text(r.Login)}}, dep, last}})
	}
	return t
}

// EscrowForms are the forms of the page's actions that post with none of
// their own: the top bar's and the notices'.
func EscrowForms(p *view.AdminEscrowPage) []w.PostForm {
	return forms(p.Top, p.Notices)
}
