// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/emersion/go-ical"
)

// Attachment is one `ATTACH` property (§23.10).
//
// Carrel writes only the URI form: the file goes on the person's WebDAV and the
// object carries a reference to it, so a screenshot in a note does not turn into
// half a megabyte of base64 that every client then syncs. `ATTACH` with a URI is
// plain RFC 5545, which is the point — a link expressible only through an
// invention of ours would be readable by Carrel alone.
//
// An attachment another client wrote inline is shown as it is and never
// rewritten into a link (§23.10). Value keeps the property exactly as it
// arrived, so an edit that touches the set of attachments puts the others back
// byte for byte.
type Attachment struct {
	// URI is the reference. Empty when the attachment is inline data.
	URI string
	// Inline marks an ATTACH carrying base64 rather than a reference.
	Inline bool
	// FmtType is FMTTYPE, the media type the writer claimed.
	FmtType string
	// Filename is FILENAME (RFC 8607) or the X-FILENAME some clients write,
	// falling back to the last segment of the URI.
	Filename string
	// Size is the SIZE parameter. It is what the writing client said, not
	// something Carrel went and measured: checking would cost one request per
	// attachment on every page that lists them.
	Size    int64
	HasSize bool
	// Value is the property as it arrived, parameters included.
	Value Value
}

// Attachment parameter names. FILENAME and SIZE are RFC 8607; X-FILENAME is
// what several clients wrote before it existed and is still what some send.
const (
	ParamFmtType   = "FMTTYPE"
	ParamFilename  = "FILENAME"
	ParamXFilename = "X-FILENAME"
	ParamSize      = "SIZE"
	ParamValue     = "VALUE"
	ParamEncoding  = "ENCODING"
)

// Attachments returns every ATTACH on the object's primary component.
func (o *Object) Attachments() []Attachment {
	if o == nil || o.kind != KindICal {
		return nil
	}
	comp := o.primaryComponent()
	if comp == nil {
		return nil
	}
	return attachmentsFrom(comp.Props)
}

func attachmentsFrom(props ical.Props) []Attachment {
	values := props[ical.PropAttach]
	if len(values) == 0 {
		return nil
	}
	out := make([]Attachment, 0, len(values))
	for _, p := range values {
		out = append(out, attachmentFrom(valueFromProp(p)))
	}
	return out
}

func attachmentFrom(v Value) Attachment {
	att := Attachment{
		Value:   v,
		FmtType: strings.TrimSpace(v.Param(ParamFmtType)),
	}
	// Inline is either VALUE=BINARY or ENCODING=BASE64; clients write one, the
	// other, or both, and any of the three means the same thing.
	att.Inline = v.HasParamValue(ParamValue, "BINARY") || v.HasParamValue(ParamEncoding, "BASE64")
	if !att.Inline {
		att.URI = strings.TrimSpace(v.Text)
	}
	if raw := strings.TrimSpace(v.Param(ParamSize)); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil && n >= 0 {
			att.Size, att.HasSize = n, true
		}
	}
	att.Filename = attachmentFilename(v, att.URI)
	return att
}

func attachmentFilename(v Value, uri string) string {
	for _, param := range []string{ParamFilename, ParamXFilename} {
		if name := strings.TrimSpace(v.Param(param)); name != "" {
			return name
		}
	}
	if uri == "" {
		return ""
	}
	target := uri
	if parsed, err := url.Parse(uri); err == nil && parsed.Path != "" {
		target = parsed.Path
	}
	name := path.Base(strings.TrimSuffix(target, "/"))
	if name == "." || name == "/" {
		return ""
	}
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}
	return name
}

// DisplayName is what a list of attachments prints: the file name when there is
// one, the reference when there is not, and a plain statement when the
// attachment is data with neither.
func (a Attachment) DisplayName() string {
	if name := strings.TrimSpace(a.Filename); name != "" {
		return name
	}
	if a.URI != "" {
		return a.URI
	}
	return "(attached file)"
}

