package card

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/emersion/go-vcard"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/view"
)

var (
	ErrEmptyETag = errors.New("card: update requires ETag")
)

// Provider reads and writes vCard contacts.
type Provider struct {
	t   dav.Transport
	c   dav.Creator
	now func() time.Time
}

// New wires a card provider.
func New(t dav.Transport) (*Provider, error) {
	c, ok := t.(dav.Creator)
	if !ok {
		return nil, fmt.Errorf("card: transport lacks PutCreate")
	}
	return &Provider{t: t, c: c, now: time.Now}, nil
}

// stamp marks a write as a revision: REV is when the card was last revised
// (RFC 6350 §6.7.4), and Carrel shows it as the server's time of change. The
// loss check ignores it.
func (p *Provider) stamp(obj *model.Object) error {
	patch, err := model.NewPatch(model.SetText{Prop: vcard.FieldRevision, Value: p.now().UTC().Format("20060102T150405Z")})
	if err != nil {
		return err
	}
	return patch.Apply(obj)
}

// Get fetches and parses one contact.
func (p *Provider) Get(ctx context.Context, path string) (*model.Object, error) {
	rc, etag, err := p.t.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return Parse(path, etag, rc)
}

// Parse reads a vCard stream into an Object.
func Parse(path, etag string, r io.Reader) (*model.Object, error) {
	doc, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return parseDoc(path, etag, doc)
}

// parseDoc reads the first card of doc, keeping the lines it came in.
func parseDoc(path, etag string, doc []byte) (*model.Object, error) {
	card, err := vcard.NewDecoder(bytes.NewReader(doc)).Decode()
	if err != nil {
		return nil, err
	}
	var lines []string
	if blocks := cardBlocks(doc); len(blocks) > 0 {
		lines = blocks[0]
	}
	return model.NewObject(path, etag, cardToProps(card, lines)), nil
}

// Create writes a new contact with If-None-Match.
func (p *Provider) Create(ctx context.Context, path string, obj *model.Object) (string, error) {
	if err := p.stamp(obj); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		return "", err
	}
	return p.c.PutCreate(ctx, path, dav.Typed(&buf, MediaType))
}

// MediaType is the Content-Type of a contact on PUT.
const MediaType = "text/vcard; charset=utf-8"

// Update writes with If-Match (S-8).
func (p *Provider) Update(ctx context.Context, obj *model.Object) (string, error) {
	if obj == nil || obj.ETag == "" {
		return "", ErrEmptyETag
	}
	if err := p.stamp(obj); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		return "", err
	}
	tag := strings.Trim(obj.ETag, `"`)
	return p.t.Put(ctx, obj.Path, dav.Typed(&buf, MediaType), `"`+tag+`"`)
}

// Encode serializes an Object to vCard. A value nobody changed goes out in
// the line it came in; the rest is written fresh.
func Encode(w io.Writer, obj *model.Object) error {
	// Folded on the way out: a photo is one line of hundreds of kilobytes
	// (docs/servers.md, SV-31). A line already folded stays as it is.
	_, err := w.Write(model.FoldLines(encodeCard(obj.Props())))
	return err
}

