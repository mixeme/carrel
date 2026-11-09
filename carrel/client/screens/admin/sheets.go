package admin

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the sheets over Users (U-1.3): the keys of their answers,
// the ID of the password's block.
const (
	enter    = "Enter"
	esc      = "Esc"
	issuedID = "admin-reset-secret"
)

// ResetSheet is «Reset a password» over Users, /admin/reset
// (PasswordResetSheet, s-admin-24, -13; refused, -16, -20): whose — the
// administrator themselves and a disabled account off with their reasons —,
// what a reset destroys, in alarm; the temporary password and what to do
// with it; the login typed to confirm; Reset password and Cancel. Its
// answer is the password shown once (s-admin-14): the login and the
// password in one block, Copy, and Done.
func ResetSheet(p *view.PasswordResetSheet) w.Sheet {
	s := w.Sheet{Icon: "shield", Tone: w.SheetIconDanger, Title: string(p.Title), Width: w.SheetWidth480}
	if iss := p.Issued; iss != nil {
		secret := "login     " + string(iss.Login) + "\npassword  " + string(iss.Password)
		row := w.CardRow{Kind: w.CardTop8, Gap: w.LineGapNormal, Items: []w.CardRowPart{
			w.Button{Label: string(iss.Copy.Label), Icon: "dup", Kind: w.ButtonPrimary, CopyFrom: issuedID, Copied: copied},
		}}
		if iss.Note != "" {
			row.Items = append(row.Items, w.Dim{Parts: []w.Inline{w.Text(iss.Note)}, Size: w.Dim11_5, Variant: w.DimWrap})
		}
		s.Body = []w.SheetPart{
			bind.Msg(view.Notice{Runs: iss.Message}, w.MsgFlush),
			w.Card{Variant: w.CardChosenFlushGrow, Items: []w.CardPart{
				w.Rubric{Text: tempPassword, Items: []w.RubricPart{w.Mono{Text: onlyNow, Variant: w.MonoTightDanger}}},
				w.Diag{Text: secret, Variant: w.DiagInk, ID: issuedID},
				row,
			}},
		}
		s.Foot = []w.SheetFootPart{bind.Button(iss.Done, ""), w.Kbd{Keys: enter}}
		return s
	}
	s.Action, s.Method, s.Fields = p.Form.Action, p.Form.Method, bind.Hidden(p.Form)
	for i, r := range bind.Rows(p.Form) {
		s.Body = append(s.Body, bind.Field(r, w.FieldW100))
		if i == 0 && p.Warning != "" {
			// What a reset destroys, under whose it is (s-admin-13).
			s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Warning)}, Tone: w.MsgAlert, Space: w.MsgFlush})
		}
	}
	s.Foot = sheetFoot(p.Actions, p.Form)
	return s
}

// DeleteSheet is the deletion of an account over Users, /admin/delete
// (AccountDeleteSheet, s-admin-34): what Carrel forgets and what stays on
// the servers, in alarm; the login typed to confirm; Delete account and
// Cancel.
func DeleteSheet(p *view.AccountDeleteSheet) w.Sheet {
	s := w.Sheet{Icon: "trash", Tone: w.SheetIconDanger, Title: string(p.Title), Width: w.SheetWidth480,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	if p.Warning != "" {
		s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Warning)}, Tone: w.MsgAlert, Space: w.MsgFlush})
	}
	for _, r := range bind.Rows(p.Form) {
		s.Body = append(s.Body, bind.Field(r, w.FieldW100))
	}
	s.Foot = sheetFoot(p.Actions, p.Form)
	return s
}

// sheetFoot is a sheet's answers: the one that sends the sheet's form
// first, Cancel pushed right with Esc.
func sheetFoot(actions []view.Action, f view.Form) []w.SheetFootPart {
	var foot []w.SheetFootPart
	for _, a := range actions {
		if a.Method == "GET" {
			foot = append(foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: esc}}, Gap: w.LineGapNormal, Place: w.Pushed})
			continue
		}
		foot = append(foot, submit(a, f))
	}
	return foot
}
