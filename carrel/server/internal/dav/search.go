package dav

import (
	"bytes"
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
)

// UnicodeCaseMap is the collation that ignores letter case in every script
// (RFC 5051). CalDAV's default, i;ascii-casemap (RFC 4791 §9.7.5), ignores it
// in Latin only, and go-webdav cannot name another (S-15.2).
const UnicodeCaseMap = "i;unicode-casemap"

const calDAVNS = "urn:ietf:params:xml:ns:caldav"

// CalSearch is one text-match in one property of one component.
type CalSearch struct {
	Component string // VEVENT, VTODO, VJOURNAL
	Prop      string
	Text      string
	Collation string // empty: the server's default
}

// CalSearcher sends a calendar-query whose text-match names its collation. It
// sits beside CalReporter, outside Transport.
type CalSearcher interface {
	SearchCalendar(ctx context.Context, calendar string, s CalSearch) ([]caldav.CalendarObject, error)
}

// SearchCalendar is a calendar-query REPORT (RFC 4791 §7.8) of Depth 1 for the
// version and the body. A resource whose body does not parse comes back with
// Data nil, so the rest of the answer stays.
func (c calReports) SearchCalendar(ctx context.Context, calendar string, s CalSearch) ([]caldav.CalendarObject, error) {
	body, err := calSearchBody(s)
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
		if text, err := innerText(raw); err == nil {
			if data, err := ical.NewDecoder(strings.NewReader(text)).Decode(); err == nil {
				co.Data = data
			}
		}
		out = append(out, co)
	}
	return out, nil
}

func calSearchBody(s CalSearch) ([]byte, error) {
	name := func(local string) xml.Name { return xml.Name{Space: calDAVNS, Local: local} }
	named := func(local, value string) xml.StartElement {
		return xml.StartElement{Name: name(local), Attr: []xml.Attr{{Name: xml.Name{Local: "name"}, Value: value}}}
	}
	match := xml.StartElement{Name: name("text-match")}
	if s.Collation != "" {
		match.Attr = []xml.Attr{{Name: xml.Name{Local: "collation"}, Value: s.Collation}}
	}
	d := newXMLDoc()
	d.token(xml.StartElement{Name: name("calendar-query")})
	d.open("prop")
	d.text(xml.Name{Space: "DAV:", Local: "getetag"}, "")
	d.text(name("calendar-data"), "")
	d.close("prop")
	d.token(xml.StartElement{Name: name("filter")})
	d.token(named("comp-filter", "VCALENDAR"))
	d.token(named("comp-filter", s.Component))
	d.token(named("prop-filter", s.Prop))
	d.token(match)
	d.token(xml.CharData(s.Text))
	for _, local := range []string{"text-match", "prop-filter", "comp-filter", "comp-filter", "filter", "calendar-query"} {
		d.token(xml.EndElement{Name: name(local)})
	}
	return d.bytes()
}

// innerText is the text of an element's inner XML, escaped or in CDATA.
func innerText(inner []byte) (string, error) {
	doc := make([]byte, 0, len(inner)+7)
	doc = append(doc, "<v>"...)
	doc = append(doc, inner...)
	doc = append(doc, "</v>"...)
	var v struct {
		Text string `xml:",chardata"`
	}
	err := xml.Unmarshal(doc, &v)
	return v.Text, err
}
