package bind

import (
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The blocks of a record and of its form that the event's and the task's
// screens draw first (C-6.5, C-6.8): the line under a head in its pieces,
// the people of an event, a form's tick, a row of days, a choice from a list
// that is a menu, a task's tick, the record a task is related to. The words
// the frame keeps the same whatever the data — the lead before a status,
// what a task's tick is called — come from the screen.

// Sub is a sentence of the model in its pieces (Runs) as a line under a
// head (HeadSub): the window of an agenda, the zone monospaced (s-agenda-1),
// a task's overdue in the danger's words (s-tasks-2). A sentence with
// nothing in it is no line: nil.
func Sub(r view.Runs) []w.HeadSub {
	return sub(r, func(t view.Text) w.HeadSubPart { return w.Span{Face: w.FaceDanger, Text: string(t)} })
}

// SourceSub is the line under a record's title that names where it lives:
// the collection's mark beside the collection's name, «▍nextcloud · Личный»
// (Typecase 434ac87, note 72) — the mark is the collection's, so it stands
// at its name, not at the record's title.
func SourceSub(colour view.Colour, stamp view.Text) w.HeadSub {
	var parts []w.HeadSubPart
	if colour != "" {
		parts = append(parts, w.Mark{Colour: string(colour), Size: w.MarkH11})
	}
	return w.HeadSub{Parts: append(parts, w.Text(stamp))}
}

// Marked puts the collection's mark in front of the first line under a
// record's title, the line that begins with where the record lives
// (s-tasks-2: «▍nextcloud · Дела · overdue by 3 days»).
func Marked(colour view.Colour, subs []w.HeadSub) []w.HeadSub {
	if colour == "" || len(subs) == 0 {
		return subs
	}
	out := append([]w.HeadSub(nil), subs...)
	out[0].Parts = append([]w.HeadSubPart{w.Mark{Colour: string(colour), Size: w.MarkH11}}, out[0].Parts...)
	return out
}

// SubAlert is a sentence of the model in its pieces as the line under a
// head, as Sub, whose alarm (RunDanger) is a badge in the alert's tone: the
// edit's «overdue by 3 days» (s-tasks-3).
func SubAlert(r view.Runs) []w.HeadSub {
	return sub(r, func(t view.Text) w.HeadSubPart { return w.Badge{Label: string(t), Tone: w.BadgeAlert} })
}

// sub is the line under a head in its pieces, the alarm as danger draws it.
func sub(r view.Runs, danger func(view.Text) w.HeadSubPart) []w.HeadSub {
	var parts []w.HeadSubPart
	for _, p := range r {
		switch p.Mark {
		case view.RunMono:
			parts = append(parts, w.Mono{Text: string(p.Text)})
		case view.RunStrong:
			parts = append(parts, w.Strong(p.Text))
		case view.RunDanger:
			parts = append(parts, danger(p.Text))
		default:
			parts = append(parts, w.Text(p.Text))
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return []w.HeadSub{{Parts: parts}}
}

// Tick is the tick of a task's row (C-6.4 TaskRow.Tick, .Done): ticked where
// the task is done, its accessible name label — the screen's word —;
// pressing it posts the action (Mark done, or not done) by the form of its
// ID (TickID), which the page prints at its end (ActionForm): without a
// script too (U-8). A row read only (no action) or an action switched off
// has a tick that is a sign: it looks as usual, is not pressed and sends
// nothing (Р-38 of Typecase, 80d1319: Tick.Sign, states-23).
func Tick(a *view.Action, done bool, label string) w.Tick {
	t := w.Tick{Done: done, Label: label}
	if a != nil && a.Enabled && a.Method == "POST" {
		t.Send = w.Send{Form: TickID(*a)}
	} else {
		t.Sign = true
	}
	return t
}

// TickID is the ID of the form a task's tick posts (Tick): its action's ID
// and address, one per row.
func TickID(a view.Action) string {
	return a.ID + a.Href
}

// Related is the field Related to of a task's and a note's form (C-6.8: an
// InputSelect several options of which may be chosen, Multiple; s-tasks-3,
// -10, -11) as a field of tags under the row's name: every record related —
// the label of each option selected, a record not loaded its UID — a tag
// the cross unticks, sent with its value (the UID). After the tags stands
// the line one more link is typed into (FormRow.Add, «related.add»,
// Typecase's task 142) — whether there are relations or not —, sent under
// its own name, its pattern and its accessible name its own, its
// suggestions where it says (Suggest, the library's live layer); without a
// script the records loaded and not yet related are its choices (Typecase's
// task 96) — each its UID sent, its label and what it is (Option.Meta:
// «event · 11 Aug») shown beside it; «By UID…» (Typed) is the line itself.
// A row without such a line has one only while nothing is related, under
// the field's own name. A relation to a loaded record carries the mark of
// that record's collection.
func Related(r view.FormRow) w.TagInput {
	in := w.TagInput{Name: r.Name}
	var choices []w.ListOption
	for _, o := range r.Options {
		switch {
		case o.Typed:
		case o.Selected:
			// A record among the loaded ones carries the mark of its
			// collection (Option.Colour; s-tasks-3, Typecase's note 46); one
			// that is not is its UID, and no collection is known for it.
			tag := w.Tag{Label: string(o.Label), Removable: true, Value: o.Value}
			if o.Colour != "" {
				tag.Mark = &w.Mark{Colour: string(o.Colour), Size: w.MarkH11}
			}
			in.Parts = append(in.Parts, tag)
		default:
			label := string(o.Label)
			if o.Meta != "" {
				label += runSep + string(o.Meta)
			}
			choices = append(choices, w.ListOption{Value: o.Value, Label: label})
		}
	}
	switch {
	case r.Add != nil:
		in.Add = &w.TagAdd{Name: r.Add.Name, Placeholder: string(r.Add.Placeholder), Label: string(r.Add.Label), Suggest: r.Add.Suggest, Options: choices}
	case len(in.Parts) == 0:
		in.Add = &w.TagAdd{Name: r.Name, Placeholder: string(r.Placeholder), Label: string(r.Label), Suggest: r.Suggest, Options: choices}
	}
	return in
}

// Attendee is a person of an event (C-6.5 AttendeeRow) as a line of the
// rubric of attendees, read only (s-agenda-1, -2): a person the ticked
// address books know by the address (Href) — the mark of the book, the name
// leading to the card —, a stranger — the address, monospaced; then what the
// person said (Status), muted after lead, the screen's.
func Attendee(a view.AttendeeRow, lead string) []w.SecItemPart {
	var out []w.SecItemPart
	if a.Href != "" {
		out = append(out, w.Mark{Colour: string(a.Colour), Size: w.MarkH11Inline}, w.Text(" "), w.Link{Text: string(a.Name), Href: a.Href})
	} else {
		out = append(out, w.Mono{Text: string(a.Email)})
	}
	if a.Status != "" {
		out = append(out, w.Text(" "), w.Dim{Parts: []w.Inline{w.Text(lead + string(a.Status))}})
	}
	return out
}

// Person is a person of a task or a note (C-6.5 AttendeeRow; Р3 of the
// server's kinds-parity list) as a line drawn the way Related draws a
// record — no frame draws a task's or a note's people yet (Typecase's task
// 179), and a rubric beside the frame's own passes only in their shape: the
// name in the accent, leading to the card when the ticked books know it,
// else the address; what the person said muted after lead, the screen's.
func Person(a view.AttendeeRow, lead string) []w.SecItemPart {
	name := string(a.Name)
	if a.Href == "" {
		name = string(a.Email)
	}
	return []w.SecItemPart{
		w.Span{Face: w.FaceAccent, Text: name, Href: a.Href},
		w.Text(" "),
		w.Dim{Parts: []w.Inline{w.Text(lead + string(a.Status))}},
	}
}

// Attendees is the field of the attendees of an event's form (C-6.8
// AttendeeField, s-agenda-3, -12 … -15) as a field of tags in columns: a tag
// for each person — one the ticked books know by its name and the mark of
// its book, a stranger by the address, monospaced; what the person said
// muted after lead, the screen's; the address where a pointer rests. The
// organiser stays (Fixed) and goes as a hidden field; every other is a
// ticked box the cross unticks, going as the field's Name with its value
// («mailto:…»); the line the next is typed in (Add) sends what is typed
// under its name, and names where the suggestions come from (Suggest, the
// library's live layer). A person invited, not the organiser (ReadOnly),
// sees the tags without crosses and sends none of them, and no line.
func Attendees(f view.AttendeeField, lead string) w.TagInput {
	in := w.TagInput{Columns: true}
	if !f.ReadOnly {
		in.Name = f.Name
	}
	for _, a := range f.Rows {
		t := w.Tag{Title: string(a.Email)}
		if a.Href != "" {
			t.Label, t.Mark = string(a.Name), &w.Mark{Colour: string(a.Colour), Size: w.MarkH11}
		} else {
			t.Address = &w.Mono{Text: string(a.Email)}
		}
		if a.Status != "" {
			t.Aside = &w.Dim{Parts: []w.Inline{w.Text(lead + string(a.Status))}}
		}
		switch {
		case f.ReadOnly:
			t.Fixed = true
		case a.Fixed:
			t.Fixed, t.Value = true, a.Value
		default:
			t.Removable, t.Value = true, a.Value
		}
		in.Parts = append(in.Parts, t)
	}
	if f.Add != nil && !f.ReadOnly {
		in.Add = &w.TagAdd{
			Name:        f.Add.Name,
			Placeholder: string(f.Add.Placeholder),
			Label:       string(f.Add.Label),
			Value:       string(f.Add.Value),
			Suggest:     f.Suggest,
		}
	}
	return in
}

// Check is a tick of a form (C-6.8 InputCheck with no options: All day) as
// a checkbox: its name, ticked where the row's value is set, its label its
// words; the field it switches off while ticked (Disables). It sends what a
// browser sends for a ticked box, which the server reads as on (C-6.8).
// A label in pieces (LabelRuns: «Allow ‹.ics› download…», s-publish-2) keeps
// its monospaced piece.
func Check(r view.FormRow) w.Check {
	parts := []w.CheckPart{w.Text(r.Label)}
	if len(r.LabelRuns) > 0 {
		parts = parts[:0]
		for _, p := range r.LabelRuns {
			if p.Mark == view.RunMono {
				parts = append(parts, w.Mono{Text: string(p.Text)})
			} else {
				parts = append(parts, w.Text(p.Text))
			}
		}
	}
	// What the tick means besides its words, muted after them
	// (LabelAside: «· VTODO — not every client understands them»,
	// s-davloom-2).
	if r.LabelAside != "" {
		if t, ok := parts[len(parts)-1].(w.Text); ok {
			parts[len(parts)-1] = t + " "
		}
		parts = append(parts, w.Dim{Parts: []w.Inline{w.Text(r.LabelAside)}})
	}
	// Off now with its reason (FormRow.Reason: letters from users without
	// mail, s-admin-3): sends nothing, says why.
	return w.Check{On: r.Value != "", Name: r.Name, Parts: parts, Disables: r.Disables, Reason: string(r.Reason)}
}

// Days is a tick of a form with several options (C-6.8 InputCheck with
// Options: the days a series repeats on, «On») as one row of them (Seg,
// Multiple — Typecase's task 72): each option ticked where it is selected,
// each ticked one going under the row's name with its value.
func Days(r view.FormRow) w.Seg {
	s := w.Seg{Name: r.Name, Multiple: true}
	for _, o := range r.Options {
		s.Options = append(s.Options, w.SegOption{Label: string(o.Label), Value: o.Value, On: o.Selected, Disables: o.Disables})
	}
	return s
}

// PickMenu is a choice from a long list (C-6.8 InputSelect whose options
// are grouped: the time zone, s-agenda-3, -8, -15) as a choice that opens its
// menu: the chosen option's words, and after them what the model says of it
// muted (ValueAside, «as sent»); the menu — the line that finds an option
// (Placeholder, its words its accessible name too), then each option a radio
// item of the form it stands in under the row's name, the one chosen on, a
// rubric before an option that starts a group (Group, «As sent»), a rule
// before one that has it (Separator), one that cannot be chosen off with its
// reason. How wide the menu is and where it pops are the screen's.
func PickMenu(r view.FormRow) w.Drop {
	p := w.Pick{Label: string(r.Label), Focus: r.Focus}
	for _, o := range r.Options {
		if o.Selected {
			p.Value = string(o.Label)
		}
	}
	if r.ValueAside != "" {
		p.Aside = &w.Dim{Parts: []w.Inline{w.Text(r.ValueAside)}, Variant: w.DimIndent}
	}
	items := []w.MenuPart{w.Input{Kind: w.InputSearch, Placeholder: string(r.Placeholder), Label: string(r.Placeholder)}}
	for i, o := range r.Options {
		switch {
		case o.Group != "":
			items = append(items, w.Rubric{Text: string(o.Group), Form: w.RubricInline})
		case o.Separator && i > 0:
			items = append(items, w.MenuSep{})
		}
		it := w.MenuItem{Label: string(o.Label), On: o.Selected, Name: r.Name, Value: o.Value}
		if o.Disabled {
			it.Reason = string(o.Reason)
		}
		items = append(items, it)
	}
	return w.Drop{Trigger: p, Menu: w.Menu{Items: items}}
}
