// Package tasks is the list of tasks, /app/tasks (the combined view) and
// /app/tasks/{account}/{col} (one list), drawn by frame «Задачи · фильтры,
// просрочка» (s-tasks-1) of the mockup: the draft frame2go printed for it,
// with the frame's data become the view model's (U-5.1, U-6.3). A task is
// ticked done right in its row: the tick changes three properties, not the
// record.
package tasks

import (
	"slices"
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The IDs of the page's forms: the rail's (its ticks submit it) and the
// panel's deletion.
const (
	formRail   = "rail"
	formDelete = "delete"
)

// The screen's words (U-1.3) for the shell: the mark, the search, the rail;
// and what the choice of columns sends (C-6.4 Columns).
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
	}
	rail    = bind.RailWords{Form: formRail, Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external"}
	columns = bind.ColumnWords{Name: "col", Reset: "default", Drag: "drag"}
)

// The screen's words (U-1.3): the dash of a row with no due date or share
// done, the share's sign, the accessible name of a row's tick, the lead of a
// kind, the attachment kept inside the task, what joins the sources of a
// linked row in its hint.
const (
	none    = "—"
	percent = "%"
	done    = "Done"
	lead    = "· "
	inside  = "in the task"
	joined  = ", "
)

// The names of the contract the screen sorts by (C-7): the bar's switch of
// order starts with sortID — the other switch is what is shown, Open, Done,
// All —; Attach the panel puts in the rubric of attachments.
const (
	sortID   = "sort."
	attachID = "attach.add"
)

// The columns of a task's row by their IDs (C-6.4 ColumnItem.ID): the ones
// the frame's row draws, in its order (DueRow, s-tasks-1) — the source in
// the combined view only —, and the cell of each column a person may choose
// (s-tasks-12, -13).
var (
	dueColumns = []string{"title", "due", "complete", "source"}
	cells      = map[string]w.Column{
		"title": w.ColumnName, "tags": w.ColumnTags, "due": w.ColumnDue, "complete": w.ColumnComplete,
		"source": w.ColumnSource, "priority": w.ColumnPriority, "status": w.ColumnStatus, "changed": w.ColumnChanged,
		// where the task is, the cell of the agenda's Where (Р10 of the
		// kinds' parity; no frame draws it)
		"where": w.ColumnDetail,
		// how many people the task has, a plain badge as the agenda's
		// column draws an event's (after Р3; no frame draws it)
		"attendees": w.ColumnBadges,
	}
)

// Screen draws the list of tasks from its view model, in the state the model
// is in.
//
//   - The shell (U-2.1): the top bar; the rail of sections (the open one
//     on), the sources by account with their ticks, which submit the rail's
//     form, the feeds' rubric «external», a source that did not answer as an
//     error, the tags, the foot.
//   - The head: the mark of what is open — every list at once, or the
//     list's colour —, the title, what is shown (Subtitle) and the bar's
//     actions: New task — with its arrow in the combined view, a closed menu
//     of the ticked lists, «last used» —, Import, Export, Print.
//   - The notices under the head, each with its technical block.
//   - The bar while something is ticked and there is something to show or
//     choose: what is shown — Open, Done, All with their counts —, the
//     order, Columns with its menu (a form).
//   - The poll's bar while the poll runs or a source did not answer (C-6.4).
//   - The groups, each with its count — the overdue one's in alarm —, a row
//     for each task: its tick, which posts Mark done without a script, the
//     list's mark, the title leading to the task (bold while open in the
//     panel) with its first badge, the tags, the due date and the share done
//     monospaced (the dash where there is none), where it lives — «2
//     sources» of a linked task, the sources in its hint; with the columns
//     a person chose (Columns), a row of them (s-tasks-13).
//   - Nothing to show (Empty): what, bold, why, and the button that helps.
//   - The panel (Shell.Details), for the row opened with ?open=: the title,
//     the list, Edit, «⋯» with the record's menu, the cross that closes it;
//     the fields, the description, the attachments with Attach and the
//     line of where a file goes, the related records, the source — its
//     path and how it compares with the server —, the deletion.
func Screen(p *view.TasksPage) w.Shell {
	main := []w.MainPart{head(p)}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if ticked(p.Rail) && (len(p.Groups) > 0 || len(p.Bar.Segments) > 0) {
		if b, ok := bar(p.Bar); ok {
			main = append(main, b)
		}
	}
	if r := bind.Runbar(p.Poll); r != nil {
		main = append(main, *r)
	}
	shown := chosen(p.Bar.Columns)
	// One collection, or a browser that hides the source column (Appearance).
	single := bind.Sourceless(p.Rail, p.Look)
	for _, g := range p.Groups {
		rows := make([]w.ListRow, 0, len(g.Rows))
		for _, r := range g.Rows {
			if shown != nil {
				rows = append(rows, columnRow(r, shown))
			} else {
				rows = append(rows, row(r, single))
			}
		}
		main = append(main,
			w.ListGroup{Title: w.Rubric{Text: string(g.Rubric), Form: w.RubricInline}, Count: &w.Num{Value: g.Count, Alert: g.Overdue}},
			w.List{Rows: rows})
	}
	if p.Empty != nil {
		main = append(main, w.Empty{Parts: bind.EmptyParts(*p.Empty)})
	}
	if p.Poll != nil && p.Poll.Note != "" {
		// What is shown and what is missing, under the rows, once a source
		// did not answer (PollBar.Note; states-20).
		main = append(main, w.Empty{Parts: []w.EmptyPart{w.Text(p.Poll.Note)}, Variant: w.EmptySmall})
	}
	body := w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}
	if p.Details != nil {
		body.Side = side(*p.Details)
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: body}
}

