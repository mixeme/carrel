package dav

import (
	"context"
	"io"
	"net/http"
)

// URL is the absolute address of p on this client's server: a leading "/"
// is from the host root, as every path of the client.
func (c *Client) URL(p string) string { return c.resolve(p) }

// Forward sends a request the davloom proxy builds (S-27.3) through the
// guard, with the connection's own credentials. The method is whatever the
// device sent — COPY among them — so this is raw HTTP beside Transport, not
// part of it (plan, S-27). A file (file=true) streams: the response cap stays
// off it and the request timeout counts silence, as S-17; everything else
// keeps the cap of CARREL_DAV_MAX_RESPONSE_BYTES. The caller closes the body.
func (c *Client) Forward(ctx context.Context, method, p string, header http.Header, body io.Reader, length int64, file bool) (*http.Response, error) {
	if file {
		ctx = context.WithValue(Stream(ctx), uncappedKey{}, true)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.resolve(p), body)
	if err != nil {
		return nil, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if body != nil && length >= 0 && req.GetBody == nil {
		req.ContentLength = length
		if length == 0 {
			req.Body = http.NoBody
		}
	}
	return c.hc.Do(req)
}
