package dav

import (
	"context"
	"io"
	"net/http"
	"strings"
)

// uncappedKey marks a request whose response body is passed on whole: the
// guard's CARREL_DAV_MAX_RESPONSE_BYTES stays off it.
type uncappedKey struct{}

// Download is a file as the server sends it (S-17): the body, neither capped
// nor buffered, and what a browser needs to take it.
type Download struct {
	Body     io.ReadCloser
	Status   int    // 200, or 206 when the server answered the range
	Length   int64  // Content-Length; -1 when the server sent none
	Type     string // Content-Type as sent
	ETag     string // without quotes
	Modified string // Last-Modified as sent
	Range    string // Content-Range of a 206
}

// Streamer opens a file without the response cap (S-17). Carrel sets no
// ceiling on a file (C-12): the far server decides its size. The cap stays on
// PROPFIND, REPORT and discovery, and on Get, whose records are parsed into
// memory. The request timeout is the same as for any request.
type Streamer interface {
	Open(ctx context.Context, path string, rng *Range) (*Download, error)
}

// Open is a GET whose body the guard does not cap. The caller closes Body.
func (c *Client) Open(ctx context.Context, p string, rng *Range) (*Download, error) {
	req, err := http.NewRequestWithContext(context.WithValue(ctx, uncappedKey{}, true), http.MethodGet, c.resolve(p), nil)
	if err != nil {
		return nil, err
	}
	setRange(req, rng)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		return nil, httpStatusError(resp)
	}
	return &Download{
		Body:     resp.Body,
		Status:   resp.StatusCode,
		Length:   resp.ContentLength,
		Type:     resp.Header.Get("Content-Type"),
		ETag:     strings.Trim(resp.Header.Get("ETag"), `"`),
		Modified: resp.Header.Get("Last-Modified"),
		Range:    resp.Header.Get("Content-Range"),
	}, nil
}

func uncapped(req *http.Request) bool {
	return req.Context().Value(uncappedKey{}) != nil
}
