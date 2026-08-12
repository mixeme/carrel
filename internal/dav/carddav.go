// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import "encoding/xml"

// MediaTypeVCard is the media type of an address object.
const MediaTypeVCard = "text/vcard; charset=utf-8"

// AddressBookMultiget is the body of an addressbook-multiget REPORT
// (RFC 6352 §8.7): the properties wanted, then one href per object.
type AddressBookMultiget struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:carddav addressbook-multiget"`
	Prop    *Prop    `xml:"DAV: prop"`
	Hrefs   []string `xml:"DAV: href"`
}

// NewAddressBookMultiget builds a multiget for the given object paths. Passing
// no property names requests the ETag and the vCard body.
func NewAddressBookMultiget(hrefs []string, props ...xml.Name) *AddressBookMultiget {
	if len(props) == 0 {
		props = []xml.Name{GetETagName, AddressDataName}
	}
	raw := make([]RawXMLValue, len(props))
	for i, name := range props {
		raw[i] = *NewRawXMLElement(name, nil, nil)
	}
	return &AddressBookMultiget{
		Prop:  &Prop{Raw: raw},
		Hrefs: append([]string(nil), hrefs...),
	}
}

// AddressBookQuery is the body of an addressbook-query REPORT (RFC 6352 §8.6).
type AddressBookQuery struct {
	XMLName xml.Name           `xml:"urn:ietf:params:xml:ns:carddav addressbook-query"`
	Prop    *Prop              `xml:"DAV: prop"`
	Filter  *AddressBookFilter `xml:"urn:ietf:params:xml:ns:carddav filter"`
}

// AddressBookFilter joins its property filters with Test, which CardDAV — unlike
// CalDAV — lets a client choose, so one request can search several fields.
type AddressBookFilter struct {
	XMLName     xml.Name          `xml:"urn:ietf:params:xml:ns:carddav filter"`
	Test        string            `xml:"test,attr,omitempty"`
	PropFilters []VCardPropFilter `xml:"urn:ietf:params:xml:ns:carddav prop-filter,omitempty"`
}

// VCardPropFilter narrows an address book by one vCard property.
type VCardPropFilter struct {
	XMLName   xml.Name        `xml:"urn:ietf:params:xml:ns:carddav prop-filter"`
	Name      string          `xml:"name,attr"`
	TextMatch *VCardTextMatch `xml:"urn:ietf:params:xml:ns:carddav text-match,omitempty"`
}

// VCardTextMatch is a substring condition on a vCard property (RFC 6352 §10.5.4).
type VCardTextMatch struct {
	XMLName   xml.Name `xml:"urn:ietf:params:xml:ns:carddav text-match"`
	Collation string   `xml:"collation,attr,omitempty"`
	MatchType string   `xml:"match-type,attr,omitempty"`
	Text      string   `xml:",chardata"`
}

// SearchedVCardProps are the properties a cross-source search looks at (§16).
var SearchedVCardProps = []string{"FN", "N", "NICKNAME", "EMAIL", "TEL", "ORG", "NOTE"}

// NewAddressBookQuery builds a search over SearchedVCardProps, or over the
// given properties, joined with "any of".
func NewAddressBookQuery(text string, properties ...string) *AddressBookQuery {
	if len(properties) == 0 {
		properties = SearchedVCardProps
	}
	filters := make([]VCardPropFilter, 0, len(properties))
	for _, name := range properties {
		filters = append(filters, VCardPropFilter{
			Name: name,
			TextMatch: &VCardTextMatch{
				Collation: CalDAVCollation,
				MatchType: "contains",
				Text:      text,
			},
		})
	}
	raw := make([]RawXMLValue, 2)
	raw[0] = *NewRawXMLElement(GetETagName, nil, nil)
	raw[1] = *NewRawXMLElement(AddressDataName, nil, nil)
	return &AddressBookQuery{
		Prop:   &Prop{Raw: raw},
		Filter: &AddressBookFilter{Test: "anyof", PropFilters: filters},
	}
}
