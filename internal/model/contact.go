// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"
	"time"

	"github.com/emersion/go-vcard"
)

// Contact is the display view of an address object.
//
// It is deliberately one-way. The interface renders a Contact; it never hands
// one back, because a Contact only knows the properties this build happens to
// render and writing it out would drop the rest (§8). Edits travel as a Patch.
type Contact struct {
	UID           string
	Version       string
	FormattedName string
	Name          Name
	Nickname      string
	Organization  []string
	Title         string
	Role          string
	Birthday      string
	Note          string
	Categories    []string
	Phones        []LabeledValue
	Emails        []LabeledValue
	URLs          []LabeledValue
	IMs           []LabeledValue
	Addresses     []Address
	Photo         Photo
	// Modified is REV, read for the "по изменению" sort of 2.6.B2. A card
	// that carries no REV parses as the zero time, the same convention the
	// due-date and last-modified sorts elsewhere in this package already use
	// for "unknown".
	Modified time.Time

	// Other holds the properties the card carries that this view does not
	// name, X- properties among them. They are shown read-only rather than
	// hidden, so it is visible that they exist and are being kept (§23.8).
	Other []Property
}

// Name is the structured N property.
type Name struct {
	Family     string
	Given      string
	Additional string
	Prefix     string
	Suffix     string
}

// LabeledValue is one repeatable detail together with the label its card gave
// it, as in a work phone number or a home email address.
type LabeledValue struct {
	Label     string
	Value     string
	Preferred bool
}

// Address is the structured ADR property.
type Address struct {
	Label      string
	Preferred  bool
	POBox      string
	Extended   string
	Street     string
	Locality   string
	Region     string
	PostalCode string
	Country    string
}

// Photo describes the PHOTO property without carrying its bytes; the image
// itself is served from its own endpoint (§11).
type Photo struct {
	Present   bool
	MediaType string
	// URI is set when the card points at an image elsewhere instead of
	// carrying it. Such a photo is shown through the proxy and cannot be
	// edited here (§11).
	URI string
	// Editable is false for a photo held as a URI.
	Editable bool
}

// contactProperties are the properties Contact surfaces in a typed field, plus
// the bookkeeping ones no one needs to see. Everything else lands in Other.
var contactProperties = map[string]bool{
	vcard.FieldVersion:       true,
	vcard.FieldUID:           true,
	vcard.FieldProductID:     true,
	vcard.FieldRevision:      true,
	vcard.FieldFormattedName: true,
	vcard.FieldName:          true,
	vcard.FieldNickname:      true,
	vcard.FieldOrganization:  true,
	vcard.FieldTitle:         true,
	vcard.FieldRole:          true,
	vcard.FieldBirthday:      true,
	vcard.FieldNote:          true,
	vcard.FieldCategories:    true,
	vcard.FieldTelephone:     true,
	vcard.FieldEmail:         true,
	vcard.FieldURL:           true,
	vcard.FieldIMPP:          true,
	vcard.FieldAddress:       true,
	vcard.FieldPhoto:         true,
}

// Contact returns the display view of an address object.
func (o *Object) Contact() (Contact, error) {
	if o == nil || o.card == nil {
		return Contact{}, ErrNotVCard
	}
	if o.kind != KindVCard {
		return Contact{}, ErrNotVCard
	}

	c := Contact{
		UID:           o.UID(),
		Version:       o.Version(),
		FormattedName: firstText(o.Property(vcard.FieldFormattedName)),
		Nickname:      firstText(o.Property(vcard.FieldNickname)),
		Title:         firstText(o.Property(vcard.FieldTitle)),
		Role:          firstText(o.Property(vcard.FieldRole)),
		Birthday:      firstText(o.Property(vcard.FieldBirthday)),
		Note:          firstText(o.Property(vcard.FieldNote)),
		Phones:        labeled(o.Property(vcard.FieldTelephone)),
		Emails:        labeled(o.Property(vcard.FieldEmail)),
		URLs:          labeled(o.Property(vcard.FieldURL)),
		IMs:           labeled(o.Property(vcard.FieldIMPP)),
		Photo:         describePhoto(o.Property(vcard.FieldPhoto)),
	}
	if modified, err := o.card.Revision(); err == nil {
		c.Modified = modified
	}

	if values := o.Property(vcard.FieldName); len(values) > 0 {
		parts := splitStructured(values[0].Text)
		c.Name = Name{
			Family:     component(parts, 0),
			Given:      component(parts, 1),
			Additional: component(parts, 2),
			Prefix:     component(parts, 3),
			Suffix:     component(parts, 4),
		}
	}
	if values := o.Property(vcard.FieldOrganization); len(values) > 0 {
		for _, part := range splitStructured(values[0].Text) {
			if part = strings.TrimSpace(part); part != "" {
				c.Organization = append(c.Organization, part)
			}
		}
	}
	for _, v := range o.Property(vcard.FieldCategories) {
		for _, part := range strings.Split(v.Text, ",") {
			if part = strings.TrimSpace(part); part != "" {
				c.Categories = append(c.Categories, part)
			}
		}
	}
	for _, v := range o.Property(vcard.FieldAddress) {
		parts := splitStructured(v.Text)
		c.Addresses = append(c.Addresses, Address{
			Label:      label(v),
			Preferred:  preferred(v),
			POBox:      component(parts, 0),
			Extended:   component(parts, 1),
			Street:     component(parts, 2),
			Locality:   component(parts, 3),
			Region:     component(parts, 4),
			PostalCode: component(parts, 5),
			Country:    component(parts, 6),
		})
	}
	for _, prop := range o.Properties() {
		if !contactProperties[prop.Name] {
			c.Other = append(c.Other, prop)
		}
	}
	return c, nil
}

