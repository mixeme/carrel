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
	// The keys of the shell (U-7; Typecase's note 59, task 87): the one
	// that presses New, «n», and the one that puts the focus into the
	// search, «/». The library has none of its own.
	NewKey, SearchKey string
	// Theme is the switch of the theme in the account's menu, before Sign
	// out (account-5): a setting of this browser, not of the server (U-12).
	// Its label empty — no switch.
	Theme ThemeWords
	// Keys is the item of the keys' sheet in the account's menu, after the
	// theme (account-5): a link to the sheet the document puts on the page
	// (KeysWords.Sheet), pressed by its key and shown only with a script — without
	// one the keys do not work either (U-7; Typecase's note 134). Its label
	// empty — no item.
	Keys KeysWords
}

// ThemeWords are the words of the theme's switch: its label and its three
// options, auto the system's.
type ThemeWords struct {
	Label, Auto, Light, Dark string
}

// KeysWords are the words of the keys' item: its label, its icon, the key
// that opens the sheet, drawn at its end, and the ID of the sheet — the
// page's own (Shell.Dialogs), closed until the item or its key opens it.
type KeysWords struct {
	Label string
	Icon  w.Icon
	Key   string
	Sheet string
}

// Top is the top bar (TopBar): the mark, the search that sends its query to
// Search.Action under Search.Name — the screen's words its placeholder and
// its accessible name —, then New — the primary button with its arrow (Action.Arrow:
// the menu of what else can be made, a closed menu the frame draws beside
// the screen) —, the refresh, which says when the last poll of this section
// ended and polls again — its own words where there is no answer to say the
// time of: switched off, with nothing ticked to ask or while the poll still
// runs, it says why (Р-39 of Typecase, states-7, -8) —, none where the page
// has no poll of its own (Refresh.Action is empty), and the account's button. With the account's
// menu (User) the button opens it, without a script too (a closed menu):
// who is signed in and where, then the items — Sign out posts its form
// (MenuForms, TopForms); without a menu the button goes to the account's
// page (UserHref).
func Top(t view.TopBar, words TopWords) w.Top {
	newBtn := Button(t.New, "")
	newBtn.Menu = len(t.New.Arrow) > 0
	newBtn.Key = words.NewKey
	var actions []w.TopAction
	switch {
	case t.New.Label == "":
		// None where nothing is to be made yet: the change of a temporary
		// password (account-17).
	case newBtn.Menu:
		actions = append(actions, w.TopDrop{Trigger: newBtn, Menu: w.Menu{Items: MenuItems(t.New.Arrow, ""), Pop: w.MenuUnder, Target: menuTarget(t.New.Arrow)}})
	default:
		actions = append(actions, newBtn)
	}
	if t.Refresh.Action.ID != "" {
		refresh := Button(t.Refresh.Action, "")
		if t.Refresh.Answered != "" {
			refresh.Label = string(t.Refresh.Answered)
		}
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
			Menu:    w.Menu{Items: append(items, withTheme(MenuItems(t.User, ""), words.Theme, words.Keys)...), Width: w.MenuWidth290},
			End:     true,
		})
	}
	top := w.Top{Brand: words.Brand, Actions: actions}
	if t.Search.Action != "" {
		// A query standing in the field (the search's result, s-find-1)
		// has the cross that empties it.
		top.Search = &w.Search{
			Query:  w.Query{Value: string(t.Search.Query), Placeholder: words.Search, Label: words.Search, Name: t.Search.Name, Key: words.SearchKey},
			Action: t.Search.Action,
			Clear:  t.Search.Query != "",
		}
	}
	return top
}

