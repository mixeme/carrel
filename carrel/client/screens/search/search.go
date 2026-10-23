// Package search is the result of a search across every ticked collection,
// /app/search, drawn by frames «Поиск по всем источникам» (s-find-1), «Поиск
// идёт» (s-find-4) and the column of the result in its states (s-find-5 …
// -10) of the mockup: the draft frame2go printed for s-find-1, with the
// frame's data become the view model's (U-5.1, U-6.1; Typecase's note 114).
package search

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The IDs of the page's forms: the rail's (its ticks submit it) and Stop of
// a search that runs (s-find-4).
const (
	formRail = "rail"
	formStop = "search-stop"
)

// The screen's words (U-1.3): the shell's; the page without a query — its
// title, its field and button, the rubric of the history (s-find-6); the
// rubric of each kind; the sign of a task; the dash of what is not there.
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
	}
	// The sources asked: a number for what each found, none while it is
	// still asked (s-find-4), no «ro» — the search writes nowhere (s-find-1).
	rail = bind.RailWords{Form: formRail, Toggle: "toggle", Tags: "Tags", Davloom: "davloom"}
)

const (
	searchTitle = "Search"
	lookFor     = "What to look for"
	searchLabel = "Search"
	recent      = "Recent searches"
	notDone     = "Not done"
	done        = "Done"
	none        = "—"
	sep         = " · "
)

// kinds are the groups of the result in its order (S-14.1), each with its
// rubric.
var kinds = []struct {
	kind   view.Kind
	rubric string
}{
	{view.KindContact, "Contacts"},
	{view.KindEvent, "Events"},
	{view.KindTask, "Tasks"},
	{view.KindNote, "Notes"},
	{view.KindFile, "Files"},
}

// Screen draws the search from its view model.
//
//   - The shell: the top bar — the query in its field, the door to the
//     search again (Refresh: when the pass ended, off while it runs); the
//     rail of sections and «Sources» — the collections asked, each with
//     what it found, one that did not answer in error —, the foot.
//   - Without a query (s-find-6): «Search», what it would search, the field
//     «What to look for» with Search, the recent searches as links.
//   - The head: the query in quotes, what was found; the kinds as segments.
//   - The bar of the pass (s-find-1, -4, -5, -9, -10): what it says; while
//     it runs Stop (its form under it); a pass that ended — how long it
//     took, and the red «N failed» with its menu of refusals.
//   - The kinds in their order, each with its count, a row for each record
//     in the look of its kind (note 114): a contact's face, an event's day
//     and the time beside the place, a task's sign and due, a note's
//     excerpt, a file's size, time and access — the match in bold.
//   - Nothing to show (s-find-5, -7, -9): the words of why; the notice of
//     too many searches (s-find-8); the line about files.
func Screen(p *view.SearchPage) w.Shell {
	t := bind.Top(p.Top, top)
	var main []w.MainPart
	main = append(main, bind.Notices(p.Notices, w.MsgFlushSides)...)
	if p.Query == "" {
		main = append(main, noQuery(p)...)
		return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}}
	}
	main = append(main, head(p))
	if r, ok := Bar(p); ok {
		main = append(main, r)
		if stop, ok := stopAction(&p.Poll); ok {
			main = append(main, bind.ActionForm(stop, formStop))
		}
	}
	for _, k := range kinds {
		var rows []w.ListRow
		for _, r := range p.Results {
			if r.Kind == k.kind {
				rows = append(rows, row(r))
			}
		}
		if len(rows) == 0 {
			continue
		}
		main = append(main,
			w.ListGroup{Title: w.Rubric{Text: k.rubric, Form: w.RubricInline}, Count: &w.Num{Value: len(rows)}},
			w.List{Rows: rows})
	}
	if p.Empty != "" {
		main = append(main, w.Empty{Parts: []w.EmptyPart{w.Text(p.Empty)}})
	}
	if len(p.Note) > 0 {
		main = append(main, w.Msg{Parts: msgParts(p.Note)})
	}
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}}
}

// Forms are the page's forms: the rail's, the retries of its sources, the
// items of the menus that post (Sign out).
func Forms(p *view.SearchPage) []w.PostForm {
	out := bind.TopForms(p.Top)
	if f := bind.RailForm(p.Rail, formRail); f != nil {
		out = append(out, *f)
	}
	out = append(out, bind.PollForms(&p.Poll)...)
	return append(out, bind.NoticeForms(p.Notices)...)
}

