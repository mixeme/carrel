// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestClientCalendarQueryReportDepth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Errorf("method = %s, want REPORT", r.Method)
		}
		if r.Header.Get("Depth") != "1" {
			t.Errorf("depth = %q, want 1 (RFC 4791 §7.8)", r.Header.Get("Depth"))
		}
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, `<multistatus xmlns="DAV:"/>`)
	}))
	defer srv.Close()

	client, err := NewClient(testGuard(), srv.URL, "mix", "secret")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	start := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	_, err = client.Report(context.Background(), "/cal/", DepthOne, NewCalendarQuery(start, start.Add(24*time.Hour)))
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
}