// withTheme are the items of the account's menu with the theme's switch and
// the keys' item before the last rule — the one over Sign out —, a rule of
// their own before them (account-5): Settings, Administration, the theme,
// Keyboard shortcuts, Sign out. The library's script keeps the theme chosen
// in this browser (Seg.Theme).
func withTheme(items []w.MenuPart, theme ThemeWords, keys KeysWords) []w.MenuPart {
	var own []w.MenuPart
	if theme.Label != "" {
		own = append(own, w.MenuColumn{Label: theme.Label, Control: w.Seg{Theme: true, Variant: w.SegStartSmall,
			Options: []w.SegOption{{Label: theme.Auto, On: true}, {Label: theme.Light}, {Label: theme.Dark}}}})
	}
	if keys.Label != "" {
		own = append(own, w.MenuItem{Icon: keys.Icon, Trail: w.Kbd{Keys: keys.Key}, Label: keys.Label,
			ScriptOnly: true, Href: "#" + keys.Sheet, Key: keys.Key})
	}
	if len(own) == 0 {
		return items
	}
	at := len(items)
	for i, it := range items {
		if _, ok := it.(w.MenuSep); ok {
			at = i
		}
	}
	out := append(append(append([]w.MenuPart(nil), items[:at]...), w.MenuSep{}), own...)
	return append(out, items[at:]...)
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
			// Unlink of a linked record sends its group with the form of
			// the decision (s-contacts-36).
			m.Form, m.Name, m.Value = it.ID, it.Name, it.Value
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

// UploadForm is the ID of the form a file goes with from a menu's file item —
// Attach «From this computer» — that a panel names as its drop (Side.Drop,
// Typecase's note 68): a file dropped on the open card is sent with it. Empty
// — no such item, or it sends the form it stands in, and nothing is dropped.
func UploadForm(items []view.MenuItem) string {
	for _, it := range items {
		if it.Method == "POST" && it.Enabled && it.File != "" && !it.Submits {
			return it.ID
		}
	}
	return ""
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

// EditFiles are an editor's file pickers: they wait in the form for Save.
// The row and list belong to the screen; From Files keeps its own action.
func EditFiles(items []view.MenuItem, list, size string, row w.PickedRow) []w.MenuPart {
	out := MenuItems(items, "")
	for i, part := range out {
		if m, ok := part.(w.MenuItem); ok && m.File != nil {
			m.File.Form, m.File.To = "", ""
			m.File.List, m.File.Size, m.File.Many = list, size, true
			m.Row = row
			out[i] = m
		}
	}
	return out
}

// EditFileForms leaves file selection in the editor's form. Other menu
// actions still have their independent forms where the contract says so.
func EditFileForms(items []view.MenuItem) []w.PostForm {
	var out []w.PostForm
	for _, it := range items {
		if it.File == "" {
			out = append(out, MenuForms([]view.MenuItem{it})...)
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
// outside (external). The files' rail (s-files-1, -6) has no combined view
// but a root, its row with an icon (Root), and a storage says its access
// where a source says its count — rw where it can be written (Writable),
// nothing but ro where not.
type RailWords struct {
	Form     string
	Toggle   string
	None     string
	ReadOnly string
	Tags     string
	Davloom  string
	External string
	Root     w.Icon
	Writable string
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
// when it has none — a feed's too (Р-34 of Typecase, 80d1319: the count and
// «ro» stand apart). A source that cannot be written to, a feed among them,
// carries ro after its count. One that does not answer is in error, with
// neither a count nor ro (states-9): its
// mark, the reason in full the mark's hint (Detail) — no words by its name
// and no Retry, which are the poll bar's menu of refusals (Runbar; the
// owner's decision of 02.10.2026, states-16). With the rail's
// form (Rail.Ticks), the tick sends the row's address under Toggle; without
// it the choice is made, and the tick shows it (the rail of return, C-6.3).
// On the files' rail (Root) the row of every source is the root of the
// section, its icon instead of a mark, its muted words the server's (Meta,
// «2 of 3»); a storage says rw where it can be written (Writable) and only
// ro where not.
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
	case r.Colour == "" && words.Root != "":
		// The root of the files: every storage ticked, a folder of them.
		row.Kind, row.Icon, row.Mark, row.Hidden = w.TickRowRoot, words.Root, w.Mark{}, false
	case r.Colour == "":
		// Every source at once: ticked off, it is nothing ticked, not a
		// source hidden (states-14).
		row.Kind, row.Mark.Kind, row.Hidden = w.TickRowAll, w.MarkMulti, false
	}
	switch {
	case r.State == view.CollectionUnavailable:
		row.Error, row.MetaTitle = true, string(r.Detail)
	case r.Meta != "":
		row.Meta = string(r.Meta)
	case r.Count != nil:
		row.Meta = strconv.Itoa(*r.Count)
	case words.Writable != "" && r.ReadOnly:
	case words.Writable != "":
		row.Meta = words.Writable
	default:
		row.Meta = words.None
	}
	if r.ReadOnly && words.ReadOnly != "" && !row.Error {
		row.Trail = w.Badge{Label: words.ReadOnly}
	}
	if words.Form != "" {
		row.Send = w.Send{Form: words.Form, Name: words.Toggle, Value: r.Href}
	}
	return row
}

// Single tells whether the list open is one collection's: the rail's row
// that is on is a source, not every source at once (which has no colour).
// The rows of such a list have no source and no column for it (Р-48 of
// Typecase, 0bf5f71: s-agenda-17, s-tasks-15, s-notes-31).
func Single(r view.Rail) bool {
	for _, sec := range r.Sections {
		for _, row := range sec.Rows {
			if row.On {
				return row.Colour != ""
			}
		}
	}
	return false
}

// Sourceless tells whether the rows of a list draw no source and no column
// for it: one collection's list (Single), or a browser that chose not to see
// the source column (Look.Source, Appearance — appearance-1).
func Sourceless(r view.Rail, look view.Look) bool {
	return Single(r) || !look.Source
}

// LinkedBadge is the badge of a row's title in the list of one collection:
// the sources of a linked record are its hint — «nextcloud · Дела,
// nextcloud · Работа» under «linked ×2» (Badge.Title; s-tasks-15,
// s-notes-31) —, for the row has no column to say them in.
func LinkedBadge(b view.Badge, sources []view.Stamp) w.Badge {
	out := Badge(b)
	for i, s := range sources {
		if i > 0 {
			out.Title += ", "
		}
		out.Title += string(s)
	}
	return out
}

// Sources are the rail's sections of sources (Rail.Sections), each its
// account as the rubric when it has one — the section of feeds read from
// outside (Feeds) with the screen's badge External after it, its count
// apart (Tally) — and its
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
			// Its count apart, monospaced: «Sources 14 / 14» (s-find-1).
			if sec.Tally != "" {
				rubric.Items = append(rubric.Items, w.Mono{Text: string(sec.Tally), Variant: w.MonoTight})
			}
			items = append(items, rubric)
		}
		for _, row := range sec.Rows {
			items = append(items, TickRow(row, rf))
		}
		// A section with nothing in it says why (RailSection.Note): «Nothing
		// connected yet» under «Sources» (states-1).
		out = append(out, w.RailSec{Items: items, Note: string(sec.Note)})
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

// runSep is what the pieces of a line are joined by, as the server joins
// them: «Polled · 5 of 6 answered · 14 events» (s-agenda-1).
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
// printed at the page's end under the screen's ID, asking the zone its target
// names — the rail, whose answer puts the list in place too (Typecase's note
// 186); nil when the rail sends no choice.
func RailForm(r view.Rail, id string) *w.PostForm {
	if r.Ticks == nil {
		return nil
	}
	f := PostForm(*r.Ticks, id)
	f.Target = r.Ticks.Target
	return &f
}

// menuTarget is the zone a menu's links ask their address for (Menu.Target):
// the one every item names (Action.Target) — New's arrow is all sheets, each
// opening over the page —; none when they differ or none names one, and the
// items load their pages.
func menuTarget(items []view.MenuItem) string {
	target := ""
	for i, it := range items {
		if i == 0 {
			target = it.Target
		}
		if it.Target != target {
			return ""
		}
	}
	return target
}

// Foot is the rail's foot: its actions as buttons of an icon (New address
// book, Connect account…, their words their accessible names) — one that
// posts submits its form (FootForms: New folder of the files) —, davloom's
// emblem while the mode runs (Rail.Davloom says how), and the last line —
// the name and the build, which leads to «about» (C-13).
func Foot(r view.Rail, words RailWords) w.RailFoot {
	var items []w.RailFootPart
	for _, a := range r.Foot {
		b := Button(a, a.ID)
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

// Runbar is a poll (PollBar) as its bar (Typecase's note 208, the owner's
// decisions of 17 and 18.11.2026): while it runs, the meter; once it is
// over, a dot of how it ended (RunResult). Then what it says in two
// lengths (RunSay), the red «N failed» with its menu of refusals
// (Refusals), and on its right what it says there (Aside: how long ago the
// shown answer was gathered, how long the poll runs), monospaced and
// pushed to the end; nil when the bar does not stand (Running).
func Runbar(p *view.PollBar) *w.Runbar {
	if !Running(p) {
		return nil
	}
	bar := &w.Runbar{Result: RunResult(p), Parts: []w.RunbarPart{RunSay(p)}}
	if bar.Result == w.RunGoing {
		bar.Meter = w.Meter{Percent: p.Percent}
	}
	if p.Failed != "" {
		bar.Parts = append(bar.Parts, Refusals(p))
	}
	if p.Aside != "" {
		bar.Parts = append(bar.Parts, w.Mono{Text: string(p.Aside), Place: w.Pushed})
	}
	return bar
}

// RunSay is what a run line says (Р-84 of Typecase): whole, «Polled · 5 of
// 6 answered · 7 events», and on a narrow bar «5 of 6 · 7 events» (Short;
// the whole where the server gives none) — each ended by the separator
// when the red «N failed» follows (s-agenda-1).
func RunSay(p *view.PollBar) w.Say {
	var full string
	for _, r := range p.Summary {
		full += string(r.Text)
	}
	short := string(p.Short)
	if short == "" {
		short = full
	}
	if p.Failed != "" {
		full, short = full+runEnd, short+runEnd
	}
	return w.Say{Full: full, Short: short}
}

// runEnd ends the words of a run line before its «N failed».
const runEnd = " ·"

// RunResult is how a run ended, the dot its line begins with: none while
// it runs (the meter); red when a source refused, was not in time or does
// not answer at all; grey when it was stopped before every source
// answered (s-find-10); green when every one answered (s-find-5).
func RunResult(p *view.PollBar) w.RunResult {
	if p.Running {
		return w.RunGoing
	}
	stopped := false
	for _, s := range p.Sources {
		switch s.State {
		case view.SourceError, view.SourceTimeout, view.SourceUnavailable:
			return w.RunFailed
		case view.SourceCancelled:
			stopped = true
		}
	}
	if stopped {
		return w.RunStopped
	}
	return w.RunAnswered
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
	switch {
	case f.Badge.Label != "" && f.Href != "":
		// Outside the link: «+7 495 120-45-00» goes to tel:, «linked» does
		// not (s-contacts-6).
		it.After = append(it.After, Badge(f.Badge))
	case f.Badge.Label != "":
		if n := len(it.Value); n > 0 {
			if t, ok := it.Value[n-1].(w.Text); ok {
				it.Value[n-1] = t + " "
			} else {
				it.Value = append(it.Value, w.Text(" "))
			}
		}
		it.Value = append(it.Value, Badge(f.Badge))
	}
	// The collection a value of a linked record came from, after it: its
	// mark and its name, muted, the last of the value as the drafts set it
	// (s-contacts-1, -33, s-notes-1; Typecase's notes 117 p. 5, 123).
	if f.From != nil {
		if n := len(it.Value); n > 0 {
			if t, ok := it.Value[n-1].(w.Text); ok {
				it.Value[n-1] = t + " "
			} else {
				it.Value = append(it.Value, w.Text(" "))
			}
		}
		it.Value = append(it.Value, *From(f.From))
	}
	return it
}

// KeptSec is «Where this … is kept» of a linked record's panel: each copy
// with its mark, a link to it and what it holds otherwise, their number in
// the rubric (s-contacts-1, s-tasks-1, s-notes-1; note 123 p. 10).
func KeptSec(rubric string, books []view.Field, lead string) w.Sec {
	var lines [][]w.SecItemPart
	for _, f := range books {
		lines = append(lines, Book(f, lead))
	}
	return w.Sec{Items: []w.SecPart{
		w.Rubric{Text: rubric, Items: []w.RubricPart{w.Badge{Label: strconv.Itoa(len(books)), Tone: w.BadgeLinked}}},
		w.SecList{Items: lines, Gap: w.StackGapTight},
	}}
}

// WithFrom is a line of a linked record's attachments or Related, the
// collection of the one copy that holds it after it (Р-78; s-agenda-25,
// s-tasks-17).
func WithFrom(line []w.SecItemPart, f *view.FieldFrom) []w.SecItemPart {
	if d := From(f); d != nil {
		return append(line, w.Text(" "), *d)
	}
	return line
}

// From is the collection a value, an attachment or a Related row of a
// linked record came from: its mark and its name, muted (s-contacts-33,
// s-notes-32; Р-78); nil when there was no choice.
func From(f *view.FieldFrom) *w.Dim {
	if f == nil {
		return nil
	}
	return &w.Dim{Mark: &w.Mark{Colour: string(f.Colour), Size: w.MarkH11Inline}, Parts: []w.Inline{w.Text(f.Name)}}
}

// Book is a line of «the same person in other books» (C-6.2 Field of
// Books): the book's mark, its name leading to the record there, and how it
// differs, muted after lead, the screen's; a copy of a linked record that
// holds what is shown says nothing (s-agenda-25).
func Book(f view.Field, lead string) []w.SecItemPart {
	out := []w.SecItemPart{
		w.Mark{Colour: string(f.Colour), Size: w.MarkH11Inline},
		w.Text(" "),
		w.Link{Text: string(f.Label), Href: f.Href},
	}
	if f.Value == "" {
		return out
	}
	return append(out, w.Text(" "), w.Dim{Parts: []w.Inline{w.Text(lead + string(f.Value))}})
}

// SegOption is an action of a switch of links (ListBar.Segments) as its
// option: its words, its icon (the view of a folder, Typecase's task 115),
// its count apart (Tally), where it goes, the one shown now (On).
func SegOption(a view.Action) w.SegOption {
	return w.SegOption{Label: string(a.Label), Icon: Icon(a.Icon), Count: string(a.Tally), Href: a.Href, On: a.On}
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
	f := w.PostForm{ID: id, Action: a.Href, File: a.File, Target: Live(a.Target)}
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
// their band (Band), where there are any — the frames draw no empty one;
// the document puts it where none stands (Typecase's note 127).
func Notices(ns []view.Notice, space w.MsgSpace) []w.MainPart {
	if len(ns) == 0 {
		return nil
	}
	return []w.MainPart{Band(ns, space)}
}

// Band is the band of the notices: each its message (Notice) and under it
// its technical block, where it has one (NoticeDiag) — one band
// (states-11).
func Band(ns []view.Notice, space w.MsgSpace) w.Notices {
	var out w.Notices
	for _, n := range ns {
		out.Items = append(out.Items, Notice(n, space))
		if d := NoticeDiag(n); d != nil {
			out.Items = append(out.Items, *d)
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

// Alone is a sheet that came without the screen under it (Under nil): an
// overlay over nothing (C-7.2).
func Alone(sheet w.Sheet) w.Overlay {
	return w.Overlay{Scrim: w.Scrim{Content: sheet, Pad: w.ScrimPad26}}
}

// Over is the screen under a sheet with the sheet over it (C-6 Under, the
// owner's decision of 15.10.2026; Typecase's Overlay, s-contacts-2): the same
// top, rail and main under the scrim, the sheet on it. Nothing under a sheet
// is pressed, so the rail's form and the panel stay out; the body is as tall
// as a sheet needs (s-contacts-2).
func Over(under w.Shell, sheet w.Sheet) w.Shell {
	body, _ := under.Body.(w.ShellBody)
	var rail []w.OverlayRailPart
	for _, p := range body.Rail {
		if o, ok := p.(w.OverlayRailPart); ok {
			rail = append(rail, o)
		}
	}
	return w.Shell{Top: under.Top, Body: w.Overlay{
		Body:  w.OverlayBody{Rail: rail, Main: body.Main, Height: w.ShellHeight560},
		Scrim: w.Scrim{Content: sheet, Pad: w.ScrimPad26},
	}}
}