// Forms are the page's forms: the rail's, which its ticks submit, the
// retries of the sources, the buttons of the notices and of the empty state
// that post, the items of the menus that post (Sign out, the arrow of New
// task), the ticks of the rows, and of the panel — its deletion, its menu,
// Attach «From this computer» with its file.
func Forms(p *view.TasksPage) []w.PostForm {
	out := bind.TopForms(p.Top)
	for _, a := range p.Bar.Actions {
		out = append(out, bind.MenuForms(a.Arrow)...)
	}
	if f := bind.RailForm(p.Rail, formRail); f != nil {
		out = append(out, *f)
	}
	out = append(out, bind.PollForms(p.Poll)...)
	out = append(out, bind.NoticeForms(p.Notices)...)
	if p.Empty != nil {
		for _, a := range p.Empty.Actions {
			if a.Method == "POST" && a.Enabled {
				out = append(out, bind.ActionForm(a, a.ID))
			}
		}
	}
	for _, g := range p.Groups {
		for _, r := range g.Rows {
			if a := r.Tick; a != nil && a.Method == "POST" && a.Enabled {
				out = append(out, bind.ActionForm(*a, bind.TickID(*a)))
			}
		}
	}
	if d := p.Details; d != nil {
		for _, a := range d.Actions {
			switch {
			case a.Danger && a.Method == "POST" && a.Enabled:
				out = append(out, bind.ActionForm(a, formDelete))
			case a.ID == attachID:
				out = append(out, bind.MenuForms(a.Arrow)...)
			}
		}
		out = append(out, bind.MenuForms(d.Menu)...)
	}
	return out
}

// ticked tells whether any source of the rail is ticked: with nothing ticked
// there is nothing to show or order, and the list draws no bar; nor does a
// list that did not answer, with no rows and no switch.
func ticked(r view.Rail) bool {
	for _, sec := range r.Sections {
		for _, row := range sec.Rows {
			if row.Tick != view.TickOff {
				return true
			}
		}
	}
	return false
}

// railParts is the rail of sections: the sections, the sources and tags, the
// foot.
func railParts(r view.Rail) []w.RailPart {
	nav := w.Nav{}
	for _, n := range r.Nav {
		nav.Items = append(nav.Items, bind.NavItem(n))
		if n.On {
			nav.On = len(nav.Items)
		}
	}
	parts := []w.RailPart{nav}
	parts = append(parts, bind.Sources(r, rail)...)
	return append(parts, bind.Foot(r, rail))
}

