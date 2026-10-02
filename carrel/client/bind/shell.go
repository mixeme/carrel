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
// Search.Action under Search.Name — the screen's words its placeholder and
// its accessible name —, then New — the primary button with its arrow (Action.Arrow:
// the menu of what else can be made, a closed menu the frame draws beside
// the screen) —, the refresh, which says when the last poll of this section
// ended and polls again (none where the page has no poll of its own:
// Refresh.Action is empty), and the account's button. With the account's
// menu (User) the button opens it, without a script too (a closed menu):
// who is signed in and where, then the items — Sign out posts its form
// (MenuForms, TopForms); without a menu the button goes to the account's
// page (UserHref).
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
	user := w.UserMenu{Name: string(t.Signed.Login)}
	if len(t.User) == 0 {
		user.Href = t.UserHref
		actions = append(actions, user)
	} else {
		items := []w.MenuPart{w.MenuHeading{
			Rubric: w.Rubric{
				Text:    string(t.Signed.Login),
				Items:   []w.RubricPart{w.Mono{Text: string(t.Signed.Instance), Variant: w.MonoTightPlain}},
				Form:    w.RubricItem,
				Variant: w.RubricItemLine,
			},
			Form: w.MenuHeadingSplit,
		}}
		actions = append(actions, w.TopDrop{
			Trigger: user,
			Menu:    w.Menu{Items: append(items, MenuItems(t.User, "")...), Width: w.MenuWidth290},
			End:     true,
		})
	}
	return w.Top{
		Brand: words.Brand,
		Search: &w.Search{
			Query:  w.Query{Value: string(t.Search.Query), Placeholder: words.Search, Label: words.Search, Name: t.Search.Name},
			Action: t.Search.Action,
		},
		Actions: actions,
	}
}

// TopForms are the forms of the top bar's menus that post (Sign out), for
// the page's end (MenuForms).
func TopForms(t view.TopBar) []w.PostForm {
	return append(MenuForms(t.New.Arrow), MenuForms(t.User)...)
}