// DisplayName returns the best name the card offers, falling back to the UID so
// a contact with neither is still identifiable in a list.
func (c Contact) DisplayName() string {
	if name := strings.TrimSpace(c.FormattedName); name != "" {
		return name
	}
	parts := make([]string, 0, 2)
	if c.Name.Given != "" {
		parts = append(parts, c.Name.Given)
	}
	if c.Name.Family != "" {
		parts = append(parts, c.Name.Family)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	if len(c.Organization) > 0 {
		return c.Organization[0]
	}
	return c.UID
}

// SortKey returns the value a contact list is ordered by: family name first when
// there is one, so the order does not change with how a client wrote FN.
func (c Contact) SortKey() string {
	parts := make([]string, 0, 2)
	if c.Name.Family != "" {
		parts = append(parts, c.Name.Family)
	}
	if c.Name.Given != "" {
		parts = append(parts, c.Name.Given)
	}
	if len(parts) == 0 {
		return strings.ToLower(c.DisplayName())
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func describePhoto(values []Value) Photo {
	if len(values) == 0 {
		return Photo{}
	}
	v := values[0]
	photo := Photo{Present: true, MediaType: mediaTypeOf(v)}
	text := strings.TrimSpace(v.Text)
	switch {
	case strings.EqualFold(v.Param(vcard.ParamValue), "uri") && !strings.HasPrefix(text, "data:"):
		photo.URI = text
	case strings.HasPrefix(text, "http://"), strings.HasPrefix(text, "https://"):
		photo.URI = text
	default:
		photo.Editable = true
	}
	return photo
}

func mediaTypeOf(v Value) string {
	if mt := v.Param(vcard.ParamMediaType); mt != "" {
		return strings.ToLower(mt)
	}
	if t := v.Param(vcard.ParamType); t != "" && strings.Contains(t, "/") {
		return strings.ToLower(t)
	}
	if t := v.Param(vcard.ParamType); t != "" {
		return "image/" + strings.ToLower(t)
	}
	if rest, ok := cutPrefix(strings.TrimSpace(v.Text), "data:"); ok {
		if i := strings.IndexAny(rest, ";,"); i > 0 {
			return strings.ToLower(rest[:i])
		}
	}
	return ""
}

func labeled(values []Value) []LabeledValue {
	if len(values) == 0 {
		return nil
	}
	out := make([]LabeledValue, 0, len(values))
	for _, v := range values {
		out = append(out, LabeledValue{
			Label:     label(v),
			Value:     v.Text,
			Preferred: preferred(v),
		})
	}
	return out
}

// label joins the TYPE parameters into something readable, dropping "pref"
// because it is shown as a flag rather than a label.
func label(v Value) string {
	types := v.Params[vcard.ParamType]
	out := make([]string, 0, len(types))
	for _, t := range types {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || t == "pref" || t == "internet" || t == "voice" {
			continue
		}
		out = append(out, t)
	}
	return strings.Join(out, ", ")
}

// preferred reads both spellings: PREF=1 in vCard 4.0 and TYPE=PREF in 3.0,
// which is also what Apple Contacts writes.
func preferred(v Value) bool {
	if p := v.Param(vcard.ParamPreferred); p != "" {
		return p == "1"
	}
	return v.HasParamValue(vcard.ParamType, "pref")
}

func firstText(values []Value) string {
	if len(values) == 0 {
		return ""
	}
	return values[0].Text
}

func component(parts []string, i int) string {
	if i >= len(parts) {
		return ""
	}
	return strings.TrimSpace(parts[i])
}

// splitStructured splits a structured value on its component separator, leaving
// an escaped semicolon inside a component alone.
func splitStructured(s string) []string {
	var (
		out []string
		cur strings.Builder
		esc bool
	)
	for _, r := range s {
		switch {
		case esc:
			if r != ';' {
				cur.WriteRune('\\')
			}
			cur.WriteRune(r)
			esc = false
		case r == '\\':
			esc = true
		case r == ';':
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if esc {
		cur.WriteRune('\\')
	}
	return append(out, cur.String())
}

func cutPrefix(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):], true
	}
	return "", false
}
