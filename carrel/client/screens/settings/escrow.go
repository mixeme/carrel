package settings

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of Key escrow (U-1.3): the terms of a copy deposited, the
// password history's field.
const (
	depositedTerm   = "Deposited"
	lastRecoveryTrm = "Last recovery"
	recoveredTerm   = "Recovered"
	whoSep          = " · "
	historyLabel    = "Password history"
)

// Escrow is /app/settings/escrow (EscrowPage): with a copy deposited, a
// dialog — what an administrator can do with it, in alarm; when it was
// deposited and last recovered; the address of the password's history to
// keep; Withdraw and what it ends (account-4). Without a copy, what a
// deposit does before Deposit my key (account-12). With the scheme off,
// one line (account-14).
func Escrow(p *view.EscrowPage) w.Shell {
	st := p.Status
	main := bind.ColumnMsgs(bind.Notices(p.Notices, w.MsgSpacePage))
	switch {
	case !st.Active:
		main = append([]w.MainPart{head(p.Title, p.Subtitle)}, main...)
		main = append(main, w.Msg{Column: true, Parts: []w.MsgPart{w.Text(st.Note)}})
	case !st.Covered:
		main = append([]w.MainPart{head(p.Title, p.Subtitle)}, main...)
		set := w.FieldSet{Rubric: w.Rubric{Text: string(p.Rubric)}, Variant: w.FieldSetLead}
		if st.Note != "" {
			set.Items = append(set.Items, w.Field{Parts: []w.FieldPart{w.FieldHint{Parts: []w.Inline{w.Text(st.Note)}, Variant: w.HintColumn}}, Place: w.Grown})
		}
		for _, a := range st.Actions {
			set.Items = append(set.Items, w.Field{Parts: []w.FieldPart{bind.Button(a, "")}})
		}
		main = append(main, w.Form{Items: []w.FormPart{set}})
	default:
		main = append(main, depositedDialog(p))
	}
	return shell(p.Top, p.Rail, main)
}

// depositedDialog is the page of a copy deposited (account-4).
func depositedDialog(p *view.EscrowPage) w.Dialog {
	st := p.Status
	d := w.Dialog{Variant: w.Dialog560, Items: []w.DialogPart{head(p.Title, p.Subtitle)}}
	if st.Note != "" {
		d.Items = append(d.Items, w.Msg{Column: true, Parts: []w.MsgPart{w.Text(st.Note)}, Tone: w.MsgAlert, Space: w.MsgFlushSides})
	}
	fields := w.Fields{Variant: w.FieldsBleed, Items: []w.FieldsItem{
		{Term: depositedTerm, Value: []w.ValuePart{w.Text(st.Deposited)}, Face: w.ValueMono},
	}}
	// A line a recovery, when and who (account-16); none yet, the day of
	// the last or never (account-4).
	for _, rc := range st.Recoveries {
		fields.Items = append(fields.Items, w.FieldsItem{Term: recoveredTerm, Value: []w.ValuePart{w.Mono{Text: string(rc.At)}, w.Text(whoSep + string(rc.Who))}})
	}
	if len(st.Recoveries) == 0 {
		fields.Items = append(fields.Items, w.FieldsItem{Term: lastRecoveryTrm, Value: []w.ValuePart{w.Text(st.LastRecovery)}, Face: w.ValueMono})
	}
	d.Items = append(d.Items, fields)
	if l := p.PasswordLink; l != nil {
		in := w.Input{Value: string(l.Address), Label: historyLabel, Mono: true, ReadOnly: true, Variant: w.InputFill}
		cp := copyButton(l.Copy, string(l.Address))
		cp.Icon = bind.Icon(l.Copy.Icon)
		f := w.Field{Label: historyLabel, Parts: []w.FieldPart{w.Line{Items: []w.LinePart{in, cp}, Gap: w.LineGap6}}, Width: w.FieldFull}
		if l.Note != "" {
			f.Parts = append(f.Parts, w.FieldHint{Parts: []w.Inline{w.Text(l.Note)}, Variant: w.HintColumn})
		}
		d.Items = append(d.Items, f)
	}
	foot := w.FormFoot{Variant: w.FormFootFlush}
	for _, a := range st.Actions {
		foot.Items = append(foot.Items, bind.Button(a, ""))
	}
	if st.ActionNote != "" {
		foot.Items = append(foot.Items, w.Dim{Parts: []w.Inline{w.Text(st.ActionNote)}, Size: w.Dim11_5, Variant: w.DimWrap})
	}
	d.Items = append(d.Items, foot)
	return d
}

// EscrowForms are the forms of the page that post with none of their own:
// the top bar's.
func EscrowForms(p *view.EscrowPage) []w.PostForm {
	return forms(p.Top, p.Notices)
}

// WithdrawSheet is the withdrawal of the key over Account,
// /app/settings/escrow/withdraw (EscrowWithdrawSheet, account-11): the
// shield in alarm, what ends and what stays, the withdrawal itself — the
// solid danger — and Cancel.
func WithdrawSheet(p *view.EscrowWithdrawSheet) w.Sheet {
	s := w.Sheet{Icon: "shield", Tone: w.SheetIconDanger, Title: string(p.Title), Width: w.SheetWidth480,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	if p.Warning != "" {
		s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Warning)}, Tone: w.MsgAlert, Space: w.MsgFlush})
	}
	for _, a := range p.Actions {
		if a.Method == "GET" {
			s.Foot = append(s.Foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGap7, Place: w.Pushed})
			continue
		}
		b := bind.Submit(a)
		if a.Href != "" && a.Href != p.Form.Action {
			b.To = a.Href
		}
		s.Foot = append(s.Foot, b)
	}
	return s
}
