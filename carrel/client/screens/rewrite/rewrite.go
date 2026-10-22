// Package rewrite is the sheet that asks whether to rewrite a contact's old
// addresses in the records that name them, /app/contacts/{account}/{col}/
// {uid}/rewrite, drawn by frame «Контакт · адрес заменён — переписать
// участников?» (s-contacts-15): the draft frame2go printed for it, its sheet
// become the view model's (U-6.16, S-15.4). One sheet at any number of
// replacements (Typecase's note 74): a choice per old address, a tick per
// group of risk. The screen under it is the card's edit page, the card as
// it was just saved (RewriteSheet.Under).
package rewrite

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3): the rubrics, the arrow of a choice, the keys
// of the sheet's two answers.
const (
	address   = "The address"
	addresses = "The addresses"
	setOff    = "What rewriting can set off"
	arrow     = "→"
	enter     = "Enter"
	esc       = "Esc"
)

// Sheet draws the sheet: a form (RewriteSheet.Form) that sends the choices
// and the ticks.
//
//   - The head: the icon, the question (Title), and whose card was saved,
//     muted, pushed right (Subject).
//   - Under «The address» (more than one — «The addresses»), each old
//     address: monospaced, how many records name it, muted; under it «→»
//     and the choice — the card's addresses now and «Do not rewrite»
//     (Pairs).
//   - Under «What rewriting can set off», each group of risk: its tick, its
//     records behind a button that counts them (each a link with its mark
//     and day), and what it may set off, muted (Groups); the line about a
//     record that stands in two groups (Both).
//   - What keeps the old address whatever is chosen, and that the unticked
//     collections were not looked in (KeptNote, Note).
//   - The records the button counts as the form changes (Tally).
//   - The foot: Rewrite N records — the form's button, counting — with
//     Enter; Leave as is — the form sent to its own address — with Esc.
func Sheet(p *view.RewriteSheet) w.Sheet {
	s := w.Sheet{Icon: "refresh", Title: string(p.Title), Width: w.SheetWidth520,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	if p.Subject != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	if len(p.Pairs) > 0 {
		rubric := address
		if len(p.Pairs) > 1 {
			rubric = addresses
		}
		s.Body = append(s.Body, w.Rubric{Text: rubric})
	}
	for _, pr := range p.Pairs {
		from := w.Line{Items: []w.LinePart{w.Mono{Text: string(pr.From)}}, Gap: w.LineGap8, Block: true}
		if pr.Note != "" {
			from.Items = append(from.Items, w.Dim{Parts: []w.Inline{w.Text(pr.Note)}, Size: w.Dim11_5})
		}
		pick := bind.Pick(pr.Choice)
		pick.Variant = w.PickT12W240
		s.Body = append(s.Body, from, w.Line{Items: []w.LinePart{w.Mono{Text: arrow}, pick}, Gap: w.LineGap8, Block: true})
	}
	if len(p.Groups) > 0 {
		s.Body = append(s.Body, w.Rubric{Text: setOff})
	}
	for _, g := range p.Groups {
		line := w.Line{Items: []w.LinePart{w.Check{On: g.On, Name: g.Name, Value: g.Value, Parts: []w.CheckPart{w.Text(g.Label)}}}, Gap: w.LineGap8, Block: true}
		if len(g.Records) > 0 {
			menu := w.Menu{Width: w.MenuWidth290, Pop: w.MenuUnder}
			for _, r := range g.Records {
				it := w.MenuItem{Mark: &w.Mark{Colour: string(r.Colour)}, Label: string(r.Label), Href: r.Href}
				if r.Value != "" {
					it.Detail = &w.Mono{Text: string(r.Value), Variant: w.Mono11}
				}
				menu.Items = append(menu.Items, it)
			}
			line.Items = append(line.Items, w.Drop{Trigger: w.Button{Label: string(g.Count), Menu: true, Small: true}, Menu: menu})
		}
		s.Body = append(s.Body, line)
		if g.Note != "" {
			s.Body = append(s.Body, w.Dim{Parts: []w.Inline{w.Text(g.Note)}, Size: w.Dim11_5, Variant: w.DimWrap})
		}
	}
	if p.Both != "" {
		s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Both)}, Space: w.MsgFlush})
	}
	if note := join(p.KeptNote, p.Note); note != "" {
		s.Body = append(s.Body, w.SheetNote{Text: note})
	}
	if len(p.Tally) > 0 {
		s.Body = append(s.Body, w.Tally{Records: p.Tally})
	}
	for _, a := range p.Actions {
		if a.Primary {
			b := bind.Submit(a)
			b.Count, b.CountOne = string(p.Count), string(p.CountOne)
			if !a.Enabled {
				b.State, b.Reason = w.ButtonDisabled, string(a.Reason)
			}
			s.Foot = append(s.Foot, b, w.Kbd{Keys: enter})
			continue
		}
		b := bind.Submit(a)
		b.To = a.Href
		s.Foot = append(s.Foot, w.Line{Items: []w.LinePart{b, w.Kbd{Keys: esc}}, Gap: w.LineGap7, Place: w.Pushed})
	}
	return s
}

// join is two sentences of a line, either may be empty.
func join(a, b view.Text) string {
	switch {
	case a == "":
		return string(b)
	case b == "":
		return string(a)
	}
	return string(a) + " " + string(b)
}
