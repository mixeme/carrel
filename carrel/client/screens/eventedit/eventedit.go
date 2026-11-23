// Package eventedit is the edit of an event,
// /app/calendar/{account}/{col}/{uid}/edit, and the full form of a new one,
// /app/calendar/{account}/{col}/new — one view model (EventEditPage) —, drawn
// by frame «Событие · правка, повторение» (s-agenda-3) of the mockup and the
// new event's form (s-agenda-15): the drafts frame2go printed for them, with
// the frames' data become the view model's (U-5.1, U-6.2). A form, and only a
// form: the event is edited in the zone it was written in, a series whole.
package eventedit

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The IDs of the page's forms: the event's, which Save in the head submits
// (Button.Form) and the line of Save names (Runbar.Progress) — the edit's
// and a new event's, as the frames name them (s-agenda-3, -15) —, and the
// deletion's, which posts with no form of its own.
const (
	formEdit   = "event-edit"
	formNew    = "event-new"
	formDelete = "delete"
)

// The screen's words (U-1.3) of the line of Save that sends files
// (Typecase ac658b0): what it says before the library's script moves it.
const (
	saving  = "Saving · "
	noBytes = "0 of 0 MB"
)

// The screen's words (U-1.3) for the shell: the mark, the search, the rail.
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
		NewKey: "n", SearchKey: "/",
		Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
		Keys:  bind.KeysWords{Label: "Keyboard shortcuts", Icon: "search", Key: "?", Sheet: "keys"},
	}
	rail = bind.RailWords{Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external"}
)

// The screen's words (U-1.3): the dash in an empty field the model gives no
// pattern for, the accessible name of the description's text, the label of
// the line of «Add reminder» (a space: the button stands under the labels),
// the lead of a status, the rubrics of the attendees, the attachments and
// what is kept, the attachment kept inside the event, a form with nothing
// attached.
const (
	none     = "—"
	text     = "Text"
	blank    = " "
	lead     = "· "
	people   = "Attendees"
	files    = "Attachments"
	kept     = "Kept as the server sent them"
	source   = "Source"
	path     = "Path"
	read     = "Read"
	sep      = " · "
	inside   = "in the event"
	nothing  = "Nothing attached"
	notKept  = "not kept"
	attachID = "attach.add"
)

// widths are the widths of the form's fields by their name, the last part
// that is no number (C-6.1: the view model carries no widths; the layout is
// the screen's, from its frame): «reminder.1» is a reminder.
var widths = map[string]w.FieldWidth{
	"summary": w.FieldXL, "location": w.FieldLG, "status": w.FieldSM, "categories": w.FieldMD, "description": w.FieldFull,
	// Link and Related to in «Other», as the task's (s-agenda-3, -15;
	// Typecase's tasks 160, 172); the Calendar last in the head (task 175).
	"url": w.FieldMD, "related": w.FieldLG, calendarRow: w.FieldMD,
	"start.date": w.FieldSM, "start.time": w.FieldXS, "end.date": w.FieldSM, "end.time": w.FieldXS, "tz": w.FieldMD,
	"reminder": w.FieldMD,
	// The numbers «Custom…» of a reminder opens (s-agenda-19).
	"reminder.count": w.FieldXS, "reminder.unit": w.FieldSM, "reminder.at": w.FieldXS,
	"repeat.freq": w.FieldSM, "repeat.interval": w.FieldXS, "repeat.until": w.FieldSM, "repeat.count": w.FieldXS, "repeat.rule": w.FieldXL,
}

// width is the width of a field by its name: the whole name, else the name
// without its parts that are a number or the template's «{n}» («reminder.0»
// is a reminder, «reminder.0.count» the number of its «Custom…»).
func width(name string) w.FieldWidth {
	if fw, ok := widths[name]; ok {
		return fw
	}
	kept, part := "", ""
	for i := 0; i <= len(name); i++ {
		if i < len(name) && name[i] != '.' {
			part += name[i : i+1]
			continue
		}
		if _, err := strconv.Atoi(part); err != nil && part != "{n}" {
			if kept != "" {
				kept += "."
			}
			kept += part
		}
		part = ""
	}
	if fw, ok := widths[kept]; ok {
		return fw
	}
	return w.FieldAuto
}

// saveKey is the key that presses Save in the editor (U-7).
const saveKey = "Ctrl+S"

