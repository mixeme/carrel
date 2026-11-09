// Package duplicates is the screen of duplicates, /app/duplicates, drawn by
// frames «Дубликаты · группы и решения» (s-dup-1), «Дубликаты · ничего не
// загружено» (s-dup-6) and the menu of weights (s-dup-3, -7) of the mockup:
// the draft frame2go printed for s-dup-1, with the frame's data become the
// view model's (U-5.1, U-6.1; Typecase's note 117).
package duplicates

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3): the shell's; the rubric of the decisions none
// of whose records is loaded (s-dup-6); the menu of weights (s-dup-3); the
// sign of a task; the dash of what is not there.
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
		NewKey: "n", SearchKey: "/",
		Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
		Keys:  bind.KeysWords{Label: "Keyboard shortcuts", Icon: "search", Key: "?", Sheet: "keys"},
	}
	// The sources compared: no ticks — what is compared is what the lists
	// read —, a number for each, the dash before it was compared (s-dup-6).
	rail = bind.RailWords{Davloom: "davloom", None: "—"}
)

const (
	decidedEarlier = "Decided earlier"
	weightsLabel   = "Weights"
	perSignal      = "Points per signal"
	byDefault      = "Default"
	yours          = "Yours"
	changed        = "changed"
	notDone        = "Not done"
	done           = "Done"
	none           = "—"
	sep            = "· "
)

// Screen draws the duplicates from their view model.
//
//   - The shell: the top bar; the rail of sections and «Sources» — each
//     collection compared, with how many of its records —, the foot.
//   - The head: «Duplicates», what was compared and how many groups wait;
//     the kinds as segments (none when nothing was compared, s-dup-6); the
//     threshold, a form of its own, and «Weights», its menu a form too.
//   - The notices: what linking means, what is not loaded.
//   - The groups, each a card and a form (note 117 p. 8): what it is
//     called, what is decided and its points; its records, each the row of
//     its kind's list; what such a pair means; the questions where the
//     records disagree — radios, and ticks of what to keep; the decisions
//     as the form's buttons, why Merge cannot be pressed under them.
//   - «Decided earlier» (s-dup-6): the decisions none of whose records is
//     loaded — where they lie, their UIDs, their undoing.
func Screen(p *view.DuplicatesPage) w.Shell {
	t := bind.Top(p.Top, top)
	main := []w.MainPart{head(p)}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	var results []w.ResultsPart
	for _, g := range p.Groups {
		results = append(results, card(g))
	}
	if len(p.Decided) > 0 {
		results = append(results, w.Rubric{Text: decidedEarlier, Items: []w.RubricPart{w.Mono{Text: strconv.Itoa(len(p.Decided)), Variant: w.MonoTight}}})
		for _, d := range p.Decided {
			results = append(results, decided(d))
		}
	}
	if len(results) > 0 {
		main = append(main, w.MainResults{Items: results})
	}
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}}
}

// Forms are the page's forms: the top bar's, the rail's foot's, the
// notices'.
func Forms(p *view.DuplicatesPage) []w.PostForm {
	out := bind.TopForms(p.Top)
	out = append(out, bind.FootForms(p.Rail)...)
	return append(out, bind.NoticeForms(p.Notices)...)
}

// head is the title, what was compared, the kinds and the threshold.
func head(p *view.DuplicatesPage) w.Head {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	acts := &w.Acts{}
	if len(p.Bar.Segments) > 0 {
		seg := w.Seg{}
		for _, a := range p.Bar.Segments {
			seg.Options = append(seg.Options, bind.SegOption(a))
		}
		acts.Items, acts.Stack = append(acts.Items, seg), true
	}
	acts.Items = append(acts.Items, threshold(p.Threshold))
	h.Acts = acts
	return h
}