// MenuItems are the items of a menu (C-6.2 MenuItem), a rule before an item
// that has one (Separator): its icon, its words, where it goes — an item
// that posts (Sign out) submits the form of its ID (MenuForms), one that
// carries a file (Action.File: «From this computer») is the label of the
// file input of that form, which the browser opens without a script
// (Typecase's task 109; its «Upload» — Uploads); an item that sends the form
// it stands in (Action.Submits: Attach of a new record) sends it to its own
// address (MenuItem.To, Typecase's task 122) — with a file, the file picked
// sends it at once with the library's script (FileField.To), and «Upload»
// without one; the mark of
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
		switch {
		case !it.Enabled:
			m.Reason = string(it.Reason)
		case it.Method == "POST" && it.Submits && it.File != "":
			m.File = &w.FileField{Name: it.File, Label: string(it.Label), To: it.Href}
		case it.Method == "POST" && it.Submits:
			m.To = it.Href
		case it.Method == "POST" && it.File != "":
			m.File = &w.FileField{Form: it.ID, Name: it.File, Label: string(it.Label)}
		case it.Method == "POST":
			m.Form = it.ID
		default:
			m.Href = it.Href
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

// Uploads are the items «Upload» of a menu's items that carry a file
// (MenuItems), each shown only on a page without a script and sending the
// form of its item's ID with the file picked (Typecase's task 109); word is
// the screen's. The frames draw them after the menu's items (s-agenda-6,
// s-tasks-7). The «Upload» of an item that sends the form it stands in
// (Submits) sends that form to the item's address (MenuItem.To).
func Uploads(items []view.MenuItem, word string) []w.MenuPart {
	var out []w.MenuPart
	for _, it := range items {
		switch {
		case it.Method != "POST" || !it.Enabled || it.File == "":
		case it.Submits:
			out = append(out, w.MenuItem{Label: word, NoScript: true, To: it.Href})
		default:
			out = append(out, w.MenuItem{Label: word, NoScript: true, Form: it.ID})
		}
	}
	return out
}

// MenuForms are the forms of a menu's items that post (MenuItems), by their
// IDs, for the page's end; an item that sends the form it stands in
// (Submits) has none of its own.
func MenuForms(items []view.MenuItem) []w.PostForm {
	var out []w.PostForm
	for _, it := range items {
		if it.Method == "POST" && it.Enabled && !it.Submits {
			out = append(out, ActionForm(it.Action, it.ID))
		}
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
// and for read-only (ro), the rubric of the tags, the emblem's symbol of
// davloom in the sprite, and the badge of the rubric of feeds read from
// outside (external).
type RailWords struct {
	Form     string
	Toggle   string
	None     string
	ReadOnly string
	Tags     string
	Davloom  string
	External string
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
// has none, is all the sources at once —, a feed read from outside (the row,
// its mark the plain colour), its
// name, the open one (On), one not shown (ticked off; the combined view's
// row ticked off is nothing ticked, and stays as it is). Its count, the dash
// when it has none; a feed shows ro instead. A source that cannot be written
// to carries ro after its count. One that does not answer is in error: its
// mark, the reason in full the mark's hint (Detail) — no words by its name
// and no Retry, which are the poll bar's menu of refusals (Runbar; the
// owner's decision of 02.10.2026, states-16). With the rail's
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
		// The row is the feed's; its mark is only its colour — the frames
		// draw the row is-ext, never the mark (s-contacts-7, s-agenda-1).
		row.Kind = w.TickRowExternal
	case r.Colour == "":
		// Every source at once: ticked off, it is nothing ticked, not a
		// source hidden (states-14).
		row.Kind, row.Mark.Kind, row.Hidden = w.TickRowAll, w.MarkMulti, false
	}
	switch {
	case r.State == view.CollectionUnavailable:
		row.Error, row.MetaTitle = true, string(r.Detail)
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
// account as the rubric when it has one — the section of feeds read from
// outside (Feeds) with the screen's badge External after it — and its
// sources' rows, and after them the tags (Rail.Tags) under the screen's
// rubric, each the list filtered by it.
func Sources(r view.Rail, words RailWords) []w.RailPart {
	rf := words
	if r.Ticks == nil {
		rf.Form = ""
	}
	var out []w.RailPart
	for _, sec := range r.Sections {
		var items []w.RailSecPart
		if sec.Account != "" {
			rubric := w.Rubric{Text: string(sec.Account)}
			if sec.Feeds && words.External != "" {
				rubric.Items = []w.RubricPart{w.Badge{Label: words.External}}
			}
			items = append(items, rubric)
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

// Tags are the categories (C-6.2 Tag) as tags (Tag), the one chosen on.
func Tags(tags []view.Tag) []w.Tag {
	out := make([]w.Tag, 0, len(tags))
	for _, t := range tags {
		out = append(out, Tag(t))
	}
	return out
}

// Tag is a category (C-6.2 Tag) as a tag: its words, the list filtered by it
// where it goes (Href), the one chosen on.
func Tag(t view.Tag) w.Tag {
	return w.Tag{Label: string(t.Label), On: t.On, Href: t.Href}
}

// RetryID is the ID of the form of a retry of the poll bar's menu of
// refusals (SourceProgress.Retry, PollBar.RetryAll): its action's ID and
// address, one per source.
func RetryID(a *view.Action) string {
	if a == nil {
		return ""
	}
	return a.ID + a.Href
}

// runSep is what the pieces of a poll bar are joined by, as the server joins
// its Summary: «Polled 5 of 6 sources · 14 events · ‹1 failed›» (s-agenda-1).
const runSep = " · "

// refused are the sources of a poll that did not answer, as its menu of
// refusals lists them: each with what was asked and answered (Answer).
func refused(p *view.PollBar) []view.SourceProgress {
	var out []view.SourceProgress
	for _, s := range p.Sources {
		if s.Answer != "" {
			out = append(out, s)
		}
	}
	return out
}

// PollForms are the forms of the retries of the poll bar's menu of refusals
// that post (RetryID): each refused source's and Retry all, for the page's
// end.
func PollForms(p *view.PollBar) []w.PostForm {
	if p == nil || p.Failed == "" {
		return nil
	}
	var out []w.PostForm
	for _, s := range refused(p) {
		if s.Retry != nil && s.Retry.Method == "POST" && s.Retry.Enabled {
			out = append(out, ActionForm(*s.Retry, RetryID(s.Retry)))
		}
	}
	if a := p.RetryAll; a != nil && a.Method == "POST" && a.Enabled {
		out = append(out, ActionForm(*a, RetryID(a)))
	}
	return out
}

// Refusals is the poll bar's «N failed» (PollBar.Failed) as its button, red
// and small, with its menu of refusals (states-16): for each source that did
// not answer its mark, its name, what was asked and what the server
// answered in full, and its Retry; Retry all under them when there are more
// than one (RetryAll).
func Refusals(p *view.PollBar) w.Drop {
	var items []w.MenuPart
	for _, s := range refused(p) {
		r := w.MenuRefusal{Mark: w.Mark{Colour: string(s.Colour)}, Source: string(s.Stamp), Answer: string(s.Answer)}
		if s.Retry != nil {
			r.Retry = Button(*s.Retry, RetryID(s.Retry))
			r.Retry.Small = true
		}
		items = append(items, r)
	}
	if p.RetryAll != nil {
		all := Button(*p.RetryAll, RetryID(p.RetryAll))
		all.Small = true
		items = append(items, w.MenuFoot{Items: []w.Button{all}})
	}
	return w.Drop{
		Trigger: w.Button{Label: string(p.Failed), Menu: true, Kind: w.ButtonDanger, Small: true},
		Menu:    w.Menu{Items: items, Width: w.MenuWidth420, Pop: w.MenuUnderEnd},
	}
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
// pieces (Summary: addresses monospaced), the red «N failed» with its menu
// of refusals after the separator the summary's pieces are joined by
// (runSep; Refusals), then
// on its right what it says there (Aside: when the shown answer was
// gathered, how long the poll runs), monospaced and pushed to the end; nil
// when the bar does not stand (Running).
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
	if p.Failed != "" {
		parts = append(parts, w.Text(runSep), Refusals(p))
	}
	if p.Aside != "" {
		parts = append(parts, w.Mono{Text: string(p.Aside), Place: w.Pushed})
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
// tags (Tag), each going where it goes, a space between them. A value in
// its pieces (Runs: the time monospaced) is drawn in them — one piece,
// monospaced, is the line's monospaced face (a task's due date in its
// panel, s-tasks-1) —, and a badge after the value (Badge: «until
// 2026-12-28») stands after a space.
func FieldsItem(f view.Field, lead string) w.FieldsItem {
	it := w.FieldsItem{Term: string(f.Label), Href: f.Href}
	if len(f.Runs) == 1 && f.Runs[0].Mark == view.RunMono && f.Value == "" {
		f.Mono, f.Value, f.Runs = true, f.Runs[0].Text, nil
	}
	if f.Mono {
		it.Face = w.ValueMono
	}
	for i, t := range f.Tags {
		if i > 0 {
			it.Value = append(it.Value, w.Text(" "))
		}
		it.Value = append(it.Value, Tag(t))
	}
	if f.Value != "" {
		v := string(f.Value)
		if f.Aside != "" && lead != "" {
			v += " "
		}
		it.Value = append(it.Value, w.Text(v))
	}
	for _, r := range f.Runs {
		switch r.Mark {
		case view.RunMono:
			it.Value = append(it.Value, w.Mono{Text: string(r.Text)})
		case view.RunDanger:
			it.Value = append(it.Value, w.Span{Face: w.FaceDanger, Text: string(r.Text)})
		default: // a card's value has no bold piece: the widget takes none
			it.Value = append(it.Value, w.Text(r.Text))
		}
	}
	if f.Aside != "" && lead != "" {
		it.Value = append(it.Value, w.Dim{Parts: []w.Inline{w.Text(lead + string(f.Aside))}})
	}
	if f.Badge.Label != "" {
		if n := len(it.Value); n > 0 {
			if t, ok := it.Value[n-1].(w.Text); ok {
				it.Value[n-1] = t + " "
			} else {
				it.Value = append(it.Value, w.Text(" "))
			}
		}
		it.Value = append(it.Value, Badge(f.Badge))
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

// csrfField is the name of the session's token in a form that posts (C-11):
// the protocol's, not a word of the interface.
const csrfField = "_csrf"

// ActionForm is an action that posts with no form of its own (Delete
// contact, Send, Sign out) as the PostForm its button submits from where it
// stands, under the screen's ID: it posts to the action's address, with the
// session's token the action carries (Action.CSRF, C-11) — without a script
// nothing else sends it —, and multipart with the action's file field
// (Action.File: Attach, «From this computer») where it carries a file.
func ActionForm(a view.Action, id string) w.PostForm {
	f := w.PostForm{ID: id, Action: a.Href, File: a.File}
	if a.CSRF != "" {
		f.Fields = []w.Hidden{{Name: csrfField, Value: a.CSRF}}
	}
	return f
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
// message (Msg) and its buttons after the words, small — every frame draws a
// message's buttons so (states-11, s-agenda-11, s-mail-*) —, an action that
// posts submitting the form of its ID (NoticeForms).
func Notice(n view.Notice, space w.MsgSpace) w.Msg {
	m := Msg(n, space)
	for _, a := range n.Actions {
		b := Button(a, a.ID)
		b.Small = true
		m.Parts = append(m.Parts, w.Text(" "), b)
	}
	return m
}

// Notices are the notices of the shell on the page (the zone notices, C-7.2):
// each its message (Notice) and under it its technical block, where it has
// one (NoticeDiag).
func Notices(ns []view.Notice, space w.MsgSpace) []w.MainPart {
	var out []w.MainPart
	for _, n := range ns {
		out = append(out, Notice(n, space))
		if d := NoticeDiag(n); d != nil {
			out = append(out, *d)
		}
	}
	return out
}

// NoticeDiag is a notice's technical block (Notice.Details, C-9) under its
// message on the page: what was asked, then its lines, one a line; nil when
// the notice has none.
func NoticeDiag(n view.Notice) *w.MainDiag {
	if n.Details == nil {
		return nil
	}
	text := string(n.Details.Summary)
	for _, l := range n.Details.Lines {
		if text != "" {
			text += "\n"
		}
		text += string(l)
	}
	return &w.MainDiag{Diag: w.Diag{Text: text}}
}

// EmptyParts are what an empty state says (C-6.2 Notice of Empty) as its
// parts: the title bold, the body after it, then its buttons, a space between
// them; an action that posts submits the form of its ID.
func EmptyParts(n view.Notice) []w.EmptyPart {
	var parts []w.EmptyPart
	if n.Title != "" {
		parts = append(parts, w.Strong(n.Title))
	}
	if n.Body != "" {
		body := string(n.Body)
		if len(parts) > 0 {
			body = " " + body
		}
		if len(n.Actions) > 0 {
			body += " "
		}
		parts = append(parts, w.Text(body))
	}
	for i, a := range n.Actions {
		if i > 0 {
			parts = append(parts, w.Text(" "))
		}
		parts = append(parts, Button(a, a.ID))
	}
	return parts
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
