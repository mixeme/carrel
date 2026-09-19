package dav

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
)

// Client is a guarded DAV transport plus optional CalDAV/CardDAV reporters.
type Client struct {
	base    *url.URL
	hc      webdav.HTTPClient
	webdav  *webdav.Client
	caldav  *caldav.Client
	carddav *carddav.Client
	maxBody int64
}

// NewClient dials endpoint through guard. Username and password may be empty.
// certPin is an optional lowercase hex SHA-256 leaf fingerprint (F-13).
func NewClient(g *Guard, endpoint, username, password, certPin string) (*Client, error) {
	httpClient := g.HTTPClientWithCertPin(certPin)
	var hc webdav.HTTPClient = httpClient
	if username != "" {
		hc = webdav.HTTPClientWithBasicAuth(httpClient, username, password)
	}
	wc, err := webdav.NewClient(hc, endpoint)
	if err != nil {
		return nil, err
	}
	cc, err := caldav.NewClient(hc, endpoint)
	if err != nil {
		return nil, err
	}
	adc, err := carddav.NewClient(hc, endpoint)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return &Client{
		base:    u,
		hc:      hc,
		webdav:  wc,
		caldav:  cc,
		carddav: adc,
		maxBody: g.maxBody,
	}, nil
}

// WebDAV returns the underlying webdav client for discovery helpers.
func (c *Client) WebDAV() *webdav.Client { return c.webdav }

// CalDAV returns the underlying caldav client.
func (c *Client) CalDAV() *caldav.Client { return c.caldav }

// CardDAV returns the underlying carddav client.
func (c *Client) CardDAV() *carddav.Client { return c.carddav }

func (c *Client) resolve(p string) string {
	hasTrail := strings.HasSuffix(p, "/") || (p == "" && strings.HasSuffix(c.base.Path, "/"))
	if !strings.HasPrefix(p, "/") {
		p = path.Join(c.base.Path, p)
	}
	if hasTrail && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	u := *c.base
	u.Path = p
	return u.String()
}

func (c *Client) PropFind(ctx context.Context, p string, depth Depth, props []xml.Name) (*MultiStatus, error) {
	pf := propFindXML(props)
	req, err := c.xmlRequest("PROPFIND", p, pf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", depth.String())
	return c.doMultiStatus(req.WithContext(ctx))
}

func (c *Client) Get(ctx context.Context, p string, rng *Range) (io.ReadCloser, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.resolve(p), nil)
	if err != nil {
		return nil, "", err
	}
	setRange(req, rng)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		return nil, "", httpStatusError(resp)
	}
	etag := strings.Trim(resp.Header.Get("ETag"), `"`)
	var body io.ReadCloser = resp.Body
	if c.maxBody > 0 {
		body = &limitedBody{ReadCloser: resp.Body, remain: c.maxBody}
	}
	return body, etag, nil
}

func setRange(req *http.Request, rng *Range) {
	if rng == nil {
		return
	}
	if rng.End >= 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rng.Start, rng.End))
	} else {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", rng.Start))
	}
}

func (c *Client) Put(ctx context.Context, p string, body io.Reader, ifMatch string) (string, error) {
	req, err := c.putRequest(ctx, p, body)
	if err != nil {
		return "", err
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", httpStatusError(resp)
	}
	return strings.Trim(resp.Header.Get("ETag"), `"`), nil
}

func (c *Client) Delete(ctx context.Context, p, ifMatch string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.resolve(p), nil)
	if err != nil {
		return err
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusMultiStatus:
		// A folder some of whose members the server could not delete (RFC
		// 4918 §9.6.1): it is still there.
		return &StatusError{Code: resp.StatusCode, Msg: "some members were not deleted"}
	case resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound:
		return httpStatusError(resp)
	}
	return nil
}

// MkCol creates a plain collection. A refusal is a StatusError: a name already
// taken answers 405 (RFC 4918 §9.3.1).
func (c *Client) MkCol(ctx context.Context, p string) error {
	req, err := http.NewRequestWithContext(ctx, "MKCOL", c.resolve(p), nil)
	if err != nil {
		return err
	}
	return c.doStatus(req, "")
}

// doStatus sends a request whose answer is its status. A 207 is not a success:
// a MOVE answers it for a folder some of whose members did not move (RFC 4918
// §9.9.4).
func (c *Client) doStatus(req *http.Request, partial string) error {
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusMultiStatus:
		if partial == "" {
			partial = resp.Status
		}
		return &StatusError{Code: resp.StatusCode, Msg: partial}
	case resp.StatusCode/100 != 2:
		return httpStatusError(resp)
	}
	return nil
}

// MkColProps creates a collection with properties in one request: extended
// MKCOL (RFC 5689) or MKCALENDAR (RFC 4791). The two differ only in the method
// name and the root element of the body (S-12.1). Both are atomic on the
// server: a refused property creates nothing.
func (c *Client) MkColProps(ctx context.Context, method, p string, props []Prop) error {
	if method == "" {
		method = "MKCOL"
	}
	body, err := mkcolBody(method, props)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.resolve(p), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return httpStatusError(resp)
	}
	return nil
}

