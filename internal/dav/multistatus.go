// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// MultiStatus is the XML body of a 207 response (RFC 4918 §14.16).
type MultiStatus struct {
	XMLName   xml.Name   `xml:"DAV: multistatus"`
	Responses []Response `xml:"response"`
}

// Response is one resource in a multistatus (RFC 4918 §14.24).
type Response struct {
	XMLName   xml.Name   `xml:"DAV: response"`
	Hrefs     []Href     `xml:"href"`
	PropStats []PropStat `xml:"propstat"`
	Status    *Status    `xml:"status"`
}

// PropStat groups properties that share one HTTP status.
type PropStat struct {
	Prop   Prop    `xml:"prop"`
	Status *Status `xml:"status"`
}

// Prop holds raw property elements.
type Prop struct {
	XMLName xml.Name      `xml:"DAV: prop"`
	Raw     []RawXMLValue `xml:",any"`
}

// Status is an HTTP status line embedded in XML.
type Status struct {
	Code int
	Text string
}

func (s *Status) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	parts := strings.SplitN(string(b), " ", 3)
	if len(parts) != 3 {
		return fmt.Errorf("dav: invalid HTTP status %q", b)
	}
	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("dav: invalid HTTP status code %q: %w", parts[1], err)
	}
	s.Code = code
	s.Text = parts[2]
	return nil
}

// Href is a WebDAV path reference.
type Href url.URL

func (h Href) String() string { return (*url.URL)(&h).String() }

func (h *Href) UnmarshalText(b []byte) error {
	u, err := url.Parse(string(b))
	if err != nil {
		return err
	}
	*h = Href(*u)
	return nil
}

// Path returns the href path, or the first href when several are present.
func (resp *Response) Path() (string, error) {
	if len(resp.Hrefs) == 0 {
		return "", errors.New("dav: response has no href")
	}
	return resp.Hrefs[0].Path, nil
}

// OKProp returns properties from the first successful propstat.
func (resp *Response) OKProp() (*Prop, error) {
	for _, ps := range resp.PropStats {
		if ps.Status != nil && ps.Status.Code/100 == 2 {
			return &ps.Prop, nil
		}
	}
	if resp.Status != nil && resp.Status.Code/100 == 2 {
		return &Prop{}, nil
	}
	return nil, &HTTPError{Code: http.StatusInternalServerError, Err: errors.New("no successful propstat")}
}

// DecodeProp unmarshals one property from the OK propstat.
func (resp *Response) DecodeProp(v interface{}) error {
	prop, err := resp.OKProp()
	if err != nil {
		return err
	}
	return prop.Decode(v)
}

// Get returns a raw property element by name.
func (p *Prop) Get(name xml.Name) *RawXMLValue {
	for i := range p.Raw {
		raw := &p.Raw[i]
		if n, ok := raw.elementName(); ok && n == name {
			return raw
		}
	}
	return nil
}

// Decode unmarshals one property value.
func (p *Prop) Decode(v interface{}) error {
	name, err := valueXMLName(v)
	if err != nil {
		return err
	}
	raw := p.Get(name)
	if raw == nil {
		return &HTTPError{Code: http.StatusNotFound, Err: fmt.Errorf("missing property %s", name.Local)}
	}
	return raw.Decode(v)
}

// PropFind is the PROPFIND request body (RFC 4918 §14.20).
type PropFind struct {
	XMLName xml.Name `xml:"DAV: propfind"`
	Prop    *Prop    `xml:"prop"`
}

// NewPropFind builds a propfind body requesting the named properties.
func NewPropFind(names ...xml.Name) *PropFind {
	raw := make([]RawXMLValue, len(names))
	for i, name := range names {
		raw[i] = *NewRawXMLElement(name, nil, nil)
	}
	return &PropFind{Prop: &Prop{Raw: raw}}
}

// Depth is the PROPFIND Depth header.
type Depth int

const (
	DepthZero Depth = iota
	DepthOne
	DepthInfinity
)

func (d Depth) String() string {
	switch d {
	case DepthZero:
		return "0"
	case DepthOne:
		return "1"
	default:
		return "infinity"
	}
}

// ParseMultiStatus decodes a 207 XML body.
func ParseMultiStatus(r io.Reader) (*MultiStatus, error) {
	var ms MultiStatus
	if err := xml.NewDecoder(r).Decode(&ms); err != nil {
		return nil, err
	}
	return &ms, nil
}
