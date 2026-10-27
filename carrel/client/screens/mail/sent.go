package mail

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The names of the contract the result places by (C-7.5): the letter again
// to the refused, Close.
const (
	againID  = "mail.again"
	closeID  = "mail.close"
	saidDash = " — "
)

// Sent draws a letter that went to some (s-mail-18): the head of the
// letter, its alarm that sending again goes only to the refused, a line a
// recipient — the book's mark, the name, the address muted, «sent» strong or
// what the relay said in alarm —; «Send again to the refused» with Enter,
// Close with Esc pushed.
func Sent(p *view.MailSentSheet) w.Sheet {
	body := []w.SheetPart{w.Msg{Parts: []w.MsgPart{w.Text(p.Notice)}, Tone: w.MsgAlert, Space: w.MsgFlushSides}}
	for _, r := range p.Rows {
		var line []w.LinePart
		if r.Colour != "" {
			line = append(line, w.Mark{Colour: string(r.Colour), Size: w.MarkH11})
		}
		if r.Name != "" {
			line = append(line, w.Text(string(r.Name)+" "), w.Dim{Parts: []w.Inline{w.Text(sep[1:] + string(r.Address))}})
		} else {
			line = append(line, w.Text(r.Address))
		}
		line = append(line, w.Text(saidDash))
		if r.Refused {
			line = append(line, w.Faced{Face: w.FaceDanger, Items: []w.LinePart{w.Text(r.Said)}})
		} else {
			line = append(line, w.Strong(r.Said))
		}
		body = append(body, w.Line{Items: line, Gap: w.LineGap6, Block: true})
	}
	var foot []w.SheetFootPart
	var closing view.Action
	for _, a := range p.Actions {
		switch a.ID {
		case againID:
			foot = append(foot, bind.Button(a, ""), w.Kbd{Keys: enterKey})
		case closeID:
			closing = a
			foot = append(foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGap7, Place: w.Pushed})
		}
	}
	return w.Sheet{
		Icon: mailIcon, Title: letterTitle,
		Head:  []w.SheetHeadPart{w.Tag{Label: fromWord + string(p.From), On: true, Place: w.Pushed}, w.Close{Label: closeWord, Href: closing.Href}},
		Body:  body,
		Foot:  foot,
		Width: w.SheetWidth680,
	}
}
