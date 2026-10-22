// Package remove is the sheet of deleting a record that has something to
// lose, /app/{kind}/{account}/{col}/{uid}/delete, drawn by frames «Удалить
// событие» (s-agenda-26) and «Удалить задачу» (s-tasks-18) — state frames
// over the record's card, so no draft of their own (U-6.16). The screen
// under the sheet is the card it was opened from (RecordDeleteSheet.Under),
// drawn by that card's screen — the registry puts the two together
// (screens.over).
package remove

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3): the rubrics of the lists, the key of Cancel.
const (
	pointing    = "What is pointing at it now"
	attachments = "Attachments"
	esc         = "Esc"
)

// Sheet draws the sheet: a form (RecordDeleteSheet.Form) that sends what it
// chose with the record.
//
//   - The head: the bin in alarm, the title («Delete event»), the record's
//     name, muted, pushed right (Subject).
//   - That it is deleted on the server and Carrel cannot undo it, in alarm
//     (Warning).
//   - What points at it — published links, related records — each named in
//     bold and what becomes of it muted (References); the line about the
//     attendees where the server may tell them (Cancels).
//   - Its attachments the same way (Attachments), then a tick for each file
//     of a link in the attachment folder: delete it too, unticked.
//   - Its copies in other collections under their rubric (the form's set
//     with a legend): each with its mark, a tick — the one opened ticked —,
//     and what it holds, muted.
//   - The foot: Delete — the form's button, the solid danger — and Cancel
//     with Esc.
//
// The sheet is 480 wide (s-tasks-18) unless it lists copies: their lines —
// a mark, a tick, what each holds — take 560 (s-agenda-26).
func Sheet(p *view.RecordDeleteSheet) w.Sheet {
	s := w.Sheet{Icon: "trash", Tone: w.SheetIconDanger, Title: string(p.Title), Width: w.SheetWidth480,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	if p.Subject != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	if p.Warning != "" {
		s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Warning)}, Tone: w.MsgAlert, Space: w.MsgFlush})
	}
	if len(p.References) > 0 {
		s.Body = append(s.Body, block(pointing, p.References))
	}
	if p.Cancels != "" {
		s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Cancels)}, Space: w.MsgFlush})
	}
	if len(p.Attachments) > 0 {
		s.Body = append(s.Body, block(attachments, p.Attachments))
	}
	for _, set := range p.Form.Sets {
		if set.Legend != "" {
			s.Body = append(s.Body, w.Rubric{Text: string(set.Legend)})
			s.Width = w.SheetWidth560
		}
		for _, r := range set.Rows {
			if r.Kind != view.InputCheck {
				continue
			}
			for _, o := range r.Options {
				c := w.Check{On: o.Selected, Parts: []w.CheckPart{w.Text(o.Label)}, Name: r.Name, Value: string(o.Value), Reason: string(o.Reason)}
				if o.Disabled && c.Reason == "" {
					c.Reason = string(o.Meta)
				}
				if set.Legend == "" {
					// A file of a link: «Delete agenda-10-08.pdf too».
					s.Body = append(s.Body, c)
					continue
				}
				// A copy: its mark, its tick, what it holds.
				line := w.Line{Items: []w.LinePart{w.Mark{Colour: string(o.Colour), Size: w.MarkH11}, c}, Gap: w.LineGap8}
				if o.Meta != "" {
					line.Items = append(line.Items, w.Dim{Parts: []w.Inline{w.Text(o.Meta)}, Size: w.Dim11_5})
				}
				s.Body = append(s.Body, line)
			}
		}
	}
	for _, a := range p.Actions {
		if a.Method == "GET" {
			s.Foot = append(s.Foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: esc}}, Gap: w.LineGap7, Place: w.Pushed})
			continue
		}
		s.Foot = append(s.Foot, bind.Submit(a))
	}
	return s
}

// block is a list under its rubric: each thing in bold, what becomes of it
// muted.
func block(title string, refs []view.Reference) w.SheetBlock {
	b := w.SheetBlock{Title: w.Rubric{Text: title, Variant: w.RubricBottom7}}
	for _, r := range refs {
		b.Items = append(b.Items, w.SheetLine{Name: string(r.What), Note: w.Dim{Parts: []w.Inline{w.Text("— " + string(r.Then))}}})
	}
	return b
}
