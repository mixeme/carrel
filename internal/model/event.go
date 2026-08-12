// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// Event is the display view of a calendar object.
//
// It is deliberately one-way. The interface renders an Event; it never hands
// one back, because an Event only knows the properties this build happens to
// render and writing it out would drop the rest (§8). Edits travel as a Patch.
type Event struct {
	UID         string
	Summary     string
	Description string
	Location    string
	Status      string
	Categories  []string
	// Start/End are wall times in the location passed to Event().
	Start     time.Time
	End       time.Time
	AllDay    bool
	RRule     string
	Attendees []Attendee
	Other     []Property
}

// Attendee is a read-only PARTICIPANT view (§10 — no iTIP in v1).
type Attendee struct {
	URI      string
	CN       string
	Role     string
	PartStat string
	RSVP     string
}

var knownEventProps = map[string]bool{
	ical.PropUID:           true,
	ical.PropSummary:       true,
	ical.PropDescription:   true,
	ical.PropLocation:      true,
	ical.PropStatus:        true,
	ical.PropCategories:    true,
	ical.PropDateTimeStart: true,
	ical.PropDateTimeEnd:   true,
	ical.PropDuration:      true,
	ical.PropDateTimeStamp: true,
	ical.PropCreated:       true,
	ical.PropLastModified:  true,
	ical.PropSequence:      true,
	ical.PropRecurrenceRule: true,
	ical.PropExceptionDates: true,
	ical.PropRecurrenceDates: true,
	ical.PropAttendee:      true,
	ical.PropOrganizer:     true,
	ical.PropTransparency:  true,
	ical.PropClass:         true,
	ical.PropPriority:      true,
	ical.PropURL:           true,
}

// Event returns the display view of a calendar object.
func (o *Object) Event(loc *time.Location) (Event, error) {
	if o == nil || o.kind != KindICal {
		return Event{}, ErrNotICal
	}
	evComp := o.primaryEvent()
	if evComp == nil {
		return Event{}, ErrNotICal
	}
	if loc == nil {
		loc = time.Local
	}

	ev := Event{
		UID:         o.UID(),
		Summary:     icalPropText(evComp.Props, ical.PropSummary),
		Description: icalPropText(evComp.Props, ical.PropDescription),
		Location:    icalPropText(evComp.Props, ical.PropLocation),
		Status:      icalPropText(evComp.Props, ical.PropStatus),
		RRule:       icalPropText(evComp.Props, ical.PropRecurrenceRule),
	}
	if cats := evComp.Props.Get(ical.PropCategories); cats != nil {
		ev.Categories = splitComma(cats.Value)
	}
	if start, err := evComp.DateTimeStart(loc); err == nil {
		ev.Start = start
	}
	if end, err := evComp.DateTimeEnd(loc); err == nil {
		ev.End = end
	}
	if p := evComp.Props.Get(ical.PropDateTimeStart); p != nil && p.ValueType() == ical.ValueDate {
		ev.AllDay = true
	}
	for _, p := range evComp.Props[ical.PropAttendee] {
		ev.Attendees = append(ev.Attendees, Attendee{
			URI:      p.Value,
			CN:       p.Params.Get("CN"),
			Role:     p.Params.Get("ROLE"),
			PartStat: p.Params.Get("PARTSTAT"),
			RSVP:     p.Params.Get("RSVP"),
		})
	}
	for _, name := range o.Names() {
		if knownEventProps[name] {
			continue
		}
		ev.Other = append(ev.Other, Property{Name: name, Values: o.Property(name)})
	}
	return ev, nil
}

// DisplayTitle is SUMMARY, or a fallback when the event has none.
func (e Event) DisplayTitle() string {
	if s := strings.TrimSpace(e.Summary); s != "" {
		return s
	}
	if e.UID != "" {
		return e.UID
	}
	return "(untitled event)"
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
