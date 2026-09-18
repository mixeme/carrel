package publish

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"strconv"
	"time"
)

// A published folder mounts as a read-only drive by the same secret (S-25):
// the answer to PROPFIND is written here, from the listing Carrel read, not
// relayed from the storage server — its hrefs, neighbours and properties
// never reach the reader. Only what a client needs to show a tree and fetch
// a file is said.

// DAVEntry is one resource of a multistatus.
type DAVEntry struct {
	Href     string // escaped, absolute path under /p/{token}/dav/
	Name     string
	Dir      bool
	Size     int64
	Type     string
	ETag     string
	Modified time.Time
}

// DAVMethods are what a published mount answers; everything else is 405.
const DAVMethods = "OPTIONS, GET, HEAD, PROPFIND"

// Multistatus is the PROPFIND answer for entries: every property found,
// none asked for missing — a read-only mount asks for these and no more.
func Multistatus(entries []DAVEntry) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n" + `<D:multistatus xmlns:D="DAV:">`)
	esc := func(s string) string {
		var e bytes.Buffer
		_ = xml.EscapeText(&e, []byte(s))
		return e.String()
	}
	for _, e := range entries {
		b.WriteString("<D:response><D:href>" + esc(e.Href) + "</D:href><D:propstat><D:prop>")
		b.WriteString("<D:displayname>" + esc(e.Name) + "</D:displayname>")
		if e.Dir {
			b.WriteString("<D:resourcetype><D:collection/></D:resourcetype>")
		} else {
			b.WriteString("<D:resourcetype/>")
			b.WriteString("<D:getcontentlength>" + strconv.FormatInt(e.Size, 10) + "</D:getcontentlength>")
			if e.Type != "" {
				b.WriteString("<D:getcontenttype>" + esc(e.Type) + "</D:getcontenttype>")
			}
		}
		if e.ETag != "" {
			b.WriteString(`<D:getetag>"` + esc(e.ETag) + `"</D:getetag>`)
		}
		if !e.Modified.IsZero() {
			b.WriteString("<D:getlastmodified>" + e.Modified.UTC().Format(http.TimeFormat) + "</D:getlastmodified>")
		}
		b.WriteString("</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>")
	}
	b.WriteString("</D:multistatus>\n")
	return b.Bytes()
}