// threshold is the field of the number of points, a form of its own sent
// by Enter (s-dup-1), with «Weights» beside it: the menu of a weight per
// signal, a form with Save and Back to defaults (s-dup-3, -7).
func threshold(c view.ThresholdControl) w.Field {
	f := w.Field{Width: w.FieldInline, Action: c.Form.Action, Post: c.Form.Method == "POST", Sends: bind.Hidden(c.Form)}
	for _, r := range bind.Rows(c.Form) {
		if r.Kind == view.InputHidden {
			continue
		}
		f.Label = string(r.Label)
		in := bind.Input(r)
		in.Variant = w.InputW52
		f.Parts = append(f.Parts, in)
	}
	items := []w.MenuPart{w.MenuHeading{Rubric: w.Rubric{Text: perSignal, Items: []w.RubricPart{
		w.Push{Items: []w.PushPart{w.Text(byDefault)}}, w.RubricCol{Text: yours},
	}}, Form: w.MenuHeadingRoomy}}
	for _, wc := range c.Weights {
		mf := w.MenuField{Label: string(wc.Label), Default: w.Dim{Parts: []w.Inline{w.Text(strconv.Itoa(wc.Default))}, Size: w.Dim10_5, Place: w.Pushed},
			Input: w.Input{Name: wc.Name, Value: strconv.Itoa(wc.Value), Label: string(wc.Label), Mono: true, Variant: w.InputChip}}
		if wc.Own {
			mf.Badge = &w.Badge{Label: changed}
		}
		items = append(items, mf)
	}
	var foot []w.Button
	for _, a := range c.WeightForm.Actions {
		b := bind.Button(a, "")
		b.Small, b.Reason = true, ""
		if !a.Enabled {
			b.Title = string(a.Reason)
		}
		foot = append(foot, b)
	}
	items = append(items, w.MenuFoot{Items: foot})
	f.Parts = append(f.Parts, w.Drop{
		Trigger: w.Button{Label: weightsLabel, Small: true},
		Menu:    w.Menu{Items: items, Width: w.MenuWidth420, Fields: true, Action: c.WeightForm.Action, Hidden: bind.Hidden(c.WeightForm)},
	})
	return f
}

// card is a group: a form with its records, its questions and its
// decisions.
func card(g view.DuplicateGroup) w.Card {
	aside := []w.RubricAsidePart{bind.Badge(g.Badge)}
	if g.Points != "" {
		aside = append(aside, w.Mono{Text: string(g.Points), Variant: w.MonoTightDusty})
	}
	c := w.Card{Items: []w.CardPart{w.Rubric{Text: string(g.Title), Items: []w.RubricPart{w.RubricAside{Items: aside}}}}}
	list := w.List{Inset: true}
	for _, m := range g.Rows {
		list.Rows = append(list.Rows, row(g.Kind, m))
	}
	c.Items = append(c.Items, list)
	if g.Hint != "" {
		c.Items = append(c.Items, w.CardHint{Parts: []w.CardTextPart{w.Text(g.Hint)}, Line: true})
	}
	if len(g.Choices) > 0 {
		questions := w.CardRow{Kind: w.CardTop11, Gap: w.LineGapFields}
		for _, q := range g.Choices {
			questions.Items = append(questions.Items, question(q))
		}
		c.Items = append(c.Items, questions)
	}
	acts := w.CardRow{Kind: w.CardActs, Gap: w.LineGapNormal}
	var why string
	for _, a := range g.Actions {
		b := bind.Button(a, "")
		if a.ID == "duplicates.merge" {
			// Merge stands last, pushed, outlined — the merge itself is
			// the solid one —; switched off, it says why under the
			// buttons, not in its title (s-dup-1).
			b.Kind, b.Place, b.Reason, why = w.ButtonDanger, w.Pushed, "", string(a.Reason)
		}
		acts.Items = append(acts.Items, b)
	}
	c.Items = append(c.Items, acts)
	if why != "" {
		c.Items = append(c.Items, w.CardText{Kind: w.CardWhy, Parts: []w.CardTextPart{w.Text(why)}})
	}
	if g.Form != nil {
		c.Action, c.Fields = g.Form.Action, bind.Hidden(*g.Form)
	}
	return c
}

