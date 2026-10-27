package mail

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the question before a letter with text closes (U-1.3).
const (
	closeTitle = "Close the letter?"
	discardID  = "mail.discard"
	keepID     = "mail.keep"
)

// CloseTitle is the sheet's title and its page's (s-mail-15).
const CloseTitle = closeTitle

// Close draws «Close the letter?» (s-mail-15): Carrel keeps no drafts, so
// closing loses the text.
//
//   - The head: the icon, the question, the address it goes from muted.
//   - Why, the words that it is lost strong.
//   - The foot: «Close and lose the text», which sends the sheet's form —
//     no key: it throws away —, «Keep writing» with Esc pushed, back to the
//     letter as it was.
func Close(p *view.MailCloseSheet) w.Sheet {
	var foot []w.SheetFootPart
	var keep w.LinePart
	for _, a := range p.Form.Actions {
		switch a.ID {
		case discardID:
			foot = append(foot, bind.Submit(a))
		case keepID:
			keep = bind.Button(a, "")
		}
	}
	var why []w.MsgPart
	for _, in := range bind.Inlines(p.Warning) {
		if mp, ok := in.(w.MsgPart); ok {
			why = append(why, mp)
		}
	}
	if keep != nil {
		foot = append(foot, w.Line{Items: []w.LinePart{keep, w.Kbd{Keys: escKey}}, Gap: w.LineGap7, Place: w.Pushed})
	}
	return w.Sheet{
		Icon: mailIcon, Title: closeTitle,
		Head:   []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(fromWord + string(p.From))}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}},
		Body:   []w.SheetPart{w.Msg{Parts: why, Space: w.MsgFlush}},
		Foot:   foot,
		Width:  w.SheetWidth480,
		Action: p.Form.Action, Fields: bind.Hidden(p.Form),
	}
}