// head is the query in quotes and what was found; the kinds as segments.
func head(p *view.SearchPage) w.Head {
	h := w.Head{Title: &w.HeadTitle{Text: "«" + string(p.Query) + "»"}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	if len(p.Results) > 0 && len(p.Bar.Segments) > 0 {
		seg := w.Seg{}
		for _, a := range p.Bar.Segments {
			seg.Options = append(seg.Options, bind.SegOption(a))
		}
		h.Acts = &w.Acts{Items: []w.ActsPart{seg}}
	}
	return h
}

// noQuery is the page without a query (s-find-6): «Search» and what it would
// search; the field and Search; the recent searches, each a link to its
// result with how much it found and when.
func noQuery(p *view.SearchPage) []w.MainPart {
	h := w.Head{Title: &w.HeadTitle{Text: searchTitle}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	name := p.Top.Search.Name
	form := w.Form{Action: p.Top.Search.Action, Method: "get", Items: []w.FormPart{
		w.Field{Label: lookFor, Width: w.FieldLG, Parts: []w.FieldPart{w.Input{Kind: w.InputSearch, Name: name, Label: lookFor, Focus: true}}},
		w.Field{Label: " ", Parts: []w.FieldPart{w.Button{Label: searchLabel, Kind: w.ButtonPrimary, Submit: true}}},
	}}
	out := []w.MainPart{h, form}
	if hs := p.Top.Search.History; len(hs) > 0 {
		items := []w.MenuPart{w.MenuHeading{Rubric: w.Rubric{Text: recent, Form: w.RubricItem}}}
		for _, r := range hs {
			trail := string(r.Outcome)
			if r.When != "" {
				trail += sep + string(r.When)
			}
			items = append(items, w.MenuItem{Icon: "clock", Label: string(r.Query), Href: r.Href,
				Trail: w.Mono{Text: trail, Variant: w.Mono11DustyTight, Place: w.Pushed}})
		}
		out = append(out, w.Pad{Items: []w.PadPart{w.Menu{Items: items, Width: w.MenuWidthFull}}})
	}
	return out
}

// Bar is the bar of the pass on this page (runbar), and whether it stands:
// the page's and the zone poll's.
func Bar(p *view.SearchPage) (w.Runbar, bool) { return runbar(&p.Poll) }

// runbar is the bar of the pass, one rule in every state (Typecase's note
// 118): what it says in two lengths — whole, and on a narrow bar «13 of
// 14», «9 of 14 · 5 records» (Short) —; after them the red «N failed» with
// its menu when a source refused; how long the pass took, pushed; while it
// runs, Stop pushed instead. No bar before a pass (no query, nothing
// ticked, too many).
func runbar(p *view.PollBar) (w.Runbar, bool) {
	if p.TaskID == "" {
		return w.Runbar{}, false
	}
	said := ""
	for _, r := range p.Summary {
		said += string(r.Text)
	}
	bar := w.Runbar{Meter: w.Meter{Percent: p.Percent}, Parts: []w.RunbarPart{w.Say{Full: said, Short: string(p.Short)}}}
	if p.Failed != "" && !p.Running {
		bar.Meter.Alert = true
		bar.Parts = append(bar.Parts, w.Text(sep), bind.Refusals(p))
	}
	switch {
	case p.Running:
		if stop, ok := stopAction(p); ok {
			bar.Parts = append(bar.Parts, w.Push{Items: []w.PushPart{w.Button{Label: string(stop.Label), Small: true, Form: formStop}}})
		}
	case p.Aside != "":
		bar.Parts = append(bar.Parts, w.Mono{Text: string(p.Aside), Place: w.Pushed})
	}
	return bar, true
}

// stopAction is Stop of a pass that runs.
func stopAction(p *view.PollBar) (view.Action, bool) {
	if !p.Running {
		return view.Action{}, false
	}
	for _, a := range p.Actions {
		if a.ID == "poll.stop" && a.Enabled && a.Method == "POST" {
			return a, true
		}
	}
	return view.Action{}, false
}

// row is a record of the result in the look of its kind (note 114).
func row(r view.FindRow) w.ListRow {
	title := []w.Inline{w.Text(r.Title)}
	if len(r.Marked) > 0 {
		title = bind.Inlines(r.Marked)
	}
	mark := w.Mark{Colour: string(r.Colour)}
	switch r.Kind {
	case view.KindContact:
		f := w.FaceRow{Mark: mark, Title: title, Subtitle: or(r.Subtitle), Detail: or(r.Detail), Caption: or(r.Caption), Source: string(r.Stamp), Href: r.Href}
		if r.Avatar != nil {
			f.Avatar = bind.Avatar(*r.Avatar)
		}
		return f
	case view.KindEvent:
		return w.TimeRow{Mark: mark, Time: string(r.When), Title: title, Detail: string(r.Detail), Source: string(r.Stamp), Href: r.Href}
	case view.KindTask:
		// A sign, not a button: a task is ticked in its list or on its card.
		tick := w.Tick{Sign: true, Done: r.Done, Label: notDone}
		if r.Done {
			tick.Label = done
		}
		return w.DueRow{Tick: tick, Mark: mark, Title: title, Due: string(r.When), Progress: string(r.Progress), Source: string(r.Stamp), Href: r.Href}
	case view.KindNote:
		return w.ExcerptRow{Mark: mark, Title: title, Excerpt: string(r.Detail), Date: string(r.When), Source: string(r.Stamp), Href: r.Href}
	}
	// A file: its collection's mark where a folder has its tick (note 114
	// p. 3) — the result has no bar of actions over what is ticked.
	return w.SizeRow{Mark: &mark, Icon: bind.Icon(r.Icon), Name: title, Bytes: string(r.Size), Time: string(r.Changed), Access: string(r.Access), Source: string(r.Stamp), Href: r.Href}
}

// railParts is the rail of sections, the sources asked and the foot.
func railParts(r view.Rail) []w.RailPart {
	nav := w.Nav{}
	for _, n := range r.Nav {
		nav.Items = append(nav.Items, bind.NavItem(n))
		if n.On {
			nav.On = len(nav.Items)
		}
	}
	parts := append([]w.RailPart{nav}, bind.Sources(r, rail)...)
	return append(parts, bind.Foot(r, rail))
}

// msgParts are the pieces of a line in their faces.
func msgParts(rs view.Runs) []w.MsgPart {
	var out []w.MsgPart
	for _, in := range bind.Inlines(rs) {
		if p, ok := in.(w.MsgPart); ok {
			out = append(out, p)
		}
	}
	return out
}

// or is a value, or the dash where the model has none.
func or(t view.Text) string {
	if t == "" {
		return none
	}
	return string(t)
}
