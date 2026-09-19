package dav

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
)

// QueryCalendar is a calendar-query REPORT (RFC 4791 §7.8) of Depth 1 for the
// version, the modification time and the whole body. go-webdav's own client
// decodes every body of the answer and fails the whole answer on the first one
// it cannot read — and go-ical panics on some malformed lines, which in a
// fan-out goroutine is the end of the process. Here a body that does not
// decode comes back with Data nil, one resource of the answer and not the
// answer: the collection lists the rest and names the one it skipped (S-21).
func (c calReports) QueryCalendar(ctx context.Context, calendar string, q *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	body, err := calQueryBody(q)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "REPORT", c.resolve(calendar), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	req.Header.Set("Depth", "1")
	ms, err := c.doMultiStatus(req)
	if err != nil {
		return nil, err
	}
	return calendarObjects(ms), nil
}

// calendarObjects reads the resources of a calendar REPORT answer.
func calendarObjects(ms *MultiStatus) []caldav.CalendarObject {
	var out []caldav.CalendarObject
	for _, r := range ms.Responses {
		raw, ok := r.Props[xml.Name{Space: calDAVNS, Local: "calendar-data"}]
		if r.StatusCode/100 != 2 || !ok {
			continue
		}
		co := caldav.CalendarObject{Path: r.Href}
		if u, err := url.Parse(r.Href); err == nil {
			co.Path = u.Path
		}
		if etag, err := innerText(r.Props[xml.Name{Space: "DAV:", Local: "getetag"}]); err == nil {
			co.ETag = etag
		}
		if mod, err := innerText(r.Props[xml.Name{Space: "DAV:", Local: "getlastmodified"}]); err == nil {
			if t, err := http.ParseTime(strings.TrimSpace(mod)); err == nil {
				co.ModTime = t
			}
		}
		if text, err := innerText(raw); err == nil {
			co.Data, _ = DecodeCalendar(text)
		}
		out = append(out, co)
	}
	return out
}

// DecodeCalendar decodes one iCalendar document and never panics: go-ical does
// on a parameter with no colon after it and on text after a closing quote, and
// the document is a far server's.
func DecodeCalendar(text string) (cal *ical.Calendar, err error) {
	defer func() {
		if r := recover(); r != nil {
			cal, err = nil, fmt.Errorf("dav: malformed calendar: %v", r)
		}
	}()
	return ical.NewDecoder(strings.NewReader(text)).Decode()
}

// calQueryBody writes the calendar-query for a filter. A param-filter is the
// one part of RFC 4791 §9.7 nothing in Carrel asks for, and it is refused
// rather than dropped: a filter silently widened answers another question.
func calQueryBody(q *caldav.CalendarQuery) ([]byte, error) {
	d := newXMLDoc()
	d.token(xml.StartElement{Name: calName("calendar-query")})
	d.open("prop")
	d.text(xml.Name{Space: "DAV:", Local: "getetag"}, "")
	d.text(xml.Name{Space: "DAV:", Local: "getlastmodified"}, "")
	d.text(calName("calendar-data"), "")
	d.close("prop")
	d.token(xml.StartElement{Name: calName("filter")})
	if err := compFilter(d, q.CompFilter); err != nil {
		return nil, err
	}
	d.token(xml.EndElement{Name: calName("filter")})
	d.token(xml.EndElement{Name: calName("calendar-query")})
	return d.bytes()
}

func calName(local string) xml.Name { return xml.Name{Space: calDAVNS, Local: local} }

func namedStart(local, name string) xml.StartElement {
	return xml.StartElement{Name: calName(local), Attr: []xml.Attr{{Name: xml.Name{Local: "name"}, Value: name}}}
}

func compFilter(d *xmlDoc, f caldav.CompFilter) error {
	d.token(namedStart("comp-filter", f.Name))
	if f.IsNotDefined {
		d.empty(xml.StartElement{Name: calName("is-not-defined")})
	}
	timeRange(d, f.Start, f.End)
	for _, p := range f.Props {
		if len(p.ParamFilter) > 0 {
			return fmt.Errorf("dav: param-filter is not supported")
		}
		d.token(namedStart("prop-filter", p.Name))
		if p.IsNotDefined {
			d.empty(xml.StartElement{Name: calName("is-not-defined")})
		}
		timeRange(d, p.Start, p.End)
		if p.TextMatch != nil {
			m := xml.StartElement{Name: calName("text-match")}
			if p.TextMatch.NegateCondition {
				m.Attr = []xml.Attr{{Name: xml.Name{Local: "negate-condition"}, Value: "yes"}}
			}
			d.token(m)
			d.token(xml.CharData(p.TextMatch.Text))
			d.token(xml.EndElement{Name: m.Name})
		}
		d.token(xml.EndElement{Name: calName("prop-filter")})
	}
	for _, c := range f.Comps {
		if err := compFilter(d, c); err != nil {
			return err
		}
	}
	d.token(xml.EndElement{Name: calName("comp-filter")})
	return nil
}

