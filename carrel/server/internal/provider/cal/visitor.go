package cal

import (
	"bytes"
	"fmt"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// Visitor is a record as a guest of a link may have it (S-25.1, Б39 of the
// audit of 11.10.2026): a new calendar built of what is allowed, never one
// cut down from the record — a property Carrel does not know, one it will
// learn of later, a reminder, is not in it without anyone having to think
// of taking it out. Each event or task of it (the series and its occurrences
// changed apart) keeps the properties allowed names; the zones its times
// are written in go whole; nothing else does. A record whose title is not
// allowed gets title instead, so a row still says something. The calendar's
// own lines are only its version and who wrote it.
func Visitor(obj *model.Object, allowed func(prop string) bool, title string) (*model.Object, error) {
	if obj == nil || !obj.IsCalendar() {
		return nil, fmt.Errorf("cal: not a calendar object")
	}
	doc, err := decode(obj.CalendarDoc())
	if err != nil {
		return nil, err
	}
	main := findMain(doc)
	if main == nil {
		return nil, fmt.Errorf("cal: no main component")
	}
	out := ical.NewCalendar()
	out.Props.SetText(ical.PropVersion, "2.0")
	out.Props.SetText(ical.PropProductID, "-//Carrel//published//EN")
	for _, c := range doc.Children {
		switch {
		case c.Name == ical.CompTimezone:
			out.Children = append(out.Children, c)
		case c.Name == main.Name:
			kept := ical.NewComponent(c.Name)
			for name, props := range c.Props {
				if allowed(name) {
					kept.Props[name] = props
				}
			}
			if !allowed(ical.PropSummary) && title != "" {
				kept.Props.SetText(ical.PropSummary, title)
			}
			out.Children = append(out.Children, kept)
		}
	}
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(out); err != nil {
		return nil, err
	}
	return Parse(obj.Path, obj.ETag, &buf)
}
