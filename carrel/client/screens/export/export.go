// Package export is the sheet that asks which collection a combined view
// hands over, /app/{kind}/export, drawn by frame «Экспорт из объединённого
// вида · лист» (s-agenda-30). The screen under the sheet is the page it was
// opened from (ExportSheet.Under), drawn by that page's screen — the registry
// puts the two together (screens.over).
package export

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3): the keys of the sheet's two answers.
const (
	enter = "Enter"
	esc   = "Esc"
)

// Sheet draws the sheet: a GET form (ExportSheet.Form) that sends the
// collection chosen and, for a calendar, what of it goes in.
//
//   - The head: the icon, the title («Export»), where it is opened from,
//     muted, pushed right (Subject).
//   - The collections of the kind under the rubric («Which calendar»), each
//     a radio under Target.Field with its mark and its server monospaced; one
//     whose server does not answer off, with why in its server's place; the
//     chosen one on.
//   - A calendar's span: the whole of it or days, as segments (the whole one
//     turns the days off), and the two days side by side.
//   - The foot: Download — the form's button, the extension monospaced — with
//     Enter, Cancel with Esc.
func Sheet(p *view.ExportSheet) w.Sheet {
	tree := w.Tree{}
	for i, o := range p.Target.Options {
		trail := string(o.Stamp)
		if o.ReadOnly {
			trail = string(o.Reason)
		}
		row := w.TreeRow{Mark: &w.Mark{Colour: string(o.Colour)}, Label: string(o.Label), Unavailable: o.ReadOnly,
			Send: w.Send{Name: p.Target.Field, Value: o.Value, One: true}, Trail: w.Mono{Text: trail, Variant: w.Mono10Dusty}}
		if o.ID == p.Target.Chosen {
			tree.On = i + 1
		}
		tree.Rows = append(tree.Rows, row)
	}
	body := []w.SheetPart{w.Rubric{Text: string(p.Rubric)}, tree}
	var days []w.LinePart
	for _, r := range bind.Rows(p.Form) {
		switch r.Kind {
		case view.InputRadio:
			seg := w.Seg{Name: r.Name, Variant: w.SegStart}
			for _, o := range r.Options {
				seg.Options = append(seg.Options, w.SegOption{Label: string(o.Label), Value: o.Value, On: o.Selected, Disables: o.Disables})
			}
			body = append(body, w.Field{Label: string(r.Label), Parts: []w.FieldPart{seg}, Width: w.FieldW100})
		case view.InputDate:
			days = append(days, bind.Field(r, w.FieldSM))
		default:
			body = append(body, bind.Field(r, w.FieldW100))
		}
	}
	if len(days) > 0 {
		body = append(body, w.Line{Items: days, Gap: w.LineGap10, Block: true})
	}
	var foot []w.SheetFootPart
	for _, a := range p.Actions {
		if a.Primary {
			foot = append(foot, download(a), w.Kbd{Keys: enter})
			continue
		}
		foot = append(foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: esc}}, Gap: w.LineGap7, Place: w.Pushed})
	}
	s := w.Sheet{
		Icon: "export", Title: string(p.Title),
		Body:   body,
		Foot:   foot,
		Width:  w.SheetWidth520,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form),
	}
	if p.Subject != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	return s
}

// download is «Download ‹.ics›» (s-agenda-30): the extension after the
// words monospaced and small.
func download(a view.Action) w.Button {
	b := bind.Button(a, "")
	label := string(a.Label)
	for i := 1; i < len(label); i++ {
		if label[i] == '.' && label[i-1] == ' ' {
			b.Label = label[:i-1]
			b.Aside = &w.Mono{Text: label[i:], Variant: w.Mono11}
			break
		}
	}
	return b
}
