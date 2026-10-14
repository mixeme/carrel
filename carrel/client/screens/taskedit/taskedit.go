// Package taskedit is the edit of a task,
// /app/tasks/{account}/{col}/{uid}/edit, and the full form of a new one,
// /app/tasks/{account}/{col}/new — one view model (TaskEditPage) —, drawn by
// frames «Задача · правка, срок, выполнение» (s-tasks-3), «Задача · новая»
// (s-tasks-14) and «Правка задачи · сервер сохранил не всё» (states-24) of
// the mockup: the drafts frame2go printed for them, with the frames' data
// become the view model's (U-5.1, U-6.3). A form, and only a form: All day is one for the
// task, making both its start and its due days.
package taskedit

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The IDs of the page's forms: the task's, which Save in the head submits
// (Button.Form) and the line of Save names (Runbar.Progress) — as the frames
// name it, an edit's (s-tasks-3) and a new task's (s-tasks-14) —, and the
// deletion's, which posts with no form of its own.
const (
	formEdit   = "task-edit"
	formNew    = "task-new"
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
	}
	rail = bind.RailWords{Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external"}
)

// The screen's words (U-1.3): the dash in an empty field the screen has no
// pattern for, the accessible name of the description's text, the label of
// a line that stands under the labels of its neighbours (a no-break space:
// Done, «Add reminder»), the rubrics of the attachments and the source, the
// attachment kept inside the task, a form with nothing attached, the names
// of the source's lines and what joins when it was read and whether it
// changed.
const (
	none    = "—"
	text    = "Text"
	blank   = " "
	files   = "Attachments"
	source  = "Source"
	inside  = "in the task"
	nothing = "Nothing attached"
	keptAs  = "Kept as the server sent them"
	path    = "Path"
	read    = "Read"
	sep     = " · "
)

// The names of the contract the screen places by (C-7.5): Attach goes in
// the set of attachments, not with Save and Cancel.
const attachID = "attach.add"

// widths are the widths of the form's fields by their name, the last part
// that is no number (C-6.1: the view model carries no widths; the layout is
// the screen's, from its frame): «reminder.1» is a reminder.
var widths = map[string]w.FieldWidth{
	"summary": w.FieldFull, "description": w.FieldFull,
	"start.date": w.FieldSM, "start.time": w.FieldXS, "due.date": w.FieldSM, "due.time": w.FieldXS,
	"status": w.FieldSM, "priority": w.FieldSM, "percent": w.FieldSM,
	"reminder": w.FieldMD,
	// The numbers «Custom…» of a reminder opens (s-agenda-19).
	"reminder.count": w.FieldXS, "reminder.unit": w.FieldSM, "reminder.at": w.FieldXS,
	"location": w.FieldMD, "categories": w.FieldMD, "related": w.FieldLG,
	// The link, which no frame draws yet (Typecase's task 172): as wide as
	// the tags before it.
	"url": w.FieldMD,
}

