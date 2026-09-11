package cal

import (
	"bytes"
	"time"

	"github.com/emersion/go-ical"

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

// Expand returns recurrence instances in [from, to] UTC (S-11).
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
	rset, err := comp.RecurrenceSet(loc)
	if err != nil {
		return nil, err
	}
	if rset == nil {
		start, err := comp.Props.DateTime(ical.PropDateTimeStart, loc)
		if err != nil || start.IsZero() {
			return nil, err
		}
		if !start.Before(from) && !start.After(to) {
			return []time.Time{start.UTC()}, nil
		}
		return nil, nil
	}
	raw := rset.Between(from, to, true)
	out := make([]time.Time, len(raw))
	for i, t := range raw {
		out[i] = t.UTC()
	}
	return out, nil
}

func loadMain(obj *model.Object) (*ical.Component, error) {
	dec := ical.NewDecoder(bytes.NewReader(obj.CalendarDoc()))
	cal, err := dec.Decode()
	if err != nil {
		return nil, err
	}
	return findMain(cal), nil
}
