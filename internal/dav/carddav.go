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
