// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// RawXMLValue holds one XML element with lazy decoding. Content is the inner
// XML of the element, without the element itself.
type RawXMLValue struct {
	Name    xml.Name
	Attrs   []xml.Attr
	Content []byte
}

// NewRawXMLElement builds a placeholder element for propfind and report bodies.
func NewRawXMLElement(name xml.Name, attrs []xml.Attr, content []byte) *RawXMLValue {
	return &RawXMLValue{Name: name, Attrs: attrs, Content: content}
}

func (r *RawXMLValue) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	r.Name = start.Name
	r.Attrs = append([]xml.Attr(nil), start.Attr...)
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	r.Content = inner.Content
	return nil
}

// MarshalXML writes the element under its own name, ignoring the name the
// encoder derived from the surrounding struct field. Without this a requested
// property is serialised as the Go field name and the server is asked for
// nothing it recognises.
func (r RawXMLValue) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	if r.Name.Local == "" {
		return fmt.Errorf("dav: raw XML value has no element name")
	}
	inner := struct {
		Content []byte `xml:",innerxml"`
	}{Content: r.Content}
	return e.EncodeElement(inner, xml.StartElement{Name: r.Name, Attr: r.Attrs})
}

func (r RawXMLValue) elementName() (xml.Name, bool) {
	if r.Name.Local != "" {
		return r.Name, true
	}
	return xml.Name{}, false
}

func (r RawXMLValue) Decode(v interface{}) error {
	if r.Name.Local == "" {
		return io.EOF
	}
	var buf bytes.Buffer
	buf.WriteByte('<')
	buf.WriteString(r.Name.Local)
	if r.Name.Space != "" {
		fmt.Fprintf(&buf, ` xmlns="%s"`, r.Name.Space)
	}
	buf.WriteByte('>')
	buf.Write(r.Content)
	buf.WriteByte('<')
	buf.WriteByte('/')
	buf.WriteString(r.Name.Local)
	buf.WriteByte('>')
	return xml.Unmarshal(buf.Bytes(), v)
}

func valueXMLName(v interface{}) (xml.Name, error) {
	t := reflect.TypeOf(v)
	if t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Struct {
		return xml.Name{}, fmt.Errorf("dav: Decode target must be a pointer to a struct, got %T", v)
	}
	field, ok := t.Elem().FieldByName("XMLName")
	if !ok {
		return xml.Name{}, fmt.Errorf("dav: %T has no XMLName field", v)
	}
	tag := field.Tag.Get("xml")
	if tag == "" {
		return xml.Name{}, fmt.Errorf("dav: XMLName field of %T has no xml tag", v)
	}
	parts := strings.SplitN(tag, " ", 2)
	if len(parts) == 2 {
		return xml.Name{Space: parts[0], Local: parts[1]}, nil
	}
	return xml.Name{Local: parts[0]}, nil
}

// ResourceType describes what a collection contains.
type ResourceType struct {
	XMLName xml.Name      `xml:"DAV: resourcetype"`
	Raw     []RawXMLValue `xml:",any"`
}

func (t *ResourceType) Is(name xml.Name) bool {
	for _, raw := range t.Raw {
		if n, ok := raw.elementName(); ok {
			if n == name || n.Local == name.Local {
				return true
			}
		}
	}
	return false
}

// DisplayName is the DAV displayname property.
type DisplayName struct {
	XMLName xml.Name `xml:"DAV: displayname"`
	Name    string   `xml:",chardata"`
}

// CurrentUserPrincipal points at the authenticated principal.
type CurrentUserPrincipal struct {
	XMLName xml.Name `xml:"DAV: current-user-principal"`
	Href    Href     `xml:"href"`
}

// HomeSet is a calendar-home-set or addressbook-home-set href.
type HomeSet struct {
	XMLName xml.Name `xml:""`
	Href    Href     `xml:"href"`
}

type CalendarHomeSet struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:caldav calendar-home-set"`
	Href    Href     `xml:"href"`
}

type AddressBookHomeSet struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:carddav addressbook-home-set"`
	Href    Href     `xml:"href"`
}

// CalendarColor is the Apple calendar-color property.
type CalendarColor struct {
	XMLName xml.Name `xml:"http://apple.com/ns/ical/ calendar-color"`
	Color   string   `xml:",chardata"`
}

// SupportedCalendarComponentSet lists VEVENT/VTODO/VJOURNAL support.
type SupportedCalendarComponentSet struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:caldav supported-calendar-component-set"`
	Comp    []struct {
		Name string `xml:"name,attr"`
	} `xml:"comp"`
}

// CurrentUserPrivilegeSet lists DAV privileges for the current user.
type CurrentUserPrivilegeSet struct {
	XMLName    xml.Name `xml:"DAV: current-user-privilege-set"`
	Privileges []struct {
		Write      *struct{} `xml:"write"`
		WriteProps *struct{} `xml:"write-properties"`
		WriteCont  *struct{} `xml:"write-content"`
		Bind       *struct{} `xml:"bind"`
		Unbind     *struct{} `xml:"unbind"`
		Read       *struct{} `xml:"read"`
		ReadACL    *struct{} `xml:"read-acl"`
		ReadCurr   *struct{} `xml:"read-current-user-privilege-set"`
	} `xml:"privilege"`
}

// GetCTag is the CalendarServer collection tag.
type GetCTag struct {
	XMLName xml.Name `xml:"http://calendarserver.org/ns/ getctag"`
	Tag     string   `xml:",chardata"`
}

// GetETag is the DAV entity tag of a resource.
type GetETag struct {
	XMLName xml.Name `xml:"DAV: getetag"`
	ETag    string   `xml:",chardata"`
}

// GetContentType is the media type of a resource.
type GetContentType struct {
	XMLName xml.Name `xml:"DAV: getcontenttype"`
	Type    string   `xml:",chardata"`
}

// GetContentLength is the octet count of a resource. Servers report it as
// chardata rather than an attribute, and a collection usually omits it
// altogether, which is one of the ways a directory is told from a file.
type GetContentLength struct {
	XMLName xml.Name `xml:"DAV: getcontentlength"`
	Length  string   `xml:",chardata"`
}

// Bytes parses the length, reporting whether the server gave a usable one.
func (l GetContentLength) Bytes() (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(l.Length), 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// GetLastModified is the modification time of a resource, in the HTTP date
// format of RFC 7231.
type GetLastModified struct {
	XMLName xml.Name `xml:"DAV: getlastmodified"`
	At      string   `xml:",chardata"`
}

// Time parses the modification time. Servers are inconsistent enough about the
// format that a failure is reported rather than guessed at.
func (m GetLastModified) Time() (time.Time, bool) {
	raw := strings.TrimSpace(m.At)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{http.TimeFormat, time.RFC1123, time.RFC1123Z, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// AddressData carries a vCard body inside a CardDAV report (RFC 6352 §10.4).
type AddressData struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:carddav address-data"`
	Data    string   `xml:",chardata"`
}

// IsNotFound reports whether err is a missing-property response.
func IsNotFound(err error) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr) && httpErr.Code == http.StatusNotFound
}
