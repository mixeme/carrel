// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/emersion/go-vcard"
)

// Kind is the payload type of an Object.
type Kind string

const (
	// KindVCard is a CardDAV address object (RFC 6350).
	KindVCard Kind = "vcard"
)

// DefaultVCardVersion is used for objects Carrel creates itself. Objects that
// come from a server keep the version they arrived in (§11).
const DefaultVCardVersion = "3.0"

var writableVCardVersions = map[string]bool{"3.0": true, "4.0": true}

// ErrNotVCard is returned when an operation only defined for address objects is
// applied to something else.
var ErrNotVCard = errors.New("model: object is not an address object")

// Object is a DAV resource together with its parsed payload.
//
// The payload is unexported (§8). Everything a caller needs for display comes
// out through Contact or Properties, and the only way in is Apply, which cannot
// touch a property the caller did not name. An object therefore carries every
// property the server sent, including the ones this build has never heard of,
// from the read that produced it to the write that stores it again.
type Object struct {
	Path string
	ETag string

	kind Kind
	raw  vcard.Card
}

// ParseVCard parses one address object as it arrived from a server.
func ParseVCard(path, etag string, body []byte) (*Object, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("model: empty vCard body for %s", path)
	}
	card, err := vcard.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return nil, fmt.Errorf("model: parse vCard %s: %w", path, err)
	}
	if strings.TrimSpace(card.Value(vcard.FieldVersion)) == "" {
		return nil, fmt.Errorf("model: vCard %s has no VERSION", path)
	}
	return &Object{Path: path, ETag: etag, kind: KindVCard, raw: card}, nil
}

// NewVCard builds a new address object holding nothing but VERSION and UID.
// Every other property has to arrive through Apply, so creating a contact goes
// down the same path as editing one.
func NewVCard(version, uid string) (*Object, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		version = DefaultVCardVersion
	}
	if !writableVCardVersions[version] {
		return nil, fmt.Errorf("model: cannot write vCard version %q", version)
	}
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return nil, errors.New("model: UID is required")
	}
	card := make(vcard.Card)
	card.SetValue(vcard.FieldVersion, version)
	card.SetValue(vcard.FieldUID, uid)
	return &Object{kind: KindVCard, raw: card}, nil
}

// Kind reports what the object holds.
func (o *Object) Kind() Kind {
	if o == nil {
		return ""
	}
	return o.kind
}

// Version returns the vCard version the object arrived in. It is written back
// unchanged: a 3.0 card stays 3.0 even after its photo is replaced (§11).
func (o *Object) Version() string {
	if o == nil || o.raw == nil {
		return ""
	}
	return o.raw.Value(vcard.FieldVersion)
}

// UID returns the object identity.
func (o *Object) UID() string {
	if o == nil || o.raw == nil {
		return ""
	}
	return o.raw.Value(vcard.FieldUID)
}

// Clone returns a deep copy, so a candidate write can be compared against the
// version it was based on without either sharing state with the other.
func (o *Object) Clone() *Object {
	if o == nil {
		return nil
	}
	return &Object{
		Path: o.Path,
		ETag: o.ETag,
		kind: o.kind,
		raw:  cloneCard(o.raw),
	}
}

// Marshal serialises the object for a PUT.
func (o *Object) Marshal() ([]byte, error) {
	if o == nil || o.raw == nil {
		return nil, errors.New("model: object has no payload")
	}
	if o.kind != KindVCard {
		return nil, ErrNotVCard
	}
	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(o.raw); err != nil {
		return nil, fmt.Errorf("model: encode vCard: %w", err)
	}
	return foldLines(buf.Bytes()), nil
}

// Names returns the property names present in the object, sorted.
func (o *Object) Names() []string {
	if o == nil || o.raw == nil {
		return nil
	}
	names := make([]string, 0, len(o.raw))
	for name := range o.raw {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Property returns every instance of one property, or nil when it is absent.
func (o *Object) Property(name string) []Value {
	if o == nil || o.raw == nil {
		return nil
	}
	canonical, err := canonicalName(name)
	if err != nil {
		return nil
	}
	fields := o.raw[canonical]
	if len(fields) == 0 {
		return nil
	}
	out := make([]Value, 0, len(fields))
	for _, f := range fields {
		out = append(out, valueFromField(f))
	}
	return out
}

// Has reports whether the object carries the named property.
func (o *Object) Has(name string) bool { return len(o.Property(name)) > 0 }

// Properties returns every property of the object, sorted by name. This is the
// read-only view the interface renders and the comparison in Compare walks; it
// is a copy, so changing it changes nothing.
func (o *Object) Properties() []Property {
	names := o.Names()
	out := make([]Property, 0, len(names))
	for _, name := range names {
		out = append(out, Property{Name: name, Values: o.Property(name)})
	}
	return out
}

// Property is one property of an object with all of its instances.
type Property struct {
	Name   string
	Values []Value
}

func cloneCard(card vcard.Card) vcard.Card {
	if card == nil {
		return nil
	}
	out := make(vcard.Card, len(card))
	for name, fields := range card {
		dup := make([]*vcard.Field, 0, len(fields))
		for _, f := range fields {
			if f == nil {
				continue
			}
			dup = append(dup, cloneField(f))
		}
		out[name] = dup
	}
	return out
}

func cloneField(f *vcard.Field) *vcard.Field {
	out := &vcard.Field{Value: f.Value, Group: f.Group}
	if f.Params != nil {
		out.Params = make(vcard.Params, len(f.Params))
		for k, v := range f.Params {
			out.Params[k] = append([]string(nil), v...)
		}
	}
	return out
}
