package cal

import (
	"errors"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// iCalendar keeps TEXT values escaped in the record (a comma is "\,"). The
// screen reads them through Text; a patch writes them through EscapeText.

// Text returns the first value of a TEXT property of the main component,
// unescaped for display.
func Text(obj *model.Object, name string) string {
	vals := obj.PropText(name)
	if len(vals) == 0 {
		return ""
	}
	p := ical.NewProp(name)
	p.Value = vals[0]
	if s, err := p.Text(); err == nil {
		return s
	}
	return vals[0]
}

// TextList returns every value of a TEXT list property such as CATEGORIES,
// split on unescaped commas and unescaped.
func TextList(obj *model.Object, name string) []string {
	var out []string
	for _, raw := range obj.PropText(name) {
		p := ical.NewProp(name)
		p.Value = raw
		l, err := p.TextList()
		if err != nil {
			l = []string{raw}
		}
		for _, v := range l {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

// TimeOf reads a DATE or DATE-TIME property of the main component, such as
// LAST-MODIFIED or DTSTAMP.
func TimeOf(obj *model.Object, name string, loc *time.Location) (time.Time, bool) {
	main, err := mainComponent(obj)
	if err != nil {
		return time.Time{}, false
	}
	p := main.Props.Get(name)
	if p == nil {
		return time.Time{}, false
	}
	t, err := propTime(p, loc)
	return t, err == nil
}

// EscapeText is the wire form of a TEXT value, for model.SetText.
func EscapeText(s string) string {
	p := ical.NewProp(ical.PropDescription)
	p.SetText(s)
	return p.Value
}

// UID returns the UID of the main component.
func UID(obj *model.Object) string {
	if obj == nil {
		return ""
	}
	return Text(obj, ical.PropUID)
}

// HasRecurrence reports an RRULE or RDATE on the main component.
func HasRecurrence(obj *model.Object) bool {
	return len(obj.PropText(ical.PropRecurrenceRule)) > 0 || len(obj.PropText(ical.PropRecurrenceDates)) > 0
}

// Timing is when a component happens, as the other client wrote it.
type Timing struct {
	Start time.Time
	End   time.Time // zero without DTEND or DURATION
	// AllDay is a DATE value: a day, not an instant.
	AllDay bool
	// Zone is the TZID the value named; empty for UTC and floating times.
	Zone string
}

// TimingOf reads DTSTART and DTEND or DURATION of the main component. Floating
// times and dates are read in loc, the instance zone (C-10).
func TimingOf(obj *model.Object, loc *time.Location) (Timing, error) {
	main, err := mainComponent(obj)
	if err != nil {
		return Timing{}, err
	}
	return timingOf(main, ical.PropDateTimeStart, loc)
}

// DueOf reads DUE of a task the same way; Start holds the due time.
func DueOf(obj *model.Object, loc *time.Location) (Timing, error) {
	main, err := mainComponent(obj)
	if err != nil {
		return Timing{}, err
	}
	return timingOf(main, ical.PropDue, loc)
}

func timingOf(c *ical.Component, startProp string, loc *time.Location) (Timing, error) {
	var t Timing
	start := c.Props.Get(startProp)
	if start == nil {
		return t, nil
	}
	var err error
	if t.Start, err = propTime(start, loc); err != nil {
		return t, err
	}
	t.AllDay = isDate(start)
	t.Zone = start.Params.Get(ical.ParamTimezoneID)
	if startProp != ical.PropDateTimeStart {
		return t, nil
	}
	if end := c.Props.Get(ical.PropDateTimeEnd); end != nil {
		t.End, err = propTime(end, loc)
	} else if d := c.Props.Get(ical.PropDuration); d != nil {
		var dur time.Duration
		if dur, err = d.Duration(); err == nil {
			t.End = t.Start.Add(dur)
		}
	}
	return t, err
}

// propTime reads a DATE or DATE-TIME. A TZID the Go zone database does not
// know — Outlook writes Windows zone names — is read as a wall clock in loc
// rather than losing the record.
func propTime(p *ical.Prop, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	t, err := p.DateTime(loc)
	if err == nil {
		return t, nil
	}
	if p.Params.Get(ical.ParamTimezoneID) != "" {
		if wall, perr := time.ParseInLocation("20060102T150405", p.Value, loc); perr == nil {
			return wall, nil
		}
	}
	return time.Time{}, err
}

func isDate(p *ical.Prop) bool {
	switch p.ValueType() {
	case ical.ValueDate:
		return true
	case ical.ValueDefault:
		return len(p.Value) == len("20060102")
	default:
		return false
	}
}

// Override is an instance of the series rewritten by another client: a
// sibling with RECURRENCE-ID. Carrel shows it in place of the instant it
// replaces and never edits it (S-11).
type Override struct {
	RecurrenceID time.Time // UTC, comparable with Expand
	Timing       Timing
	Summary      string
	Location     string
	Cancelled    bool
}

// Overrides lists the RECURRENCE-ID siblings of the main component.
func Overrides(obj *model.Object, loc *time.Location) ([]Override, error) {
	if obj == nil || !obj.IsCalendar() {
		return nil, nil
	}
	cal, err := decode(obj.CalendarDoc())
	if err != nil {
		return nil, err
	}
	main := findMain(cal)
	if main == nil {
		return nil, nil
	}
	uid := propText(main.Props, ical.PropUID)
	var out []Override
	for _, child := range cal.Children {
		rid := child.Props.Get(ical.PropRecurrenceID)
		if child.Name != main.Name || rid == nil {
			continue
		}
		if u := propText(child.Props, ical.PropUID); u != "" && u != uid {
			continue
		}
		at, err := propTime(rid, loc)
		if err != nil {
			return nil, err
		}
		tm, err := timingOf(child, ical.PropDateTimeStart, loc)
		if err != nil {
			return nil, err
		}
		out = append(out, Override{
			RecurrenceID: at.UTC(),
			Timing:       tm,
			Summary:      propText(child.Props, ical.PropSummary),
			Location:     propText(child.Props, ical.PropLocation),
			Cancelled:    strings.EqualFold(propText(child.Props, ical.PropStatus), "CANCELLED"),
		})
	}
	return out, nil
}

// Attendee is one ATTENDEE, read only: no invitations, no replies (S-11).
type Attendee struct {
	Name   string
	Email  string
	Status string // PARTSTAT as written
}

// Attendees lists ATTENDEE of the main component.
func Attendees(obj *model.Object) []Attendee {
	var out []Attendee
	for _, v := range obj.Props()[ical.PropAttendee] {
		addr := v.Text
		if len(addr) >= len("mailto:") && strings.EqualFold(addr[:len("mailto:")], "mailto:") {
			addr = addr[len("mailto:"):]
		}
		out = append(out, Attendee{
			Name:   v.Params[ical.ParamCommonName],
			Email:  addr,
			Status: v.Params[ical.ParamParticipationStatus],
		})
	}
	return out
}

// RelatedTo lists the UIDs in RELATED-TO of the main component.
func RelatedTo(obj *model.Object) []string {
	var out []string
	for _, v := range obj.PropText(ical.PropRelatedTo) {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func propText(props ical.Props, name string) string {
	p := props.Get(name)
	if p == nil {
		return ""
	}
	if s, err := p.Text(); err == nil {
		return s
	}
	return p.Value
}

func mainComponent(obj *model.Object) (*ical.Component, error) {
	if obj == nil || !obj.IsCalendar() {
		return nil, errors.New("cal: not a calendar object")
	}
	main, err := loadMain(obj)
	if err != nil {
		return nil, err
	}
	if main == nil {
		return nil, errors.New("cal: no main component")
	}
	return main, nil
}
