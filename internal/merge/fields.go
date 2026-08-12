// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package merge

import (
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// MergedField is one field of a linked group, merged by the rules of §15: a
// field only one record carries is taken as it is, a field the records disagree
// about is a choice, and a repeatable field is the union of what they all carry
// rather than one of them winning.
type MergedField struct {
	// Name is the property, upper-cased: FN, TEL, EMAIL.
	Name string
	// Label is what the interface calls it.
	Label string
	// Multi marks a repeatable field, whose Values are a union.
	Multi bool
	// Values is what the merged record shows: one entry for a single-value
	// field, the union for a repeatable one.
	Values []string
	// Options is everything the records offered for a single-value field they
	// disagree about, in the order the records were given.
	Options []string
	// Chosen is the value in use for a single-value field.
	Chosen string
	// Conflict marks a single-value field the records disagree about, which is
	// the one case §15 asks the person rather than deciding.
	Conflict bool
	// Remembered reports that Chosen came from a stored preference rather than
	// from the order the records happen to be in.
	Remembered bool
}

// Merged is the one row a linked group shows in a merged list (§15).
type Merged struct {
	Title  string
	Fields []MergedField
}

// Field returns one merged field by property name.
func (m Merged) Field(name string) (MergedField, bool) {
	for _, field := range m.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return MergedField{}, false
}

// Values returns the merged values of one property, or nil when the group
// carries none.
func (m Merged) Values(name string) []string {
	field, ok := m.Field(name)
	if !ok {
		return nil
	}
	return field.Values
}

// Conflicts returns the fields the records disagree about, which are the only
// ones the interface has to ask about.
func (m Merged) Conflicts() []MergedField {
	var out []MergedField
	for _, field := range m.Fields {
		if field.Conflict {
			out = append(out, field)
		}
	}
	return out
}

// contactField describes one field of the merged view and how to read it off a
// card. The order is the order a merged card is printed in.
type contactField struct {
	Name  string
	Label string
	Multi bool
	// key folds a value into the form duplicates are removed under. An
	// address and a telephone number are compared the way they are scored, so
	// the same number written twice does not appear twice (§15).
	key  func(string) string
	read func(model.Contact) []string
}

var contactFields = []contactField{
	{Name: "FN", Label: "Name", read: func(c model.Contact) []string { return single(c.DisplayName()) }},
	{Name: "NICKNAME", Label: "Nickname", read: func(c model.Contact) []string { return single(c.Nickname) }},
	{Name: "ORG", Label: "Organisation", read: func(c model.Contact) []string {
		return single(strings.Join(c.Organization, ", "))
	}},
	{Name: "TITLE", Label: "Title", read: func(c model.Contact) []string { return single(c.Title) }},
	{Name: "ROLE", Label: "Role", read: func(c model.Contact) []string { return single(c.Role) }},
	{Name: "BDAY", Label: "Birthday", read: func(c model.Contact) []string { return single(c.Birthday) }},
	{Name: "TEL", Label: "Phone", Multi: true, key: PhoneKey, read: func(c model.Contact) []string {
		return labeledValues(c.Phones)
	}},
	{Name: "EMAIL", Label: "Email", Multi: true, key: model.NormalizeEmail, read: func(c model.Contact) []string {
		return labeledValues(c.Emails)
	}},
	{Name: "ADR", Label: "Address", Multi: true, read: func(c model.Contact) []string {
		out := make([]string, 0, len(c.Addresses))
		for _, addr := range c.Addresses {
			if text := formatAddress(addr); text != "" {
				out = append(out, text)
			}
		}
		return out
	}},
	{Name: "URL", Label: "Link", Multi: true, read: func(c model.Contact) []string { return labeledValues(c.URLs) }},
	{Name: "IMPP", Label: "Chat", Multi: true, read: func(c model.Contact) []string { return labeledValues(c.IMs) }},
	{Name: "CATEGORIES", Label: "Tags", Multi: true, read: func(c model.Contact) []string { return c.Categories }},
	{Name: "NOTE", Label: "Note", read: func(c model.Contact) []string { return single(c.Note) }},
}

// MergeContacts merges the cards of a linked group into the single row §15 asks
// for. prefs are the remembered field preferences, keyed by property name; a
// preference that no record offers any more is ignored rather than shown.
//
// Nothing is written anywhere: this is the merged view of records that each stay
// where they are, which is what distinguishes linking from merging on the server.
func MergeContacts(records []Record, prefs map[string]string) Merged {
	views := make([]model.Contact, 0, len(records))
	for _, rec := range records {
		if rec.Object == nil {
			continue
		}
		if contact, err := rec.Object.Contact(); err == nil {
			views = append(views, contact)
		}
	}

	var merged Merged
	for _, field := range contactFields {
		values := make([]string, 0, len(views))
		seen := make(map[string]bool, len(views))
		for _, view := range views {
			for _, value := range field.read(view) {
				value = strings.TrimSpace(value)
				if value == "" {
					continue
				}
				key := field.foldKey(value)
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				values = append(values, value)
			}
		}
		if len(values) == 0 {
			continue
		}
		out := MergedField{Name: field.Name, Label: field.Label, Multi: field.Multi, Values: values}
		if field.Multi {
			merged.Fields = append(merged.Fields, out)
			continue
		}
		out.Chosen = values[0]
		if len(values) > 1 {
			out.Conflict = true
			out.Options = values
			if want := strings.TrimSpace(prefs[field.Name]); want != "" {
				for _, value := range values {
					if value == want {
						out.Chosen = value
						out.Remembered = true
						break
					}
				}
			}
		}
		out.Values = []string{out.Chosen}
		merged.Fields = append(merged.Fields, out)
	}

	if name, ok := merged.Field("FN"); ok {
		merged.Title = name.Chosen
	}
	return merged
}

func (f contactField) foldKey(value string) string {
	if f.key != nil {
		if key := f.key(value); key != "" {
			return key
		}
	}
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func single(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return []string{value}
}

func labeledValues(values []model.LabeledValue) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v.Value) != "" {
			out = append(out, v.Value)
		}
	}
	return out
}

func formatAddress(addr model.Address) string {
	parts := make([]string, 0, 6)
	for _, part := range []string{addr.Street, addr.Extended, addr.Locality, addr.Region, addr.PostalCode, addr.Country} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ", ")
}
