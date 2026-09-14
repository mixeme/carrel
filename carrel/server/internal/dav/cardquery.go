package dav

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav/carddav"
)

// VCard4 is the vCard version a card query asks for (RFC 6352 §10.4.2). A
// server that converts cards (sabre/dav: Baikal, Nextcloud) otherwise answers
// in its default, 3.0, and what 3.0 cannot hold — KIND, among others — is gone
// before any patch; an edit then writes 3.0 back.
const VCard4 = "4.0"

const cardDAVNS = "urn:ietf:params:xml:ns:carddav"

// CardQuery is an addressbook-query: which cards, in which version.
type CardQuery struct {
	Version string      // address-data version; empty: the server's default
	AnyOf   bool        // the matches are alternatives; otherwise all must hold
	Matches []CardMatch // none: every card
}

// CardMatch is one text-match in one property.
type CardMatch struct {
	Prop   string
	Text   string
	Equals bool // the whole value; otherwise a substring
}

// CardQuerier sends an addressbook-query whose address-data names a version,
// which go-webdav has no field for. It sits beside CardReporter, outside
// Transport.
type CardQuerier interface {
	QueryCards(ctx context.Context, addressBook string, q CardQuery) ([]carddav.AddressObject, error)
}

// QueryCards is an addressbook-query REPORT (RFC 6352 §8.6) of Depth 1 for the
// version, the modification time and the whole card. A refusal is a
// StatusError: a server that does not offer the version asked for answers the
// precondition supported-address-data.
func (c cardReports) QueryCards(ctx context.Context, addressBook string, q CardQuery) ([]carddav.AddressObject, error) {
	body, err := cardQueryBody(q)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "REPORT", c.resolve(addressBook), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	req.Header.Set("Depth", "1")
	ms, err := c.doMultiStatus(req)
	if err != nil {
		return nil, err
	}
	var out []carddav.AddressObject
	for _, r := range ms.Responses {
		raw, ok := r.Props[xml.Name{Space: cardDAVNS, Local: "address-data"}]
		if r.StatusCode/100 != 2 || !ok {
			continue
		}
		ao := carddav.AddressObject{Path: r.Href}
		if u, err := url.Parse(r.Href); err == nil {
			ao.Path = u.Path
		}
		if etag, err := innerText(r.Props[xml.Name{Space: "DAV:", Local: "getetag"}]); err == nil {
			ao.ETag = etag
		}
		if mod, err := innerText(r.Props[xml.Name{Space: "DAV:", Local: "getlastmodified"}]); err == nil {
			if t, err := http.ParseTime(strings.TrimSpace(mod)); err == nil {
				ao.ModTime = t
			}
		}
		text, err := innerText(raw)
		if err != nil {
			return nil, fmt.Errorf("dav: %s: %w", ao.Path, err)
		}
		card, err := vcard.NewDecoder(strings.NewReader(text)).Decode()
		if err != nil {
			return nil, fmt.Errorf("dav: %s: %w", ao.Path, err)
		}
		ao.Card = card
		out = append(out, ao)
	}
	return out, nil
}

func cardQueryBody(q CardQuery) ([]byte, error) {
	name := func(local string) xml.Name { return xml.Name{Space: cardDAVNS, Local: local} }
	attr := func(local, value string) xml.Attr { return xml.Attr{Name: xml.Name{Local: local}, Value: value} }
	data := xml.StartElement{Name: name("address-data")}
	if q.Version != "" {
		data.Attr = []xml.Attr{attr("content-type", "text/vcard"), attr("version", q.Version)}
	}
	filter := xml.StartElement{Name: name("filter")}
	if q.AnyOf {
		filter.Attr = []xml.Attr{attr("test", "anyof")}
	}
	d := newXMLDoc()
	d.token(xml.StartElement{Name: name("addressbook-query")})
	d.open("prop")
	d.text(xml.Name{Space: "DAV:", Local: "getetag"}, "")
	d.text(xml.Name{Space: "DAV:", Local: "getlastmodified"}, "")
	d.token(data)
	d.text(name("allprop"), "")
	d.token(xml.EndElement{Name: name("address-data")})
	d.close("prop")
	d.token(filter)
	for _, m := range q.Matches {
		d.token(xml.StartElement{Name: name("prop-filter"), Attr: []xml.Attr{attr("name", m.Prop)}})
		match := xml.StartElement{Name: name("text-match")}
		if m.Equals {
			match.Attr = []xml.Attr{attr("match-type", "equals")}
		}
		d.token(match)
		d.token(xml.CharData(m.Text))
		d.token(xml.EndElement{Name: name("text-match")})
		d.token(xml.EndElement{Name: name("prop-filter")})
	}
	d.token(xml.EndElement{Name: name("filter")})
	d.token(xml.EndElement{Name: name("addressbook-query")})
	return d.bytes()
}
