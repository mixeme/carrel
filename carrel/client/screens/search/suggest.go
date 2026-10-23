package search

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the line's suggestions (U-1.3; s-find-2, -3).
const (
	recentRubric = "Recent searches"
	jumpRubric   = "Jump to"
	typeToSearch = "Type to search everything"
	showAllFor   = "Show all results for «"
	showAllEnd   = "»"
	timeline     = "Timeline · "
	asksEvery    = "asks every server"
	enterKey     = "Enter"
)

// kindIcons are the icons of a jump's kind, as the rail's sections have them.
var kindIcons = map[view.Kind]w.Icon{
	view.KindContact: "con", view.KindEvent: "cal", view.KindTask: "task", view.KindNote: "note", view.KindFile: "file",
}

// Suggestions are the search line's suggestions (Typecase's note 118; s-find-2,
// -3), the menu the live layer puts under the line, each item saying when it
// shows (MenuWhen), which the library's core filters by as more is typed.
//
//   - Nothing typed (s-find-2): the recent searches, each a link to its
//     result with how much it found and when; under the rule «Type to search
//     everything», Enter.
//   - Something typed (s-find-3): «Jump to» — what the session holds that has
//     it, each with its collection's mark, what was typed bold, the day of an
//     event and where it lives —; under the rule «Show all results for
//     «…»», Enter, chosen, and the timeline of every person among them,
//     which asks every server; under another the recent searches that have
//     it.
func Suggestions(p *view.SearchSuggest) w.Menu {
	var items []w.MenuPart
	if p.Query == "" {
		items = append(items, w.MenuHeading{Rubric: w.Rubric{Text: recentRubric, Form: w.RubricItem}})
		for _, h := range p.History {
			items = append(items, recentItem(h))
		}
		items = append(items, w.MenuSep{}, w.MenuItem{Label: typeToSearch, Trail: w.Kbd{Keys: enterKey}, Tone: w.MenuToneDusty, Href: p.All, When: w.WhenEmpty})
		return w.Menu{Items: items, Width: w.MenuWidthFull}
	}
	if len(p.Jump) > 0 {
		items = append(items, w.MenuHeading{Rubric: w.Rubric{Text: jumpRubric, Form: w.RubricItem}})
		for _, j := range p.Jump {
			where := string(j.Stamp)
			if j.Day != "" {
				where = string(j.Day) + sep + where
			}
			items = append(items, w.MenuItem{Icon: kindIcons[j.Kind], Mark: &w.Mark{Colour: string(j.Colour)}, Found: found(j.Marked, j.Title),
				Trail: w.Dim{Parts: []w.Inline{w.Text(where)}, Size: w.Dim11, Variant: w.DimTight, Place: w.Pushed}, Href: j.Href, When: w.WhenFound})
		}
		items = append(items, w.MenuSep{})
	}
	items = append(items, w.MenuItem{Icon: "search", Label: showAllFor + string(p.Query) + showAllEnd, Trail: w.Kbd{Keys: enterKey}, On: true,
		Href: p.All, When: w.WhenTyped})
	for _, h := range p.Whole {
		items = append(items, w.MenuItem{Icon: "all", Found: append([]w.Inline{w.Text(timeline)}, found(h.Marked, h.Name)...),
			Trail: w.Dim{Parts: []w.Inline{w.Text(asksEvery)}, Size: w.Dim11, Variant: w.DimTight, Place: w.Pushed}, Href: h.Href, When: w.WhenFound})
	}
	if len(p.History) > 0 {
		items = append(items, w.MenuSep{}, w.MenuHeading{Rubric: w.Rubric{Text: recentRubric, Form: w.RubricItem}})
		for _, h := range p.History {
			it := recentItem(h)
			it.Label, it.Found = "", found(h.Marked, h.Query)
			items = append(items, it)
		}
	}
	return w.Menu{Items: items, Width: w.MenuWidthFull}
}

// recentItem is a recent search: a link to its result, how much it found
// and when, monospaced.
func recentItem(h view.SearchHistoryRow) w.MenuItem {
	trail := string(h.Outcome)
	if h.When != "" {
		trail += sep + string(h.When)
	}
	return w.MenuItem{Icon: "clock", Label: string(h.Query), Href: h.Href,
		Trail: w.Mono{Text: trail, Variant: w.Mono11DustyTight, Place: w.Pushed}}
}

// found is a title with what was typed bold, or the title as it is.
func found(marked view.Runs, title view.Text) []w.Inline {
	if len(marked) > 0 {
		return bind.Inlines(marked)
	}
	return []w.Inline{w.Text(title)}
}
