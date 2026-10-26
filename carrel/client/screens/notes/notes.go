// Package notes is a notebook, /app/notes (the combined notebook) and
// /app/notes/{account}/{col} (one notebook), drawn by frame «Заметки · список
// и заметка в панели» (s-notes-1) of the mockup: the draft frame2go printed
// for it, with the frame's data become the view model's (U-5.1, U-6.4). The
// panel reads a note; a note is edited on its page.
package notes

import (
	"slices"
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The IDs of the page's forms: the rail's (its ticks submit it).
const formRail = "rail"

// The screen's words (U-1.3) for the shell: the mark, the search, the rail;
// and what the choice of columns sends (C-6.4 Columns).
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
		NewKey: "n", SearchKey: "/",
		Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
		Keys:  bind.KeysWords{Label: "Keyboard shortcuts", Icon: "search", Key: "?", Sheet: "keys"},
	}
	rail    = bind.RailWords{Form: formRail, Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external"}
	columns = bind.ColumnWords{Name: "col", Reset: "default", Drag: "drag"}
)

// The screen's words (U-1.3): the filter's pattern and accessible name, the
// dash of a cell with nothing, the lead of a kind, the attachment kept
// inside the note, what joins the sources of a linked row in its hint and
// the pieces of the panel's line, «Upload» after Attach's arrow (Typecase's
// task 109).
const (
	filter = "Filter by word"
	none   = "—"
	lead   = "· "
	inside = "in the note"
	joined = ", "
	sep    = " · "
	upload = "Upload"
)

// The names of the contract the screen places by (C-7): Attach the panel
// puts in the rubric of attachments.
const attachID = "attach.add"

// faces are the faces the panel draws a note's text in (s-notes-1).
var faces = bind.TextFaces{Lead: w.ProseRead, Tick: w.TickNudge, Code: w.MonoChip}

// The columns of a note's row by their IDs (C-6.4 ColumnItem.ID): the ones
// the frame's row draws, in its order (ExcerptRow, s-notes-1) — the source
// in the combined view only —, and the cell of each column a person may
// choose (s-notes-24).
var (
	excerptColumns = []string{"title", "excerpt", "date", "source"}
	cells          = map[string]w.Column{
		"title": w.ColumnName, "excerpt": w.ColumnExcerpt, "tags": w.ColumnTags, "date": w.ColumnDate,
		"source": w.ColumnSource, "changed": w.ColumnChanged,
	}
)