// question is a question of a group: radios of a value held once, ticks of
// what to keep of a value held many times (note 117 p. 1).
func question(q view.MergeRow) w.Field {
	line := w.Line{Gap: w.LineGapWide, Block: true}
	for i, o := range q.Options {
		ch := w.Check{Name: q.Name, Parts: []w.CheckPart{w.Text(o)}, Radio: !q.Many}
		if i < len(q.Values) {
			ch.Value = q.Values[i]
		}
		if q.Many {
			for _, k := range q.Kept {
				ch.On = ch.On || k == i
			}
		} else {
			ch.On = i == q.Chosen
		}
		line.Items = append(line.Items, ch)
	}
	return w.Field{Label: string(q.Property), Parts: []w.FieldPart{line}}
}

// row is a record of a group as the row of its kind's list (s-dup-1): a
// contact's face, phone, address and organisation; an event's time, place
// and repetition; a task's sign, due and progress; a note's excerpt and
// day — a cell with nothing in it a dash.
func row(k view.Kind, m view.DuplicateMember) w.ListRow {
	mark := w.Mark{Colour: string(m.Colour)}
	title := []w.Inline{w.Text(m.Title)}
	switch k {
	case view.KindContact:
		f := w.FaceRow{Mark: mark, Title: title, Subtitle: or(m.Phone), Detail: or(m.Email), Caption: or(m.Org), Source: string(m.Stamp), Href: m.Href, Inset: true}
		if m.Avatar != nil {
			f.Avatar = bind.Avatar(*m.Avatar)
		}
		return f
	case view.KindEvent:
		r := w.TimeRow{Mark: mark, Time: string(m.Lead), Title: title, Detail: or(m.Place), Source: string(m.Stamp), Href: m.Href, Inset: true}
		if m.Repeats != "" {
			r.Badges = []w.Badge{{Label: string(m.Repeats)}}
		}
		return r
	case view.KindTask:
		// A sign, not a button: a task is ticked in its list or on its card
		// (note 117 p. 11).
		tick := w.Tick{Sign: true, Label: notDone}
		if m.Done != nil && *m.Done {
			tick.Done, tick.Label = true, done
		}
		progress := none
		if m.Percent != nil {
			progress = strconv.Itoa(*m.Percent) + "%"
		}
		return w.DueRow{Tick: tick, Mark: mark, Title: title, Due: or(m.Due), Progress: progress, Source: string(m.Stamp), Href: m.Href, Inset: true}
	}
	return w.ExcerptRow{Mark: mark, Title: title, Excerpt: string(m.Excerpt), Date: string(m.Date), Source: string(m.Stamp), Href: m.Href, Inset: true}
}

// decided is a decision none of whose records is loaded (s-dup-6): what
// was decided, where its records lie and their UIDs, its undoing.
func decided(d view.DecidedCard) w.Card {
	c := w.Card{Items: []w.CardPart{
		w.Rubric{Text: string(d.Title), Items: []w.RubricPart{w.RubricAside{Items: []w.RubricAsidePart{
			bind.Badge(d.Badge), w.Mono{Text: string(d.Note), Variant: w.MonoTightDusty},
		}}}},
		w.CardText{Kind: w.CardLine, Parts: []w.CardTextPart{w.Text(string(d.Where) + " "), w.Dim{Parts: []w.Inline{w.Text(sep + string(d.UIDs))}}}},
	}}
	acts := w.CardRow{Kind: w.CardActs, Gap: w.LineGapNormal}
	for _, a := range d.Actions {
		acts.Items = append(acts.Items, bind.Button(a, ""))
	}
	c.Items = append(c.Items, acts)
	if d.Form != nil {
		c.Action, c.Fields = d.Form.Action, bind.Hidden(*d.Form)
	}
	return c
}

// railParts is the rail of sections, the sources compared and the foot.
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

// or is a value, or the dash where the model has none.
func or(t view.Text) string {
	if t == "" {
		return none
	}
	return string(t)
}