func (c *Client) PropPatch(ctx context.Context, p string, set []Prop, remove []xml.Name) error {
	body, err := propPatchBody(set, remove)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "PROPPATCH", c.resolve(p), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return httpStatusError(resp)
	}
	if resp.StatusCode == http.StatusMultiStatus {
		return c.propPatchRefusal(resp.Body)
	}
	return nil
}

// propPatchRefusal reads the 207 of a PROPPATCH. The request is atomic, and a
// server that refuses one property says so per property inside a 207: taking
// the 207 alone for success would report a rename that did not happen. The
// refused property's code wins over the 424 Failed Dependency of the others.
func (c *Client) propPatchRefusal(body io.Reader) error {
	if c.maxBody > 0 {
		body = io.LimitReader(body, c.maxBody)
	}
	var ms xmlMultiStatus
	if err := xml.NewDecoder(body).Decode(&ms); err != nil {
		return fmt.Errorf("dav: PROPPATCH answered 207 without a readable multistatus: %w", err)
	}
	var refused *StatusError
	for _, resp := range ms.Responses {
		for _, ps := range resp.PropStat {
			code := parseStatus(ps.Status)
			if code/100 == 2 {
				continue
			}
			var names []string
			for _, raw := range ps.Prop.Raw {
				names = append(names, raw.XMLName.Local)
			}
			if refused == nil || (refused.Code == http.StatusFailedDependency && code != http.StatusFailedDependency) {
				refused = &StatusError{Code: code, Msg: "refused " + strings.Join(names, ", ")}
			}
		}
	}
	if refused != nil {
		return refused
	}
	return nil
}

// Move moves or renames inside this server; no body travels. Without
// overwrite a taken destination answers 412, a StatusError like every other
// refusal.
func (c *Client) Move(ctx context.Context, src, dst string, overwrite bool) error {
	req, err := http.NewRequestWithContext(ctx, "MOVE", c.resolve(src), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Destination", c.resolve(dst))
	req.Header.Set("Overwrite", "F")
	if overwrite {
		req.Header.Set("Overwrite", "T")
	}
	return c.doStatus(req, "some members were not moved")
}

// CalReporter returns the CalDAV report surface for this client.
func (c *Client) CalReporter() CalReporter { return calReports{c} }

// CardReporter returns the CardDAV report surface for this client.
func (c *Client) CardReporter() CardReporter { return cardReports{c} }

type calReports struct{ *Client }

// MultiGetCalendar is go-webdav's, which decodes every body with go-ical; a
// panic there is an error of the answer, not the end of the process.
func (c calReports) MultiGetCalendar(ctx context.Context, p string, multiGet *caldav.CalendarMultiGet) (out []caldav.CalendarObject, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("dav: malformed calendar: %v", r)
		}
	}()
	return c.caldav.MultiGetCalendar(ctx, p, multiGet)
}

func (c calReports) SyncCollection(ctx context.Context, p string, query *caldav.SyncQuery) (*caldav.SyncResponse, error) {
	return c.caldav.SyncCollection(ctx, p, query)
}

type cardReports struct{ *Client }

func (c cardReports) QueryAddressBook(ctx context.Context, addressBook string, query *carddav.AddressBookQuery) ([]carddav.AddressObject, error) {
	return c.carddav.QueryAddressBook(ctx, addressBook, query)
}

func (c cardReports) MultiGetAddressBook(ctx context.Context, p string, multiGet *carddav.AddressBookMultiGet) ([]carddav.AddressObject, error) {
	return c.carddav.MultiGetAddressBook(ctx, p, multiGet)
}

func (c cardReports) SyncCollection(ctx context.Context, p string, query *carddav.SyncQuery) (*carddav.SyncResponse, error) {
	return c.carddav.SyncCollection(ctx, p, query)
}

func (c *Client) xmlRequest(method, p string, v any) (*http.Request, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	if err := xml.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, c.resolve(p), &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	return req, nil
}

func (c *Client) doMultiStatus(req *http.Request) (*MultiStatus, error) {
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMultiStatus {
		return nil, httpStatusError(resp)
	}
	return decodeMultiStatus(resp.Body)
}

func propFindXML(props []xml.Name) *propFind {
	raw := make([]rawXML, len(props))
	for i, n := range props {
		raw[i] = rawXML{XMLName: n}
	}
	return &propFind{Prop: propBody{Raw: raw}}
}

// xmlDoc builds a request body token by token and keeps the first error.
type xmlDoc struct {
	buf bytes.Buffer
	enc *xml.Encoder
	err error
}

func newXMLDoc() *xmlDoc {
	d := &xmlDoc{}
	d.buf.WriteString(xml.Header)
	d.enc = xml.NewEncoder(&d.buf)
	return d
}

func (d *xmlDoc) open(local string) {
	d.token(xml.StartElement{Name: xml.Name{Space: "DAV:", Local: local}})
}

func (d *xmlDoc) close(local string) {
	d.token(xml.EndElement{Name: xml.Name{Space: "DAV:", Local: local}})
}

