package dav

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav/carddav"
)

// VCard4 is the vCard version a card query asks for (RFC 6352 §10.4.2). A
// server that converts cards (sabre/dav: Baikal, Nextcloud) otherwise answers
// in its default, 3.0, and what 3.0 cannot hold — KIND, among others — is gone
// before any patch; an edit then writes 3.0 back (docs/servers.md, SV-1).
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

// CardObject is a card as a REPORT returned it: decoded, and the text it was
// decoded from. The text is what lets a value nobody edited go back as it
// came (S-7): the decoded card has already lost groups and parameter values.
type CardObject struct {
	carddav.AddressObject
	Data string
}

// CardQuerier sends an addressbook-query whose address-data names a version,
// which go-webdav has no field for. It sits beside CardReporter, outside
// Transport.
type CardQuerier interface {
	QueryCards(ctx context.Context, addressBook string, q CardQuery) ([]CardObject, error)
}

// QueryCards is an addressbook-query REPORT (RFC 6352 §8.6) of Depth 1 for the
// version, the modification time and the whole card. A refusal is a
// StatusError: a server that does not offer the version asked for answers the
// precondition supported-address-data.
func (c cardReports) QueryCards(ctx context.Context, addressBook string, q CardQuery) ([]CardObject, error) {
	body, err := cardQueryBody(q)
	if err != nil {
		return nil, err
	}
	return c.cardReport(ctx, addressBook, body)
}

// cardReport sends a CardDAV REPORT of Depth 1 and reads the cards out of the
// multistatus it answers with.
func (c cardReports) cardReport(ctx context.Context, addressBook string, body []byte) ([]CardObject, error) {
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
	var out []CardObject
	for _, r := range ms.Responses {
		raw, ok := r.Props[xml.Name{Space: cardDAVNS, Local: "address-data"}]
		if r.StatusCode/100 != 2 || !ok {
			continue
		}
		ao := CardObject{AddressObject: carddav.AddressObject{Path: r.Href}}
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
		ao.Card, ao.Data = card, text
		out = append(out, ao)
	}
	return out, nil
}

// CardFetcher reads an address book without an addressbook-query. Some servers
// have no such report at all: Yandex answers 400 to a plain one and 403 with
// the supported-report precondition to a filtered one, while PROPFIND and
// addressbook-multiget both work (docs/servers.md, SV-23). It sits beside
// CardQuerier, outside Transport.
// A hollow query is the other reason to read a book this way: Google answers
// an addressbook-query 207 with no responses at all, filter or none, while the
// same book lists its cards to PROPFIND and hands them over to a multiget
// (docs/servers.md, SV-26). ListCards and QueryEtags are what tells the two
// apart without pulling a single card.
type CardFetcher interface {
	FetchCards(ctx context.Context, addressBook string) ([]CardObject, error)
	ListCards(ctx context.Context, addressBook string) ([]string, error)
	MultiGetCards(ctx context.Context, addressBook string, paths []string) ([]CardObject, error)
	QueryEtags(ctx context.Context, addressBook string) ([]string, error)
}

// FetchCards lists the book at depth one and fetches what it found by
// multiget. Two requests instead of one, and no filter travels with them: a
// caller that wanted a search has to sift the answer itself, the way cal.Search
// does for a server that ignores text-match (SV-16).
func (c cardReports) FetchCards(ctx context.Context, addressBook string) ([]CardObject, error) {
	paths, err := c.ListCards(ctx, addressBook)
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	return c.MultiGetCards(ctx, addressBook, paths)
}

// ListCards is the book's own listing: the address of every resource in it,
// the collection itself left out. One PROPFIND, no card bodies.
func (c cardReports) ListCards(ctx context.Context, addressBook string) ([]string, error) {
	ms, err := c.PropFind(ctx, addressBook, DepthOne, []xml.Name{{Space: "DAV:", Local: "getetag"}})
	if err != nil {
		return nil, err
	}
	self := strings.TrimSuffix(cardHrefPath(addressBook), "/")
	var paths []string
	for _, r := range ms.Responses {
		p := cardHrefPath(r.Href)
		if p == "" || strings.HasSuffix(p, "/") || strings.TrimSuffix(p, "/") == self {
			continue
		}
		paths = append(paths, p)
	}
	return paths, nil
}

// multiGetBatch is how many cards one addressbook-multiget asks for at first.
// An answer past CARREL_DAV_MAX_RESPONSE_BYTES splits its batch in two.
const multiGetBatch = 100

