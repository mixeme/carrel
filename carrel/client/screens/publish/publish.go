// Package publish is the sheet that publishes a link to a collection, a
// record, a file or a folder, /app/publish (PublishSheet), drawn by frames
// «Опубликовать календарь» (s-publish-2), a contact (s-publish-3), a note
// (s-publish-4), a folder (s-publish-5), an event (s-publish-19), and the
// link made (s-publish-13, -22): the drafts frame2go printed for them, their
// data become the view model's (U-5.1). The sheet stands over the page it
// was opened from (Under), which the registry draws (screens.over).
package publish

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The sheet's words (U-1.3): the badge of a local instance and its hint,
// the cross, Copy of the link and what it says once it copied, the icon of
// Copy.
const (
	local     = "local"
	localHint = "Local instance: the link will not open from outside your network"
	closeWord = "Close"
	copyWord  = "Copy"
	copied    = "Copied"
	copyIcon  = "dup"
	enterKey  = "Enter"
	escKey    = "Esc"
)

// The names of the contract the screen places by (C-7.5): the rows of the
// sheet that are not plain fields, its cross and Cancel.
const (
	howField   = "how"
	linkField  = "link"
	livesField = "lives"
	closeID    = "publish.close"
	cancelID   = "publish.cancel"
)

// Sheet draws the sheet, before the link is made or after.
//
//   - The head: «Publish a calendar», the badge of a local instance, what
//     is published muted and pushed right (Target), the cross once the
//     link is made.
//   - Before: how much of a calendar (a switch, The whole calendar turning
//     the days off) and its days side by side; «What the visitor sees» —
//     its ticks one under another, Time on and off with its reason, the
//     line under them; «Allow .ics download…»; «Link lives for» — a choice
//     with its menu; what a folder's visitor gets (Note). The foot: Create
//     link with Enter, Cancel with Esc pushed right.
//   - After: «Link» — the address whole, read-only, with Copy — and its
//     day under it (Expires). The foot: Send by letter with Enter, Open the
//     link, All published links.
func Sheet(p *view.PublishSheet) w.Sheet {
	head := []w.SheetHeadPart{}
	if p.Local {
		head = append(head, w.Badge{Label: local, Tone: w.BadgeLocal, Title: localHint})
	}
	head = append(head, w.Dim{Parts: []w.Inline{w.Text(p.Target)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed})
	var body []w.SheetPart
	// A calendar's kinds hold their download among the sheet's ticks
	// (s-publish-2, -19); a contact's stands alone (s-publish-3).
	calendar := p.Kind == view.KindEvent || p.Kind == view.KindTask
	for _, set := range p.Form.Sets {
		if set.Legend != "" {
			// «What the visitor sees»: the ticks one under another, the line
			// under them.
			g := w.Group{Form: w.GroupColumnLoose}
			for _, r := range set.Rows {
				if r.Kind == view.InputCheck {
					g.Items = append(g.Items, aside(bind.Check(r)))
					// What the tick does, under it (a note's attachments,
					// s-publish-4).
					if r.Hint != "" {
						g.Items = append(g.Items, w.FieldHint{Parts: []w.Inline{w.Text(r.Hint)}})
					}
				}
			}
			f := w.Field{Label: string(set.Legend), Parts: []w.FieldPart{g}, Width: w.FieldW100}
			if set.LegendAside != "" {
				// How many and how large, after the rubric (s-publish-4).
				f.LabelAside = w.Plain{Text: string(set.LegendAside)}
			}
			if set.Hint != "" {
				f.Parts = append(f.Parts, w.FieldHint{Parts: []w.Inline{w.Text(set.Hint)}})
			}
			body = append(body, f)
			continue
		}
		var days []w.LinePart
		for _, r := range set.Rows {
			switch {
			case r.Kind == view.InputHidden:
			case r.Name == howField:
				seg := w.Seg{Name: r.Name}
				for _, o := range r.Options {
					seg.Options = append(seg.Options, w.SegOption{Label: string(o.Label), Value: o.Value, On: o.Selected, Disables: o.Disables})
				}
				body = append(body, w.Field{Label: string(r.Label), Parts: []w.FieldPart{seg}, Width: w.FieldW100})
			case r.Kind == view.InputDate:
				// The days side by side (s-publish-2).
				days = append(days, bind.Field(r, w.FieldSM))
			case r.Name == linkField:
				// The link made, whole, to copy (s-publish-13).
				in := bind.Input(r)
				in.Variant = w.InputFill
				f := w.Field{Label: string(r.Label), Parts: []w.FieldPart{w.Line{Items: []w.LinePart{in,
					w.Button{Label: copyWord, Icon: copyIcon, Small: true, Copy: string(r.Value), Copied: copied}}, Gap: w.LineGap6}}, Width: w.FieldW100}
				if p.Expires != "" {
					f.Parts = append(f.Parts, w.FieldHint{Parts: []w.Inline{w.Text(p.Expires)}})
				}
				body = append(body, f)
			case r.Name == livesField:
				body = append(body, w.Field{Label: string(r.Label), Parts: []w.FieldPart{lives(r)}, Width: w.FieldSM})
			case r.Kind == view.InputCheck && calendar:
				body = append(body, w.SheetChecks{Items: []w.Check{bind.Check(r)}})
			case r.Kind == view.InputCheck:
				body = append(body, bind.Check(r))
			default:
				body = append(body, bind.Field(r, w.FieldW100))
			}
			if len(days) == 2 {
				body = append(body, w.Line{Items: days, Gap: w.LineGap10, Block: true})
				days = nil
			}
		}
	}
	if p.Note != "" {
		body = append(body, w.Msg{Parts: []w.MsgPart{w.Text(p.Note)}, Space: w.MsgFlush})
	}
	var foot []w.SheetFootPart
	for _, a := range p.Actions {
		switch {
		case a.ID == closeID:
			head = append(head, w.Close{Label: closeWord, Href: a.Href})
		case a.ID == cancelID:
			foot = append(foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGap7, Place: w.Pushed})
		case a.Method == "POST" && a.Href == p.Form.Action:
			foot = append(foot, bind.Submit(a))
		default:
			foot = append(foot, bind.Button(a, ""))
		}
		if a.Primary {
			foot = append(foot, w.Kbd{Keys: enterKey})
		}
	}
	return w.Sheet{
		Title:  string(p.Title),
		Head:   head,
		Body:   body,
		Foot:   foot,
		Action: p.Form.Action, Fields: bind.Hidden(p.Form),
	}
}

// aside is a tick with what it says besides its words at 11.5 px, as Time's
// «always shown» is (s-publish-2).
func aside(c w.Check) w.Check {
	for i, p := range c.Parts {
		if d, ok := p.(w.Dim); ok {
			d.Size = w.Dim11_5
			c.Parts[i] = d
		}
	}
	return c
}

// lives is «Link lives for» (s-publish-2): the term chosen, its menu the
// terms each a radio of the form, a rule before No expiry.
func lives(r view.FormRow) w.Drop {
	pick := w.Pick{}
	var items []w.MenuPart
	for i, o := range r.Options {
		if o.Selected {
			pick.Value = string(o.Label)
		}
		if o.Separator && i > 0 {
			items = append(items, w.MenuSep{})
		}
		items = append(items, w.MenuItem{Label: string(o.Label), On: o.Selected, Name: r.Name, Value: o.Value})
	}
	return w.Drop{Trigger: pick, Menu: w.Menu{Items: items, Width: w.MenuWidth212, Pop: w.MenuUnder}}
}
