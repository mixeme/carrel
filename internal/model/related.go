// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"

	"github.com/emersion/go-ical"
)

// Relation reltype values (RFC 5545 §3.2.15). PARENT is the default when a
// RELATED-TO carries no RELTYPE at all.
const (
	RelTypeParent  = "PARENT"
	RelTypeChild   = "CHILD"
	RelTypeSibling = "SIBLING"
)

// Relation is one RELATED-TO link: the UID of another object and how this one
// relates to it.
//
// This is the only link Carrel writes between iCalendar objects, and it is
// written because jtx Board writes it too. A link expressible only through an
// X- property of our own would be readable by Carrel alone, which is the
// opposite of the point (§23.9).
type Relation struct {
	UID     string
	RelType string
}

// Relations returns every RELATED-TO on the object's primary component.
func (o *Object) Relations() []Relation {
	comp := o.primaryComponent()
	if comp == nil {
		return nil
	}
	return relationsFrom(comp.Props)
}

func relationsFrom(props ical.Props) []Relation {
	values := props[ical.PropRelatedTo]
	if len(values) == 0 {
		return nil
	}
	out := make([]Relation, 0, len(values))
	for _, p := range values {
		uid := strings.TrimSpace(p.Value)
		if uid == "" {
			continue
		}
		relType := strings.ToUpper(strings.TrimSpace(p.Params.Get("RELTYPE")))
		if relType == "" {
			relType = RelTypeParent
		}
		out = append(out, Relation{UID: uid, RelType: relType})
	}
	return out
}

// RelationValues turns relations into patch values, so a set of links is
// written as repeated RELATED-TO properties rather than one joined value.
func RelationValues(relations []Relation) []Value {
	out := make([]Value, 0, len(relations))
	for _, rel := range relations {
		uid := strings.TrimSpace(rel.UID)
		if uid == "" {
			continue
		}
		v := Value{Text: uid}
		if relType := strings.ToUpper(strings.TrimSpace(rel.RelType)); relType != "" && relType != RelTypeParent {
			v = v.WithParam("RELTYPE", relType)
		}
		out = append(out, v)
	}
	return out
}

// ParseRelations reads a comma or whitespace separated list of UIDs typed into
// a form. Every entry becomes a PARENT relation, which is what jtx Board uses
// for "this note belongs to that entry".
func ParseRelations(text string) []Relation {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	})
	seen := make(map[string]bool, len(fields))
	out := make([]Relation, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, Relation{UID: f, RelType: RelTypeParent})
	}
	return out
}

// RelationUIDs returns just the identifiers, in order.
func RelationUIDs(relations []Relation) []string {
	out := make([]string, 0, len(relations))
	for _, rel := range relations {
		out = append(out, rel.UID)
	}
	return out
}
