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

func contentTypeOf(body io.Reader) string {
	if tb, ok := body.(typedBody); ok {
		return tb.contentType
	}
	return ""
}