// head is the list's head: what is open, what is shown, the bar's actions.
func head(p *view.TasksPage) w.Head {
	title := &w.HeadTitle{Text: string(p.Title)}
	for _, sec := range p.Rail.Sections {
		for _, r := range sec.Rows {
			switch {
			case !r.On:
			case r.Colour == "":
				title.Mark = &w.Mark{Kind: w.MarkMulti, Size: w.MarkH20}
			default:
				title.Mark = &w.Mark{Colour: string(r.Colour), Size: w.MarkH20}
			}
		}
	}
	var acts []w.ActsPart
	for _, a := range p.Bar.Actions {
		b := bind.Button(a, "")
		if len(a.Arrow) == 0 {
			acts = append(acts, b)
			continue
		}
		b.Menu = true
		acts = append(acts, w.Drop{Trigger: b, Menu: w.Menu{Items: bind.MenuItems(a.Arrow, rail.ReadOnly), Pop: w.MenuUnder}})
	}
	h := w.Head{Title: title, Acts: &w.Acts{Items: acts}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	return h
}

// bar is the bar over the list: what is shown (Open, Done, All, each with
// its count), the order, Columns; false when it has none of them.
func bar(b view.ListBar) (w.Bar, bool) {
	var items []w.BarPart
	shown, order := w.Seg{}, w.Seg{Secondary: true}
	for _, a := range b.Segments {
		if len(a.ID) >= len(sortID) && a.ID[:len(sortID)] == sortID {
			order.Options = append(order.Options, bind.SegOption(a))
		} else {
			shown.Options = append(shown.Options, bind.SegOption(a))
		}
	}
	for _, s := range []w.Seg{shown, order} {
		if len(s.Options) > 0 {
			items = append(items, s)
		}
	}
	if b.Columns != nil {
		cols := bind.Columns(*b.Columns, columns)
		cols.Menu.Pop = w.MenuUnder
		items = append(items, cols)
	}
	return w.Bar{Items: items}, len(items) > 0
}

// row is a task in the list; in the list of one collection (single) it has
// no source and no column for it, and the sources of a linked task are the
// hint of its badge (s-tasks-15).
func row(r view.TaskRow, single bool) w.DueRow {
	d := w.DueRow{
		Tick:     bind.Tick(r.Tick, r.Done, done),
		Mark:     w.Mark{Colour: string(r.Colour)},
		Title:    []w.Inline{w.Text(r.Summary)},
		Strong:   r.On,
		Due:      or(r.Due),
		Progress: progress(r.Percent),
		Source:   string(r.Stamp),
		On:       r.On,
		Overdue:  r.Overdue,
		Href:     r.Href,
	}
	if len(r.Badges) > 0 {
		b := bind.Badge(r.Badges[0])
		d.TitleBadge = &b
	}
	if single {
		d.Source, d.Single = "", true
		if len(r.Badges) > 0 {
			b := bind.LinkedBadge(r.Badges[0], r.Sources)
			d.TitleBadge = &b
		}
		return d
	}
	for i, s := range r.Sources {
		if i > 0 {
			d.SourceHint += joined
		}
		d.SourceHint += string(s)
	}
	return d
}

// source is the technical block of a task's source: its path, when it was
// read and whether it changed on the server.
func source(s view.SourceBlock) w.Diag {
	read := string(s.LastRead)
	if s.Compared != "" {
		read += " " + lead + string(s.Compared)
	}
	return bind.Diag(bind.DiagLine{Name: "Path", Value: string(s.Path)}, bind.DiagLine{Name: "Read", Value: read})
}

// chosen are the columns a person chose for the list, by ID in their order
// (Columns.Items on); nil where they are the frame's own row's.
func chosen(c *view.Columns) []string {
	if c == nil {
		return nil
	}
	var on []string
	for _, it := range c.Items {
		if it.On {
			on = append(on, it.ID)
		}
	}
	if slices.Equal(on, dueColumns) || slices.Equal(on, dueColumns[:len(dueColumns)-1]) {
		return nil
	}
	return on
}

// columnRow is a task in a list of the columns a person chose (s-tasks-13):
// its tick, the list's mark, a cell for each column in their order — the
// dash where the task has nothing —, leading to the task.
func columnRow(r view.TaskRow, shown []string) w.ColumnRow {
	t := bind.Tick(r.Tick, r.Done, done)
	c := w.ColumnRow{Tick: &t, Mark: w.Mark{Colour: string(r.Colour)}, Strong: r.On, On: r.On, Overdue: r.Overdue, Href: r.Href}
	for _, id := range shown {
		col, ok := cells[id]
		if !ok {
			continue
		}
		cell := w.ColumnCell{Column: col}
		switch col {
		case w.ColumnName:
			cell.Text = string(r.Summary)
		case w.ColumnTags:
			for _, tag := range bind.Tags(r.Tags) {
				cell.Tags = append(cell.Tags, tag)
			}
		case w.ColumnDue:
			cell.Text = or(r.Due)
		case w.ColumnComplete:
			cell.Text = progress(r.Percent)
		case w.ColumnSource:
			cell.Text = string(r.Stamp)
		case w.ColumnPriority:
			cell.Text = or(r.Priority)
		case w.ColumnStatus:
			cell.Text = or(r.Status)
		case w.ColumnChanged:
			cell.Text = or(r.Changed)
		case w.ColumnDetail:
			cell.Text = or(r.Where)
		case w.ColumnBadges:
			if r.Attendees != "" {
				cell.Badges = []w.Badge{{Label: string(r.Attendees)}}
			}
		}
		c.Cells = append(c.Cells, cell)
	}
	return c
}

// or is a cell's words, the dash when the task has none.
func or(t view.Text) string {
	if t == "" {
		return none
	}
	return string(t)
}

// progress is the share of a task done, «40%», the dash when it has none.
func progress(p *int) string {
	if p == nil {
		return none
	}
	return strconv.Itoa(*p) + percent
}

// side is the panel of the task opened (Details).
func side(d view.Details) *w.Side {
	var acts []w.ActsPart
	var attach *view.Action
	var danger []w.Button
	for _, a := range d.Actions {
		switch {
		case a.Danger:
			b := bind.Button(a, formDelete)
			b.Icon = ""
			danger = append(danger, b)
		case a.ID == attachID:
			attach = &a
		default:
			b := bind.Button(a, "")
			b.Small = true
			acts = append(acts, b)
		}
	}
	if len(d.Menu) > 0 {
		acts = append(acts, w.Drop{
			Trigger: w.Button{Icon: "dots", Small: true, Title: "More"},
			Menu:    w.Menu{Items: bind.MenuItems(d.Menu, rail.ReadOnly), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		})
	}
	acts = append(acts, w.Close{Label: "Close the panel", Href: d.Close})
	items := []w.SidePart{w.Head{
		Title: &w.HeadTitle{Text: string(d.Title), Size: w.HeadTitle19},
		Subs:  []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(d.Subtitle)}}},
		Acts:  &w.Acts{Items: acts},
	}}
	fields := w.Fields{}
	for _, f := range d.Fields {
		fields.Items = append(fields.Items, bind.FieldsItem(f, lead))
	}
	items = append(items, fields)
	if d.Body != "" {
		items = append(items, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: "Description"},
			w.Prose{Parts: []w.Inline{w.Text(d.Body)}, Variant: w.Prose13},
		}})
	}
	if attach != nil || len(d.Attach) > 0 {
		rubric := w.Rubric{Text: "Attachments"}
		if attach != nil {
			b := bind.Button(*attach, "")
			b.Icon, b.Menu, b.Small = "", true, true
			rubric.Items = []w.RubricPart{w.Drop{Trigger: b, Menu: w.Menu{Items: append(bind.MenuItems(attach.Arrow, ""), bind.Uploads(attach.Arrow, upload)...), Width: w.MenuWidth212, Pop: w.MenuUnder}}}
		}
		sec := []w.SecPart{rubric}
		if len(d.Attach) > 0 {
			sec = append(sec, w.SecList{Items: attachments(d.Attach), Gap: 6})
		}
		if d.AttachHint != "" {
			sec = append(sec, w.SecHint{Text: string(d.AttachHint), Apart: true})
		}
		items = append(items, w.Sec{Items: sec})
	}
	if len(d.Related) > 0 {
		var lines [][]w.SecItemPart
		for _, r := range d.Related {
			lines = append(lines, []w.SecItemPart{
				w.Span{Face: w.FaceAccent, Text: string(r.Title), Href: r.Href},
				w.Text(" "),
				w.Dim{Parts: []w.Inline{w.Text(lead + string(r.Kind))}},
			})
		}
		items = append(items, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Related"}, w.SecList{Items: lines}}})
	}
	// The task's people, as the agenda's panel names an event's (Р3 of the
	// server's kinds-parity list; no frame draws them yet — Typecase's task
	// 179), drawn as Related draws its records (bind.Person).
	if len(d.Attendees) > 0 {
		var lines [][]w.SecItemPart
		for _, a := range d.Attendees {
			lines = append(lines, bind.Person(a, lead))
		}
		items = append(items, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Attendees"}, w.SecList{Items: lines}}})
	}
	if d.Source.Path != "" {
		// Where the task lies and how it compares with the server, as its
		// card says it (s-tasks-1).
		items = append(items, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Source"}, source(d.Source)}})
	}
	for _, b := range danger {
		items = append(items, w.Danger{Button: b, Note: string(d.DeleteNote)})
	}
	// A file dropped on the open card goes with Attach «From this computer»
	// (Side.Drop, Typecase's note 68; s-io-2).
	side := &w.Side{Items: items}
	if attach != nil {
		side.Drop = bind.UploadForm(attach.Arrow)
	}
	return side
}

// attachments are the lines of a task's attachments: the name, a link where
// it goes (data inside the task downloads, a file in Files opens there), the
// type and size muted, and «in the task» for data inside it (S-20).
func attachments(rows []view.AttachRow) [][]w.SecItemPart {
	var out [][]w.SecItemPart
	for _, a := range rows {
		var line []w.SecItemPart
		if a.Href != "" {
			line = append(line, w.Link{Text: string(a.Name), Href: a.Href}, w.Text(" "))
		} else {
			line = append(line, w.Text(string(a.Name)+" "))
		}
		line = append(line, w.Dim{Parts: []w.Inline{w.Text(string(a.Type) + " · " + string(a.Size))}})
		if a.Inline {
			line = append(line, w.Text(" "), w.Badge{Label: inside})
		}
		out = append(out, line)
	}
	return out
}

// upload is the screen's word (U-1.3) for the item after Attach's arrow that
// sends the file picked on a page without a script (Typecase's task 109).
const upload = "Upload"
