// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package calendar

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/model"
)

const testCollection = "/calendars/mix/default/"

type fakeCalendar struct {
	ctag             string
	objects          map[string]*fakeCalendarObject
	lastPut          dav.PutOptions
	failPrecondition bool

	// queries records the calendar-query reports, so a test can tell what was
	// asked of the server and how often.
	queries []*dav.CalendarQuery
	// ignoreFilter stands in for a server that answers a filter it does not
	// implement with everything it has.
	ignoreFilter bool
	// refuseQuery makes calendar-query unavailable.
	refuseQuery bool
}

type fakeCalendarObject struct {
	etag string
	body string
}

func newFakeCalendar() *fakeCalendar {
	return &fakeCalendar{ctag: "ctag-1", objects: make(map[string]*fakeCalendarObject)}
}

func (s *fakeCalendar) add(name, etag, body string) string {
	path := testCollection + name
	s.objects[path] = &fakeCalendarObject{etag: etag, body: body}
	return path
}

func (s *fakeCalendar) PropFind(_ context.Context, path string, depth dav.Depth, _ []xml.Name) (*dav.MultiStatus, error) {
	if depth == dav.DepthZero {
		if obj := s.objects[path]; obj != nil {
			return calendarMS(fmt.Sprintf(`<multistatus xmlns="DAV:"><response><href>%s</href><propstat><prop><getetag>%s</getetag></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, path, obj.etag))
		}
		return calendarMS(fmt.Sprintf(`<multistatus xmlns="DAV:" xmlns:cs="http://calendarserver.org/ns/"><response><href>%s</href><propstat><prop><cs:getctag>%s</cs:getctag></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, path, s.ctag))
	}
	var b strings.Builder
	b.WriteString(`<multistatus xmlns="DAV:">`)
	fmt.Fprintf(&b, `<response><href>%s</href><propstat><prop><resourcetype><collection/></resourcetype></prop><status>HTTP/1.1 200 OK</status></propstat></response>`, path)
	for objectPath, obj := range s.objects {
		fmt.Fprintf(&b, `<response><href>%s</href><propstat><prop><resourcetype/><getetag>%s</getetag><getcontenttype>text/calendar</getcontenttype></prop><status>HTTP/1.1 200 OK</status></propstat></response>`, objectPath, obj.etag)
	}
	b.WriteString(`</multistatus>`)
	return calendarMS(b.String())
}

func (s *fakeCalendar) Report(_ context.Context, _ string, _ dav.Depth, report any) (*dav.MultiStatus, error) {
	var paths []string
	switch body := report.(type) {
	case *dav.CalendarMultiget:
		paths = body.Hrefs
	case *dav.CalendarQuery:
		s.queries = append(s.queries, body)
		if s.refuseQuery {
			return nil, &dav.HTTPError{Code: http.StatusNotImplemented}
		}
		for path, obj := range s.objects {
			if s.ignoreFilter || matchesCalendarQuery(obj.body, body) {
				paths = append(paths, path)
			}
		}
	default:
		return nil, fmt.Errorf("unexpected report %T", report)
	}
	var b strings.Builder
	b.WriteString(`<multistatus xmlns="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">`)
	for _, path := range paths {
		obj := s.objects[path]
		if obj == nil {
			continue
		}
		fmt.Fprintf(&b, `<response><href>%s</href><propstat><prop><getetag>%s</getetag><cal:calendar-data>%s</cal:calendar-data></prop><status>HTTP/1.1 200 OK</status></propstat></response>`,
			path, obj.etag, calendarEscape(obj.body))
	}
	b.WriteString(`</multistatus>`)
	return calendarMS(b.String())
}

func (s *fakeCalendar) Get(_ context.Context, path string, _ *dav.Range) (io.ReadCloser, string, error) {
	obj := s.objects[path]
	if obj == nil {
		return nil, "", &dav.HTTPError{Code: http.StatusNotFound}
	}
	return io.NopCloser(strings.NewReader(obj.body)), dav.MediaTypeCalendar, nil
}

func (s *fakeCalendar) PutOpts(_ context.Context, path string, body io.Reader, opts dav.PutOptions) (string, error) {
	s.lastPut = opts
	if s.failPrecondition {
		s.failPrecondition = false
		return "", &dav.HTTPError{Code: http.StatusPreconditionFailed}
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	s.objects[path] = &fakeCalendarObject{etag: `"put"`, body: string(raw)}
	return `"put"`, nil
}

func (s *fakeCalendar) Delete(_ context.Context, path, etag string) error {
	obj := s.objects[path]
	if obj == nil || obj.etag != etag {
		return &dav.HTTPError{Code: http.StatusPreconditionFailed}
	}
	delete(s.objects, path)
	return nil
}

// matchesCalendarQuery applies the component and property filters of a
// calendar-query to one object. The time range is left to the provider, which
// expands recurrences the fake knows nothing about.
func matchesCalendarQuery(body string, query *dav.CalendarQuery) bool {
	if query.Filter == nil || query.Filter.CompFilter == nil || query.Filter.CompFilter.CompFilter == nil {
		return true
	}
	inner := query.Filter.CompFilter.CompFilter
	if !strings.Contains(body, "BEGIN:"+inner.Name+"\r\n") {
		return false
	}
	for _, filter := range inner.PropFilters {
		if filter.TextMatch == nil {
			continue
		}
		if !calendarPropContains(body, filter.Name, filter.TextMatch.Text) {
			return false
		}
	}
	return true
}

func calendarPropContains(body, property, text string) bool {
	want := strings.ToLower(text)
	for _, line := range strings.Split(body, "\r\n") {
		if !strings.HasPrefix(line, property+":") && !strings.HasPrefix(line, property+";") {
			continue
		}
		if strings.Contains(strings.ToLower(line), want) {
			return true
		}
	}
	return false
}

func calendarMS(body string) (*dav.MultiStatus, error) {
	return dav.ParseMultiStatus(strings.NewReader(xml.Header + body))
}

func calendarEscape(body string) string {
	var out bytes.Buffer
	if err := xml.EscapeText(&out, []byte(body)); err != nil {
		panic(err)
	}
	return out.String()
}

func eventBody(uid, summary, start, end, extra string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\n" +
		"UID:" + uid + "\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:" + summary + "\r\nDTSTART:" + start +
		"\r\nDTEND:" + end + "\r\n" + extra + "END:VEVENT\r\nEND:VCALENDAR\r\n"
}

func testProvider(t *testing.T, server *fakeCalendar) *Provider {
	t.Helper()
	p, err := New(server, Options{Location: time.UTC, Losses: model.NewLossRegistry(nil)})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestQueryReturnsEventsInRange(t *testing.T) {
	server := newFakeCalendar()
	server.add("inside.ics", `"1"`, eventBody("inside", "Inside", "20260812T100000Z", "20260812T110000Z", ""))
	server.add("outside.ics", `"2"`, eventBody("outside", "Outside", "20260813T100000Z", "20260813T110000Z", ""))
	p := testProvider(t, server)

	agenda, err := p.Query(context.Background(), testCollection,
		time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(agenda.Occurrences) != 1 || agenda.Occurrences[0].Summary != "Inside" {
		t.Fatalf("occurrences = %+v", agenda.Occurrences)
	}
}

func TestQueryExpandsWeeklyRule(t *testing.T) {
	server := newFakeCalendar()
	server.add("weekly.ics", `"1"`, eventBody("weekly", "Standup", "20260803T090000Z", "20260803T093000Z", "RRULE:FREQ=WEEKLY;COUNT=4\r\n"))
	p := testProvider(t, server)

	agenda, err := p.Query(context.Background(), testCollection,
		time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(agenda.Occurrences) != 2 {
		t.Fatalf("occurrences = %+v", agenda.Occurrences)
	}
	if got := agenda.Occurrences[0].Start.Day(); got != 10 {
		t.Errorf("first day = %d, want 10", got)
	}
	if got := agenda.Occurrences[1].Start.Day(); got != 17 {
		t.Errorf("second day = %d, want 17", got)
	}
}

func TestUpdateConflictCarriesBothVersions(t *testing.T) {
	server := newFakeCalendar()
	path := server.add("event.ics", `"1"`, eventBody("event", "Original", "20260812T100000Z", "20260812T110000Z", ""))
	p := testProvider(t, server)
	obj, err := p.Get(context.Background(), testCollection, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := obj.Apply((&model.Patch{}).SetText("SUMMARY", "Local")); err != nil {
		t.Fatal(err)
	}
	server.objects[path] = &fakeCalendarObject{etag: `"2"`, body: eventBody("event", "Remote", "20260812T100000Z", "20260812T110000Z", "")}
	server.failPrecondition = true

	_, err = p.Update(context.Background(), testCollection, obj)
	var conflict *ConflictError
	if !errorsAs(err, &conflict) {
		t.Fatalf("Update error = %v", err)
	}
	if server.lastPut.IfMatch != `"1"` || server.lastPut.ContentType != dav.MediaTypeCalendar {
		t.Errorf("PUT options = %+v", server.lastPut)
	}
	if conflict.Local == nil || conflict.Remote == nil {
		t.Fatalf("conflict = %+v", conflict)
	}
	if conflict.Local.Property("SUMMARY")[0].Text != "Local" ||
		conflict.Remote.Property("SUMMARY")[0].Text != "Remote" {
		t.Errorf("conflict versions are wrong")
	}
}

func TestUnknownICalPropertiesSurviveApplyMarshal(t *testing.T) {
	raw := eventBody("event", "Original", "20260812T100000Z", "20260812T110000Z",
		"X-CUSTOM-FLAG;VALUE=TEXT:keep-me\r\n")
	obj, err := model.ParseICal("/event.ics", `"1"`, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := obj.Apply((&model.Patch{}).SetText("SUMMARY", "Edited")); err != nil {
		t.Fatal(err)
	}
	body, err := obj.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "SUMMARY:Edited") ||
		!strings.Contains(string(body), "X-CUSTOM-FLAG;VALUE=TEXT:keep-me") {
		t.Fatalf("marshaled body lost properties:\n%s", body)
	}
}

func errorsAs(err error, target **ConflictError) bool {
	if conflict, ok := err.(*ConflictError); ok {
		*target = conflict
		return true
	}
	return false
}
