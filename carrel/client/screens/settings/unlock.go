package settings

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The keys of a sheet's answers (U-1.3).
const (
	enterKey = "Enter"
)

// UnlockSheet is the password's sheet, /app/settings/unlock and
// /app/settings/escrow/deposit (UnlockSheet): what it is asked for in its
// title, whose on its right; the password and what the window opens; the
// action itself with Enter and Cancel with Esc pushed right (s-plan-3,
// s-davloom-5). Deleting the account
// says what goes, in alarm, with the bin (account-15); depositing the key
// what an administrator gets, with the shield, the password's hint under
// it (account-2).
func UnlockSheet(p *view.UnlockSheet) w.Sheet {
	s := w.Sheet{Title: string(p.Purpose), Width: w.SheetWidth400, Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	switch p.For {
	case view.UnlockForDelete:
		s.Icon, s.Tone = "trash", w.SheetIconDanger
	case view.UnlockForDeposit:
		s.Icon, s.Tone, s.Width = "shield", w.SheetIconInk, w.SheetWidth480
	}
	if p.Subject != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	if p.Alert != "" {
		s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Alert)}, Tone: w.MsgAlert, Space: w.MsgFlush})
	}
	for _, r := range bind.Rows(p.Form) {
		s.Body = append(s.Body, bind.Field(r, w.FieldW100))
	}
	if p.Warning != "" {
		s.Body = append(s.Body, w.Dim{Parts: []w.Inline{w.Text(p.Warning)}, Size: w.Dim11_5, Variant: w.DimWrap})
	}
	for _, a := range p.Form.Actions {
		if a.Method == "GET" {
			s.Foot = append(s.Foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGap7, Place: w.Pushed})
			continue
		}
		b := bind.Submit(a)
		if a.Href != "" && a.Href != p.Form.Action {
			b.To = a.Href
		}
		s.Foot = append(s.Foot, b, w.Kbd{Keys: enterKey})
	}
	return s
}
