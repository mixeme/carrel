// Package quick is the quick sheet of a new record (U-6.16),
// /app/{kind}/{account}/{col}/new/quick and its way in from the top bar,
// /app/{kind}/quick, drawn by frames «Быстрая заметка поверх текущего
// экрана» (s-notes-5) and the items of New note's arrow (s-notes-8, -9, -10).
// The screen under the sheet is the page New was pressed on (NewSheet.Under),
// drawn by that page's screen — the registry puts the two together
// (screens.over).
package quick

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3): the title of «where», the mark of the
// collection last written into and of a read-only one, the cross, the keys of
// the sheet's answers.
const (
	writeInto = "Write into"
	lastUsed  = "last used"
	readOnly  = "ro"
	closeWord = "Close"
	enter     = "Enter"
	ctrlEnter = "Ctrl+Enter"
	esc       = "Esc"
)

// Sheet draws the sheet: a form (NewSheet.Form) that writes the record into
// the collection chosen in its head.
//
//   - The head: the kind's icon, the title («New note»), «where» pushed right
//     — the collection chosen with its mark, a menu of the kind's
//     collections, each a radio under Target.Field, the one last written into
//     first with «last used», a read-only one off with its reason and «ro»
//     —, and the cross.
//   - A note's body as one area; the other kinds' few fields in one row, the
//     first taking the rest of it: the days and the times as wide as they
//     are, All day under no label, the rest 200 wide; what the sheet does not
//     ask, muted, under them (Note).
//   - The foot: Save — the form's button — with Ctrl+Enter in a note's area,
//     Enter in the fields; Open the card, which carries what was typed into
//     the full form; Cancel with Esc.
//
// Nowhere to write (s-notes-6): no «where» and no fields, why in the body
// (Note), «Connect account» and Cancel; the sheet is 480 wide.
func Sheet(p *view.NewSheet) w.Sheet {
	s := w.Sheet{Icon: bind.Icon(p.Icon), Title: string(p.Title)}
	// The cross goes where Cancel goes: the form's, else the sheet's.
	closer := w.Close{Label: closeWord, Href: cancelHref(append(p.Form.Actions, p.Actions...))}
	if len(p.Target.Options) == 0 {
		s.Width = w.SheetWidth480
		closer.Place = w.Pushed
		s.Head = []w.SheetHeadPart{closer}
		if p.Note != "" {
			s.Body = []w.SheetPart{w.Msg{Parts: []w.MsgPart{w.Text(p.Note)}, Space: w.MsgFlush}}
		}
		s.Foot = foot(p.Actions, "")
		return s
	}
	s.Head = []w.SheetHeadPart{where(p.Target), closer}
	if p.Form.Action == "" {
		// «Where» first, nothing to fill in yet (U-6.16): no frame draws it.
		s.Foot = foot(p.Actions, "")
		return s
	}
	s.Action, s.Method, s.Fields = p.Form.Action, p.Form.Method, bind.Hidden(p.Form)
	keys := enter
	var fields []w.Field
	for _, r := range bind.Rows(p.Form) {
		if r.Kind == view.InputArea {
			s.Body = append(s.Body, w.Area{Name: r.Name, Text: string(r.Value), Variant: w.AreaBodySm, Focus: r.Focus, Label: string(r.Label)})
			keys = ctrlEnter
			continue
		}
		fields = append(fields, field(r, len(fields) == 0))
	}
	if len(fields) > 0 {
		s.Body = append(s.Body, w.SheetFields{Items: fields})
	}
	if p.Note != "" {
		s.Body = append(s.Body, w.Dim{Parts: []w.Inline{w.Text(p.Note)}, Size: w.Dim11_5, Variant: w.DimWrap})
	}
	s.Foot = foot(p.Form.Actions, keys)
	return s
}

// cancelHref is where the sheet's Cancel goes: back to the page under it.
func cancelHref(actions []view.Action) string {
	for _, a := range actions {
		if a.Method == "GET" && !a.Primary {
			return a.Href
		}
	}
	return ""
}

// where is the head's choice of the collection: the chosen one's server and
// name with its mark, and the menu of all of them under it.
func where(t view.TargetPicker) w.Drop {
	pick := w.Pick{Label: writeInto, Variant: w.PickT12W200}
	menu := w.Menu{Width: w.MenuWidth290, Pop: w.MenuUnderEnd}
	for _, o := range t.Options {
		it := w.MenuItem{Mark: &w.Mark{Colour: string(o.Colour)}, Label: string(o.Stamp)}
		switch {
		case o.ReadOnly:
			it.Reason, it.Trail = string(o.Reason), w.Badge{Label: readOnly, Place: w.Pushed}
		case t.Field != "":
			it.Name, it.Value = t.Field, o.Value
		default:
			it.Href = o.ID
		}
		if o.Last {
			it.Trail = w.Mono{Text: lastUsed, Variant: w.Mono10Dusty, Place: w.Pushed}
		}
		if o.ID == t.Chosen {
			it.On = true
			pick.Value, pick.Mark = string(o.Stamp), &w.Mark{Colour: string(o.Colour)}
		}
		menu.Items = append(menu.Items, it)
	}
	if pick.Value == "" {
		pick.Value, pick.Unset = writeInto, true
	}
	return w.Drop{Trigger: pick, Menu: menu, Place: w.Pushed}
}

// field is a row of the sheet as its field in the row: the first takes the
// rest of it, a day and a time are as wide as they are, a tick stands under
// no label, the others are 200 wide.
func field(r view.FormRow, first bool) w.Field {
	switch {
	case first:
		f := bind.Field(r, 0)
		f.Place = w.Grown
		return f
	case r.Kind == view.InputDate:
		return bind.Field(r, w.FieldFix130)
	case r.Kind == view.InputTime:
		return bind.Field(r, w.FieldFix96)
	case r.Kind == view.InputCheck:
		return w.Field{Label: " ", Parts: []w.FieldPart{bind.Check(r)}, Width: w.FieldFill120}
	}
	return bind.Field(r, w.FieldFill200)
}

// foot is the sheet's foot: the main answer with its keys, when it has them,
// the others after it, the way out pushed right with Esc.
func foot(actions []view.Action, keys string) []w.SheetFootPart {
	var out []w.SheetFootPart
	for _, a := range actions {
		switch {
		case a.Method == "GET" && !a.Primary:
			out = append(out, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: esc}}, Gap: w.LineGap7, Place: w.Pushed})
		case a.Method == "POST":
			b := bind.Submit(a)
			b.To = a.Href
			switch {
			case !a.Enabled:
				b.State, b.Reason = w.ButtonDisabled, string(a.Reason)
			case a.Waits:
				// Save while «where» is not chosen (s-notes-34): the
				// browser does not send the form without it.
				b.State, b.Reason = w.ButtonWaits, string(a.Reason)
			}
			if a.Primary && keys == ctrlEnter {
				b.Key = ctrlEnter // U-7: Save a quick note
			}
			out = append(out, b)
			if a.Primary && keys != "" {
				out = append(out, w.Kbd{Keys: keys})
			}
		default:
			out = append(out, bind.Button(a, ""))
		}
	}
	return out
}
