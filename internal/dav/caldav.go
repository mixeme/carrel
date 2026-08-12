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
	XMLName    xml.Name    `xml:"urn:ietf:params:xml:ns:caldav comp-filter"`
	Name       string      `xml:"name,attr"`
	CompFilter *CompFilter `xml:"urn:ietf:params:xml:ns:caldav comp-filter,omitempty"`
	TimeRange  *TimeRange  `xml:"urn:ietf:params:xml:ns:caldav time-range,omitempty"`
}

// TimeRange limits a calendar-query to a UTC interval (RFC 4791 §9.9).
type TimeRange struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:caldav time-range"`
	Start   string   `xml:"start,attr,omitempty"`
	End     string   `xml:"end,attr,omitempty"`
}

// NewCalendarQuery builds a VEVENT query covering [start, end). Times are
// formatted as UTC DATE-TIME values without separators beyond the RFC form.
func NewCalendarQuery(start, end time.Time, props ...xml.Name) *CalendarQuery {
	if len(props) == 0 {
		props = []xml.Name{GetETagName, CalendarDataName}
	}
	raw := make([]RawXMLValue, len(props))
	for i, name := range props {
		raw[i] = *NewRawXMLElement(name, nil, nil)
	}
	tr := &TimeRange{}
	if !start.IsZero() {
		tr.Start = formatCalDAVTime(start)
	}
	if !end.IsZero() {
		tr.End = formatCalDAVTime(end)
	}
	return &CalendarQuery{
		Prop: &Prop{Raw: raw},
		Filter: &CalendarFilter{
			CompFilter: &CompFilter{
				Name: "VCALENDAR",
				CompFilter: &CompFilter{
					Name:      "VEVENT",
					TimeRange: tr,
				},
			},
		},
	}
}

func formatCalDAVTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}
