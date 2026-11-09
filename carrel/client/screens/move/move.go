// Package move is the sheet that moves or copies a record,
// /app/{kind}/{account}/{col}/{uid}/move, drawn by frame «Контакт ·
// переместить в другую книгу» (s-contacts-2): the draft frame2go printed for
// it, its sheet become the view model's (U-5.1, U-6.16). The screen under
// the sheet is the page it was opened from (MoveSheet.Under), drawn by that
// page's screen — the registry puts the two together (screens.over).
package move

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3): the rubric over the collections, the keys of
// the sheet's two answers.
const (
	into  = "Write into"
	enter = "Enter"
	esc   = "Esc"
)

// Sheet draws the sheet: a form (MoveSheet.Form) that sends the collection
// chosen.
//
//   - The head: the icon, the title («Move contact»), and what and from
//     where, muted, pushed right (Subject).
//   - The collections of the kind under «Write into», each a radio under
//     Target.Field with its mark: the one it is in — «already there»,
//     monospaced —, a read-only one with its badge, the others with their
//     server; the chosen one on. Each radio's address is the sheet with it
//     chosen, which says what the move then does (TargetOption.Href).
//   - What the person would not think of, when there is such (Warning; Р-77:
//     a copy under a new UID), and the line about the attendees where the
//     server may tell them (Cancels).
//   - The foot: Move — the form's button — with Enter, Cancel with Esc.
func Sheet(p *view.MoveSheet) w.Sheet {
	tree := w.Tree{}
	for i, o := range p.Target.Options {
		row := w.TreeRow{Mark: &w.Mark{Colour: string(o.Colour)}, Label: string(o.Label), Unavailable: o.ReadOnly,
			Send: w.Send{Name: p.Target.Field, Value: o.Value, One: true}, Href: o.Href}
		switch {
		case o.Here:
			row.Trail = w.Mono{Text: string(o.Reason), Variant: w.Mono10Dusty}
		case o.ReadOnly:
			row.Trail = w.Badge{Label: string(o.Reason)}
		default:
			row.Trail = w.Mono{Text: string(o.Stamp), Variant: w.Mono10Dusty}
		}
		if o.ID == p.Target.Chosen {
			tree.On = i + 1
		}
		tree.Rows = append(tree.Rows, row)
	}
	body := []w.SheetPart{w.Rubric{Text: into}, tree}
	if len(p.Warning) > 0 {
		body = append(body, warning(p.Warning))
	}
	if p.Cancels != "" {
		body = append(body, w.Msg{Parts: []w.MsgPart{w.Text(p.Cancels)}, Space: w.MsgFlushTop2})
	}
	var foot []w.SheetFootPart
	for _, a := range p.Actions {
		b := bind.Button(a, "")
		if a.Primary {
			foot = append(foot, b, w.Kbd{Keys: enter})
			continue
		}
		foot = append(foot, w.Line{Items: []w.LinePart{b, w.Kbd{Keys: esc}}, Gap: w.LineGapNormal, Place: w.Pushed})
	}
	return w.Sheet{
		Icon: "move", Title: string(p.Title),
		Head:   []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}},
		Body:   body,
		Foot:   foot,
		Width:  w.SheetWidth520,
		Action: p.Form.Action, Fields: bind.Hidden(p.Form),
	}
}

// warning is what the move will do, its pieces in their faces — MOVE as
// code (s-contacts-2).
func warning(r view.Runs) w.Msg {
	var parts []w.MsgPart
	for _, in := range bind.Inlines(r) {
		if m, ok := in.(w.Mono); ok {
			m.Form = w.MonoCode
			in = m
		}
		if mp, ok := in.(w.MsgPart); ok {
			parts = append(parts, mp)
		}
	}
	return w.Msg{Parts: parts, Space: w.MsgFlushTop2}
}