// MultiGetCards reads these addresses as cards, in addressbook-multigets of
// at most multiGetBatch. A batch whose answer is larger than one answer may
// be is asked again in halves, so a book of any size is read, a part at a
// time, under the same ceiling (S-5.1). A single card larger than the ceiling
// is left out and logged by path: it cannot be read without lifting the
// ceiling for everyone.
func (c cardReports) MultiGetCards(ctx context.Context, addressBook string, paths []string) ([]CardObject, error) {
	var out []CardObject
	for len(paths) > 0 {
		n := min(multiGetBatch, len(paths))
		got, err := c.multiGetCards(ctx, addressBook, paths[:n])
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
		paths = paths[n:]
	}
	return out, nil
}

func (c cardReports) multiGetCards(ctx context.Context, addressBook string, paths []string) ([]CardObject, error) {
	got, err := c.multiGetVersion(ctx, addressBook, paths, VCard4)
	if refusedCardVersion(err) {
		got, err = c.multiGetVersion(ctx, addressBook, paths, "")
	}
	if !errors.Is(err, ErrResponseTooLarge) {
		return got, err
	}
	if len(paths) == 1 {
		slog.Warn("dav: a card larger than one answer may be is left out", "path", paths[0])
		return nil, nil
	}
	half := len(paths) / 2
	a, err := c.multiGetCards(ctx, addressBook, paths[:half])
	if err != nil {
		return nil, err
	}
	b, err := c.multiGetCards(ctx, addressBook, paths[half:])
	if err != nil {
		return nil, err
	}
	return append(a, b...), nil
}

func (c cardReports) multiGetVersion(ctx context.Context, addressBook string, paths []string, version string) ([]CardObject, error) {
	body, err := cardMultiGetBody(paths, version)
	if err != nil {
		return nil, err
	}
	return c.cardReport(ctx, addressBook, body)
}

// refusedCardVersion is a refusal of the address-data version, as the card
// provider reads it: a converting server answers the multiget in 3.0
// otherwise, and an edit would write 3.0 back (SV-1).
func refusedCardVersion(err error) bool {
	for _, code := range []int{400, 403, 409, 412, 415, 422, 501} {
		if IsStatus(err, code) {
			return true
		}
	}
	return false
}

// QueryEtags asks an unfiltered addressbook-query for versions alone: what a
// server says the book holds, in one small answer and no card bodies. A server
// whose query is hollow (SV-26) answers with nothing here while the book
// itself lists cards.
func (c cardReports) QueryEtags(ctx context.Context, addressBook string) ([]string, error) {
	body, err := cardEtagQueryBody()
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
	self := strings.TrimSuffix(cardHrefPath(addressBook), "/")
	var paths []string
	for _, r := range ms.Responses {
		p := cardHrefPath(r.Href)
		if p == "" || strings.HasSuffix(p, "/") || strings.TrimSuffix(p, "/") == self {
			continue
		}
		paths = append(paths, p)
	}
	return paths, nil
}

func cardEtagQueryBody() ([]byte, error) {
	name := func(local string) xml.Name { return xml.Name{Space: cardDAVNS, Local: local} }
	d := newXMLDoc()
	d.token(xml.StartElement{Name: name("addressbook-query")})
	d.open("prop")
	d.text(xml.Name{Space: "DAV:", Local: "getetag"}, "")
	d.close("prop")
	d.empty(xml.StartElement{Name: name("filter")})
	d.token(xml.EndElement{Name: name("addressbook-query")})
	return d.bytes()
}

// cardMultiGetBody is an addressbook-multiget of these addresses asking for
// the whole card — an empty address-data, which is how RFC 6352 §10.4 says to
// ask for it. go-webdav's own multiget puts <allprop/> inside address-data
// instead, and that element is not defined there: Google answers 400
// INVALID_ARGUMENT to it (docs/servers.md, SV-26).
func cardMultiGetBody(paths []string, version string) ([]byte, error) {
	name := func(local string) xml.Name { return xml.Name{Space: cardDAVNS, Local: local} }
	data := xml.StartElement{Name: name("address-data")}
	if version != "" {
		data.Attr = []xml.Attr{{Name: xml.Name{Local: "content-type"}, Value: "text/vcard"}, {Name: xml.Name{Local: "version"}, Value: version}}
	}
	d := newXMLDoc()
	d.token(xml.StartElement{Name: name("addressbook-multiget")})
	d.open("prop")
	d.text(xml.Name{Space: "DAV:", Local: "getetag"}, "")
	d.text(xml.Name{Space: "DAV:", Local: "getlastmodified"}, "")
	d.empty(data)
	d.close("prop")
	for _, p := range paths {
		d.text(xml.Name{Space: "DAV:", Local: "href"}, p)
	}
	d.token(xml.EndElement{Name: name("addressbook-multiget")})
	return d.bytes()
}

// cardHrefPath is an href reduced to its path: servers answer with absolute
// URLs or paths, percent-encoded or not.
func cardHrefPath(href string) string {
	if u, err := url.Parse(href); err == nil {
		return u.Path
	}
	return href
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
	d.empty(data)
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
