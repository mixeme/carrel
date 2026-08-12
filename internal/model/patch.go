// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/emersion/go-vcard"
)

// Value is one instance of a property: its text, its parameters and its group.
type Value struct {
	Text   string
	Params map[string][]string
	Group  string
}

// Text builds a value with no parameters.
func Text(s string) Value { return Value{Text: s} }

// WithParam returns a copy of the value carrying one more parameter.
func (v Value) WithParam(name string, values ...string) Value {
	out := v.clone()
	if out.Params == nil {
		out.Params = make(map[string][]string)
	}
	key := strings.ToUpper(strings.TrimSpace(name))
	out.Params[key] = append(out.Params[key], values...)
	return out
}

// Param returns the first value of one parameter.
func (v Value) Param(name string) string {
	values := v.Params[strings.ToUpper(strings.TrimSpace(name))]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// HasParamValue reports whether one parameter carries the given value, ignoring
// case. Servers and clients disagree about the case of parameter values.
func (v Value) HasParamValue(name, want string) bool {
	for _, got := range v.Params[strings.ToUpper(strings.TrimSpace(name))] {
		if strings.EqualFold(got, want) {
			return true
		}
	}
	return false
}

func (v Value) clone() Value {
	out := Value{Text: v.Text, Group: v.Group}
	if v.Params != nil {
		out.Params = make(map[string][]string, len(v.Params))
		for k, values := range v.Params {
			out.Params[k] = append([]string(nil), values...)
		}
	}
	return out
}

func (v Value) field() *vcard.Field {
	f := &vcard.Field{Value: v.Text, Group: v.Group}
	if len(v.Params) > 0 {
		f.Params = make(vcard.Params, len(v.Params))
		for k, values := range v.Params {
			key := strings.ToUpper(strings.TrimSpace(k))
			f.Params[key] = append([]string(nil), values...)
		}
	}
	return f
}

func valueFromField(f *vcard.Field) Value {
	if f == nil {
		return Value{}
	}
	out := Value{Text: f.Value, Group: f.Group}
	if len(f.Params) > 0 {
		out.Params = make(map[string][]string, len(f.Params))
		for k, values := range f.Params {
			out.Params[strings.ToUpper(k)] = append([]string(nil), values...)
		}
	}
	return out
}

// protectedProperties cannot be changed through a patch. VERSION is the format
// the object arrived in and is written back as it came (§11); UID is its
// identity, and an edit that quietly changes it creates a second contact
// instead of updating the first.
var protectedProperties = map[string]bool{
	vcard.FieldVersion: true,
	vcard.FieldUID:     true,
}

type patchOp struct {
	name   string
	remove bool
	values []Value
}

// Patch is the set of properties an edit touches. Applying it leaves every
// other property of the object exactly as the server sent it (§8).
type Patch struct {
	ops []patchOp
}

// Set replaces every instance of one property. It needs at least one value;
// removing a property is Remove, which is a different thing on the wire and to
// other clients (§11).
func (p *Patch) Set(name string, values ...Value) *Patch {
	p.ops = append(p.ops, patchOp{name: name, values: values})
	return p
}

// SetText replaces one property with a single unparameterised value.
func (p *Patch) SetText(name, text string) *Patch {
	return p.Set(name, Text(text))
}

// Remove deletes one property in full.
func (p *Patch) Remove(name string) *Patch {
	p.ops = append(p.ops, patchOp{name: name, remove: true})
	return p
}

// IsEmpty reports whether the patch changes nothing.
func (p *Patch) IsEmpty() bool { return p == nil || len(p.ops) == 0 }

// Names returns the properties the patch touches, sorted and deduplicated.
func (p *Patch) Names() []string {
	if p == nil {
		return nil
	}
	seen := make(map[string]bool, len(p.ops))
	out := make([]string, 0, len(p.ops))
	for _, op := range p.ops {
		name, err := canonicalName(op.name)
		if err != nil {
			name = strings.ToUpper(strings.TrimSpace(op.name))
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Apply changes only the properties the patch names. A patch that is rejected
// changes nothing at all: every operation is checked before the first one is
// carried out.
func (o *Object) Apply(p *Patch) error {
	if o == nil || o.raw == nil {
		return errors.New("model: object has no payload")
	}
	if o.kind != KindVCard {
		return ErrNotVCard
	}
	if p.IsEmpty() {
		return nil
	}

	names := make([]string, len(p.ops))
	for i, op := range p.ops {
		name, err := canonicalName(op.name)
		if err != nil {
			return err
		}
		if protectedProperties[name] {
			return fmt.Errorf("model: %s cannot be changed by a patch", name)
		}
		if !op.remove && len(op.values) == 0 {
			return fmt.Errorf("model: patch for %s carries no value; use Remove to delete the property", name)
		}
		names[i] = name
	}

	for i, op := range p.ops {
		name := names[i]
		if op.remove {
			delete(o.raw, name)
			continue
		}
		fields := make([]*vcard.Field, 0, len(op.values))
		for _, v := range op.values {
			fields = append(fields, v.field())
		}
		o.raw[name] = fields
	}
	return nil
}

// canonicalName upper-cases a property name and rejects anything that is not
// one, so a patch cannot inject a whole line into the serialised card.
func canonicalName(name string) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(name))
	if trimmed == "" {
		return "", errors.New("model: property name is empty")
	}
	if trimmed == "BEGIN" || trimmed == "END" {
		return "", fmt.Errorf("model: %s is not a property", trimmed)
	}
	for _, r := range trimmed {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
		default:
			return "", fmt.Errorf("model: invalid property name %q", name)
		}
	}
	return trimmed, nil
}
