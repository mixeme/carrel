package dav

import (
	"context"
	"io"
	"net/http"
)

// typedBody is a request body that knows its media type.
type typedBody struct {
	io.Reader
	contentType string
}

// sizedBody is a request body that knows its length.
type sizedBody struct {
	io.Reader
	length int64
}

// Typed labels a PUT body with its media type. Put and PutCreate send it as
// Content-Type; CalDAV and CardDAV servers refuse a record without one
// (RFC 4791 §5.3.2, RFC 6352 §6.3.2). Transport keeps its signature: the type
// travels with the body.
func Typed(r io.Reader, contentType string) io.Reader {
	return typedBody{Reader: r, contentType: contentType}
}

// Sized labels a PUT body with its length, for a reader http.NewRequest cannot
// measure: a file on disk, a stream from another server. Without it the PUT
// goes out chunked. Sized and Typed combine in either order.
func Sized(r io.Reader, length int64) io.Reader {
	return sizedBody{Reader: r, length: length}
}

// unwrap returns the reader to send, its media type and its length (-1 when
// no label says). The wrappers must not reach http.NewRequest: it learns the
// length only from a *bytes.Buffer, *bytes.Reader or *strings.Reader, and
// without it the PUT goes out chunked — Radicale answers that with 400
// (docs/servers.md, SV-5).
func unwrap(body io.Reader) (io.Reader, string, int64) {
	contentType, length := "", int64(-1)
	for {
		switch b := body.(type) {
		case typedBody:
			if contentType == "" {
				contentType = b.contentType
			}
			body = b.Reader
		case sizedBody:
			if length < 0 {
				length = b.length
			}
			body = b.Reader
		default:
			return body, contentType, length
		}
	}
}

// putRequest is a PUT of body with the type and the length its labels give. A
// length http.NewRequest measured itself stays.
func (c *Client) putRequest(ctx context.Context, p string, body io.Reader) (*http.Request, error) {
	r, contentType, length := unwrap(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.resolve(p), r)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if length >= 0 && req.GetBody == nil {
		req.ContentLength = length
		if length == 0 {
			req.Body = http.NoBody
		}
	}
	return req, nil
}
