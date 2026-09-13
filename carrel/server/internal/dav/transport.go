package dav

import (
	"context"
	"encoding/xml"
	"io"
)

// Depth is a PROPFIND depth (S-5).
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
	case DepthInfinity:
		return "infinity"
	default:
		return "0"
	}
}

// Range is a byte range for GET.
type Range struct {
	Start, End int64 // End < 0 means open-ended.
}

// Prop is one DAV property for PROPPATCH, MKCOL or MKCALENDAR. Value is the
// content of the property element and goes out as XML, not as text:
// `<collection xmlns="DAV:"/>` stays an element. A text value goes through
// TextValue, so a name with "&" or "<" is escaped once.
type Prop struct {
	Name  xml.Name
	Value []byte
}

// Response is one resource in a 207 Multi-Status.
type Response struct {
	Href       string
	StatusCode int
	Props      map[xml.Name][]byte
}

// MultiStatus is a parsed 207 Multi-Status body.
type MultiStatus struct {
	Responses []Response
}

// Transport is the DAV method surface (S-5). It does not know calendars.
type Transport interface {
	PropFind(ctx context.Context, path string, depth Depth, props []xml.Name) (*MultiStatus, error)
	Get(ctx context.Context, path string, rng *Range) (io.ReadCloser, string, error)
	Put(ctx context.Context, path string, body io.Reader, ifMatch string) (string, error)
	Delete(ctx context.Context, path, ifMatch string) error
	MkCol(ctx context.Context, path string) error
	MkColProps(ctx context.Context, method, path string, props []Prop) error
	PropPatch(ctx context.Context, path string, set []Prop, remove []xml.Name) error
	Move(ctx context.Context, src, dst string, overwrite bool) error
}
