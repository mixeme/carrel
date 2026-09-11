package card

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

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
	t dav.Transport
	c dav.Creator
}

// New wires a card provider.
func New(t dav.Transport) (*Provider, error) {
	c, ok := t.(dav.Creator)
	if !ok {
		return nil, fmt.Errorf("card: transport lacks PutCreate")
	}
	return &Provider{t: t, c: c}, nil
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
	dec := vcard.NewDecoder(r)
	card, err := dec.Decode()
	if err != nil {
		return nil, err
	}
	return model.NewObject(path, etag, cardToProps(card)), nil
}

// Create writes a new contact with If-None-Match.
func (p *Provider) Create(ctx context.Context, path string, obj *model.Object) (string, error) {
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		return "", err
	}
	return p.c.PutCreate(ctx, path, &buf)
}

// Update writes with If-Match (S-8).
func (p *Provider) Update(ctx context.Context, obj *model.Object) (string, error) {
	if obj == nil || obj.ETag == "" {
		return "", ErrEmptyETag
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		return "", err
	}
	tag := strings.Trim(obj.ETag, `"`)
	return p.t.Put(ctx, obj.Path, &buf, `"`+tag+`"`)
}

// Encode serializes an Object to vCard.
func Encode(w io.Writer, obj *model.Object) error {
	card := propsToCard(obj.Props())
	if card.Get(vcard.FieldVersion) == nil {
		card.SetValue(vcard.FieldVersion, "4.0")
	}
	return vcard.NewEncoder(w).Encode(card)
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

// PhotoURI reports an external PHOTO; such URLs are never fetched (SSRF).
func PhotoURI(obj *model.Object) (string, bool) {
	for _, v := range obj.Props()["PHOTO"] {
		if strings.EqualFold(v.Params["VALUE"], "uri") {
			return v.Text, true
		}
	}
	return "", false
}

// PhotoOpen returns an inline PHOTO stream and its declared media type.
func PhotoOpen(obj *model.Object) (io.ReadCloser, string, error) {
	if uri, ok := PhotoURI(obj); ok {
		return nil, "", fmt.Errorf("card: external photo %q", uri)
	}
	props := obj.Props()["PHOTO"]
	if len(props) == 0 {
		return nil, "", errors.New("card: no photo")
	}
	data, err := decodePhoto(props[0])
	if err != nil {
		return nil, "", err
	}
	return io.NopCloser(bytes.NewReader(data)), photoContentType(props[0]), nil
}

func decodePhoto(v model.RawValue) ([]byte, error) {
	if strings.EqualFold(v.Params["VALUE"], "uri") {
		return nil, errors.New("card: uri photo")
	}
	if enc := strings.ToUpper(v.Params["ENCODING"]); enc == "B" || enc == "BASE64" {
		return base64.StdEncoding.DecodeString(v.Text)
	}
	return base64.StdEncoding.DecodeString(v.Text)
}

func photoContentType(v model.RawValue) string {
	if mt := v.Params["MEDIATYPE"]; mt != "" {
		return mt
	}
	if t := v.Params["TYPE"]; t != "" {
		return t
	}
	return "application/octet-stream"
}
