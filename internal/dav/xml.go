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
	"strings"
)

// RawXMLValue holds one XML element with lazy decoding.
type RawXMLValue struct {
	Name    xml.Name
	Content []byte
}

// NewRawXMLElement builds a placeholder element for propfind requests.
func NewRawXMLElement(name xml.Name, attrs []xml.Attr, content []byte) *RawXMLValue {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	start := xml.StartElement{Name: name, Attr: attrs}
	if err := enc.EncodeToken(start); err != nil {
		panic(err)
	}
	if len(content) > 0 {
		if err := enc.EncodeToken(xml.CharData(content)); err != nil {
			panic(err)
		}
	}
	if err := enc.EncodeToken(start.End()); err != nil {
		panic(err)
	}
	_ = enc.Flush()
	return &RawXMLValue{Name: name, Content: buf.Bytes()}
}

func (r *RawXMLValue) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	r.Name = start.Name
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	r.Content = inner.Content
	return nil
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

// IsNotFound reports whether err is a missing-property response.
func IsNotFound(err error) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr) && httpErr.Code == http.StatusNotFound
}
