// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-vcard"
)

// Kind is the payload type of an Object.
type Kind string

const (
	// KindVCard is a CardDAV address object (RFC 6350).
	KindVCard Kind = "vcard"
	// KindICal is a CalDAV calendar object (RFC 5545).
	KindICal Kind = "ical"
)

// DefaultVCardVersion is used for objects Carrel creates itself. Objects that
// come from a server keep the version they arrived in (§11).
const DefaultVCardVersion = "3.0"

// DefaultICalProductID identifies calendars Carrel creates itself.
const DefaultICalProductID = "-//Carrel//Carrel//EN"

var writableVCardVersions = map[string]bool{"3.0": true, "4.0": true}

// ErrNotVCard is returned when an operation only defined for address objects is
// applied to something else.
var ErrNotVCard = errors.New("model: object is not an address object")

// ErrNotICal is returned when an operation only defined for calendar objects is
// applied to something else.
var ErrNotICal = errors.New("model: object is not a calendar object")

// ErrNotTodo is returned when a task view is asked of something that is not a
// VTODO.
var ErrNotTodo = errors.New("model: object is not a task")

// ErrNotJournal is returned when a note view is asked of something that is not
// a VJOURNAL.
var ErrNotJournal = errors.New("model: object is not a note")

// Object is a DAV resource together with its parsed payload.
//
// The payload is unexported (§8). Everything a caller needs for display comes
// out through Contact, Event or Properties, and the only way in is Apply, which
// cannot touch a property the caller did not name. An object therefore carries
// every property the server sent, including the ones this build has never heard
// of, from the read that produced it to the write that stores it again.
type Object struct {
	Path string
	ETag string

	kind Kind
	card vcard.Card
	cal  *ical.Calendar
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
	return &Object{Path: path, ETag: etag, kind: KindVCard, card: card}, nil
}

// ParseICal parses one calendar object as it arrived from a server.
func ParseICal(path, etag string, body []byte) (*Object, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("model: empty iCalendar body for %s", path)
	}
	cal, err := ical.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return nil, fmt.Errorf("model: parse iCalendar %s: %w", path, err)
	}
	if cal == nil || cal.Component == nil {
		return nil, fmt.Errorf("model: empty iCalendar for %s", path)
	}
	return &Object{Path: path, ETag: etag, kind: KindICal, cal: cal}, nil
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
	return &Object{kind: KindVCard, card: card}, nil
}

// NewEvent builds a new calendar object with a single VEVENT that holds UID and
// DTSTAMP. Every other property arrives through Apply.
func NewEvent(uid string) (*Object, error) { return newCalendarObject(ical.CompEvent, uid) }

// NewTodo builds a new calendar object with a single VTODO (§10).
func NewTodo(uid string) (*Object, error) { return newCalendarObject(ical.CompToDo, uid) }

// NewJournal builds a new calendar object with a single VJOURNAL (§23.9).
func NewJournal(uid string) (*Object, error) { return newCalendarObject(ical.CompJournal, uid) }

func newCalendarObject(component, uid string) (*Object, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return nil, errors.New("model: UID is required")
	}
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, DefaultICalProductID)
	comp := ical.NewComponent(component)
	comp.Props.SetText(ical.PropUID, uid)
	comp.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	cal.Children = append(cal.Children, comp)
	return &Object{kind: KindICal, cal: cal}, nil
}

// Kind reports what the object holds.
func (o *Object) Kind() Kind {
	if o == nil {
		return ""
	}
	return o.kind
}

// Version returns the format version the object arrived in.
func (o *Object) Version() string {
	if o == nil {
		return ""
	}
	switch o.kind {
	case KindVCard:
		if o.card == nil {
			return ""
		}
		return o.card.Value(vcard.FieldVersion)
	case KindICal:
		if o.cal == nil {
			return ""
		}
		return icalPropText(o.cal.Props, ical.PropVersion)
	default:
		return ""
	}
}

// UID returns the object identity.
func (o *Object) UID() string {
	if o == nil {
		return ""
	}
	switch o.kind {
	case KindVCard:
		if o.card == nil {
			return ""
		}
		return o.card.Value(vcard.FieldUID)
	case KindICal:
		if comp := o.primaryComponent(); comp != nil {
			return icalPropText(comp.Props, ical.PropUID)
		}
		return ""
	default:
		return ""
	}
}

func icalPropText(props ical.Props, name string) string {
	if p := props.Get(name); p != nil {
		return p.Value
	}
	return ""
}

// Clone returns a deep copy, so a candidate write can be compared against the
// version it was based on without either sharing state with the other.
func (o *Object) Clone() *Object {
	if o == nil {
		return nil
	}
	out := &Object{Path: o.Path, ETag: o.ETag, kind: o.kind}
	switch o.kind {
	case KindVCard:
		out.card = cloneCard(o.card)
	case KindICal:
		out.cal = cloneCalendar(o.cal)
	}
	return out
}

