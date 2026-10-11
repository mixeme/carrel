package cal

import (
	"errors"
	"fmt"
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
	// SentBy is who actually sent this on the attendee's behalf, and
	// Delegated* are the people participation passed between (RFC 5545
	// §3.2.18, §3.2.4, §3.2.5). All of them name people who are not this
	// attendee, and a person who is only named here is named nowhere else
	// in the record: the one who delegated away is often removed from the
	// list entirely.
	SentBy        string
	DelegatedFrom []string
	DelegatedTo   []string
}

// Attendees lists ATTENDEE of the main component.
func Attendees(obj *model.Object) []Attendee {
	var out []Attendee
	for _, v := range obj.Props()[ical.PropAttendee] {
		out = append(out, Attendee{
			Name:          v.Params[ical.ParamCommonName],
			Email:         CalAddress(v.Text),
			Status:        v.Params[ical.ParamParticipationStatus],
			SentBy:        CalAddress(v.Params[ical.ParamSentBy]),
			DelegatedFrom: calAddresses(ical.ParamDelegatedFrom, v.Params),
			DelegatedTo:   calAddresses(ical.ParamDelegatedTo, v.Params),
		})
	}
	return out
}

// Organiser is who a record is organised by, and who sent it for them.
// ORGANIZER is one per component by RFC 5545 §3.8.4.3, so the first is it.
func Organiser(obj *model.Object) Attendee {
	for _, v := range obj.Props()[ical.PropOrganizer] {
		return Attendee{
			Name:   v.Params[ical.ParamCommonName],
			Email:  CalAddress(v.Text),
			SentBy: CalAddress(v.Params[ical.ParamSentBy]),
		}
	}
	return Attendee{}
}

// Person is one person a record names, and the part they have in it.
type Person struct {
	Attendee
	Organiser bool
}

// People lists everyone a record names: the organiser and the attendees of
// the main component and of every RECURRENCE-ID sibling beside it. A person
// can belong to one occurrence only — added to a single week of a series,
// or standing in for someone that Thursday — and reading the main component
// alone would not find them at all (S-15.4).
//
// A record whose document cannot be read, or that has none because it was
// built rather than fetched, falls back to its main component: less than
// the truth, but never a lie.
func People(obj *model.Object) []Person {
	main := append(organiserOf(obj), attendeesOf(obj)...)
	if obj == nil || !obj.IsCalendar() {
		return main
	}
	doc, err := decode(obj.CalendarDoc())
	if err != nil {
		return main
	}
	root := findMain(doc)
	if root == nil {
		return main
	}
	uid := propText(root.Props, ical.PropUID)
	out := peopleIn(root)
	for _, child := range doc.Children {
		if child.Name != root.Name || child.Props.Get(ical.PropRecurrenceID) == nil {
			continue
		}
		if u := propText(child.Props, ical.PropUID); u != "" && u != uid {
			continue
		}
		out = append(out, peopleIn(child)...)
	}
	return out
}

func peopleIn(c *ical.Component) []Person {
	var out []Person
	for _, v := range c.Props[ical.PropOrganizer] {
		out = append(out, Person{Attendee: personOf(v), Organiser: true})
	}
	for _, v := range c.Props[ical.PropAttendee] {
		out = append(out, Person{Attendee: personOf(v)})
	}
	return out
}

func personOf(v ical.Prop) Attendee {
	params := map[string]string{}
	for name, values := range v.Params {
		params[strings.ToUpper(name)] = strings.Join(values, ",")
	}
	return Attendee{
		Name:          params[ical.ParamCommonName],
		Email:         CalAddress(v.Value),
		Status:        params[ical.ParamParticipationStatus],
		SentBy:        CalAddress(params[ical.ParamSentBy]),
		DelegatedFrom: calAddresses(ical.ParamDelegatedFrom, params),
		DelegatedTo:   calAddresses(ical.ParamDelegatedTo, params),
	}
}

func organiserOf(obj *model.Object) []Person {
	if o := Organiser(obj); o.Email != "" || o.SentBy != "" {
		return []Person{{Attendee: o, Organiser: true}}
	}
	return nil
}

func attendeesOf(obj *model.Object) []Person {
	var out []Person
	for _, a := range Attendees(obj) {
		out = append(out, Person{Attendee: a})
	}
	return out
}

// CalAddress is a CAL-ADDRESS as an address: the mailto: a calendar writes
// and this side does not want, whatever case it came in.
func CalAddress(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= len("mailto:") && strings.EqualFold(v[:len("mailto:")], "mailto:") {
		v = v[len("mailto:"):]
	}
	return v
}

// calAddresses reads a parameter that holds a list of addresses. Its commas
// are separators by definition, and model.ParamValues knows which
// parameters those are.
func calAddresses(name string, params map[string]string) []string {
	joined, ok := params[name]
	if !ok || joined == "" {
		return nil
	}
	var out []string
	for _, v := range model.ParamValues(name, joined) {
		if a := CalAddress(v); a != "" {
			out = append(out, a)
		}
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

// FormatDuration writes a length of time as a DURATION (RFC 5545 §3.3.6): in
// days when it is whole days of an all-day event, else in hours, minutes and
// seconds, the parts that are zero left out.
func FormatDuration(d time.Duration, days bool) string {
	sign := ""
	if d < 0 {
		sign, d = "-", -d
	}
	if days && d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%sP%dD", sign, d/(24*time.Hour))
	}
	h, m, s := d/time.Hour, d%time.Hour/time.Minute, d%time.Minute/time.Second
	out := sign + "PT"
	if h > 0 {
		out += fmt.Sprintf("%dH", h)
	}
	if m > 0 {
		out += fmt.Sprintf("%dM", m)
	}
	if s > 0 || h == 0 && m == 0 {
		out += fmt.Sprintf("%dS", s)
	}
	return out
}