// Screen draws the edit from its view model, in the state the model is in.
//
//   - The shell: the top bar; the rail of return — «Back to the card» (or
//     «Back to list» for a new event, with the phone's short «Back»), the
//     calendar's sources, the foot.
//   - The head: the event's title (a new one's «New event»), the calendar,
//     Cancel and Save, which submits the form by its name.
//   - The line of Save that sends files (Runbar.Progress), hidden until the
//     library's live layer moves it.
//   - What the server lost of the last save (Loss): a notice over the form.
//   - The form: Summary with the cursor, Location, Status, Tags, the
//     description; When — All day, the dates and times monospaced, the time
//     zone a choice with its menu (find a zone, «As sent», Floating last);
//     Reminders — each a choice, «Add reminder» adding one in place;
//     Attendees — the people as tags, the line the next is typed in, who
//     sends the invitations (or, invited, why the list is read only);
//     Attachments — Attach with its arrow, each attachment where it is kept,
//     or «Nothing attached»; Repeats — the frequency, every, the days as one
//     row, until, count, the advanced rule with its pattern; the properties
//     kept as the server sent them.
//   - Under the form the deletion, and the line of a series under it.
func Screen(p *view.EventEditPage) w.Shell {
	id := formID(p)
	var plain, primary []w.ActsPart
	var attach *view.Action
	for _, a := range p.Form.Actions {
		if a.ID == attachID {
			attach = &a
			continue
		}
		b := bind.Button(a, id)
		if a.Primary {
			b.Key = saveKey // U-7: Save in the editor
			primary = append(primary, b)
		} else {
			plain = append(plain, b)
		}
	}
	// The calendar it is saved in, in the head left of Cancel (Typecase's
	// note 72; s-agenda-3, -15).
	var saved []w.ActsPart
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			if r.Name == calendarRow {
				saved = append(saved, bind.SavedIn(r, ro, id))
			}
		}
	}
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Name)}, Acts: &w.Acts{Items: append(append(saved, plain...), primary...)}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	// The alerts stand under the head, before the form and its bar of
	// saving (s-agenda-32; Typecase's note 181 p. 2).
	main := append([]w.MainPart{h}, bind.Notices(p.Notices, w.MsgFlushTop2)...)
	main = append(main, w.Runbar{Progress: id, Parts: []w.RunbarPart{w.Text(saving), w.Mono{Text: noBytes}}})
	if p.Loss != nil {
		main = append(main, bind.Loss(*p.Loss, w.MsgFlushTop2)...)
	}
	form := bind.Form(p.Form, id)
	after, related := reminders(p.Form), setOf(p.Form, relatedRow)
	if related <= after {
		related = after
	}
	for i, set := range p.Form.Sets {
		rows := visible(set.Rows)
		if set.Legend == "" {
			for _, r := range rows {
				if r.Name == calendarRow {
					continue // in the head
				}
				form.Items = append(form.Items, field(r))
			}
		} else {
			form.Items = append(form.Items, fieldSet(set.Legend, rows))
		}
		// The attendees after the reminders, the attachments after the set
		// of «Related to» (s-agenda-3: When, Reminders, Attendees, Other,
		// Attachments, Repeats); without one, right after the attendees.
		if i == after {
			form.Items = append(form.Items, attendees(p.Attendees))
		}
		if i == related {
			form.Items = append(form.Items, attachments(attach, p.Attach))
		}
	}
	if len(p.Foreign) > 0 {
		var lines string
		for i, f := range p.Foreign {
			if i > 0 {
				lines += "\n"
			}
			lines += string(f.Label) + "  " + string(f.Value)
		}
		form.Items = append(form.Items, w.FieldSet{Rubric: w.Rubric{Text: kept}, Items: []w.FieldSetPart{w.Diag{Text: lines, Place: w.Grown}}})
	}
	if p.UID != "" {
		// Source, as the task's form has it (s-tasks-3; Р18 of the
		// server's kinds-parity list).
		form.Items = append(form.Items, sourceSet(p.Source))
	}
	if _, drops := Drop(p); drops {
		// A picture pasted into the description joins the attachments, as
		// one dropped on the page (s-io-21).
		form.Items = bind.PasteInto(form.Items, id)
	}
	main = append(main, form)
	if p.Delete != nil {
		main = append(main, w.Danger{Button: bind.Button(*p.Delete, formDelete), Note: string(p.DeleteNote)})
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail, p.UID == ""), Main: w.Main{Items: main}}}
}

// calendarRow is the name of the Calendar the event lies in (C-6.8); ro the
// badge of one read only in its choice.
const calendarRow = "notebook"

var ro = w.Badge{Label: "ro", Place: w.Pushed}

// formID is the ID of the event's form: a new event's, or the edit's.
func formID(p *view.EventEditPage) string {
	if p.UID == "" {
		return formNew
	}
	return formEdit
}