// empty writes one element with its attributes and nothing inside it.
func (d *xmlDoc) empty(e xml.StartElement) {
	d.token(e)
	d.token(xml.EndElement{Name: e.Name})
}

func (d *xmlDoc) token(t xml.Token) {
	if d.err == nil {
		d.err = d.enc.EncodeToken(t)
	}
}

func (d *xmlDoc) text(name xml.Name, v string) {
	if d.err == nil {
		d.err = d.enc.EncodeElement(v, xml.StartElement{Name: name})
	}
}

// prop writes a property element with its value as XML content. Before the
// content goes into the buffer the encoder is flushed, so the bytes land after
// the start tag and not ahead of it.
func (d *xmlDoc) prop(p Prop) {
	if d.err == nil && !xmlContent(p.Value) {
		d.err = fmt.Errorf("dav: property %s: value is not XML content", p.Name.Local)
	}
	d.token(xml.StartElement{Name: p.Name})
	if d.err == nil {
		d.err = d.enc.Flush()
	}
	if d.err == nil {
		d.buf.Write(p.Value)
	}
	d.token(xml.EndElement{Name: p.Name})
}

// xmlContent reports whether v can stand between a start and an end tag:
// balanced elements and escaped text.
func xmlContent(v []byte) bool {
	dec := xml.NewDecoder(io.MultiReader(strings.NewReader("<v>"), bytes.NewReader(v), strings.NewReader("</v>")))
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return depth == 0
		}
		if err != nil {
			return false
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
}

// TextValue escapes s as the text content of a Prop.
func TextValue(s string) []byte {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.Bytes()
}

// bytes flushes before reading the buffer. `return buf.Bytes(), enc.Flush()`
// took the slice first and sent Radicale a truncated body (400).
func (d *xmlDoc) bytes() ([]byte, error) {
	if d.err == nil {
		d.err = d.enc.Flush()
	}
	if d.err != nil {
		return nil, d.err
	}
	return d.buf.Bytes(), nil
}

func propPatchBody(set []Prop, remove []xml.Name) ([]byte, error) {
	d := newXMLDoc()
	d.open("propertyupdate")
	for _, n := range remove {
		d.open("remove")
		d.open("prop")
		d.text(n, "")
		d.close("prop")
		d.close("remove")
	}
	if len(set) > 0 {
		d.open("set")
		d.open("prop")
		for _, p := range set {
			d.prop(p)
		}
		d.close("prop")
		d.close("set")
	}
	d.close("propertyupdate")
	return d.bytes()
}

// mkcolBody is the body of extended MKCOL, or of MKCALENDAR, whose root is
// CalDAV's mkcalendar; the rest is the same.
func mkcolBody(method string, props []Prop) ([]byte, error) {
	root := xml.Name{Space: "DAV:", Local: "mkcol"}
	if strings.EqualFold(method, "MKCALENDAR") {
		root = xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "mkcalendar"}
	}
	d := newXMLDoc()
	d.token(xml.StartElement{Name: root})
	d.open("set")
	d.open("prop")
	for _, p := range props {
		d.prop(p)
	}
	d.close("prop")
	d.close("set")
	d.token(xml.EndElement{Name: root})
	return d.bytes()
}

type propFind struct {
	XMLName xml.Name `xml:"DAV: propfind"`
	Prop    propBody `xml:"prop"`
}

type propBody struct {
	Raw []rawXML `xml:",any"`
}

type rawXML struct {
	XMLName  xml.Name
	InnerXML []byte `xml:",innerxml"`
}

type xmlMultiStatus struct {
	Responses []xmlResponse `xml:"response"`
}

type xmlResponse struct {
	Href     string        `xml:"href"`
	PropStat []xmlPropStat `xml:"propstat"`
}

type xmlPropStat struct {
	Prop   propBody `xml:"prop"`
	Status string   `xml:"status"`
}

// propFindXML asks for named properties; the reply carries their values, so
// rawXML keeps the inner XML rather than the element name alone.

func decodeMultiStatus(r io.Reader) (*MultiStatus, error) {
	var ms xmlMultiStatus
	if err := xml.NewDecoder(r).Decode(&ms); err != nil {
		return nil, err
	}
	out := &MultiStatus{Responses: make([]Response, 0, len(ms.Responses))}
	for _, resp := range ms.Responses {
		r := Response{Href: resp.Href, Props: make(map[xml.Name][]byte)}
		// A reply normally carries several propstat blocks: 200 for the
		// properties that exist, 404 for the ones that do not. Only the
		// successful block contributes values, and the resource status is
		// that block's — a trailing 404 for absent properties does not make
		// the resource missing.
		for _, ps := range resp.PropStat {
			code := parseStatus(ps.Status)
			if code/100 != 2 {
				if r.StatusCode == 0 {
					r.StatusCode = code
				}
				continue
			}
			r.StatusCode = code
			for _, raw := range ps.Prop.Raw {
				r.Props[raw.XMLName] = append([]byte(nil), raw.InnerXML...)
			}
		}
		out.Responses = append(out.Responses, r)
	}
	return out, nil
}

func parseStatus(s string) int {
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return 0
	}
	code, _ := strconv.Atoi(parts[1])
	return code
}
