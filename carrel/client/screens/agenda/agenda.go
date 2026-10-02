// Package agenda is the agenda, /app/calendar (the combined view) and
// /app/calendar/{account}/{col} (one calendar), drawn by frame «Календарь ·
// агенда со списком и карточкой» (s-agenda-1) of the mockup and by the
// frames of its states — the poll running (states-13), nothing ticked
// (states-14), one calendar's empty window (states-15): the drafts frame2go
// printed for them, with the frames' data become the view model's (U-5.1,
// U-6.2). The agenda is a list, not a grid: Day, Week and Month are windows
// of the same list.
package agenda

import (
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

// The screen's words (U-1.3) for the shell: the mark, the search, the rail.
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
	}
	rail = bind.RailWords{Form: formRail, Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external"}
)

// The screen's words (U-1.3): the dash of a row with no place, the lead of a
// status and a kind, what a badge of a time made in another zone says, the
// attachment kept inside the event.
const (
	none     = "—"
	lead     = "· "
	zoneHint = "Created in another time zone"
	inside   = "in the event"
)

// The IDs of the range's presets the bar draws as buttons (C-7): the step
// back, today, the step forward; the others are its switch of windows. And
// the name of the action Attach (C-7.5), which the panel puts in the rubric
// of attachments.
const (
	presetPrevious = "range.previous"
	presetToday    = "range.current"
	presetNext     = "range.next"
	attachID       = "attach.add"
)