// Forms are the forms of the actions that post with none of their own: the
// deletion, Attach «From this computer» with its file, the notices' buttons
// and the menus' items that post (Sign out).
func Forms(p *view.EventEditPage) []w.PostForm {
	out := append(bind.TopForms(p.Top), bind.NoticeForms(p.Notices)...)
	for _, a := range p.Form.Actions {
		if a.ID == attachID {
			out = append(out, bind.EditFileForms(a.Arrow)...)
		}
	}
	if d := p.Delete; d != nil && d.Method == "POST" && d.Enabled {
		out = append(out, bind.ActionForm(*d, formDelete))
	}
	for _, a := range p.Attach {
		for _, act := range a.Actions {
			if act.Method == "POST" && act.Enabled && !act.Submits {
				out = append(out, bind.ActionForm(act, removeID(act)))
			}
		}
	}
	return out
}

// removeID is the ID of the form of an attachment's Remove: its action's ID
// and address, one per attachment.
func removeID(a view.Action) string {
	return a.ID + a.Href
}

// The names the form's rows of reminders and of the repetition start with
// (C-6.8: reminder.N, repeat.freq…).
const (
	reminderRow = "reminder"
	repeatRow   = "repeat."
	relatedRow  = "related"
)

// reminders is the index of the form's set of reminders — the set with the
// rows reminder.N or «Add reminder» —: the attendees and the attachments
// stand after it (U-6.2, s-agenda-3); with none, before the repetition, or
// after the last set.
func reminders(f view.Form) int {
	at := -1
	for i, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputAdd || starts(r.Name, reminderRow) {
				at = i
			}
		}
	}
	if at >= 0 {
		return at
	}
	for i, set := range f.Sets {
		for _, r := range set.Rows {
			if starts(r.Name, repeatRow) {
				return i - 1
			}
		}
	}
	return len(f.Sets) - 1
}

// setOf is the index of the form's set holding the row name; -1 — none.
func setOf(f view.Form, name string) int {
	for i, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Name == name {
				return i
			}
		}
	}
	return -1
}

// starts tells whether a row's name starts with a prefix.
func starts(name, prefix string) bool {
	return len(name) >= len(prefix) && name[:len(prefix)] == prefix
}

// fieldSet is a rubric of the form: its fields, a tick bare, «Add …» on the
// line of the labels with the template of what it adds.
func fieldSet(legend view.Text, rows []view.FormRow) w.FieldSet {
	fs := w.FieldSet{Rubric: w.Rubric{Text: string(legend)}}
	for _, r := range rows {
		if r.Kind == view.InputAdd {
			add, tmpl := bind.Add(r, func(t view.FormRow) []w.FieldSetPart { return []w.FieldSetPart{field(t)} })
			fs.Items = append(fs.Items, w.Field{Label: blank, Parts: []w.FieldPart{add}}, tmpl)
			continue
		}
		fs.Items = append(fs.Items, field(r))
	}
	return fs
}

// visible are the rows of a set a person sees: all but the hidden ones.
func visible(rows []view.FormRow) []view.FormRow {
	var out []view.FormRow
	for _, r := range rows {
		if r.Kind != view.InputHidden {
			out = append(out, r)
		}
	}
	return out
}

// field is a row of the form as its field: a tick (All day) bare, the days a
// series repeats on as one row (Seg), a choice with a line that finds an
// option (the time zone) as a choice with its menu, the description's text,
// and any other row as a field of the width of its name — an empty line of
// text showing the dash where the model gives no pattern of its own.
func field(r view.FormRow) w.Field {
	switch {
	case r.Kind == view.InputCheck && len(r.Options) > 0:
		return w.Field{Label: string(r.Label), Parts: []w.FieldPart{bind.Days(r)}}
	case r.Kind == view.InputCheck:
		return w.Field{Parts: []w.FieldPart{bind.Check(r)}}
	case r.Kind == view.InputSelect && r.Suggest != "":
		// «Related to»: the records as tags with the line the next is
		// typed in, as the task's and the note's (s-agenda-3, -15).
		return w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{bind.Related(r)}, bind.Hint(r)...), Width: width(r.Name)}
	case r.Kind == view.InputSelect && r.Placeholder != "":
		d := bind.PickMenu(r)
		d.Menu.Width, d.Menu.Pop = w.MenuWidth290, w.MenuUnder
		return w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{d}, bind.Hint(r)...), Width: width(r.Name)}
	case r.Kind == view.InputArea:
		a := bind.Area(r)
		a.Label = text
		return w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{a}, bind.Hint(r)...), Width: width(r.Name)}
	}
	f := bind.Field(r, width(r.Name))
	if in, ok := f.Parts[0].(w.Input); ok && in.Placeholder == "" && in.Value == "" && in.Kind != w.InputDate && in.Kind != w.InputTime {
		in.Placeholder = none
		f.Parts[0] = in
	}
	return f
}

