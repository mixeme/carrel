// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ColProp is one property to set when creating or patching a collection.
type ColProp struct {
	Name  xml.Name
	Value string // inner XML for complex values; plain text otherwise
}

// MkColProps creates a collection with initial properties. method is MKCOL or
// MKCALENDAR; the body shape follows RFC 5689 and RFC 4791 (§10.1).
func (c *Client) MkColProps(ctx context.Context, method, href string, props []ColProp) error {
	target, err := c.resolve(href)
	if err != nil {
		return err
	}
	body, err := buildMkColBody(method, props)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	_, err = c.do(req)
	return err
}

// PropPatch sets and removes properties on one resource (§10.1).
func (c *Client) PropPatch(ctx context.Context, href string, set []ColProp, remove []xml.Name) error {
	if len(set) == 0 && len(remove) == 0 {
		return fmt.Errorf("dav: PROPPATCH with nothing to change")
	}
	target, err := c.resolve(href)
	if err != nil {
		return err
	}
	body, err := buildPropPatchBody(set, remove)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, "PROPPATCH", target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	_, err = c.do(req)
	return err
}

func buildMkColBody(method string, props []ColProp) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	switch strings.ToUpper(method) {
	case "MKCALENDAR":
		buf.WriteString(`<C:mkcalendar xmlns:D="` + NamespaceDAV + `" xmlns:C="` + NamespaceCalDAV + `" xmlns:A="` + NamespaceApple + `">`)
		buf.WriteString(`<D:set><D:prop>`)
		writeColProps(&buf, props)
		buf.WriteString(`</D:prop></D:set></C:mkcalendar>`)
	case "MKCOL":
		buf.WriteString(`<D:mkcol xmlns:D="` + NamespaceDAV + `" xmlns:C="` + NamespaceCardDAV + `">`)
		buf.WriteString(`<D:set><D:prop>`)
		writeColProps(&buf, props)
		buf.WriteString(`</D:prop></D:set></D:mkcol>`)
	default:
		return nil, fmt.Errorf("dav: unsupported MkCol method %q", method)
	}
	return buf.Bytes(), nil
}

func buildPropPatchBody(set []ColProp, remove []xml.Name) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.WriteString(`<D:propertyupdate xmlns:D="` + NamespaceDAV + `" xmlns:A="` + NamespaceApple + `" xmlns:C="` + NamespaceCalDAV + `">`)
	if len(set) > 0 {
		buf.WriteString(`<D:set><D:prop>`)
		writeColProps(&buf, set)
		buf.WriteString(`</D:prop></D:set>`)
	}
	if len(remove) > 0 {
		buf.WriteString(`<D:remove><D:prop>`)
		for _, name := range remove {
			writeEmptyElement(&buf, name)
		}
		buf.WriteString(`</D:prop></D:remove>`)
	}
	buf.WriteString(`</D:propertyupdate>`)
	return buf.Bytes(), nil
}

func writeColProps(buf *bytes.Buffer, props []ColProp) {
	for _, p := range props {
		if p.Value != "" && strings.HasPrefix(strings.TrimSpace(p.Value), "<") {
			writeElementRaw(buf, p.Name, p.Value)
			continue
		}
		writeElementText(buf, p.Name, p.Value)
	}
}

func writeElementText(buf *bytes.Buffer, name xml.Name, text string) {
	buf.WriteByte('<')
	if name.Space != "" {
		prefix := nsPrefix(name.Space)
		if prefix != "" {
			buf.WriteString(prefix)
			buf.WriteByte(':')
		}
	}
	buf.WriteString(name.Local)
	buf.WriteByte('>')
	xml.EscapeText(buf, []byte(text))
	buf.WriteByte('<')
	buf.WriteByte('/')
	if name.Space != "" {
		prefix := nsPrefix(name.Space)
		if prefix != "" {
			buf.WriteString(prefix)
			buf.WriteByte(':')
		}
	}
	buf.WriteString(name.Local)
	buf.WriteByte('>')
}

func writeElementRaw(buf *bytes.Buffer, name xml.Name, inner string) {
	buf.WriteByte('<')
	if name.Space != "" {
		prefix := nsPrefix(name.Space)
		if prefix != "" {
			buf.WriteString(prefix)
			buf.WriteByte(':')
		}
	}
	buf.WriteString(name.Local)
	buf.WriteByte('>')
	buf.WriteString(inner)
	buf.WriteByte('<')
	buf.WriteByte('/')
	if name.Space != "" {
		prefix := nsPrefix(name.Space)
		if prefix != "" {
			buf.WriteString(prefix)
			buf.WriteByte(':')
		}
	}
	buf.WriteString(name.Local)
	buf.WriteByte('>')
}

func writeEmptyElement(buf *bytes.Buffer, name xml.Name) {
	buf.WriteByte('<')
	if name.Space != "" {
		prefix := nsPrefix(name.Space)
		if prefix != "" {
			buf.WriteString(prefix)
			buf.WriteByte(':')
		}
	}
	buf.WriteString(name.Local)
	buf.WriteString(`/>`)
}

func nsPrefix(space string) string {
	switch space {
	case NamespaceDAV:
		return "D"
	case NamespaceCalDAV:
		return "C"
	case NamespaceCardDAV:
		return "C"
	case NamespaceApple:
		return "A"
	default:
		return ""
	}
}

// RequestError captures a failed DAV write with enough detail for the UI (§10.1).
type RequestError struct {
	Method string
	Path   string
	Code   int
	Body   string
	Err    error
}

func (e *RequestError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s %s → %d", e.Method, e.Path, e.Code)
}

func (e *RequestError) Unwrap() error { return e.Err }

// WrapRequestError turns a transport failure into a RequestError when possible.
func WrapRequestError(method, path string, err error) error {
	if err == nil {
		return nil
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		msg := ""
		if httpErr.Err != nil {
			msg = httpErr.Err.Error()
		}
		return &RequestError{Method: method, Path: path, Code: httpErr.Code, Body: msg, Err: friendlyCreateErr(httpErr.Code, msg)}
	}
	return err
}

func friendlyCreateErr(code int, body string) error {
	if code != http.StatusForbidden {
		return fmt.Errorf("server returned HTTP %d", code)
	}
	lower := strings.ToLower(body)
	if strings.Contains(lower, "need-privileges") || strings.Contains(lower, "bind") {
		return fmt.Errorf("this account may sign in and read, but not create collections on the server")
	}
	return fmt.Errorf("the server refused to create the collection (HTTP 403)")
}

// ResponseBody returns the first kilobyte of an error response when err is a
// RequestError or HTTPError.
func ResponseBody(err error) string {
	var reqErr *RequestError
	if errors.As(err, &reqErr) && reqErr.Body != "" {
		return reqErr.Body
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.Err != nil {
		return httpErr.Err.Error()
	}
	return ""
}
