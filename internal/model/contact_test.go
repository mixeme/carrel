// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import "testing"

func TestContactView(t *testing.T) {
	c, err := mustParse(t, thirdPartyCard).Contact()
	if err != nil {
		t.Fatalf("Contact: %v", err)
	}

	if c.DisplayName() != "Ada Lovelace" {
		t.Errorf("display name = %q", c.DisplayName())
	}
	if c.SortKey() != "lovelace ada" {
		t.Errorf("sort key = %q", c.SortKey())
	}
	if c.Name.Family != "Lovelace" || c.Name.Given != "Ada" || c.Name.Additional != "Byron" {
		t.Errorf("name = %+v", c.Name)
	}
	if len(c.Organization) != 2 || c.Organization[1] != "Research" {
		t.Errorf("organization = %v", c.Organization)
	}
	if len(c.Categories) != 2 || c.Categories[0] != "Mathematics" {
		t.Errorf("categories = %v", c.Categories)
	}
	if len(c.Phones) != 2 {
		t.Fatalf("phones = %+v", c.Phones)
	}
	if c.Phones[0].Label != "work" || c.Phones[0].Preferred {
		t.Errorf("first phone = %+v", c.Phones[0])
	}
	if c.Phones[1].Label != "cell" || !c.Phones[1].Preferred {
		t.Errorf("second phone = %+v", c.Phones[1])
	}
	if len(c.Emails) != 1 || c.Emails[0].Label != "home" {
		t.Errorf("emails = %+v", c.Emails)
	}
	if !c.Photo.Present || !c.Photo.Editable || c.Photo.MediaType != "image/jpeg" {
		t.Errorf("photo = %+v", c.Photo)
	}
}

// TestContactShowsUnknownProperties keeps the X- properties visible rather than
// silently carried: what is being kept should be inspectable (§23.8).
func TestContactShowsUnknownProperties(t *testing.T) {
	c, err := mustParse(t, thirdPartyCard).Contact()
	if err != nil {
		t.Fatalf("Contact: %v", err)
	}
	names := make(map[string]bool, len(c.Other))
	for _, prop := range c.Other {
		names[prop.Name] = true
	}
	for _, want := range []string{"X-EVOLUTION-FILE-AS", "X-CUSTOM-FLAG", "X-ABLABEL"} {
		if !names[want] {
			t.Errorf("%s is not shown among the other properties: %v", want, names)
		}
	}
	for _, unwanted := range []string{"FN", "TEL", "VERSION", "REV", "PHOTO"} {
		if names[unwanted] {
			t.Errorf("%s is listed twice: once typed and once as an other property", unwanted)
		}
	}
}

// TestContactPhotoByURI is the §21 criterion that a photo held as a link is
// shown but not editable (§11).
func TestContactPhotoByURI(t *testing.T) {
	for _, body := range []string{
		"PHOTO;VALUE=uri:https://example.org/ada.jpg",
		"PHOTO:https://example.org/ada.jpg",
	} {
		c, err := card(t, body).Contact()
		if err != nil {
			t.Fatalf("Contact: %v", err)
		}
		if !c.Photo.Present {
			t.Errorf("%s: photo not detected", body)
		}
		if c.Photo.URI != "https://example.org/ada.jpg" {
			t.Errorf("%s: uri = %q", body, c.Photo.URI)
		}
		if c.Photo.Editable {
			t.Errorf("%s: a photo held as a link is offered for editing", body)
		}
	}
}

func TestContactInlineDataURIPhotoIsEditable(t *testing.T) {
	c, err := card(t, "PHOTO:data:image/png;base64,iVBORw0KGgo=").Contact()
	if err != nil {
		t.Fatalf("Contact: %v", err)
	}
	if !c.Photo.Editable || c.Photo.URI != "" {
		t.Errorf("photo = %+v, want an editable inline photo", c.Photo)
	}
	if c.Photo.MediaType != "image/png" {
		t.Errorf("media type = %q", c.Photo.MediaType)
	}
}

func TestContactDisplayNameFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		want  string
	}{
		{"formatted name wins", []string{"FN:Ada Lovelace", "N:Lovelace;Ada;;;"}, "Ada Lovelace"},
		{"falls back to the name parts", []string{"N:Lovelace;Ada;;;"}, "Ada Lovelace"},
		{"falls back to the organisation", []string{"ORG:Analytical Engine"}, "Analytical Engine"},
		{"falls back to the identity", nil, "x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := card(t, tc.lines...).Contact()
			if err != nil {
				t.Fatalf("Contact: %v", err)
			}
			if got := c.DisplayName(); got != tc.want {
				t.Errorf("display name = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestStructuredValueKeepsEscapedSeparator: a comma or semicolon inside a
// component belongs to that component and must not split it.
func TestStructuredValueKeepsEscapedSeparator(t *testing.T) {
	c, err := card(t, `ADR;TYPE=WORK:;;10 Downing St\, Flat 3;London;;SW1;UK`).Contact()
	if err != nil {
		t.Fatalf("Contact: %v", err)
	}
	if len(c.Addresses) != 1 {
		t.Fatalf("addresses = %+v", c.Addresses)
	}
	adr := c.Addresses[0]
	if adr.Street != "10 Downing St, Flat 3" {
		t.Errorf("street = %q", adr.Street)
	}
	if adr.Locality != "London" || adr.PostalCode != "SW1" || adr.Country != "UK" {
		t.Errorf("address = %+v", adr)
	}
	if adr.Label != "work" {
		t.Errorf("label = %q", adr.Label)
	}
}

func TestSplitStructuredKeepsEscapedSemicolon(t *testing.T) {
	got := splitStructured(`a\;b;c`)
	if len(got) != 2 || got[0] != "a;b" || got[1] != "c" {
		t.Fatalf("splitStructured = %q", got)
	}
}
