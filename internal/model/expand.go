// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"fmt"
	"time"

	"github.com/emersion/go-ical"
)

// Occurrence is one expanded instance of a VEVENT in a time range (§10).
type Occurrence struct {
	UID       string
	Path      string
	ETag      string
	Summary   string
	Location  string
	Start     time.Time
	End       time.Time
	AllDay    bool
	Recurring bool
}

// ExpandOccurrences expands the object's VEVENT into instances that overlap
// [from, to). Non-recurring events yield at most one occurrence. Expansion
// uses rrule-go via go-ical; the server's expand is never relied on (§10).
func (o *Object) ExpandOccurrences(from, to time.Time, loc *time.Location) ([]Occurrence, error) {
	if o == nil || o.kind != KindICal {
		return nil, ErrNotICal
	}
	ev := o.primaryEvent()
	if ev == nil {
		return nil, nil
	}
	if loc == nil {
		loc = time.Local
	}

	start, err := ev.DateTimeStart(loc)
	if err != nil {
		return nil, fmt.Errorf("model: DTSTART: %w", err)
	}
	end, err := ev.DateTimeEnd(loc)
	if err != nil {
		return nil, fmt.Errorf("model: DTEND: %w", err)
	}
	allDay := false
	if p := ev.Props.Get(ical.PropDateTimeStart); p != nil && p.ValueType() == ical.ValueDate {
		allDay = true
	}
	dur := end.Sub(start)
	if dur < 0 {
		dur = 0
	}
	summary := icalPropText(ev.Props, ical.PropSummary)
	location := icalPropText(ev.Props, ical.PropLocation)
	uid := o.UID()

	set, err := ev.RecurrenceSet(loc)
	if err != nil {
		return nil, fmt.Errorf("model: RRULE: %w", err)
	}
	if set == nil {
		if overlaps(start, end, from, to) {
			return []Occurrence{{
				UID: uid, Path: o.Path, ETag: o.ETag,
				Summary: summary, Location: location,
				Start: start, End: end, AllDay: allDay,
			}}, nil
		}
		return nil, nil
	}

	// Between is half-open on the end; pad the start by duration so an event
	// that began before `from` but still overlaps is included.
	windowStart := from.Add(-dur)
	if windowStart.After(from) {
		windowStart = from
	}
	starts := set.Between(windowStart, to, true)
	out := make([]Occurrence, 0, len(starts))
	for _, s := range starts {
		e := s.Add(dur)
		if !overlaps(s, e, from, to) {
			continue
		}
		out = append(out, Occurrence{
			UID: uid, Path: o.Path, ETag: o.ETag,
			Summary: summary, Location: location,
			Start: s, End: e, AllDay: allDay, Recurring: true,
		})
	}
	return out, nil
}

func overlaps(start, end, from, to time.Time) bool {
	if start.IsZero() {
		return false
	}
	if end.IsZero() || !end.After(start) {
		end = start.Add(time.Minute)
	}
	return start.Before(to) && end.After(from)
}
