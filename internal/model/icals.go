// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/emersion/go-ical"
)

// ParsedCalendar is one VCALENDAR taken from an import stream.
type ParsedCalendar struct {
	Object *Object
	Source string
	Error  string
}

// ParseICals reads every calendar object from a body that may hold more than
// one VCALENDAR. A bad calendar is reported and skipped; the rest are returned.
func ParseICals(body []byte) []ParsedCalendar {
	chunks := SplitVCalendars(body)
	if len(chunks) == 0 {
		return nil
	}
	out := make([]ParsedCalendar, 0, len(chunks))
	for i, chunk := range chunks {
		obj, err := ParseICal(fmt.Sprintf("cal-%d", i+1), "", chunk)
		if err != nil {
			out = append(out, ParsedCalendar{Error: err.Error()})
			continue
		}
		// One VCALENDAR usually is the whole calendar, not one entry: this is
		// what every export looks like. Each entry becomes its own object,
		// because that is what a CalDAV server stores.
		for _, part := range obj.SplitByComponent() {
			out = append(out, ParsedCalendar{Object: part})
		}
	}
	return out
}

// SplitVCalendars cuts a body into individual BEGIN:VCALENDAR…END:VCALENDAR
// blocks.
func SplitVCalendars(body []byte) [][]byte {
	lines := splitKeepNL(body)
	var out [][]byte
	var cur [][]byte
	in := false
	for _, line := range lines {
		upper := bytes.ToUpper(bytes.TrimSpace(line))
		if bytes.HasPrefix(upper, []byte("BEGIN:VCALENDAR")) {
			if in && len(cur) > 0 {
				out = append(out, bytes.Join(cur, nil))
			}
			cur = [][]byte{line}
			in = true
			continue
		}
		if !in {
			continue
		}
		cur = append(cur, line)
		if bytes.HasPrefix(upper, []byte("END:VCALENDAR")) {
			out = append(out, bytes.Join(cur, nil))
			cur = nil
			in = false
		}
	}
	if in && len(cur) > 0 {
		out = append(out, bytes.Join(cur, nil))
	}
	return out
}

// SplitByComponent breaks one VCALENDAR into the objects a CalDAV server wants:
// one resource per UID, carrying the calendar envelope and the time zones it
// refers to. A file exported by Thunderbird, Google or Nextcloud is a single
// VCALENDAR holding every entry, and reading it as one object kept the first
// component and quietly lost the rest.
//
// Grouping is by UID rather than by component because a recurrence override
// shares the UID of its master and has to stay in the same resource; splitting
// them apart would make two entries out of one series.
func (o *Object) SplitByComponent() []*Object {
	if o == nil || o.kind != KindICal || o.cal == nil {
		return nil
	}
	var order []string
	groups := map[string][]*ical.Component{}
	zones := map[string]*ical.Component{}
	for _, child := range o.cal.Children {
		if child == nil {
			continue
		}
		switch strings.ToUpper(child.Name) {
		case ical.CompTimezone:
			zones[icalPropRaw(child.Props, ical.PropTimezoneID)] = child
		case ical.CompEvent, ical.CompToDo, ical.CompJournal:
			// A component with no UID is malformed, but dropping it would be a
			// silent loss of exactly the kind this function exists to stop; it
			// gets a group of its own instead.
			key := icalPropRaw(child.Props, ical.PropUID)
			if key == "" {
				key = fmt.Sprintf("\x00no-uid-%d", len(order))
			}
			if _, seen := groups[key]; !seen {
				order = append(order, key)
			}
			groups[key] = append(groups[key], child)
		}
	}
	if len(order) <= 1 {
		return []*Object{o}
	}
	out := make([]*Object, 0, len(order))
	for _, key := range order {
		cal := &ical.Calendar{Component: &ical.Component{
			Name:  o.cal.Name,
			Props: cloneComponent(o.cal.Component).Props,
		}}
		for _, tzid := range timezonesUsedBy(groups[key]) {
			if zone := zones[tzid]; zone != nil {
				cal.Children = append(cal.Children, cloneComponent(zone))
			}
		}
		for _, comp := range groups[key] {
			cal.Children = append(cal.Children, cloneComponent(comp))
		}
		out = append(out, &Object{kind: KindICal, cal: cal})
	}
	return out
}

// timezonesUsedBy lists the TZIDs a group of components names, so each split
// object carries the VTIMEZONE definitions it needs and not the twenty others
// an export may have collected.
func timezonesUsedBy(comps []*ical.Component) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, comp := range comps {
		if comp == nil {
			continue
		}
		for _, props := range comp.Props {
			for _, p := range props {
				add(p.Params.Get(ical.ParamTimezoneID))
			}
		}
		for _, child := range comp.Children {
			for _, id := range timezonesUsedBy([]*ical.Component{child}) {
				add(id)
			}
		}
	}
	return out
}
