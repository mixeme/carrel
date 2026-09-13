package dav

import (
	"context"
	"io"
	"strings"
)

// Creator covers conditional create (S-8): PUT with If-None-Match: *.
// Transport.Put is unchanged and sends If-Match when ifMatch is non-empty.
type Creator interface {
	PutCreate(ctx context.Context, path string, body io.Reader) (string, error)
}

// PutCreate writes a new resource; the server must reject an existing one (412).
func (c *Client) PutCreate(ctx context.Context, p string, body io.Reader) (string, error) {
	req, err := c.putRequest(ctx, p, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("If-None-Match", "*")
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
