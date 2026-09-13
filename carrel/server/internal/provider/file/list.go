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
	ETag     string
	Modified time.Time
}

var (
	propResourceType  = xml.Name{Space: "DAV:", Local: "resourcetype"}
	propContentLength = xml.Name{Space: "DAV:", Local: "getcontentlength"}
	propETag          = xml.Name{Space: "DAV:", Local: "getetag"}
	propLastModified  = xml.Name{Space: "DAV:", Local: "getlastmodified"}
)

// List reads one folder with PROPFIND depth 1 (S-17): names, sizes and
// versions, no bodies. The folder itself is not among the entries.
func (p *Provider) List(ctx context.Context, folder string) ([]Entry, error) {
	ms, err := p.t.PropFind(ctx, folder, dav.DepthOne,
		[]xml.Name{propResourceType, propContentLength, propETag, propLastModified})
	if err != nil {
		return nil, err
	}
	self := strings.TrimSuffix(hrefPath(folder), "/")
	out := make([]Entry, 0, len(ms.Responses))
	for _, r := range ms.Responses {
		if r.StatusCode != 0 && r.StatusCode/100 != 2 {
			continue
		}
		bare := strings.TrimSuffix(hrefPath(r.Href), "/")
		if bare == self || bare == "" {
			continue
		}
		e := Entry{
			Path: bare,
			Name: bare[strings.LastIndex(bare, "/")+1:],
			Dir:  isCollection(r.Props[propResourceType]),
			ETag: strings.Trim(propText(r.Props[propETag]), `"`),
		}
		if e.Dir {
			e.Path += "/"
		}
		if n, err := strconv.ParseInt(propText(r.Props[propContentLength]), 10, 64); err == nil {
			e.Size = n
		}
		if t, err := http.ParseTime(propText(r.Props[propLastModified])); err == nil {
			e.Modified = t
		}
		out = append(out, e)
	}
	return out, nil
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
