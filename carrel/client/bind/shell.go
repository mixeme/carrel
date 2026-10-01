package bind

import (
	"strconv"

	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The shell (U-2.1, C-6.3): the top bar, the rail of three kinds, the poll
// bar, the notices — the same on every screen inside the sign-in, and each
// shell screen draws it from its model's Shell with these translations. The
// words of the frame that stay the same whatever the data — the search's
// accessible name, «ro», the dash of a count nobody has, the rubric «Tags»
// — are the screen's (U-1.3), and come in with the screen's words (TopWords,
// RailWords); so do the names of the protocol a form sends a choice by,
// which the contract names in C-5 and not in a field.

// TopWords are what the top bar takes from the screen: the mark and the
// accessible name of the search.
type TopWords struct {
	Brand  w.Brand
	Search string // «Search everything…»
}

// Top is the top bar (TopBar): the mark, the search that sends its query to
// Search.Action under Search.Name, then New — the primary button with its arrow (Action.Arrow:
// the menu of what else can be made, a closed menu the frame draws beside
// the screen) —, the refresh, which says when the last poll of this section
// ended and polls again (none where the page has no poll of its own:
// Refresh.Action is empty), and the account's button, which goes to the
// account's page (UserHref).
func Top(t view.TopBar, words TopWords) w.Top {
	newBtn := Button(t.New, "")
	newBtn.Menu = len(t.New.Arrow) > 0
	var actions []w.TopAction
	if newBtn.Menu {
		actions = append(actions, w.TopDrop{Trigger: newBtn, Menu: w.Menu{Items: MenuItems(t.New.Arrow, ""), Pop: w.MenuUnder}})
	} else {
		actions = append(actions, newBtn)
	}
	if t.Refresh.Action.ID != "" {
		refresh := Button(t.Refresh.Action, "")
		refresh.Label = string(t.Refresh.Answered)
		actions = append(actions, refresh)
	}
	actions = append(actions, w.UserMenu{Name: string(t.Signed.Login), Href: t.UserHref})
	return w.Top{
		Brand: words.Brand,
		Search: &w.Search{
			Query:  w.Query{Value: string(t.Search.Query), Label: words.Search, Name: t.Search.Name},
			Action: t.Search.Action,
		},
		Actions: actions,
	}
}

// MenuItems are the items of a menu (C-6.2 MenuItem), a rule before an item
// that has one (Separator): its icon, its words, where it goes; the mark of
// its collection (Colour) and the muted words at its end (Aside, «last
// used»); one switched off says why (Reason), and one of a collection that
// cannot be written to carries ro, the screen's word for it; a destructive
// one is in the danger's colour; the one chosen is on.
func MenuItems(items []view.MenuItem, ro string) []w.MenuPart {
	var out []w.MenuPart
	for _, it := range items {
		if it.Separator && len(out) > 0 {
			out = append(out, w.MenuSep{})
		}
		m := w.MenuItem{Icon: Icon(it.Icon), Label: string(it.Label), On: it.On}
		if it.Enabled {
			m.Href = it.Href
		} else {
			m.Reason = string(it.Reason)
		}
		if it.Colour != "" {
			m.Mark = &w.Mark{Colour: string(it.Colour)}
		}
		switch {
		case it.ReadOnly && ro != "":
			m.Trail = w.Badge{Label: ro}
		case it.Aside != "":
			m.Trail = w.Dim{Parts: []w.Inline{w.Text(it.Aside)}}
		}
		if it.Danger {
			m.Tone = w.MenuToneDanger
		}
		out = append(out, m)
	}
	return out
}

// NavItem is a section of the rail (NavRow) as an item of the navigation:
// its icon, its name, where it goes, and its count when it has one — a
// section whose count is not counted has none (nil), not a zero.
func NavItem(n view.NavRow) w.NavItem {
	it := w.NavItem{Icon: Icon(n.Icon), Label: string(n.Label), Href: n.Href}
	if n.Count != nil {
		it.Count = &w.Num{Value: *n.Count}
	}
	return it
}

// RailWords are what the rail takes from the screen: the ID of the rail's
// form (Rail.Ticks), the name a tick sends its row by (toggle, C-5
// /app/sources/{kind}), the screen's words for a count nobody has (the dash)
// and for read-only (ro), the rubric of the tags, and the emblem's symbol of
// davloom in the sprite.
type RailWords struct {
	Form     string
	Toggle   string
	None     string
	ReadOnly string
	Tags     string
	Davloom  string
}

// Ticked is the state of a source's tick (C-8 TickState) as the box's.
func Ticked(t view.TickState) w.Ticked {
	switch t {
	case view.TickOff:
		return w.TickedOff
	case view.TickPart:
		return w.TickedPart
	}
	return w.TickedOn
}

// TickRow is a source of the rail (SourceRow) as a row with its tick: the
// tick's state, the mark of its colour — the row of the combined view, which
// has none, is all the sources at once —, a feed read from outside, its
// name, the open one (On), one not shown (ticked off). Its count, the dash
// when it has none; a feed shows ro instead. A source that cannot be written
// to carries ro after its count. One that does not answer is in error,
// says why short and in full, and offers to retry only it. With the rail's
// form (Rail.Ticks), the tick sends the row's address under Toggle; without
// it the choice is made, and the tick shows it (the rail of return, C-6.3).
func TickRow(r view.SourceRow, words RailWords) w.TickRow {
	row := w.TickRow{
		Ticked:  Ticked(r.Tick),
		Mark:    w.Mark{Colour: string(r.Colour)},
		Label:   string(r.Label),
		Current: r.On,
		Hidden:  r.Tick == view.TickOff,
	}
	switch {
	case r.Feed:
		row.Kind, row.Mark.Kind = w.TickRowExternal, w.MarkExternal
	case r.Colour == "":
		row.Kind, row.Mark.Kind = w.TickRowAll, w.MarkMulti
	}
	switch {
	case r.State == view.CollectionUnavailable:
		row.Error, row.Meta, row.MetaTitle = true, string(r.Message), string(r.Detail)
		if r.Retry != nil {
			retry := Button(*r.Retry, RetryID(r))
			retry.Small = true
			row.Trail = retry
		}
	case r.Feed:
		row.Meta = words.ReadOnly
	case r.Count != nil:
		row.Meta = strconv.Itoa(*r.Count)
	default:
		row.Meta = words.None
	}
	if r.ReadOnly && !r.Feed && row.Trail == nil {
		row.Trail = w.Badge{Label: words.ReadOnly}
	}
	if words.Form != "" {
		row.Send = w.Send{Form: words.Form, Name: words.Toggle, Value: r.Href}
	}
	return row
}

// Sources are the rail's sections of sources (Rail.Sections), each its
// account as the rubric when it has one and its sources' rows, and after
// them the tags (Rail.Tags) under the screen's rubric. A tag does not lead
// anywhere yet: ждёт библиотеку: U-5.5 #10 — a tag of the library has no
// address, and the rail's filter by category goes nowhere until it has.
func Sources(r view.Rail, words RailWords) []w.RailPart {
	rf := words
	if r.Ticks == nil {
		rf.Form = ""
	}
	var out []w.RailPart
	for _, sec := range r.Sections {
		var items []w.RailSecPart
		if sec.Account != "" {
			items = append(items, w.Rubric{Text: string(sec.Account)})
		}
		for _, row := range sec.Rows {
			items = append(items, TickRow(row, rf))
		}
		out = append(out, w.RailSec{Items: items})
	}
	if len(r.Tags) > 0 {
		out = append(out, w.RailSec{Items: []w.RailSecPart{w.Rubric{Text: words.Tags}}, Tags: Tags(r.Tags)})
	}
	return out
}

// Tags are the categories (C-6.2 Tag) as tags, the one chosen on; the tag's
// address waits for the library (ждёт библиотеку: U-5.5 #10).
func Tags(tags []view.Tag) []w.Tag {
	out := make([]w.Tag, 0, len(tags))
	for _, t := range tags {
		out = append(out, w.Tag{Label: string(t.Label), On: t.On})
	}
	return out
}

// RetryID is the ID of the form of a source's retry (SourceRow.Retry): its
// action's and the source's address, one per source.
func RetryID(r view.SourceRow) string {
	if r.Retry == nil {
		return ""
	}
	return r.Retry.ID + r.Href
}

// RetryForms are the forms of the retries of the rail's sources that post
// (RetryID), for the page's end.
func RetryForms(r view.Rail) []w.PostForm {
	var out []w.PostForm
	for _, sec := range r.Sections {
		for _, row := range sec.Rows {
			if row.Retry != nil && row.Retry.Method == "POST" && row.Retry.Enabled {
				out = append(out, ActionForm(*row.Retry, RetryID(row)))
			}
		}
	}
	return out
}

// RailForm is the rail's form (Rail.Ticks) as the PostForm its ticks submit,
// printed at the page's end under the screen's ID; nil when the rail sends
// no choice.
func RailForm(r view.Rail, id string) *w.PostForm {
	if r.Ticks == nil {
		return nil
	}
	f := PostForm(*r.Ticks, id)
	return &f
}

// Foot is the rail's foot: its actions as buttons of an icon (New address
// book, Connect account…, their words their accessible names), davloom's
// emblem while the mode runs (Rail.Davloom says how), and the last line —
// the name and the build, which leads to «about» (C-13).
func Foot(r view.Rail, words RailWords) w.RailFoot {
	var items []w.RailFootPart
	for _, a := range r.Foot {
		b := Button(a, "")
		b.Title, b.Label = b.Label, ""
		items = append(items, b)
	}
	if r.Davloom != "" {
		items = append(items, w.Emblem{Symbol: words.Davloom, Title: string(r.Davloom)})
	}
	return w.RailFoot{
		Items: items,
		About: &w.About{Name: string(r.About.Label), Build: w.Mono{Text: string(r.About.Build)}, Href: r.About.Href},
	}
}

// Running tells whether a poll's bar stands (C-6.4): while the poll runs, or
// when a source did not answer; a poll finished clean draws none.
func Running(p *view.PollBar) bool {
	if p == nil {
		return false
	}
	if p.Running {
		return true
	}
	for _, s := range p.Sources {
		if s.State != view.SourceDone && s.State != view.SourceEmpty {
			return true
		}
	}
	return false
}

// Runbar is a poll (PollBar) as its bar: how far, and what it says in its
// pieces (Summary: addresses monospaced, a source that did not answer as an
// alarm); nil when the bar does not stand (Running).
func Runbar(p *view.PollBar) *w.Runbar {
	if !Running(p) {
		return nil
	}
	var parts []w.RunbarPart
	for _, r := range p.Summary {
		switch r.Mark {
		case view.RunMono:
			parts = append(parts, w.Mono{Text: string(r.Text)})
		case view.RunDanger:
			parts = append(parts, w.Span{Face: w.FaceDanger, Text: string(r.Text)})
		default:
			parts = append(parts, w.Text(r.Text))
		}
	}
	return &w.Runbar{Meter: w.Meter{Percent: p.Percent}, Parts: parts}
}

// Badge is a badge of the model (C-6.2 Badge) in the tone of its role.
func Badge(b view.Badge) w.Badge {
	tone := w.BadgeNeutral
	switch b.Role {
	case view.BadgeLinked:
		tone = w.BadgeLinked
	case view.BadgeAlert:
		tone = w.BadgeAlert
	case view.BadgeLocal:
		tone = w.BadgeLocal
	}
	return w.Badge{Label: string(b.Label), Tone: tone}
}

// Avatar is a person's avatar (C-6.2 Avatar): the photo where there is one,
// else the initials.
func Avatar(a view.Avatar) w.Avatar {
	return w.Avatar{Initials: string(a.Initials), Src: a.Href}
}

// FieldsItem is a field of a record (C-6.2 Field) as a line of a card: its
// name and value, monospaced where the model says (Mono), going where it
// goes (Href: tel:, mailto:, the record in another book), the muted words
// after it (Aside, «in 7 months») led by lead, the screen's — none where
// the frame draws no aside (lead is empty); categories as
// tags, a space between them (ждёт библиотеку: U-5.5 #10 — a tag's address).
func FieldsItem(f view.Field, lead string) w.FieldsItem {
	it := w.FieldsItem{Term: string(f.Label), Href: f.Href}
	if f.Mono {
		it.Face = w.ValueMono
	}
	for i, t := range f.Tags {
		if i > 0 {
			it.Value = append(it.Value, w.Text(" "))
		}
		it.Value = append(it.Value, w.Tag{Label: string(t.Label), On: t.On})
	}
	if f.Value != "" {
		v := string(f.Value)
		if f.Aside != "" && lead != "" {
			v += " "
		}
		it.Value = append(it.Value, w.Text(v))
	}
	if f.Aside != "" && lead != "" {
		it.Value = append(it.Value, w.Dim{Parts: []w.Inline{w.Text(lead + string(f.Aside))}})
	}
	return it
}

// Book is a line of «the same person in other books» (C-6.2 Field of
// Books): the book's mark, its name leading to the record there, and how it
// differs, muted after lead, the screen's.
func Book(f view.Field, lead string) []w.SecItemPart {
	return []w.SecItemPart{
		w.Mark{Colour: string(f.Colour), Size: w.MarkH11Inline},
		w.Text(" "),
		w.Link{Text: string(f.Label), Href: f.Href},
		w.Text(" "),
		w.Dim{Parts: []w.Inline{w.Text(lead + string(f.Value))}},
	}
}

// SegOption is an action of a switch of links (ListBar.Segments) as its
// option: its words, its count apart (Tally), where it goes, the one shown
// now (On).
func SegOption(a view.Action) w.SegOption {
	return w.SegOption{Label: string(a.Label), Count: string(a.Tally), Href: a.Href, On: a.On}
}

// Submit is an action that sends the form it stands in, as its submit
// button: its words, its icon, its kind.
func Submit(a view.Action) w.Button {
	return w.Button{Label: string(a.Label), Icon: Icon(a.Icon), Kind: Kind(a), Submit: true}
}

// ActionForm is an action that posts with no form of its own (Delete
// contact, Send) as the PostForm its button submits from where it stands,
// under the screen's ID: it posts to the action's address.
func ActionForm(a view.Action, id string) w.PostForm {
	return w.PostForm{ID: id, Action: a.Href}
}

// DiagLine is a line of a technical block: its name and its value.
type DiagLine struct {
	Name, Value string
}

// Diag is a technical block (the source of a record, s-contacts-1): a line
// for each of lines, the values in one column after the names.
func Diag(lines ...DiagLine) w.Diag {
	width := 0
	for _, l := range lines {
		width = max(width, len([]rune(l.Name)))
	}
	var text string
	for i, l := range lines {
		if i > 0 {
			text += "\n"
		}
		text += l.Name
		for range width - len([]rune(l.Name)) + 2 {
			text += " "
		}
		text += l.Value
	}
	return w.Diag{Text: text}
}

// Notice is a notice of the shell (Shell.Notices, C-6.2) over the page: its
// message (Msg) and its buttons after the words, an action that posts
// submitting the form of its ID (NoticeForms).
func Notice(n view.Notice, space w.MsgSpace) w.Msg {
	m := Msg(n, space)
	for _, a := range n.Actions {
		m.Parts = append(m.Parts, w.Text(" "), Button(a, a.ID))
	}
	return m
}

// NoticeForms are the forms of the notices' actions that post, by their
// IDs, for the page's end.
func NoticeForms(ns []view.Notice) []w.PostForm {
	var out []w.PostForm
	for _, n := range ns {
		for _, a := range n.Actions {
			if a.Method == "POST" && a.Enabled {
				out = append(out, ActionForm(a, a.ID))
			}
		}
	}
	return out
}
