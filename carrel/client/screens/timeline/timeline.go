// Package timeline is everything a person is in,
// /app/contacts/{account}/{col}/{uid}/timeline, drawn by frame «Контакт ·
// хронология» (s-contacts-7) of the mockup: the draft frame2go printed for
// it, with the frame's data become the view model's (U-5.1, U-6.1).
package timeline

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The IDs of the page's forms: the rail's (its ticks submit it) and the
// card's deletion.
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
	rail    = bind.RailWords{Form: formRail, Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom"}
	columns = bind.ColumnWords{Name: "col", Reset: "default", Drag: "drag"}
)

// Screen draws the timeline from its view model.
//
//   - The shell: the top bar; the rail of sections with «Back to list» apart
//     under them, and «Where to look» — the sources the timeline polled,
//     with their ticks, a feed ro, one that did not answer in error with
//     its retry —, the foot.
//   - The head: the person's avatar, «Timeline», the name, where and who,
//     «in 2 books» for a person in several (Linked); the actions — write a
//     note, a new event, a new task about the person, Print.
//   - The bar: the range of dates (a GET form, From, To, Show), the filter
//     by kind with each count (links; on a narrow bar it folds), Columns,
//     «⋯», «newest first».
//   - The poll's bar while it runs or a source did not answer: what it
//     matched by, monospaced, the source that did not answer as an alarm.
//   - The months with their count, a row for each record: its mark, the
//     day, the title, why it is here, its kind, where it lives.
//   - The foot: from when the timeline looked (Note).
//   - The card in the panel, the screen's context (Card): photo, «Card»,
//     Open the card, Edit, «⋯» with the card's menu; its fields, the same
//     person in other books, the source, the deletion.
func Screen(p *view.ContactTimelinePage) w.Shell {
	var avatar w.Avatar
	if p.Card.Avatar != nil {
		avatar = bind.Avatar(*p.Card.Avatar)
	}
	avatar.Size = w.Avatar44
	h := w.Head{
		Avatar: &avatar,
		Kicker: &w.Rubric{Text: "Timeline", Variant: w.RubricBottom2},
		Title:  &w.HeadTitle{Text: string(p.Name)},
		Pad:    w.HeadPadBottom10,
	}
	sub := w.HeadSub{}
	if p.Subtitle != "" {
		lead := string(p.Subtitle)
		if p.Linked != nil {
			lead += " "
		}
		sub.Parts = append(sub.Parts, w.Text(lead))
	}
	if p.Linked != nil {
		sub.Parts = append(sub.Parts, bind.Badge(*p.Linked))
	}
	if len(sub.Parts) > 0 {
		h.Subs = []w.HeadSub{sub}
	}
	var acts []w.ActsPart
	for _, a := range p.Actions {
		acts = append(acts, bind.Button(a, ""))
	}
	h.Acts = &w.Acts{Items: acts}
	main := bind.Notices(p.Notices, w.MsgSpacePage)
	main = append(main, h, bar(p))
	if r := bind.Runbar(&p.Poll); r != nil {
		main = append(main, *r)
	}
	for _, g := range p.Groups {
		rows := make([]w.ListRow, 0, len(g.Rows))
		for _, r := range g.Rows {
			rows = append(rows, w.FindRow{
				Mark:   w.Mark{Colour: string(r.Colour)},
				Date:   string(r.When),
				Title:  []w.Inline{w.Text(r.Title)},
				Detail: []w.Inline{w.Text(r.Why)},
				What:   string(r.Kind),
				Source: string(r.Stamp),
				Href:   r.Href,
			})
		}
		main = append(main,
			w.ListGroup{Title: w.Rubric{Text: string(g.Rubric), Form: w.RubricInline}, Count: &w.Num{Value: g.Count}},
			w.List{Rows: rows})
	}
	if p.Note != "" {
		main = append(main, w.Empty{Parts: []w.EmptyPart{w.Text(p.Note)}, Variant: w.EmptySmallTop})
	}
	t := bind.Top(p.Top, top)
	var panel *w.Side
	if !p.CardClosed {
		panel = side(p.Card)
	}
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}, Side: panel}}
}

// Forms are the page's forms: the rail's, the retries of its sources, the
// card's deletion, the items of the menus that post (Sign out).
func Forms(p *view.ContactTimelinePage) []w.PostForm {
	out := append(bind.TopForms(p.Top), bind.MenuForms(p.Card.Menu)...)
	if f := bind.RailForm(p.Rail, formRail); f != nil {
		out = append(out, *f)
	}
	out = append(out, bind.PollForms(&p.Poll)...)
	out = append(out, bind.NoticeForms(p.Notices)...)
	for _, a := range p.Card.Actions {
		if a.Danger && a.Method == "POST" && a.Enabled {
			out = append(out, bind.ActionForm(a, formDelete))
		}
	}
	return out
}

