// Package contacts is the list of contacts, /app/contacts (the combined view)
// and /app/contacts/{account}/{col} (one book), drawn by frame «Контакты ·
// список и карточка» (s-contacts-1) of the mockup, and the read-only book by
// its state's frame (states-4): the drafts frame2go printed for them, with
// the frames' data become the view model's (U-5.1, U-6.1).
package contacts

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

// keptHint is what the properties kept as the server sent them are, under
// their rubric in the panel (s-contacts-1), as the card says it.
const keptHint = "Properties Carrel does not edit are stored byte for byte and go back untouched."

// The screen's words (U-1.3) for the shell: the mark, the search, the rail.
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
		NewKey: "n", SearchKey: "/",
		Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
		Keys:  bind.KeysWords{Label: "Keyboard shortcuts", Icon: "search", Key: "?", Sheet: "keys"},
	}
	rail    = bind.RailWords{Form: formRail, Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom"}
	columns = bind.ColumnWords{Name: "col", Reset: "default", Drag: "drag"}
)

// empty is the screen's word for a cell the record has nothing in (L9).
const empty = "—"

// Screen draws the list from its view model, in the state the model is in.
//
//   - The shell (U-2.1): the top bar; the rail of sections (Rail.Nav, the
//     open one on), the sources by account with their ticks, which submit
//     the rail's form (Rail.Ticks), the tags, the foot.
//   - The head: the title with the mark of what is open — all the books at
//     once, or the book's colour —, read-only where the book is (ReadOnly),
//     what is in it (Subtitle), and the bar's actions: New contact with its
//     arrow «Write into» (a closed menu of the ticked books, «last used»,
//     «ro» for one that takes nothing), Import, Export, Print.
//   - The notices under the head (Notices: a book that did not answer, its
//     Try again small), each with its technical block (states-11).
//   - The bar, while there are cards or a filter to clear (the empty states'
//     frames draw none): the filter of what is loaded (a GET form,
//     FilterForm, with the list's other choices), the order (Segments,
//     links), Columns with its menu (a form, closed), «⋯», «Compact rows» (a
//     GET form with its Apply) and the count loaded.
//   - The poll's bar while the poll runs or a source did not answer (C-6.4).
//   - The groups by letter with their count, a row for each card: its book's
//     mark, the avatar, the name (bold while it is open in the panel, a badge
//     of a linked record), phone, email, organisation — the dash where there
//     is none —, the book; the row leads to the card.
//   - Nothing to show (Empty): what, bold, why, and the buttons that help
//     (states-7, states-10).
//   - The panel (Shell.Details), for the row opened with ?open=: the photo or
//     initials, the name, Edit and All records, «⋯» with the record's menu,
//     the book, the cross that closes it (Details.Close); the fields; the
//     properties kept as the server sent them; the source; the deletion.
func Screen(p *view.ContactsListPage) w.Shell {
	main := append([]w.MainPart{head(p)}, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if len(p.Groups) > 0 || p.Bar.Filter != "" {
		main = append(main, bar(p))
	}
	if r := bind.Runbar(p.Poll); r != nil {
		main = append(main, *r)
	}
	compact := bind.Checked(p.Bar.Compact)
	// One collection, or a browser that hides the source column (Appearance).
	single := bind.Sourceless(p.Rail, p.Look)
	for _, g := range p.Groups {
		rows := make([]w.ListRow, 0, len(g.Rows))
		for _, r := range g.Rows {
			rows = append(rows, row(r, single))
		}
		main = append(main,
			w.ListGroup{Title: w.Rubric{Text: string(g.Rubric), Form: w.RubricInline}, Count: &w.Num{Value: g.Count}},
			w.List{Rows: rows, Compact: compact})
	}
	if p.Empty != nil {
		main = append(main, w.Empty{Parts: bind.EmptyParts(*p.Empty)})
	}
	if p.Poll != nil && p.Poll.Note != "" {
		// What is shown and what is missing, under the rows, once a source
		// did not answer (PollBar.Note; states-9).
		main = append(main, w.Empty{Parts: []w.EmptyPart{w.Text(p.Poll.Note)}, Variant: w.EmptySmall})
	}
	body := w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}
	if p.Details != nil {
		body.Side = side(*p.Details)
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: body}
}

// Forms are the page's forms: the rail's, which its ticks submit, and the
// panel's deletion, the retries of the sources, the buttons of the empty
// state and the items of the menus that post (Sign out).
func Forms(p *view.ContactsListPage) []w.PostForm {
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
	if p.Details != nil {
		for _, a := range p.Details.Actions {
			if a.Danger && a.Method == "POST" {
				out = append(out, bind.ActionForm(a, formDelete))
			}
		}
		out = append(out, bind.MenuForms(p.Details.Menu)...)
	}
	return out
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

// head is the list's head: what is open, what is in it, the bar's actions.
func head(p *view.ContactsListPage) w.Head {
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
	if p.ReadOnly {
		title.Mark = nil
		title.Badge = &w.Badge{Label: "read-only"}
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
	return w.Head{
		Title: title,
		Subs:  []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}},
		Acts:  &w.Acts{Items: acts},
	}
}