// SizeLabel is the size in the units a person reads, or empty when the writer
// did not say.
func (a Attachment) SizeLabel() string {
	if !a.HasSize {
		return ""
	}
	return ByteSize(a.Size)
}

// ByteSize formats an octet count for display.
func ByteSize(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	value := float64(n)
	for _, suffix := range []string{"kB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			if value < 10 {
				return strconv.FormatFloat(value, 'f', 1, 64) + " " + suffix
			}
			return strconv.FormatFloat(value, 'f', 0, 64) + " " + suffix
		}
	}
	return strconv.FormatFloat(value, 'f', 0, 64) + " TB"
}

// AttachmentValue builds the ATTACH property for a file that has just been put
// on a WebDAV server. FMTTYPE and SIZE are filled in because they are what
// another client needs to show the attachment without fetching it, and FILENAME
// because the last segment of a URI is not always a readable name.
func AttachmentValue(uri, fmtType, filename string, size int64) Value {
	v := Value{Text: strings.TrimSpace(uri)}
	if t := strings.TrimSpace(fmtType); t != "" {
		v = v.WithParam(ParamFmtType, t)
	}
	if name := strings.TrimSpace(filename); name != "" {
		v = v.WithParam(ParamFilename, name)
	}
	if size > 0 {
		v = v.WithParam(ParamSize, strconv.FormatInt(size, 10))
	}
	return v
}

// AttachmentValues turns a set of attachments back into patch values, each one
// as the property it came in as.
//
// This is what makes detaching safe: removing one attachment rewrites the whole
// ATTACH set, and every attachment that stays goes back with the parameters the
// client that wrote it used — including an inline one, which §23.10 forbids
// turning into a link.
func AttachmentValues(list []Attachment) []Value {
	out := make([]Value, 0, len(list))
	for _, att := range list {
		if att.Value.Text == "" {
			continue
		}
		out = append(out, att.Value)
	}
	return out
}

// AttachmentLinks returns the URIs of the attachments that have one. Inline
// data has no link and is left out, which is what a Markdown export carries.
func AttachmentLinks(list []Attachment) []string {
	out := make([]string, 0, len(list))
	for _, att := range list {
		if uri := strings.TrimSpace(att.URI); uri != "" {
			out = append(out, uri)
		}
	}
	return out
}

// AttachmentLinkValues turns a list of URIs back into ATTACH values, filling in
// the file name from the reference and the media type from its extension. It is
// the import half of AttachmentLinks: the substance of the attachment survives a
// Markdown round trip even though the parameters the original writer chose do
// not.
func AttachmentLinkValues(uris []string) []Value {
	out := make([]Value, 0, len(uris))
	seen := make(map[string]bool, len(uris))
	for _, uri := range uris {
		uri = strings.TrimSpace(uri)
		if uri == "" || seen[uri] {
			continue
		}
		seen[uri] = true
		name := attachmentFilename(Value{Text: uri}, uri)
		out = append(out, AttachmentValue(uri, mediaTypeForName(name), name, 0))
	}
	return out
}

// mediaTypeForName is a best guess from the extension, used only to fill in
// FMTTYPE for another client's benefit. Nothing Carrel serves relies on it
// (§24.4).
func mediaTypeForName(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".md", ".markdown":
		return "text/markdown"
	case ".ics":
		return "text/calendar"
	case ".vcf":
		return "text/vcard"
	case ".zip":
		return "application/zip"
	}
	return ""
}

// AttachPatch sets or clears ATTACH on an object. An empty set is a Remove
// rather than an empty value, because those are different things to every other
// client (§11).
func AttachPatch(p *Patch, list []Attachment) *Patch {
	if values := AttachmentValues(list); len(values) > 0 {
		return p.Set(ical.PropAttach, values...)
	}
	return p.Remove(ical.PropAttach)
}