// Screen draws the notebook from its view model, in the state the model is
// in.
//
//   - The shell (U-2.1): the top bar; the rail of sections (the open one
//     on), the notebooks by account with their ticks, which submit the
//     rail's form, a notebook that did not answer as an error, the tags, the
//     foot.
//   - The head: the mark of what is open — every notebook at once, or the
//     notebook's colour —, the title, what is shown (Subtitle) and the bar's
//     actions: New note — with its arrow in the combined view, a closed menu
//     of the ticked notebooks, «last used» —, Import, Export.
//   - The notices under the head, each with its technical block.
//   - The bar while something is ticked and there is something to show or
//     order: the filter by word (a GET form), the order (links), Columns
//     with its menu (a form); on its right how many notes are loaded.
//   - The poll's bar while the poll runs or a source did not answer (C-6.4).
//   - The months, each with its count, a row for each note: the notebook's
//     mark, the title leading to the note (bold while open in the panel)
//     with its first badge, the excerpt, the tags, the date monospaced, where
//     it lives — «2 sources» of a linked note, the sources in its hint; with
//     the columns a person chose (Columns), a row of them.
//   - Nothing to show (Empty): what, bold, why, and the button that helps.
//   - The panel (Shell.Details), for the row opened with ?open=: the title,
//     Edit, the note's page, «⋯» with the record's menu (Delete note last),
//     the cross that closes it; the notebook and when; the text — its
//     paragraphs and tasks, a tick a sign, not pressed (U-6.4) —; the tags,
//     the attachments with Attach, the related records.
func Screen(p *view.NotesPage) w.Shell {
	main := []w.MainPart{head(p)}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if ticked(p.Rail) && (len(p.Groups) > 0 || len(p.Bar.Segments) > 0) {
		main = append(main, bar(p.Bar))
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
		if g.Rubric != "" {
			main = append(main, w.ListGroup{Title: w.Rubric{Text: string(g.Rubric), Form: w.RubricInline}, Count: &w.Num{Value: g.Count}})
		}
		main = append(main, w.List{Rows: rows})
	}
	if p.Empty != nil {
		main = append(main, w.Empty{Parts: bind.EmptyParts(*p.Empty)})
	}
	if p.Poll != nil && p.Poll.Note != "" {
		// What is shown and what is missing, under the rows, once a source
		// did not answer (PollBar.Note; states-27).
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
// note), and of the panel — its menu (Delete note), Attach «From this
// computer» with its file.
func Forms(p *view.NotesPage) []w.PostForm {
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
	if d := p.Details; d != nil {
		for _, a := range d.Actions {
			if a.ID == attachID {
				out = append(out, bind.MenuForms(a.Arrow)...)
			}
		}
		out = append(out, bind.MenuForms(d.Menu)...)
	}
	return out
}

// ticked tells whether any notebook of the rail is ticked: with nothing
// ticked there is nothing to filter or order, and the notebook draws no bar;
// nor does a notebook that did not answer, with no rows and no order.
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

// railParts is the rail of sections: the sections, the notebooks and tags,
// the foot.
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

// head is the notebook's head: what is open, what is shown, the bar's
// actions.
func head(p *view.NotesPage) w.Head {
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

// bar is the bar over the notes: the filter by word, the order — of the
// second rank, hidden under «⋯» on a narrow column as the tasks' (note 69
// of Typecase) —, Columns; on its right how many notes are loaded.
func bar(b view.ListBar) w.Bar {
	var items []w.BarPart
	if b.FilterForm != nil {
		f := bind.Filter(b, filter)
		f.Variant = w.FilterH26W200
		items = append(items, f)
	}
	if len(b.Segments) > 0 {
		seg := w.Seg{Secondary: true}
		for _, a := range b.Segments {
			seg.Options = append(seg.Options, bind.SegOption(a))
		}
		items = append(items, seg)
	}
	if b.Columns != nil {
		cols := bind.Columns(*b.Columns, columns)
		cols.Menu.Pop = w.MenuUnder
		items = append(items, cols)
	}
	return w.Bar{Items: items, Right: &w.BarRight{Items: []w.BarPart{w.Mono{Text: strconv.Itoa(b.Loaded)}}, Secondary: true}}
}

// row is a note in the notebook; in one notebook (single) it has no source
// and no column for it, and the sources of a linked note are the hint of
// its badge (s-notes-31).
func row(r view.NoteRow, single bool) w.ExcerptRow {
	e := w.ExcerptRow{
		Mark:    w.Mark{Colour: string(r.Colour)},
		Title:   []w.Inline{w.Text(r.Title)},
		Strong:  r.On,
		Excerpt: string(r.Excerpt),
		Date:    string(r.Date),
		Source:  string(r.Stamp),
		On:      r.On,
		Href:    r.Href,
	}
	if len(r.Badges) > 0 {
		b := bind.Badge(r.Badges[0])
		e.TitleBadge = &b
	}
	if single {
		e.Source, e.Single = "", true
		if len(r.Badges) > 0 {
			b := bind.LinkedBadge(r.Badges[0], r.Sources)
			e.TitleBadge = &b
		}
		return e
	}
	for i, s := range r.Sources {
		if i > 0 {
			e.SourceHint += joined
		}
		e.SourceHint += string(s)
	}
	return e
}

// chosen are the columns a person chose for the notebook, by ID in their
// order (Columns.Items on); nil where they are the frame's own row's.
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
	if slices.Equal(on, excerptColumns) || slices.Equal(on, excerptColumns[:len(excerptColumns)-1]) {
		return nil
	}
	return on
}

// columnRow is a note in a notebook of the columns a person chose: the
// notebook's mark, a cell for each column in their order — the dash where
// the note has nothing —, leading to the note.
func columnRow(r view.NoteRow, shown []string) w.ColumnRow {
	c := w.ColumnRow{Mark: w.Mark{Colour: string(r.Colour)}, Strong: r.On, On: r.On, Href: r.Href}
	for _, id := range shown {
		col, ok := cells[id]
		if !ok {
			continue
		}
		cell := w.ColumnCell{Column: col}
		switch col {
		case w.ColumnName:
			cell.Text = string(r.Title)
		case w.ColumnExcerpt:
			cell.Text = or(r.Excerpt)
		case w.ColumnTags:
			for _, tag := range bind.Tags(r.Tags) {
				cell.Tags = append(cell.Tags, tag)
			}
		case w.ColumnDate:
			cell.Text = or(r.Date)
		case w.ColumnSource:
			cell.Text = string(r.Stamp)
		case w.ColumnChanged:
			cell.Text = or(r.Changed)
		}
		c.Cells = append(c.Cells, cell)
	}
	return c
}

// or is a cell's words, the dash when the note has none.
func or(t view.Text) string {
	if t == "" {
		return none
	}
	return string(t)
}

// side is the panel of the note opened (Details): a reading, no deletion at
// its foot — Delete note is the last item of its «⋯» (s-notes-12).
func side(d view.Details) *w.Side {
	var tools []w.HeadTool
	var attach *view.Action
	for _, a := range d.Actions {
		if a.ID == attachID {
			attach = &a
			continue
		}
		b := bind.Button(a, "")
		b.Small = true
		if b.Icon != "" {
			b.Title, b.Label = b.Label, ""
		}
		tools = append(tools, b)
	}
	if len(d.Menu) > 0 {
		tools = append(tools, w.Drop{
			Trigger: w.Button{Icon: "dots", Small: true, Title: "More"},
			Menu:    w.Menu{Items: bind.MenuItems(d.Menu, rail.ReadOnly), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		})
	}
	sub := []w.HeadSubPart{w.Text(d.Subtitle)}
	if d.Date != "" {
		sub = []w.HeadSubPart{w.Text(string(d.Subtitle) + sep), w.Mono{Text: string(d.Date)}}
	}
	switch {
	case d.LinkedRecord && d.TextFrom != nil:
		// «in 2 notebooks · text from ▪ Записи · ‹day›» (s-notes-1): the
		// notebook the text was chosen from, no one copy's source.
		sub = []w.HeadSubPart{w.Text(string(d.Subtitle) + " "), w.Mark{Colour: string(d.TextFrom.Colour), Size: w.MarkH11}, w.Text(string(d.TextFrom.Name))}
		if d.Date != "" {
			sub = append(sub, w.Text(sep), w.Mono{Text: string(d.Date)})
		}
	case d.LinkedRecord:
	case d.Source.Colour != "":
		// The notebook's mark at its name (Typecase 434ac87, note 72).
		sub = append([]w.HeadSubPart{w.Mark{Colour: string(d.Source.Colour), Size: w.MarkH11}}, sub...)
	}
	items := []w.SidePart{w.Head{
		Title:  &w.HeadTitle{Text: string(d.Title), Size: w.HeadTitle19},
		Tools:  tools,
		Subs:   []w.HeadSub{{Parts: sub}},
		Acts:   &w.Acts{Items: []w.ActsPart{w.Close{Label: "Close the panel", Href: d.Close}}},
		Middle: true,
	}}
	if text := bind.SecText(d.Rendered, faces); len(text) > 0 {
		items = append(items, w.Sec{Items: text})
	}
	if len(d.Fields) > 0 {
		fields := w.Fields{}
		for _, f := range d.Fields {
			fields.Items = append(fields.Items, bind.FieldsItem(f, lead))
		}
		items = append(items, fields)
	}
	if attach != nil || len(d.Attach) > 0 {
		rubric := w.Rubric{Text: "Attachments"}
		if attach != nil {
			b := bind.Button(*attach, "")
			b.Menu, b.Small = true, true
			rubric.Items = []w.RubricPart{w.Drop{Trigger: b, Menu: w.Menu{Items: append(bind.MenuItems(attach.Arrow, ""), bind.Uploads(attach.Arrow, upload)...), Width: w.MenuWidth212, Pop: w.MenuUnder}}}
		}
		sec := []w.SecPart{rubric}
		if len(d.Attach) > 0 {
			sec = append(sec, w.SecList{Items: attachments(d.Attach), Gap: 6})
		}
		items = append(items, w.Sec{Items: sec})
	}
	if len(d.Related) > 0 {
		var lines [][]w.SecItemPart
		for _, r := range d.Related {
			about := lead + string(r.Kind)
			if r.When != "" {
				about += sep + string(r.When)
			}
			lines = append(lines, bind.WithFrom([]w.SecItemPart{
				w.Span{Face: w.FaceAccent, Text: string(r.Title), Href: r.Href},
				w.Text(" "),
				w.Dim{Parts: []w.Inline{w.Text(about)}},
			}, r.From))
		}
		items = append(items, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Related"}, w.SecList{Items: lines, Gap: 6}}})
	}
	// The note's people, as the agenda's panel names an event's (Р3 of the
	// server's kinds-parity list; no frame draws them yet — Typecase's task
	// 179), drawn as Related draws its records (bind.Person).
	if d.LinkedRecord {
		items = append(items, bind.KeptSec("Where this note is kept", d.Books, lead))
	}
	if len(d.Attendees) > 0 { // after what the frames draw
		var lines [][]w.SecItemPart
		for _, a := range d.Attendees {
			lines = append(lines, bind.Person(a, lead))
		}
		items = append(items, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Attendees"}, w.SecList{Items: lines}}}) // as the task's
	}
	// A file dropped on the open card goes with Attach «From this computer»
	// (Side.Drop, Typecase's note 68; s-io-2).
	side := &w.Side{Items: items}
	if attach != nil {
		side.Drop = bind.UploadForm(attach.Arrow)
	}
	return side
}

// attachments are the lines of a note's attachments: the name, a link where
// it goes (data inside the note downloads, a file in Files opens there), the
// type and size muted, and where it is kept — «in the note» for data inside
// it, the folder of a link (AttachRow.Place) — as a badge (S-20).
func attachments(rows []view.AttachRow) [][]w.SecItemPart {
	var out [][]w.SecItemPart
	for _, a := range rows {
		var line []w.SecItemPart
		if a.Href != "" {
			line = append(line, w.Link{Text: string(a.Name), Href: a.Href}, w.Text(" "))
		} else {
			line = append(line, w.Text(string(a.Name)+" "))
		}
		line = append(line, w.Dim{Parts: []w.Inline{w.Text(string(a.Type) + sep + string(a.Size))}})
		switch {
		case a.Inline:
			line = append(line, w.Text(" "), w.Badge{Label: inside})
		case a.Place != "":
			line = append(line, w.Text(" "), w.Badge{Label: string(a.Place)})
		}
		out = append(out, bind.WithFrom(line, a.From))
	}
	return out
}

// Rail is the page's rail alone and the form its ticks send: the end of a
// poll brings it out of band (C-7.4; Typecase's note 124).
func Rail(r view.Rail) ([]w.RailPart, *w.PostForm) { return railParts(r), bind.RailForm(r, formRail) }