// bar is the bar over the list.
func bar(p *view.ContactsListPage) w.Bar {
	b := p.Bar
	var items []w.BarPart
	if b.FilterForm != nil {
		f := bind.Filter(b, "Filter the loaded cards")
		f.Variant = w.FilterH26W220
		items = append(items, f)
	}
	// «⋯» holds the twins of what leaves the bar as it narrows
	// (components-19, task 82): the order as items, the tick its same GET
	// form.
	var more []w.MenuPart
	if len(b.Segments) > 0 {
		seg := w.Seg{Secondary: true}
		var order []view.MenuItem
		for _, a := range b.Segments {
			seg.Options = append(seg.Options, bind.SegOption(a))
			order = append(order, view.MenuItem{Action: a})
		}
		items = append(items, seg)
		more = append(more, bind.MenuItems(order, "")...)
	}
	if b.Columns != nil {
		cols := bind.Columns(*b.Columns, columns)
		cols.Menu.Pop = w.MenuUnder
		items = append(items, cols)
	}
	var right []w.BarPart
	if b.Compact != nil {
		compact := bind.CheckForm(*b.Compact, "Apply")
		right = append(right, compact)
		if len(more) > 0 {
			more = append(more, w.MenuSep{})
		}
		more = append(more, compact)
	}
	right = append(right, w.Mono{Text: strconv.Itoa(b.Loaded)})
	if len(more) > 0 {
		items = append(items, w.Drop{
			Trigger: w.Button{Icon: "dots", Title: "More"},
			More:    true,
			Menu:    w.Menu{Items: more, Pop: w.MenuUnderEnd, Inline: true},
		})
	}
	return w.Bar{Items: items, Right: &w.BarRight{Items: right, Secondary: true}}
}

// row is a card in the list; in one book (single) it has no source column —
// the book is named in the head (states-4).
func row(r view.ContactRow, single bool) w.FaceRow {
	f := w.FaceRow{
		Mark:     w.Mark{Colour: string(r.Colour)},
		Avatar:   bind.Avatar(r.Avatar),
		Title:    []w.Inline{w.Text(r.Name)},
		Strong:   r.On,
		Subtitle: or(r.Phone),
		Detail:   or(r.Email),
		Caption:  or(r.Org),
		Source:   string(r.Stamp),
		On:       r.On,
		Href:     r.Href,
	}
	if len(r.Badges) > 0 {
		b := bind.Badge(r.Badges[0])
		f.TitleBadge = &b
	}
	if single {
		f.Source, f.Single = "", true
	}
	return f
}

// or is a cell's words, the dash when the record has none.
func or(t view.Text) string {
	if t == "" {
		return empty
	}
	return string(t)
}

// side is the panel of the row opened (Details).
func side(d view.Details) *w.Side {
	var tools []w.HeadTool
	var danger []w.Button
	for _, a := range d.Actions {
		b := bind.Button(a, formDelete)
		if a.Danger {
			b.Icon = ""
			danger = append(danger, b)
			continue
		}
		b.Small = true
		tools = append(tools, b)
	}
	if len(d.Menu) > 0 {
		tools = append(tools, w.Drop{
			Trigger: w.Button{Icon: "dots", Small: true, Title: "More"},
			Menu:    w.Menu{Items: bind.MenuItems(d.Menu, rail.ReadOnly), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		})
	}
	sub := bind.SourceSub(d.Source.Colour, d.Subtitle)
	if d.LinkedRecord {
		// The linked record: «in 3 books», no mark of one book (s-contacts-1).
		sub = w.HeadSub{Parts: []w.HeadSubPart{w.Text(d.Subtitle)}}
	}
	h := w.Head{
		Title:  &w.HeadTitle{Text: string(d.Title), Size: w.HeadTitle19},
		Tools:  tools,
		Subs:   []w.HeadSub{sub},
		Acts:   &w.Acts{Items: []w.ActsPart{w.Close{Label: "Close the panel", Href: d.Close}}},
		Middle: true,
	}
	if d.Avatar != nil {
		h.Photo = &w.HeadPhoto{Initials: string(d.Avatar.Initials), Src: d.Avatar.Href}
	}
	items := []w.SidePart{h}
	fields := w.Fields{}
	for _, f := range d.Fields {
		fields.Items = append(fields.Items, bind.FieldsItem(f, "· "))
	}
	items = append(items, fields)
	if d.ForeignShow != nil {
		show := bind.Button(*d.ForeignShow, "")
		show.Small = true
		// What it is, under the rubric, as the card says it (s-contacts-1).
		items = append(items, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: "Kept as the server sent them", Items: []w.RubricPart{show}},
			w.SecHint{Text: keptHint},
		}})
	}
	if d.LinkedRecord {
		// Where the person is kept, each book with what it holds
		// otherwise; the source is a copy's (s-contacts-1, note 117 p. 5).
		var lines [][]w.SecItemPart
		for _, f := range d.Books {
			lines = append(lines, bind.Book(f, "· "))
		}
		items = append(items, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: "Where this person is kept", Items: []w.RubricPart{w.Badge{Label: strconv.Itoa(len(d.Books)), Tone: w.BadgeLinked}}},
			w.SecList{Items: lines, Gap: 5},
		}})
		return &w.Side{Items: items}
	}
	items = append(items, w.Sec{Items: []w.SecPart{
		w.Rubric{Text: "Source"},
		bind.Diag(bind.DiagLine{Name: "UID", Value: string(d.Source.UID)}, bind.DiagLine{Name: "Read", Value: string(d.Source.LastRead)}),
	}})
	for _, b := range danger {
		items = append(items, w.Danger{Button: b})
	}
	return &w.Side{Items: items}
}

// Rail is the page's rail alone and the form its ticks send: the end of a
// poll brings it out of band (C-7.4; Typecase's note 124).
func Rail(r view.Rail) ([]w.RailPart, *w.PostForm) { return railParts(r), bind.RailForm(r, formRail) }
