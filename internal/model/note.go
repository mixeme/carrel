// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// Note is the display view of a VJOURNAL (§23.9).
//
// Like Event it is one-way: the interface renders a Note and never hands one
// back, because a Note holds only the properties this build knows to render and
// writing it out whole would drop the X- properties jtx Board leaves behind.
// Edits travel as a Patch.
type Note struct {
	UID         string
	Summary     string
	Description string
	Categories  []string
	// Date is DTSTART in the location passed to Note(), the property §23.9
	// uses for the chronology.
	Date     time.Time
	DateOnly bool
	Related  []Relation
	// Attachments are the ATTACH properties of §23.10 — the pictures and
	// screenshots that were the one thing notes could not carry.
	Attachments []Attachment
	Other       []Property
}

var knownNoteProps = map[string]bool{
	ical.PropUID:           true,
	ical.PropSummary:       true,
	ical.PropDescription:   true,
	ical.PropCategories:    true,
	ical.PropDateTimeStart: true,
	ical.PropDateTimeStamp: true,
	ical.PropCreated:       true,
	ical.PropLastModified:  true,
	ical.PropSequence:      true,
	ical.PropStatus:        true,
	ical.PropClass:         true,
	ical.PropRelatedTo:     true,
	ical.PropAttach:        true,
}

// Note returns the display view of a journal object.
func (o *Object) Note(loc *time.Location) (Note, error) {
	if o == nil || o.kind != KindICal {
		return Note{}, ErrNotICal
	}
	comp := o.primaryComponent()
	if comp == nil || strings.ToUpper(comp.Name) != ical.CompJournal {
		return Note{}, ErrNotJournal
	}
	if loc == nil {
		loc = time.Local
	}
	note := Note{
		UID:         o.UID(),
		Summary:     icalPropText(comp.Props, ical.PropSummary),
		Description: icalPropText(comp.Props, ical.PropDescription),
		Related:     relationsFrom(comp.Props),
		Attachments: attachmentsFrom(comp.Props),
	}
	if cats := comp.Props.Get(ical.PropCategories); cats != nil {
		note.Categories = splitComma(cats.Value)
	}
	if p := comp.Props.Get(ical.PropDateTimeStart); p != nil {
		if start, err := p.DateTime(loc); err == nil {
			note.Date = start
		}
		note.DateOnly = p.ValueType() == ical.ValueDate
	}
	for _, name := range o.Names() {
		if knownNoteProps[name] {
			continue
		}
		note.Other = append(note.Other, Property{Name: name, Values: o.Property(name)})
	}
	return note, nil
}

// DisplayTitle is what a note is called in a list. A note created the fast way
// has only a body, so the first line of it is the title rather than a row
// reading "(untitled)" (§23.9).
func (n Note) DisplayTitle() string {
	if s := strings.TrimSpace(n.Summary); s != "" {
		return s
	}
	if line := firstLine(n.Description); line != "" {
		return line
	}
	if n.UID != "" {
		return n.UID
	}
	return "(empty note)"
}

// Excerpt is a one-line preview of the body for a list row.
func (n Note) Excerpt(max int) string {
	body := strings.TrimSpace(n.Description)
	if strings.TrimSpace(n.Summary) == "" {
		// The first line is already the title; preview what follows it.
		if _, rest, ok := strings.Cut(body, "\n"); ok {
			body = strings.TrimSpace(rest)
		} else {
			return ""
		}
	}
	return truncateRunes(collapseSpace(body), max)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return truncateRunes(strings.TrimSpace(line), 120)
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return strings.TrimSpace(string(runes[:max])) + "…"
}
