// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"encoding/xml"
	"time"
)

// MediaTypeCalendar is the media type of a calendar object.
const MediaTypeCalendar = "text/calendar; charset=utf-8"

// CalendarData carries an iCalendar body inside a CalDAV report (RFC 4791 §9.6).
type CalendarData struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:caldav calendar-data"`
	Data    string   `xml:",chardata"`
}

// CalendarMultiget is the body of a calendar-multiget REPORT (RFC 4791 §7.9).
type CalendarMultiget struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:caldav calendar-multiget"`
	Prop    *Prop    `xml:"DAV: prop"`
	Hrefs   []string `xml:"DAV: href"`
}

// NewCalendarMultiget builds a multiget for the given object paths. Passing no
// property names requests the ETag and the calendar body.
func NewCalendarMultiget(hrefs []string, props ...xml.Name) *CalendarMultiget {
	if len(props) == 0 {
		props = []xml.Name{GetETagName, CalendarDataName}
	}
	raw := make([]RawXMLValue, len(props))
	for i, name := range props {
		raw[i] = *NewRawXMLElement(name, nil, nil)
	}
	return &CalendarMultiget{
		Prop:  &Prop{Raw: raw},
		Hrefs: append([]string(nil), hrefs...),
	}
}

// CalendarQuery is the body of a calendar-query REPORT (RFC 4791 §7.8).
type CalendarQuery struct {
	XMLName xml.Name        `xml:"urn:ietf:params:xml:ns:caldav calendar-query"`
	Prop    *Prop           `xml:"DAV: prop"`
	Filter  *CalendarFilter `xml:"urn:ietf:params:xml:ns:caldav filter"`
}

// CalendarFilter is the top-level filter element of a calendar-query.
type CalendarFilter struct {
	XMLName    xml.Name    `xml:"urn:ietf:params:xml:ns:caldav filter"`
	CompFilter *CompFilter `xml:"urn:ietf:params:xml:ns:caldav comp-filter"`
}

// CompFilter selects a component by name and optional nested filters.
type CompFilter struct {
	XMLName     xml.Name     `xml:"urn:ietf:params:xml:ns:caldav comp-filter"`
	Name        string       `xml:"name,attr"`
	CompFilter  *CompFilter  `xml:"urn:ietf:params:xml:ns:caldav comp-filter,omitempty"`
	TimeRange   *TimeRange   `xml:"urn:ietf:params:xml:ns:caldav time-range,omitempty"`
	PropFilters []PropFilter `xml:"urn:ietf:params:xml:ns:caldav prop-filter,omitempty"`
}

// PropFilter narrows a component by one of its properties (RFC 4791 §9.7.2).
type PropFilter struct {
	XMLName   xml.Name   `xml:"urn:ietf:params:xml:ns:caldav prop-filter"`
	Name      string     `xml:"name,attr"`
	TextMatch *TextMatch `xml:"urn:ietf:params:xml:ns:caldav text-match,omitempty"`
}

// CalDAVCollation is the case-insensitive collation every CalDAV and CardDAV
// server has to support (RFC 4791 §7.5.1).
const CalDAVCollation = "i;unicode-casemap"

// TextMatch is a substring condition on a property value (RFC 4791 §9.7.5).
type TextMatch struct {
	XMLName   xml.Name `xml:"urn:ietf:params:xml:ns:caldav text-match"`
	Collation string   `xml:"collation,attr,omitempty"`
	Text      string   `xml:",chardata"`
}

// TimeRange limits a calendar-query to a UTC interval (RFC 4791 §9.9).
type TimeRange struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:caldav time-range"`
	Start   string   `xml:"start,attr,omitempty"`
	End     string   `xml:"end,attr,omitempty"`
}

// Component names a calendar-query can filter on (§10).
const (
	CompEvent   = "VEVENT"
	CompTodo    = "VTODO"
	CompJournal = "VJOURNAL"
)

// NewCalendarQuery builds a VEVENT query covering [start, end). Times are
// formatted as UTC DATE-TIME values without separators beyond the RFC form.
func NewCalendarQuery(start, end time.Time, props ...xml.Name) *CalendarQuery {
	return NewCalendarComponentQuery(CompEvent, start, end, props...)
}

// NewCalendarComponentQuery builds a query for one component kind, optionally
// limited to [start, end). A zero start and end asks for the whole collection,
// which is what a journal or a task list wants: neither carries a time range
// that a server would filter on usefully.
func NewCalendarComponentQuery(component string, start, end time.Time, props ...xml.Name) *CalendarQuery {
	inner := &CompFilter{Name: component}
	if !start.IsZero() || !end.IsZero() {
		tr := &TimeRange{}
		if !start.IsZero() {
			tr.Start = formatCalDAVTime(start)
		}
		if !end.IsZero() {
			tr.End = formatCalDAVTime(end)
		}
		inner.TimeRange = tr
	}
	return &CalendarQuery{
		Prop:   &Prop{Raw: rawProps(props)},
		Filter: &CalendarFilter{CompFilter: &CompFilter{Name: "VCALENDAR", CompFilter: inner}},
	}
}

// NewCalendarTextQuery builds a query matching one property of one component
// against a substring (§16).
//
// CalDAV joins several prop-filters with AND, and there is no "any of" at this
// level, so a search over more than one property is more than one query. The
// alternative — pulling every object down and matching locally — is the cost the
// report exists to avoid.
func NewCalendarTextQuery(component, property, text string, props ...xml.Name) *CalendarQuery {
	inner := &CompFilter{
		Name: component,
		PropFilters: []PropFilter{{
			Name:      property,
			TextMatch: &TextMatch{Collation: CalDAVCollation, Text: text},
		}},
	}
	return &CalendarQuery{
		Prop:   &Prop{Raw: rawProps(props)},
		Filter: &CalendarFilter{CompFilter: &CompFilter{Name: "VCALENDAR", CompFilter: inner}},
	}
}

func rawProps(props []xml.Name) []RawXMLValue {
	if len(props) == 0 {
		props = []xml.Name{GetETagName, CalendarDataName}
	}
	raw := make([]RawXMLValue, len(props))
	for i, name := range props {
		raw[i] = *NewRawXMLElement(name, nil, nil)
	}
	return raw
}

func formatCalDAVTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}
