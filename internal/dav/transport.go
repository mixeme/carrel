// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// Range is a byte range for GET requests.
type Range struct {
	Start int64
	End   int64 // inclusive; zero means open-ended
}

// Transport is the DAV wire protocol (spec §7).
type Transport interface {
	PropFind(ctx context.Context, path string, depth Depth, props []xml.Name) (*MultiStatus, error)
	Get(ctx context.Context, path string, rng *Range) (io.ReadCloser, string, error)
	Put(ctx context.Context, path string, body io.Reader, ifMatch string) (string, error)
	Delete(ctx context.Context, path, ifMatch string) error
	MkCol(ctx context.Context, path string) error
	Move(ctx context.Context, src, dst string, overwrite bool) error
}

// Reporter issues REPORT requests. It is kept out of Transport because §7 fixes
// that interface at the plain DAV methods; only the CardDAV and CalDAV
// providers need REPORT.
type Reporter interface {
	Report(ctx context.Context, path string, depth Depth, body any) (*MultiStatus, error)
}

// ConditionalPutter uploads a resource with a media type and a precondition.
// Transport carries neither, and CardDAV servers want both: a media type so the
// object is stored as a vCard, and a precondition so a write cannot silently
// overwrite someone else's change (§9).
type ConditionalPutter interface {
	PutOpts(ctx context.Context, path string, body io.Reader, opts PutOptions) (string, error)
}

// PutOptions are the media type and precondition of one write.
type PutOptions struct {
	ContentType string
	// IfMatch names the version being replaced. An update without it is an
	// unconditional overwrite and is refused (§9).
	IfMatch string
	// IfNoneMatch asks the server to store the object only if nothing is
	// there yet, which is how a new object is created without clobbering an
	// existing one that happens to share its UID.
	IfNoneMatch bool
}

// Client is a DAV endpoint with Basic Auth and SSRF protection.
type Client struct {
	base    *url.URL
	http    *http.Client
	guard   *Guard
	user    string
	pass    string
	maxBody int64
}

// NewClient connects to endpoint with credentials. The guard must not be nil.
func NewClient(guard *Guard, endpoint, username, password string) (*Client, error) {
	if guard == nil {
		return nil, errors.New("dav: guard is required")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("dav: invalid endpoint %q", endpoint)
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return &Client{
		base:    u,
		http:    guard.HTTPClient(),
		guard:   guard,
		user:    username,
		pass:    password,
		maxBody: guard.maxResponseBytes,
	}, nil
}

func (c *Client) resolve(href string) (*url.URL, error) {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		u, err := url.Parse(href)
		if err != nil {
			return nil, err
		}
		return u, nil
	}
	if !strings.HasPrefix(href, "/") {
		href = path.Join(c.base.Path, href)
	}
	out := *c.base
	out.Path = href
	out.RawQuery = ""
	out.Fragment = ""
	return &out, nil
}

func (c *Client) newRequest(ctx context.Context, method string, target *url.URL, body io.Reader) (*http.Request, error) {
	if err := c.guard.ValidateURL(ctx, target); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		return nil, c.httpError(resp)
	}
	return resp, nil
}

func (c *Client) httpError(resp *http.Response) error {
	body, _ := readLimited(resp.Body, 1024)
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = resp.Status
	}
	return &HTTPError{Code: resp.StatusCode, Err: fmt.Errorf("%s", msg)}
}

// PropFind issues a PROPFIND and parses the multistatus body.
func (c *Client) PropFind(ctx context.Context, href string, depth Depth, props []xml.Name) (*MultiStatus, error) {
	target, err := c.resolve(href)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	if err := xml.NewEncoder(&buf).Encode(NewPropFind(props...)); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, "PROPFIND", target, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("Depth", depth.String())

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMultiStatus {
		return nil, &HTTPError{Code: resp.StatusCode, Err: fmt.Errorf("expected 207 Multi-Status, got %s", resp.Status)}
	}
	data, err := readLimited(resp.Body, c.maxBody)
	if err != nil {
		return nil, err
	}
	return ParseMultiStatus(bytes.NewReader(data))
}

