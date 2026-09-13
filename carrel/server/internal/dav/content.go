package dav

import "io"

// typedBody is a request body that knows its media type.
type typedBody struct {
	io.Reader
	contentType string
}

// Typed labels a PUT body with its media type. Put and PutCreate send it as
// Content-Type; CalDAV and CardDAV servers refuse a record without one
// (RFC 4791 §5.3.2, RFC 6352 §6.3.2). Transport keeps its signature: the type
// travels with the body.
func Typed(r io.Reader, contentType string) io.Reader {
	return typedBody{Reader: r, contentType: contentType}
}

// unwrap returns the reader to send and its media type. The wrapper must not
// reach http.NewRequest: it learns the length only from a *bytes.Buffer,
// *bytes.Reader or *strings.Reader, and without it the PUT goes out chunked —
// Radicale answers that with 400.
func unwrap(body io.Reader) (io.Reader, string) {
	if tb, ok := body.(typedBody); ok {
		return tb.Reader, tb.contentType
	}
	return body, ""
}
