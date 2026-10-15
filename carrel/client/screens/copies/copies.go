// Package copies is the sheet that asks which copy of a linked record an
// action goes to, /app/{kind}/{account}/{col}/{uid}/copies (Edit) and
// /app/tasks/{account}/{col}/{uid}/done (a task's tick in a list), drawn by
// the fragments «Связанная запись · какую копию править» (s-contacts-32)
// and «Связанная задача · какую копию отметить» (s-tasks-16): no frame
// draws it at its address yet (Typecase's task 183), and the sheet guard
// holds it by the fragments alone (U-6.16). The screen under the sheet is
// the page it was opened from (CopiesSheet.Under), drawn by that page's
// screen — the registry puts the two together (screens.over).
package copies

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3): the keys of the sheet's two answers, and
// what stands between what a copy differs in and why it cannot be chosen.
const (
	enter = "Enter"
	esc   = "Esc"
	sep   = " · "
)

// Sheet draws the sheet: a form (CopiesSheet.Form) that sends the copies
// chosen.
//
//   - The head: the icon — a pencil for Edit, a task for Mark done —, the
//     title, and what and how many copies, muted, pushed right (Subject).
//   - The note: what the action does to the copies (Note); then why the
//     sheet came back, an alarm (Form.Errors: Mark done with no copy
//     ticked — no frame draws it yet, Typecase's task 197).
//   - A line for each copy (the row «copy»): its collection's mark, its
//     tick — a radio for Edit, a box for Mark done —, the copy's
//     source as its words, and, muted, how it differs (Option.Meta). A copy
//     that cannot be chosen has no tick: its source muted, and after what it
//     differs in, why (Option.Reason).
//   - The foot: the form's button with Enter, Cancel with Esc.
func Sheet(p *view.CopiesSheet) w.Sheet {
	var icon w.Icon = "pencil"
	var body []w.SheetPart
	if p.Note != "" {
		body = append(body, w.SheetNote{Text: string(p.Note)})
	}
	for _, e := range p.Form.Errors {
		body = append(body, w.Msg{Parts: []w.MsgPart{w.Text(e.Message)}, Tone: w.MsgAlert, Space: w.MsgFlush})
	}
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			if r.Kind != view.InputRadio && r.Kind != view.InputCheck {
				continue
			}
			for _, o := range r.Options {
				body = append(body, line(r, o))
			}
		}
	}
	var foot []w.SheetFootPart
	for _, a := range p.Actions {
		b := bind.Button(a, "")
		if a.Primary {
			if a.Method == "POST" {
				icon = "task"
			}
			foot = append(foot, b, w.Kbd{Keys: enter})
			continue
		}
		foot = append(foot, w.Line{Items: []w.LinePart{b, w.Kbd{Keys: esc}}, Gap: w.LineGap7, Place: w.Pushed})
	}
	return w.Sheet{
		Icon: icon, Title: string(p.Title),
		Head:   []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}},
		Body:   body,
		Foot:   foot,
		Width:  w.SheetWidth520,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form),
	}
}

// line is a copy of the row r (s-contacts-32, s-tasks-16).
func line(r view.FormRow, o view.Option) w.Line {
	items := []w.LinePart{w.Mark{Colour: string(o.Colour), Size: w.MarkH11}}
	aside := string(o.Meta)
	if o.Disabled {
		items = append(items, w.Dim{Parts: []w.Inline{w.Text(o.Label)}})
		if o.Reason != "" {
			if aside != "" {
				aside += sep
			}
			aside += string(o.Reason)
		}
	} else {
		items = append(items, w.Check{On: o.Selected, Name: r.Name, Value: o.Value, Radio: r.Kind == view.InputRadio,
			Parts: []w.CheckPart{w.Text(o.Label)}})
	}
	if aside != "" {
		items = append(items, w.Dim{Parts: []w.Inline{w.Text(aside)}, Size: w.Dim11_5})
	}
	return w.Line{Items: items, Gap: w.LineGap8, Block: true}
}