// patterns are the patterns of the form's empty fields by their name, as
// the frame draws them (U-1.3: the screen's words); an empty line of text
// the frame draws none for shows the dash.
var patterns = map[string]string{
	"start.date": "YYYY-MM-DD", "due.date": "YYYY-MM-DD",
	"start.time": "--:--", "due.time": "--:--",
	"location": "optional",
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

// Screen draws the edit from its view model, in the state the model is in.
//
//   - The shell: the top bar; the rail of return — «Back to the card» (or
//     «Back to list» for a new task, with the phone's short «Back»), the
//     list's sources, the foot.
//   - The head: the list's mark, the task's title (a new one's «New task»),
//     the list and — overdue — by how long, as a badge in alarm; Cancel and
//     Save, which submits the form by its name.
//   - The line of Save that sends files (Runbar.Progress), hidden until the
//     library's live layer moves it.
//   - What the server lost of the last save (Loss): a notice over the form.
//   - The form: Summary with the cursor and the description; When — All
//     day, which makes the start and the due days, the dates and the times
//     monospaced; Progress — Status, Priority, Percent complete, Done;
//     Reminders — each a choice, «Add reminder» adding one in place; Other
//     — Location, Tags, Related to as the record chosen; Attachments —
//     Attach with its arrow, each attachment where it is kept and its
//     Remove, or «Nothing attached»; the source — its path, when it was
//     read and whether it changed on the server —, none for a new task,
//     which has no record yet (s-tasks-14).
//   - Under the form the deletion.
func Screen(p *view.TaskEditPage) w.Shell {
	formID := formEdit
	if p.UID == "" {
		formID = formNew
	}
	var plain, primary []w.ActsPart
	var attach *view.Action
	for _, a := range p.Form.Actions {
		if a.ID == attachID {
			attach = &a
			continue
		}
		b := bind.Button(a, formID)
		if a.Primary {
			primary = append(primary, b)
		} else {
			plain = append(plain, b)
		}
	}
	title := &w.HeadTitle{Text: string(p.Name)}
	if p.Source.Colour != "" {
		title.Mark = &w.Mark{Colour: string(p.Source.Colour), Size: w.MarkH20}
	}
	h := w.Head{Title: title, Subs: bind.SubAlert(p.Subtitle), Acts: &w.Acts{Items: append(plain, primary...)}}
	main := []w.MainPart{h, w.Runbar{Progress: formID, Parts: []w.RunbarPart{w.Text(saving), w.Mono{Text: noBytes}}}}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if p.Loss != nil {
		main = append(main, bind.Loss(*p.Loss, w.MsgFlushTop2)...)
	}
	form := bind.Form(p.Form, formID)
	for _, set := range p.Form.Sets {
		rows := visible(set.Rows)
		if repeats(rows) {
			// ждёт кадра: задача Typecase 173 — the task's Repeats (Р1 of
			// the server's kinds-parity list) has no frame yet, and the
			// screen draws none. Its values go with the form as they came,
			// so a save keeps the task's rule: a rubric left out would read
			// as «Does not repeat» and take the rule away.
			form.Fields = append(form.Fields, carried(rows)...)
			continue
		}
		if set.Legend == "" {
			for i, r := range rows {
				if r.Name == listRow {
					// ждёт кадра: задача Typecase 175 — the List the task
					// lies in (Р19) has no frame yet. Left out, the form
					// keeps the task where it is.
					continue
				}
				form.Items = append(form.Items, field(r, i))
			}
			continue
		}
		form.Items = append(form.Items, fieldSet(set.Legend, rows))
	}
	form.Items = append(form.Items, attachments(attach, p.Attach))
	if len(p.Foreign) > 0 {
		// What Carrel does not know of the task, as it came — the event's
		// set (s-agenda-3; Р17 of the server's kinds-parity list).
		var lines string
		for i, f := range p.Foreign {
			if i > 0 {
				lines += "\n"
			}
			lines += string(f.Label) + "  " + string(f.Value)
		}
		form.Items = append(form.Items, w.FieldSet{Rubric: w.Rubric{Text: keptAs}, Items: []w.FieldSetPart{w.Diag{Text: lines, Place: w.Grown}}})
	}
	if p.UID != "" {
		form.Items = append(form.Items, sourceSet(p.Source))
	}
	main = append(main, form)
	if p.Delete != nil {
		main = append(main, w.Danger{Button: bind.Button(*p.Delete, formDelete)})
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail, p.UID == ""), Main: w.Main{Items: main}}}
}