// railParts is the rail of sections with the way back apart under them.
func railParts(r view.Rail) []w.RailPart {
	nav, back := w.Nav{}, w.Nav{Apart: true}
	for _, n := range r.Nav {
		it := bind.NavItem(n)
		if n.ID == "nav.back" {
			it.Short = "Back"
			back.Items = append(back.Items, it)
			continue
		}
		nav.Items = append(nav.Items, it)
		if n.On {
			nav.On = len(nav.Items)
		}
	}
	parts := []w.RailPart{nav}
	if len(back.Items) > 0 {
		parts = append(parts, back)
	}
	parts = append(parts, bind.Sources(r, rail)...)
	return append(parts, bind.Foot(r, rail))
}

// bar is the bar over the timeline.
func bar(p *view.ContactTimelinePage) w.Bar {
	var items []w.BarPart
	if f := p.Range.Form; f != nil {
		rng := w.Range{Action: f.Action}
		rows := bind.Rows(*f)
		if len(rows) > 0 {
			rng.From = bind.Input(rows[0])
		}
		if len(rows) > 1 {
			rng.To = bind.Input(rows[1])
		}
		for _, a := range f.Actions {
			rng.Show = bind.Submit(a)
		}
		items = append(items, rng)
	}
	if len(p.Bar.Segments) > 0 {
		seg := w.Seg{}
		for _, a := range p.Bar.Segments {
			seg.Options = append(seg.Options, bind.SegOption(a))
		}
		items = append(items, w.SegFold{Seg: seg})
	}
	if c := p.Bar.Columns; c != nil {
		cols := bind.Columns(*c, columns)
		trigger := cols.Trigger.(w.Button)
		trigger.Secondary = true
		cols.Trigger, cols.Menu.Pop = trigger, w.MenuUnder
		items = append(items, cols)
	}
	return w.Bar{
		Items: items,
		// ждёт библиотеку: U-5.5 #22 — the bar's «⋯»: a button with no menu
		// of what left the bar (components-19); a tick that is a form has no
		// twin in a menu
		More:    &w.Button{Icon: "dots", Title: "More"},
		Right:   &w.BarRight{Items: []w.BarPart{w.Mono{Text: "newest first"}}, Secondary: true},
		Variant: w.BarWide,
	}
}

// side is the card in the panel: the screen's context, not a selection.
func side(c view.Details) *w.Side {
	var tools []w.HeadTool
	var danger []w.Button
	for _, a := range c.Actions {
		b := bind.Button(a, formDelete)
		if a.Danger {
			b.Icon = ""
			danger = append(danger, b)
			continue
		}
		b.Small = true
		tools = append(tools, b)
	}
	if len(c.Menu) > 0 {
		tools = append(tools, w.Drop{
			Trigger: w.Button{Icon: "dots", Small: true, Title: "More"},
			Menu:    w.Menu{Items: bind.MenuItems(c.Menu, rail.ReadOnly), Width: w.MenuWidth212, Pop: w.MenuUnder},
		})
	}
	photo := &w.HeadPhoto{Large: true}
	if c.Avatar != nil {
		photo.Initials, photo.Src = string(c.Avatar.Initials), c.Avatar.Href
	}
	h := w.Head{
		Photo:  photo,
		Kicker: &w.Rubric{Text: "Card"},
		Tools:  tools,
		Acts:   &w.Acts{Items: []w.ActsPart{w.Close{Label: "Close the panel", Href: c.Close}}},
		Middle: true,
		Pad:    w.HeadPadBottom10,
	}
	items := []w.SidePart{h}
	fields := w.Fields{}
	for _, f := range c.Fields {
		fields.Items = append(fields.Items, bind.FieldsItem(f, "· "))
	}
	items = append(items, fields)
	if len(c.Books) > 0 {
		var lines [][]w.SecItemPart
		for _, f := range c.Books {
			lines = append(lines, bind.Book(f, "· "))
		}
		items = append(items, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: "The same person in other books", Items: []w.RubricPart{w.Badge{Label: strconv.Itoa(len(c.Books)), Tone: w.BadgeLinked}}},
			w.SecList{Items: lines, Gap: 6},
			w.SecHint{Text: "Linked on the duplicates screen. The records stay on their own servers; here they are shown as one person.", Apart: true},
		}})
	}
	items = append(items, w.Sec{Items: []w.SecPart{
		w.Rubric{Text: "Source"},
		bind.Diag(bind.DiagLine{Name: "UID", Value: string(c.Source.UID)}, bind.DiagLine{Name: "Read", Value: string(c.Source.LastRead)}),
	}})
	for _, b := range danger {
		items = append(items, w.Danger{Button: b})
	}
	return &w.Side{Items: items}
}
