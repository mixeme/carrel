// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func TestCalendarMultigetBody(t *testing.T) {
	body, err := xml.Marshal(NewCalendarMultiget([]string{"/cal/1.ics", "/cal/2.ics"}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(body)
	for _, want := range []string{
		`<calendar-multiget xmlns="urn:ietf:params:xml:ns:caldav">`,
		`<getetag xmlns="DAV:">`,
		`<calendar-data xmlns="urn:ietf:params:xml:ns:caldav">`,
		`<href xmlns="DAV:">/cal/1.ics</href>`,
		`<href xmlns="DAV:">/cal/2.ics</href>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("body has no %s\n%s", want, got)
		}
	}
}

func TestCalendarQueryBody(t *testing.T) {
	start := time.Date(2026, 8, 12, 10, 30, 0, 0, time.FixedZone("test", 2*60*60))
	end := start.Add(24 * time.Hour)
	body, err := xml.Marshal(NewCalendarQuery(start, end))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(body)
	for _, want := range []string{
		`<calendar-query xmlns="urn:ietf:params:xml:ns:caldav">`,
		`<getetag xmlns="DAV:">`,
		`<calendar-data xmlns="urn:ietf:params:xml:ns:caldav">`,
		`<filter xmlns="urn:ietf:params:xml:ns:caldav">`,
		`<comp-filter xmlns="urn:ietf:params:xml:ns:caldav" name="VCALENDAR">`,
		`<comp-filter xmlns="urn:ietf:params:xml:ns:caldav" name="VEVENT">`,
		`<time-range xmlns="urn:ietf:params:xml:ns:caldav" start="20260812T083000Z" end="20260813T083000Z">`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("body has no %s\n%s", want, got)
		}
	}
}