// Forms are the forms of the actions that post with none of their own: the
// deletion, Attach «From this computer» with its file, an attachment's
// Remove, the notices' buttons and the menus' items that post (Sign out).
func Forms(p *view.TaskEditPage) []w.PostForm {
	out := append(bind.TopForms(p.Top), bind.NoticeForms(p.Notices)...)
	for _, a := range p.Form.Actions {
		if a.ID == attachID {
			out = append(out, bind.MenuForms(a.Arrow)...)
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

// fieldSet is a rubric of the form: its fields, «Add …» on the line of the
// labels with the template of what it adds — under a blank label beside
// fields (s-tasks-3), bare where it stands alone (s-tasks-14: a new task has
// no reminder yet).
func fieldSet(legend view.Text, rows []view.FormRow) w.FieldSet {
	fs := w.FieldSet{Rubric: w.Rubric{Text: string(legend)}}
	for i, r := range rows {
		if r.Kind == view.InputAdd {
			add, tmpl := bind.Add(r, func(t view.FormRow) []w.FieldSetPart { return []w.FieldSetPart{field(t, 0)} })
			f := w.Field{Parts: []w.FieldPart{add}}
			if i > 0 {
				f.Label = blank
			}
			fs.Items = append(fs.Items, f, tmpl)
			continue
		}
		fs.Items = append(fs.Items, field(r, i))
	}
	return fs
}

// repeatRow is the start of the names of the repetition's rows (C-6.8:
// repeat.freq…); listRow is the name of the List the task lies in.
const (
	repeatRow = "repeat."
	listRow   = "notebook"
)

// repeats tells whether a set is the repetition's: its rows are repeat.….
func repeats(rows []view.FormRow) bool {
	return len(rows) > 0 && len(rows[0].Name) > len(repeatRow) && rows[0].Name[:len(repeatRow)] == repeatRow
}

// carried are the rows of a set as the hidden fields that send what they
// hold: a row of days, each day ticked; any other row, its value.
func carried(rows []view.FormRow) []w.Hidden {
	var out []w.Hidden
	for _, r := range rows {
		if r.Kind == view.InputCheck && len(r.Options) > 0 {
			for _, o := range r.Options {
				if o.Selected {
					out = append(out, w.Hidden{Name: r.Name, Value: o.Value})
				}
			}
			continue
		}
		out = append(out, w.Hidden{Name: r.Name, Value: string(r.Value)})
	}
	return out
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

// field is a row of the form as its field, the row's place in its set at:
// a tick bare at the head of its set (All day) and under a blank label
// beside fields (Done); Related to as the record chosen (Related); the
// description's text; any other row as a field of the width of its name —
// an empty one showing the frame's pattern for it, or the dash.
func field(r view.FormRow, at int) w.Field {
	switch {
	case r.Kind == view.InputCheck:
		c := w.Field{Parts: []w.FieldPart{bind.Check(r)}}
		if at > 0 {
			c.Label = blank
		}
		return c
	case r.Kind == view.InputSelect && r.Suggest != "":
		return w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{bind.Related(r)}, bind.Hint(r)...), Width: width(r.Name)}
	case r.Kind == view.InputArea:
		a := bind.Area(r)
		a.Label = text
		return w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{a}, bind.Hint(r)...), Width: width(r.Name)}
	}
	f := bind.Field(r, width(r.Name))
	if in, ok := f.Parts[0].(w.Input); ok && in.Placeholder == "" && in.Value == "" {
		in.Placeholder = none
		if p, ok := patterns[r.Name]; ok {
			in.Placeholder = p
		}
		f.Parts[0] = in
	}
	return f
}

// attachments is the set of the attachments: Attach with its arrow (From
// this computer, From Files), then each attachment — its name, its type and
// size muted, where it is kept — and its Remove (AttachRow.Actions; red for
// data that goes with its only copy, plain for a link whose file stays), or
// «Nothing attached».
func attachments(attach *view.Action, rows []view.AttachRow) w.FieldSet {
	var items []w.LinePart
	if attach != nil {
		// ждёт библиотеку: U-5.5 #43 — the frame draws the file a field of
		// the form that goes with Save (FileField.List); it goes by its own
		// form, as the contract has it, until Save takes files
		b := bind.Button(*attach, "")
		b.Menu = true
		items = append(items, w.Drop{Trigger: b, Menu: w.Menu{Items: append(bind.MenuItems(attach.Arrow, ""), bind.Uploads(attach.Arrow, upload)...), Width: w.MenuWidth212, Pop: w.MenuUnder}})
	}
	for _, a := range rows {
		parts := []w.LinePart{w.Text(string(a.Name) + " "), w.Dim{Parts: []w.Inline{w.Text(string(a.Type) + sep + string(a.Size))}}}
		if a.Inline {
			parts = append(parts, w.Text(" "), w.Badge{Label: inside})
		}
		items = append(items, w.Faced{Face: w.FaceLarge, Items: parts})
		for _, act := range a.Actions {
			b := bind.Button(act, removeID(act))
			b.Kind, b.Small = w.ButtonPlain, true
			if act.Danger {
				b.Kind = w.ButtonDanger
			}
			items = append(items, b)
		}
	}
	if len(rows) == 0 {
		items = append(items, w.Dim{Parts: []w.Inline{w.Text(nothing)}, Size: w.Dim11_5, Variant: w.DimWrap, Empty: true})
	}
	return w.FieldSet{Rubric: w.Rubric{Text: files}, Items: []w.FieldSetPart{w.Line{Items: items, Gap: w.LineGap8, Block: true, Place: w.Grown}}}
}

// sourceSet is the set of the task's source: its path monospaced (none for a
// new task), when it was read and whether it changed on the server.
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

// railParts is the rail of return: the way back — to the card, or to the
// list for a new task, which has the phone's short «Back» (U-1.3) —, the
// list's sources, the foot.
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

// upload is the screen's word (U-1.3) for the item after Attach's arrow that
// sends the file picked on a page without a script (Typecase's task 109).
const upload = "Upload"