// Foreign returns unknown properties for ContactPage.Foreign (C-6).
func Foreign(obj *model.Object) []view.Field {
	if obj == nil {
		return nil
	}
	var out []view.Field
	for name, values := range obj.Props() {
		upper := strings.ToUpper(name)
		if skipForeign[upper] || knownProps[upper] {
			continue
		}
		for _, v := range values {
			out = append(out, view.Field{
				Label: view.Text(upper),
				Value: view.Text(v.Text),
				Mono:  true,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].Value < out[j].Value
	})
	return out
}

// A PHOTO either holds its image or points at one. A data: URI and a value
// with ENCODING=b hold it; any other URI points outside the card, with or
// without VALUE=uri (vCard 4.0 writes none); a bare value is the base64 of
// vCard 2.1 and 3.0. Only the first PHOTO counts.

// PhotoURI reports a PHOTO that points outside the card. It is never fetched
// here (SSRF): internal/photo fetches it, through the guard.
func PhotoURI(obj *model.Object) (string, bool) {
	props := obj.Props()["PHOTO"]
	if len(props) == 0 {
		return "", false
	}
	if link := photoLink(props[0]); link != "" {
		return link, true
	}
	return "", false
}

// PhotoOpen returns the image a PHOTO holds and the type the card declares
// for it — the data: URI's own, else MEDIATYPE, else TYPE — as written.
// Which types are acceptable, and what TYPE=JPEG is served as, is
// internal/photo's question.
func PhotoOpen(obj *model.Object) (io.ReadCloser, string, error) {
	props := obj.Props()["PHOTO"]
	if len(props) == 0 {
		return nil, "", errors.New("card: no photo")
	}
	v := props[0]
	if link := photoLink(v); link != "" {
		return nil, "", fmt.Errorf("card: external photo %q", link)
	}
	declared := v.Params["MEDIATYPE"]
	if declared == "" {
		declared = v.Params["TYPE"]
	}
	text := strings.TrimSpace(v.Text)
	var data []byte
	var err error
	if isDataURI(text) {
		var own string
		if data, own, err = decodeDataURI(text); own != "" {
			declared = own
		}
	} else {
		data, err = decodeBase64(text)
	}
	if err != nil {
		return nil, "", err
	}
	return io.NopCloser(bytes.NewReader(data)), declared, nil
}

// PhotoOps write a JPEG as the card's PHOTO in the form of the card's own
// version, which stays as it came (S-13): a data: URI in 4.0, base64 with
// ENCODING and TYPE in 3.0 and 2.1. The old PHOTO goes first, with every
// parameter it had; the image travels as text, there is no binary operation.
func PhotoOps(obj *model.Object, jpeg []byte) []model.PatchOp {
	b64 := base64.StdEncoding.EncodeToString(jpeg)
	ops := []model.PatchOp{model.RemoveProp{Prop: vcard.FieldPhoto}}
	version := ""
	if v := obj.PropText(vcard.FieldVersion); len(v) > 0 {
		version = strings.TrimSpace(v[0])
	}
	encoding := "b"
	switch version {
	case "2.1":
		encoding = "BASE64"
	case "3.0":
	default:
		return append(ops, model.SetText{Prop: vcard.FieldPhoto, Value: "data:image/jpeg;base64," + b64})
	}
	return append(ops,
		model.SetText{Prop: vcard.FieldPhoto, Value: b64},
		model.SetParam{Prop: vcard.FieldPhoto, Param: "ENCODING", Value: encoding},
		model.SetParam{Prop: vcard.FieldPhoto, Param: "TYPE", Value: "JPEG"},
	)
}

// PhotoOpsBase64 write the JPEG in the older form, base64 with ENCODING and
// TYPE=JPEG, whatever the card's version; VERSION stays (S-13). vCard 4.0 has
// no ENCODING parameter, and a strict 4.0 client may not show the photo. It is
// what a server that cuts a data: URI (Radicale) keeps whole, and only the
// person's answer to a loss report writes it.
func PhotoOpsBase64(obj *model.Object, jpeg []byte) []model.PatchOp {
	encoding := "b"
	if v := obj.PropText(vcard.FieldVersion); len(v) > 0 && strings.TrimSpace(v[0]) == "2.1" {
		encoding = "BASE64"
	}
	return []model.PatchOp{
		model.RemoveProp{Prop: vcard.FieldPhoto},
		model.SetText{Prop: vcard.FieldPhoto, Value: base64.StdEncoding.EncodeToString(jpeg)},
		model.SetParam{Prop: vcard.FieldPhoto, Param: "ENCODING", Value: encoding},
		model.SetParam{Prop: vcard.FieldPhoto, Param: "TYPE", Value: "JPEG"},
	}
}

// PhotoIsDataURI reports a first PHOTO written as a data: URI, the vCard 4.0
// form.
func PhotoIsDataURI(obj *model.Object) bool {
	props := obj.Props()[vcard.FieldPhoto]
	return len(props) > 0 && isDataURI(strings.TrimSpace(props[0].Text))
}

func photoLink(v model.RawValue) string {
	text := strings.TrimSpace(v.Text)
	if isDataURI(text) {
		return ""
	}
	if enc := strings.ToUpper(v.Params["ENCODING"]); enc == "B" || enc == "BASE64" {
		return ""
	}
	if strings.EqualFold(v.Params["VALUE"], "uri") || hasScheme(text) {
		return text
	}
	return ""
}

func isDataURI(s string) bool {
	return len(s) >= 5 && strings.EqualFold(s[:5], "data:")
}

// hasScheme reports an RFC 3986 scheme before a colon. Base64 has no colon.
func hasScheme(s string) bool {
	i := strings.IndexByte(s, ':')
	if i < 1 {
		return false
	}
	for j, r := range s[:i] {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case j > 0 && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'):
		default:
			return false
		}
	}
	return true
}

// decodeDataURI reads data:[type][;base64],payload (RFC 2397).
func decodeDataURI(s string) ([]byte, string, error) {
	meta, payload, ok := strings.Cut(s[len("data:"):], ",")
	if !ok {
		return nil, "", errors.New("card: data URI without a comma")
	}
	parts := strings.Split(meta, ";")
	declared := strings.TrimSpace(parts[0])
	for _, p := range parts[1:] {
		if strings.EqualFold(strings.TrimSpace(p), "base64") {
			data, err := decodeBase64(payload)
			return data, declared, err
		}
	}
	text, err := url.PathUnescape(payload)
	return []byte(text), declared, err
}

// decodeBase64 takes base64 with or without padding, and whatever whitespace
// the folding of a long line left in it.
func decodeBase64(s string) ([]byte, error) {
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	return base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
}