// Marshal serialises the object for a PUT.
func (o *Object) Marshal() ([]byte, error) {
	if o == nil {
		return nil, errors.New("model: object has no payload")
	}
	switch o.kind {
	case KindVCard:
		if o.card == nil {
			return nil, errors.New("model: object has no payload")
		}
		var buf bytes.Buffer
		if err := vcard.NewEncoder(&buf).Encode(o.card); err != nil {
			return nil, fmt.Errorf("model: encode vCard: %w", err)
		}
		return foldLines(buf.Bytes()), nil
	case KindICal:
		if o.cal == nil {
			return nil, errors.New("model: object has no payload")
		}
		var buf bytes.Buffer
		if err := ical.NewEncoder(&buf).Encode(o.cal); err != nil {
			return nil, fmt.Errorf("model: encode iCalendar: %w", err)
		}
		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("model: unknown object kind %q", o.kind)
	}
}

// Names returns the property names present in the object, sorted.
func (o *Object) Names() []string {
	if o == nil {
		return nil
	}
	switch o.kind {
	case KindVCard:
		if o.card == nil {
			return nil
		}
		names := make([]string, 0, len(o.card))
		for name := range o.card {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	case KindICal:
		comp := o.primaryComponent()
		if comp == nil {
			return nil
		}
		names := make([]string, 0, len(comp.Props))
		for name := range comp.Props {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	default:
		return nil
	}
}

// Property returns every instance of one property, or nil when it is absent.
func (o *Object) Property(name string) []Value {
	if o == nil {
		return nil
	}
	canonical, err := canonicalName(name)
	if err != nil {
		return nil
	}
	switch o.kind {
	case KindVCard:
		if o.card == nil {
			return nil
		}
		fields := o.card[canonical]
		if len(fields) == 0 {
			return nil
		}
		out := make([]Value, 0, len(fields))
		for _, f := range fields {
			out = append(out, valueFromField(f))
		}
		return out
	case KindICal:
		comp := o.primaryComponent()
		if comp == nil {
			return nil
		}
		props := comp.Props[canonical]
		if len(props) == 0 {
			return nil
		}
		out := make([]Value, 0, len(props))
		for _, p := range props {
			out = append(out, valueFromProp(p))
		}
		return out
	default:
		return nil
	}
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

// primaryEvent returns the first VEVENT in a calendar object.
func (o *Object) primaryEvent() *ical.Event {
	if o == nil || o.cal == nil {
		return nil
	}
	events := o.cal.Events()
	if len(events) == 0 {
		return nil
	}
	return &events[0]
}

// primaryComponent returns the component a calendar object is about: its
// VEVENT, VTODO or VJOURNAL. Everything that reads or patches properties goes
// through this, so a task or a note keeps the unknown properties another client
// left on it exactly as an event does (§8, §23.9). VTIMEZONE and alarms are
// carried along untouched and are never the primary component.
func (o *Object) primaryComponent() *ical.Component {
	if o == nil || o.cal == nil {
		return nil
	}
	for _, child := range o.cal.Children {
		if child == nil {
			continue
		}
		switch strings.ToUpper(child.Name) {
		case ical.CompEvent, ical.CompToDo, ical.CompJournal:
			return child
		}
	}
	return nil
}

// Component reports which iCalendar component the object carries, upper-cased,
// or "" for anything that is not a calendar object.
func (o *Object) Component() string {
	comp := o.primaryComponent()
	if comp == nil {
		return ""
	}
	return strings.ToUpper(comp.Name)
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

func cloneCalendar(cal *ical.Calendar) *ical.Calendar {
	if cal == nil || cal.Component == nil {
		return nil
	}
	return &ical.Calendar{Component: cloneComponent(cal.Component)}
}

func cloneComponent(c *ical.Component) *ical.Component {
	if c == nil {
		return nil
	}
	out := &ical.Component{
		Name:  c.Name,
		Props: make(ical.Props, len(c.Props)),
	}
	for name, props := range c.Props {
		dup := make([]ical.Prop, 0, len(props))
		for _, p := range props {
			dup = append(dup, cloneProp(p))
		}
		out.Props[name] = dup
	}
	if len(c.Children) > 0 {
		out.Children = make([]*ical.Component, 0, len(c.Children))
		for _, child := range c.Children {
			out.Children = append(out.Children, cloneComponent(child))
		}
	}
	return out
}

func cloneProp(p ical.Prop) ical.Prop {
	out := ical.Prop{Name: p.Name, Value: p.Value}
	if p.Params != nil {
		out.Params = make(ical.Params, len(p.Params))
		for k, v := range p.Params {
			out.Params[k] = append([]string(nil), v...)
		}
	}
	return out
}

func valueFromProp(p ical.Prop) Value {
	out := Value{Text: p.Value}
	if len(p.Params) > 0 {
		out.Params = make(map[string][]string, len(p.Params))
		for k, values := range p.Params {
			out.Params[strings.ToUpper(k)] = append([]string(nil), values...)
		}
	}
	return out
}