// attendees is the set of the attendees (AttendeeField): the people as tags
// in columns with the line the next is typed in — a field whose suggestions
// are a menu under it —, and under them who sends the invitations; invited,
// the tags alone and why the list is read only.
func attendees(f view.AttendeeField) w.FieldSet {
	in := bind.Attendees(f, lead)
	var parts []w.FieldPart
	if f.ReadOnly {
		parts = append(parts, in)
	} else {
		parts = append(parts, w.Drop{Trigger: in, Menu: w.Menu{Items: []w.MenuPart{}, Width: w.MenuWidth420, Pop: w.MenuUnder}})
	}
	if f.Hint != "" {
		parts = append(parts, w.FieldHint{Parts: []w.Inline{w.Text(f.Hint)}})
	}
	if f.Add != nil && f.Add.Error != "" {
		parts = append(parts, w.FieldHint{Parts: []w.Inline{w.Text(f.Add.Error)}, Variant: w.HintDanger})
	}
	return w.FieldSet{Rubric: w.Rubric{Text: people}, Items: []w.FieldSetPart{w.Field{Parts: parts, Width: w.FieldFull}}}
}

// attachments is the set of the attachments: Attach with its arrow (From
// this computer, From Files), then each attachment — its name, its type and
// size muted, where it is kept — and its Remove (AttachRow.Actions; red for
// data that goes with its only copy, plain for a link whose file stays;
// s-agenda-3, task 93), or «Nothing attached».
func attachments(attach *view.Action, rows []view.AttachRow) w.FieldSet {
	var items []w.LinePart
	if attach != nil {
		b := bind.Button(*attach, "")
		b.Menu = true
		row := w.Line{Items: []w.LinePart{
			w.Faced{Face: w.FaceLarge, Items: []w.LinePart{w.Text("{name} "), w.Dim{Parts: []w.Inline{w.Text("{size}")}}}},
			w.Button{Label: "Remove", Kind: w.ButtonDanger, Small: true, Removes: true},
		}, Gap: w.LineGapNormal}
		items = append(items, w.Drop{Trigger: b, Menu: w.Menu{Items: bind.EditFiles(attach.Arrow, "event-attachments", "size", row), Width: w.MenuWidth212, Pop: w.MenuUnder}})
	}
	for _, a := range rows {
		parts := []w.LinePart{w.Text(string(a.Name) + " "), w.Dim{Parts: []w.Inline{w.Text(string(a.Type) + " · " + string(a.Size))}}}
		switch {
		case a.State == view.RecordUnavailable:
			// The session no longer keeps it (s-agenda-32).
			parts = append(parts, w.Text(" "), w.Badge{Label: notKept, Tone: w.BadgeAlert})
		case a.Inline:
			parts = append(parts, w.Text(" "), w.Badge{Label: inside})
		}
		items = append(items, w.Faced{Face: w.FaceLarge, Items: parts})
		for _, act := range a.Actions {
			b := bind.Button(act, removeID(act))
			b.Kind, b.Small = w.ButtonPlain, true
			if act.Danger {
				b.Kind = w.ButtonDanger
			}
			// A row with nothing to send — a file the session no longer
			// keeps — goes off the page (s-agenda-32).
			b.Removes = act.Href == "" && !act.Submits
			items = append(items, b)
		}
	}
	if len(rows) == 0 {
		items = append(items, w.Dim{Parts: []w.Inline{w.Text(nothing)}, Size: w.Dim11_5, Variant: w.DimWrap, Empty: true})
	}
	return w.FieldSet{Rubric: w.Rubric{Text: files}, Items: []w.FieldSetPart{w.Line{Items: items, Gap: w.LineGapNormal, Block: true, Place: w.Grown, ID: "event-attachments"}}}
}

// railParts is the rail of return: the way back — to the card, or to the
// list for a new event, which has the phone's short «Back» (U-1.3) —, the
// calendar's sources, the foot.
func railParts(r view.Rail, list bool) []w.RailPart {
	nav := w.Nav{}
	for _, n := range r.Nav {
		it := bind.NavItem(n)
		if list {
			it.Short = "Back"
		}
		nav.Items = append(nav.Items, it)
	}
	parts := []w.RailPart{nav}
	parts = append(parts, bind.Sources(r, rail)...)
	return append(parts, bind.Foot(r, rail))
}

// sourceSet is the rubric Source of the form: where the event is and when it
// was read, and whether it changed on its server since — the task's set.
func sourceSet(s view.SourceBlock) w.FieldSet {
	lines := w.Fields{Place: w.Grown}
	if s.Path != "" {
		lines.Items = append(lines.Items, w.FieldsItem{Term: path, Value: []w.ValuePart{w.Text(s.Path)}, Face: w.ValueMono})
	}
	when := string(s.LastRead)
	if s.Compared != "" {
		when += sep + string(s.Compared)
	}
	if when != "" {
		lines.Items = append(lines.Items, w.FieldsItem{Term: read, Value: []w.ValuePart{w.Text(when)}})
	}
	return w.FieldSet{Rubric: w.Rubric{Text: source}, Items: []w.FieldSetPart{lines}}
}