// Get downloads a resource. The caller must close the returned body.
func (c *Client) Get(ctx context.Context, href string, rng *Range) (io.ReadCloser, string, error) {
	target, err := c.resolve(href)
	if err != nil {
		return nil, "", err
	}
	req, err := c.newRequest(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", err
	}
	if rng != nil {
		if rng.End > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rng.Start, rng.End))
		} else {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", rng.Start))
		}
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, "", err
	}
	ctype := resp.Header.Get("Content-Type")
	if ctype != "" {
		if t, _, err := mime.ParseMediaType(ctype); err == nil {
			ctype = t
		}
	}
	return limitedBody(resp.Body, c.maxBody), ctype, nil
}

// Put uploads a resource and returns the new ETag when present.
func (c *Client) Put(ctx context.Context, href string, body io.Reader, ifMatch string) (string, error) {
	return c.PutOpts(ctx, href, body, PutOptions{IfMatch: ifMatch})
}

// PutOpts uploads a resource with a media type and a precondition.
func (c *Client) PutOpts(ctx context.Context, href string, body io.Reader, opts PutOptions) (string, error) {
	target, err := c.resolve(href)
	if err != nil {
		return "", err
	}
	req, err := c.newRequest(ctx, http.MethodPut, target, body)
	if err != nil {
		return "", err
	}
	if opts.ContentType != "" {
		req.Header.Set("Content-Type", opts.ContentType)
	}
	if opts.IfMatch != "" {
		req.Header.Set("If-Match", opts.IfMatch)
	}
	if opts.IfNoneMatch {
		req.Header.Set("If-None-Match", "*")
	}
	resp, err := c.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return resp.Header.Get("ETag"), nil
}

// Report issues a REPORT and parses the Multi-Status body (RFC 3253 §3.6).
func (c *Client) Report(ctx context.Context, href string, depth Depth, body any) (*MultiStatus, error) {
	target, err := c.resolve(href)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	if err := xml.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, "REPORT", target, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("Depth", depth.String())

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMultiStatus {
		return nil, &HTTPError{Code: resp.StatusCode, Err: fmt.Errorf("expected 207 Multi-Status, got %s", resp.Status)}
	}
	data, err := readLimited(resp.Body, c.maxBody)
	if err != nil {
		return nil, err
	}
	return ParseMultiStatus(bytes.NewReader(data))
}

// Delete removes a resource.
func (c *Client) Delete(ctx context.Context, href, ifMatch string) error {
	target, err := c.resolve(href)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, http.MethodDelete, target, nil)
	if err != nil {
		return err
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	_, err = c.do(req)
	return err
}

// MkCol creates a collection.
func (c *Client) MkCol(ctx context.Context, href string) error {
	target, err := c.resolve(href)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, "MKCOL", target, nil)
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}

// Move renames or moves a resource.
func (c *Client) Move(ctx context.Context, src, dst string, overwrite bool) error {
	srcURL, err := c.resolve(src)
	if err != nil {
		return err
	}
	dstURL, err := c.resolve(dst)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, "MOVE", srcURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Destination", dstURL.String())
	if !overwrite {
		req.Header.Set("Overwrite", "F")
	}
	_, err = c.do(req)
	return err
}

// Head follows redirects to a well-known path and returns the final URL.
func (c *Client) Head(ctx context.Context, href string) (*url.URL, int, error) {
	target, err := c.resolve(href)
	if err != nil {
		return nil, 0, err
	}
	req, err := c.newRequest(ctx, http.MethodHead, target, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	final, err := url.Parse(resp.Request.URL.String())
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return final, resp.StatusCode, nil
}

// BaseURL returns the configured endpoint.
func (c *Client) BaseURL() *url.URL {
	u := *c.base
	return &u
}
