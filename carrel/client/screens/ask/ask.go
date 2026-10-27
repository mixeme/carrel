// Package ask is the question asked before a save is written (AskSheet; the
// owner's decision А1 of 11.10.2026): the series whose meetings changed by
// hand the edit leaves without their day, drawn by the fragments «Вопрос
// перед сохранением серии» (s-agenda-22), «Одна встреча без своего дня»
// (s-agenda-23) and «Записано не всё» (s-agenda-24). It is the answer of
// the edit's POST: no frame draws it at an address of its own yet
// (Typecase's task 184), and the sheet guard holds it by the fragments
// alone (U-6.16). The screen under the sheet is the edit as typed, or the
// series' card once it is written (AskSheet.Under) — the registry puts the
// two together (screens.over).
package ask

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3): the keys of the sheet's answers.
const (
	enter = "Enter"
	esc   = "Esc"
)

// Sheet draws the sheet: a form (AskSheet.Form) that carries the edit as
// typed, hidden, and whose buttons are the answers.
//
//   - The head: the icon, the question, and what and in which state, muted,
//     pushed right (Subject: «· not saved yet», «· saved in part»).
//   - Each block — a rubric and its lines: what is in bold, the aside
//     muted after it (Blocks).
//   - The alarm (Alert), and the line that says what each answer does
//     (Note).
//   - The foot: the answers in their order — the first the primary, with
//     Enter; the last (Cancel, «Leave it in the
//     series») pushed right with Esc. Each sends the form to its own
//     address, or leads where it says.
func Sheet(p *view.AskSheet) w.Sheet {
	var body []w.SheetPart
	for _, b := range p.Blocks {
		block := w.SheetBlock{Title: w.Rubric{Text: string(b.Title), Variant: w.RubricBottom7}}
		for _, l := range b.Lines {
			block.Items = append(block.Items, w.SheetLine{Name: string(l.Text), Note: w.Dim{Parts: []w.Inline{w.Text(l.Aside)}}})
		}
		body = append(body, block)
	}
	if p.Alert != "" {
		body = append(body, w.Msg{Parts: []w.MsgPart{w.Text(p.Alert)}, Tone: w.MsgAlert, Space: w.MsgFlush})
	}
	if p.Note != "" {
		body = append(body, w.SheetNote{Text: string(p.Note)})
	}
	var foot []w.SheetFootPart
	for i, a := range p.Actions {
		b := bind.Button(a, "")
		switch {
		case i == len(p.Actions)-1 && i > 0:
			foot = append(foot, w.Line{Items: []w.LinePart{b, w.Kbd{Keys: esc}}, Gap: w.LineGap7, Place: w.Pushed})
		case a.Primary:
			foot = append(foot, b, w.Kbd{Keys: enter})
		default:
			foot = append(foot, b)
		}
	}
	return w.Sheet{
		Icon: bind.Icon(p.Icon), Title: string(p.Title),
		Head:   []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}},
		Body:   body,
		Foot:   foot,
		Width:  w.SheetWidth520,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form),
	}
}