// Screen draws the agenda from its view model, in the state the model is in.
//
//   - The shell (U-2.1): the top bar; the rail of sections (the open one
//     on), the sources by account with their ticks, which submit the rail's
//     form, the feeds' rubric «external», a source that did not answer in
//     error with its retry, the tags, the foot.
//   - The head: the mark of what is open — every calendar at once, or the
//     calendar's colour —, the title, what is shown (Subtitle: the window,
//     the zone monospaced) and the bar's actions: New event — with its arrow
//     in the combined view, a closed menu of the ticked calendars, «last
//     used» —, Import, Export, Print.
//   - The notices under the head.
//   - The bar while something is ticked (states-14 draws none): the step
//     back, Today and the step forward, the switch of windows (Day, Week,
//     Month, Range — links), the range of dates (a GET form, From, To,
//     Show), «Events · Tasks due · Notes» (a GET form of ticks with its
//     Apply), «⋯» with their twins for a narrow bar, and on its right
//     «Compact rows» (a GET form).
//   - The poll's bar while the poll runs or a source did not answer (C-6.4):
//     what it polled, the source that did not answer as an alarm, on its
//     right when the answer was gathered or how long the poll runs.
//   - The days with events only (a day without one is not printed), each
//     with its count, a row for each record: its calendar's mark, the time
//     monospaced, the title (bold while open in the panel), the place (the
//     dash where there is none), the first badge, where it lives; the row
//     leads to the record.
//   - Nothing to show (Empty): what, bold, why, and the button that helps.
//   - The panel (Shell.Details), for the row opened with ?open=: the
//     calendar's mark, the title, the calendar, Edit, «⋯» with the record's
//     menu, the cross that closes it; the fields (When in its pieces, the zone
//     it was made in as a badge, Repeats with its end as a badge), the
//     description, the attendees, the attachments with Attach, the related
//     records, the source, the deletion and the line of a series under it.
func Screen(p *view.AgendaPage) w.Shell {
	main := []w.MainPart{head(p)}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if ticked(p.Rail) {
		main = append(main, bar(p))
	}
	if r := bind.Runbar(p.Poll); r != nil {
		main = append(main, *r)
	}
	compact := bind.Checked(p.Bar.Compact)
	for _, d := range p.Days {
		rows := make([]w.ListRow, 0, len(d.Rows))
		for _, r := range d.Rows {
			rows = append(rows, row(r))
		}
		main = append(main,
			w.ListGroup{Title: w.Rubric{Text: string(d.Rubric), Form: w.RubricInline}, Count: &w.Num{Value: d.Count}},
			w.List{Rows: rows, Compact: compact})
	}
	if p.Empty != nil {
		main = append(main, w.Empty{Parts: bind.EmptyParts(*p.Empty)})
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
// event), and of the panel — its deletion, its menu, Attach «From this
// computer» with its file.
func Forms(p *view.AgendaPage) []w.PostForm {
	out := bind.TopForms(p.Top)
	for _, a := range p.Bar.Actions {
		out = append(out, bind.MenuForms(a.Arrow)...)
	}
	if f := bind.RailForm(p.Rail, formRail); f != nil {
		out = append(out, *f)
	}
	out = append(out, bind.RetryForms(p.Rail)...)
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
// the agenda has no window to move, and draws no bar (states-14).
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

// head is the agenda's head: what is open, what is shown, the bar's actions.
func head(p *view.AgendaPage) w.Head {
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
	return w.Head{Title: title, Subs: bind.Sub(p.Subtitle), Acts: &w.Acts{Items: acts}}
}

// bar is the bar over the agenda.
func bar(p *view.AgendaPage) w.Bar {
	b := p.Bar
	var items []w.BarPart
	var twins []w.Range
	var menu []w.MenuPart
	if r := b.Range; r != nil {
		seg := w.Seg{}
		for _, a := range r.Presets {
			switch a.ID {
			case presetPrevious, presetToday, presetNext:
				btn := bind.Button(a, "")
				if btn.Icon != "" {
					btn.Title, btn.Label = btn.Label, ""
				}
				items = append(items, btn)
			default:
				seg.Options = append(seg.Options, bind.SegOption(a))
			}
		}
		if len(seg.Options) > 0 {
			items = append(items, seg)
		}
		if r.Form != nil {
			// ждёт библиотеку: U-5.5 #23 — the range sends From and To only:
			// Range has no hidden fields, and the window «Range» of the
			// contract (view=range) does not go with them
			rng := dates(*r.Form)
			rng.Secondary = true
			items = append(items, rng)
			twin := dates(*r.Form)
			twin.Wrap = true
			twins = append(twins, twin)
		}
	}
	if f := b.Show; f != nil {
		show := bind.CheckForm(*f, "Apply")
		show.Tertiary = true
		items = append(items, w.BarSep{Secondary: true}, show)
		for _, c := range show.Checks {
			menu = append(menu, tick(c, true))
		}
		menu = append(menu, w.MenuSep{Tertiary: true})
	}
	for _, t := range twins {
		menu = append(menu, t)
	}
	var right []w.BarPart
	if b.Compact != nil {
		compact := bind.CheckForm(*b.Compact, "Apply")
		right = append(right, compact)
		menu = append(menu, w.MenuSep{})
		for _, c := range compact.Checks {
			menu = append(menu, tick(c, false))
		}
	}
	if len(menu) > 0 {
		// ждёт библиотеку: U-5.5 #22 — the twins of the bar's ticks in «⋯»
		// (components-19): a tick of a form that goes GET has no twin in a
		// menu, whose form posts
		items = append(items, w.Drop{
			Trigger: w.Button{Icon: "dots", Title: "More"},
			More:    true,
			Menu:    w.Menu{Items: menu, Pop: w.MenuUnderEnd, Inline: true},
		})
	}
	bar := w.Bar{Items: items, Variant: w.BarWide}
	if len(right) > 0 {
		bar.Right = &w.BarRight{Items: right, Secondary: true}
	}
	return bar
}

// dates is the range of dates of the bar (Range.Form, a GET form): From and
// To, Show its submit.
func dates(f view.Form) w.Range {
	rng := w.Range{Action: f.Action}
	rows := bind.Rows(f)
	if len(rows) > 0 {
		rng.From = bind.Input(rows[0])
	}
	if len(rows) > 1 {
		rng.To = bind.Input(rows[1])
	}
	for _, a := range f.Actions {
		rng.Show = bind.Submit(a)
	}
	return rng
}

// tick is the twin of a bar's tick in its «⋯»: the item, its box as the
// tick is, its words.
func tick(c w.Check, tertiary bool) w.MenuItem {
	box := &w.TickBox{}
	if !c.On {
		box.Ticked = w.TickedOff
	}
	var label string
	for _, p := range c.Parts {
		if t, ok := p.(w.Text); ok {
			label += string(t)
		}
	}
	return w.MenuItem{Tick: box, Label: label, Tertiary: tertiary}
}

// row is a record of the agenda.
func row(r view.AgendaRow) w.TimeRow {
	t := w.TimeRow{
		Mark:   w.Mark{Colour: string(r.Colour)},
		Time:   string(r.Time),
		Title:  []w.Inline{w.Text(r.Summary)},
		Strong: r.On,
		Detail: string(r.Location),
		Source: string(r.Stamp),
		On:     r.On,
		Href:   r.Href,
	}
	if t.Detail == "" {
		t.Detail = none
	}
	if len(r.Badges) > 0 {
		b := bind.Badge(r.Badges[0])
		t.Caption = &b
	}
	return t
}

// side is the panel of the event opened (Details).
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
	title := &w.HeadTitle{Text: string(d.Title), Size: w.HeadTitle19}
	if d.Source.Colour != "" {
		title.Mark = &w.Mark{Colour: string(d.Source.Colour), Size: w.MarkH19}
	}
	items := []w.SidePart{w.Head{
		Title: title,
		Subs:  []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(d.Subtitle)}}},
		Acts:  &w.Acts{Items: acts},
	}}
	items = append(items, fields(d.Fields))
	if d.Body != "" {
		items = append(items, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: "Description"},
			w.Prose{Parts: []w.Inline{w.Text(d.Body)}, Variant: w.Prose13},
		}})
	}
	if len(d.Attendees) > 0 {
		var lines [][]w.SecItemPart
		for _, a := range d.Attendees {
			lines = append(lines, bind.Attendee(a, lead))
		}
		items = append(items, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: "Attendees", Items: []w.RubricPart{w.Mono{Text: strconv.Itoa(len(d.Attendees)), Variant: w.MonoTight}}},
			w.SecList{Items: lines, Columns: true},
		}})
	}
	if attach != nil || len(d.Attach) > 0 {
		rubric := w.Rubric{Text: "Attachments"}
		if attach != nil {
			b := bind.Button(*attach, "")
			b.Icon, b.Menu, b.Small = "", true, true
			rubric.Items = []w.RubricPart{w.Drop{Trigger: b, Menu: w.Menu{Items: bind.MenuItems(attach.Arrow, ""), Width: w.MenuWidth212, Pop: w.MenuUnder}}}
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
			lines = append(lines, []w.SecItemPart{
				w.Span{Face: w.FaceAccent, Text: string(r.Title), Href: r.Href},
				w.Text(" "),
				w.Dim{Parts: []w.Inline{w.Text(lead + string(r.Kind))}},
			})
		}
		items = append(items, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Related"}, w.SecList{Items: lines}}})
	}
	items = append(items, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Source"}, source(d.Source)}})
	for _, b := range danger {
		items = append(items, w.Danger{Button: b, Note: string(d.DeleteNote)})
	}
	return &w.Side{Items: items}
}

// fields are a record's fields as the lines of a card: When in its pieces,
// a badge of a time made in another zone saying so where a pointer rests.
func fields(fs []view.Field) w.Fields {
	out := w.Fields{}
	for _, f := range fs {
		it := bind.FieldsItem(f, lead)
		if len(f.Runs) > 0 && f.Badge.Label != "" {
			b := it.Value[len(it.Value)-1].(w.Badge)
			b.Title = zoneHint
			it.Value[len(it.Value)-1] = b
		}
		out.Items = append(out.Items, it)
	}
	return out
}

// attachments are the lines of a record's attachments: the name, the type
// and size muted, and where the file is kept — «in the event» for data inside
// it (S-20).
//
// ждёт библиотеку: U-5.5 #29 — no frame draws a line's actions
// (AttachRow.Actions, Remove): an attachment is not taken off here
func attachments(rows []view.AttachRow) [][]w.SecItemPart {
	var out [][]w.SecItemPart
	for _, a := range rows {
		line := []w.SecItemPart{w.Text(string(a.Name) + " "), w.Dim{Parts: []w.Inline{w.Text(string(a.Type) + " · " + string(a.Size))}}}
		if a.Inline {
			line = append(line, w.Text(" "), w.Badge{Label: inside})
		}
		out = append(out, line)
	}
	return out
}

// source is the technical block of a record's source: its path, when it was
// read and whether it changed on the server.
func source(s view.SourceBlock) w.Diag {
	read := string(s.LastRead)
	if s.Compared != "" {
		read += " · " + string(s.Compared)
	}
	return bind.Diag(bind.DiagLine{Name: "Path", Value: string(s.Path)}, bind.DiagLine{Name: "Read", Value: read})
}
