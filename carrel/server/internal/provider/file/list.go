package file

import (
	"bytes"
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
)

// Entry is one member of a folder, as a depth-1 PROPFIND reports it.
type Entry struct {
	Path     string // server path, decoded; a folder's ends in "/"
	Name     string
	Dir      bool
	Size     int64
	Type     string // getcontenttype as the server sent it; empty for a folder
	ETag     string
	Modified time.Time
}

var (
	propResourceType  = xml.Name{Space: "DAV:", Local: "resourcetype"}
	propContentLength = xml.Name{Space: "DAV:", Local: "getcontentlength"}
	propContentType   = xml.Name{Space: "DAV:", Local: "getcontenttype"}
	propETag          = xml.Name{Space: "DAV:", Local: "getetag"}
	propLastModified  = xml.Name{Space: "DAV:", Local: "getlastmodified"}

	entryProps = []xml.Name{propResourceType, propContentLength, propContentType, propETag, propLastModified}
)

// List reads one folder with PROPFIND depth 1 (S-17): names, sizes, types and
// versions, no bodies. The folder itself is not among the entries, and neither
// is anything the server names outside it.
func (p *Provider) List(ctx context.Context, folder string) ([]Entry, error) {
	ms, err := p.t.PropFind(ctx, folder, dav.DepthOne, entryProps)
	if err != nil {
		return nil, err
	}
	self := strings.TrimSuffix(hrefPath(folder), "/")
	out := make([]Entry, 0, len(ms.Responses))
	for _, r := range ms.Responses {
		if r.StatusCode != 0 && r.StatusCode/100 != 2 {
			continue
		}
		e, ok := entryOf(r)
		if !ok || strings.TrimSuffix(e.Path, "/") == self || !strings.HasPrefix(e.Path, self+"/") ||
			strings.Contains(strings.TrimSuffix(strings.TrimPrefix(e.Path, self+"/"), "/"), "/") {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// Stat reads one file or folder with PROPFIND depth 0. Nothing at the path is
// a 404, whether the server said so by status or by an empty answer.
func (p *Provider) Stat(ctx context.Context, path string) (Entry, error) {
	ms, err := p.t.PropFind(ctx, path, dav.DepthZero, entryProps)
	if err != nil {
		return Entry{}, err
	}
	for _, r := range ms.Responses {
		if r.StatusCode != 0 && r.StatusCode/100 != 2 {
			continue
		}
		if e, ok := entryOf(r); ok {
			return e, nil
		}
	}
	return Entry{}, &dav.StatusError{Code: http.StatusNotFound, Msg: "nothing at " + path}
}

// NotFound reports whether err says nothing is at the path.
func NotFound(err error) bool {
	return dav.IsStatus(err, http.StatusNotFound)
}

func entryOf(r dav.Response) (Entry, bool) {
	bare := strings.TrimSuffix(hrefPath(r.Href), "/")
	if bare == "" {
		return Entry{}, false
	}
	e := Entry{
		Path: bare,
		Name: bare[strings.LastIndex(bare, "/")+1:],
		Dir:  isCollection(r.Props[propResourceType]),
		ETag: strings.Trim(propText(r.Props[propETag]), `"`),
	}
	if e.Dir {
		e.Path += "/"
	} else {
		e.Type = propText(r.Props[propContentType])
	}
	if n, err := strconv.ParseInt(propText(r.Props[propContentLength]), 10, 64); err == nil {
		e.Size = n
	}
	if t, err := http.ParseTime(propText(r.Props[propLastModified])); err == nil {
		e.Modified = t
	}
	return e, true
}

func hrefPath(href string) string {
	if u, err := url.Parse(href); err == nil {
		return u.Path
	}
	return href
}

// isCollection looks for DAV:collection in a resourcetype. The inner XML keeps
// the reply's prefixes, whose declarations sit on outer elements, so the
// element is matched by its local name.
func isCollection(inner []byte) bool {
	d := xml.NewDecoder(bytes.NewReader(inner))
	for {
		tok, err := d.Token()
		if err != nil {
			return false
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "collection" {
			return true
		}
	}
}

// propText is the text of a property: its character data, unescaped.
func propText(inner []byte) string {
	d := xml.NewDecoder(bytes.NewReader(inner))
	var b strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			return strings.TrimSpace(b.String())
		}
		if cd, ok := tok.(xml.CharData); ok {
			b.Write(cd)
		}
	}
}
