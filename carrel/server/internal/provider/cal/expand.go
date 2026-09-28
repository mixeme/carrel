package cal

import (
	"time"

	"github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"

	"carrel.dev/carrel/server/internal/model"
)

// RawRRULE returns the RRULE text on the main component.
func RawRRULE(obj *model.Object) string {
	if obj == nil {
		return ""
	}
	if txt := obj.PropText("RRULE"); len(txt) > 0 {
		return txt[0]
	}
	return ""
}

// Expand returns recurrence instances in [from, to] UTC. The window is the
// caller's; EXDATE is subtracted and RDATE added. RECURRENCE-ID siblings are
// not expanded as a second series (S-11).
func Expand(obj *model.Object, from, to time.Time, loc *time.Location) ([]time.Time, error) {
	if obj == nil || !obj.IsCalendar() {
		return nil, nil
	}
	comp, err := loadMain(obj)
	if err != nil || comp == nil {
		return nil, err
	}
	if loc == nil {
		loc = time.UTC
	}

	set, err := seriesOf(comp, loc)
	if err != nil {
		return nil, err
	}
	if set == nil {
		start, ok := startOf(comp, loc)
		if !ok {
			return nil, nil
		}
		if !start.Before(from) && !start.After(to) {
			return []time.Time{start.UTC()}, nil
		}
		return nil, nil
	}

	raw := set.Between(from, to, true)
	out := make([]time.Time, len(raw))
	for i, t := range raw {
		out[i] = t.UTC()
	}
	return out, nil
}

// startOf is DTSTART of a component, false without one.
func startOf(comp *ical.Component, loc *time.Location) (time.Time, bool) {
	p := comp.Props.Get(ical.PropDateTimeStart)
	if p == nil {
		return time.Time{}, false
	}
	start, err := propTime(p, loc)
	return start, err == nil
}

// seriesOf is the recurrence set of a component — RRULE, RDATE and EXDATE
// from its DTSTART — or nil when it neither repeats nor names extra dates.
func seriesOf(comp *ical.Component, loc *time.Location) (*rrule.Set, error) {
	var start time.Time
	if p := comp.Props.Get(ical.PropDateTimeStart); p != nil {
		var err error
		if start, err = propTime(p, loc); err != nil {
			return nil, err
		}
	}

	set := &rrule.Set{}
	hasRule := false
	opt, err := comp.Props.RecurrenceRule()
	if err != nil {
		return nil, err
	}
	if opt != nil {
		rule, err := rrule.NewRRule(*opt)
		if err != nil {
			return nil, err
		}
		set.RRule(rule)
		hasRule = true
	}
	if !start.IsZero() {
		set.DTStart(start)
	}

	for _, p := range comp.Props[ical.PropExceptionDates] {
		ex, err := propTime(&p, loc)
		if err != nil {
			return nil, err
		}
		set.ExDate(ex)
	}
	hasRDate := false
	for _, p := range comp.Props[ical.PropRecurrenceDates] {
		rd, err := propTime(&p, loc)
		if err != nil {
			return nil, err
		}
		set.RDate(rd)
		hasRDate = true
	}
	if !hasRule && !hasRDate {
		return nil, nil
	}
	return set, nil
}

func loadMain(obj *model.Object) (*ical.Component, error) {
	cal, err := decode(obj.CalendarDoc())
	if err != nil {
		return nil, err
	}
	return findMain(cal), nil
}