// timeRange writes the bounds that are set; RFC 4791 §9.9 lets either stand
// alone.
func timeRange(d *xmlDoc, start, end time.Time) {
	if start.IsZero() && end.IsZero() {
		return
	}
	e := xml.StartElement{Name: calName("time-range")}
	if !start.IsZero() {
		e.Attr = append(e.Attr, xml.Attr{Name: xml.Name{Local: "start"}, Value: start.UTC().Format("20060102T150405Z")})
	}
	if !end.IsZero() {
		e.Attr = append(e.Attr, xml.Attr{Name: xml.Name{Local: "end"}, Value: end.UTC().Format("20060102T150405Z")})
	}
	d.empty(e)
}

// CalFetcher reads a calendar without a calendar-query: its own listing, then
// calendar-multiget in parts. It is the way to a calendar whose whole answer
// is larger than one answer may be (CARREL_DAV_MAX_RESPONSE_BYTES); both
// requests are required of every CalDAV server (RFC 4791 §7.9). It sits
// beside CalReporter, outside Transport.
type CalFetcher interface {
	FetchCalendar(ctx context.Context, calendar string) ([]caldav.CalendarObject, error)
}

// FetchCalendar lists the calendar at depth one and reads what it holds by
// calendar-multiget, at most multiGetBatch a request. A batch whose answer is
// too large is asked again in halves; a single object larger than the ceiling
// comes back without data, as an object that could not be read.
func (c calReports) FetchCalendar(ctx context.Context, calendar string) ([]caldav.CalendarObject, error) {
	ms, err := c.PropFind(ctx, calendar, DepthOne, []xml.Name{{Space: "DAV:", Local: "getetag"}})
	if err != nil {
		return nil, err
	}
	self := strings.TrimSuffix(cardHrefPath(calendar), "/")
	var paths []string
	for _, r := range ms.Responses {
		p := cardHrefPath(r.Href)
		if p == "" || strings.HasSuffix(p, "/") || strings.TrimSuffix(p, "/") == self {
			continue
		}
		paths = append(paths, p)
	}
	var out []caldav.CalendarObject
	for len(paths) > 0 {
		n := min(multiGetBatch, len(paths))
		got, err := c.multiGetCalendar(ctx, calendar, paths[:n])
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
		paths = paths[n:]
	}
	return out, nil
}

func (c calReports) multiGetCalendar(ctx context.Context, calendar string, paths []string) ([]caldav.CalendarObject, error) {
	d := newXMLDoc()
	d.token(xml.StartElement{Name: calName("calendar-multiget")})
	d.open("prop")
	d.text(xml.Name{Space: "DAV:", Local: "getetag"}, "")
	d.text(xml.Name{Space: "DAV:", Local: "getlastmodified"}, "")
	d.text(calName("calendar-data"), "")
	d.close("prop")
	for _, p := range paths {
		d.text(xml.Name{Space: "DAV:", Local: "href"}, p)
	}
	d.token(xml.EndElement{Name: calName("calendar-multiget")})
	body, err := d.bytes()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "REPORT", c.resolve(calendar), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	req.Header.Set("Depth", "1")
	ms, err := c.doMultiStatus(req)
	if errors.Is(err, ErrResponseTooLarge) {
		if len(paths) == 1 {
			return []caldav.CalendarObject{{Path: paths[0]}}, nil
		}
		half := len(paths) / 2
		a, err := c.multiGetCalendar(ctx, calendar, paths[:half])
		if err != nil {
			return nil, err
		}
		b, err := c.multiGetCalendar(ctx, calendar, paths[half:])
		if err != nil {
			return nil, err
		}
		return append(a, b...), nil
	}
	if err != nil {
		return nil, err
	}
	return calendarObjects(ms), nil
}
