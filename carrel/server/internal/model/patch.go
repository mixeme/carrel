package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Object is a DAV record. raw is not exportable: callers see only a view for
// display and send changes only through Patch (S-7).
type Object struct {
	Path string
	ETag string
	// Modified is when the server says the record last changed
	// (getlastmodified in the reply that brought it); zero when it did not say.
	// It is not part of the record and never written back.
	Modified time.Time
	raw      payload
	cal      *calendarPayload
}

// Patch is a validated, atomic set of property operations (S-7). An rejected
// patch changes nothing.
type Patch struct {
	ops []PatchOp
}

// PatchOp is the closed set of patch operations from architecture.md.
type PatchOp interface {
	patchOp()
	prop() string
}

// SetText replaces a single-value property's text.
type SetText struct {
	Prop  string
	Value string
}

func (SetText) patchOp()       {}
func (o SetText) prop() string { return o.Prop }

// SetTextAt replaces the text of one value of a property and leaves the
// value its parameters and its group, and every other value of the property
// as it is: the edit of what a form showed, where the record holds more than
// the form shows (S-7.3).
type SetTextAt struct {
	Prop  string
	Index int
	Value string
}

func (SetTextAt) patchOp()       {}
func (o SetTextAt) prop() string { return o.Prop }

// RemoveValue deletes one value of a property; the last one takes the
// property with it.
type RemoveValue struct {
	Prop  string
	Index int
}

func (RemoveValue) patchOp()       {}
func (o RemoveValue) prop() string { return o.Prop }

// AddValue appends a value to a multi-value property.
type AddValue struct {
	Prop  string
	Value string
}

func (AddValue) patchOp()       {}
func (o AddValue) prop() string { return o.Prop }

// SetValues replaces all values of a multi-value property.
type SetValues struct {
	Prop   string
	Values []string
}

func (SetValues) patchOp()       {}
func (o SetValues) prop() string { return o.Prop }

// SetRaw puts a property back as another version of the record held it:
// every value with its parameters, its group and the line it arrived in
// (Props of that version). No values removes the property. It is how a
// property the form does not show is restored (S-8, the conflict screen's
// «only on the server»), not a way to build one.
type SetRaw struct {
	Prop   string
	Values []RawValue
}

func (SetRaw) patchOp()       {}
func (o SetRaw) prop() string { return o.Prop }

// SetGroup ties a value of a vCard property to a group (item1.X-ABDATE):
// Apple names a value's label, X-ABLabel, by the group they share. A value
// in its group already is left as it is.
type SetGroup struct {
	Prop  string
	Index int
	Group string
}

func (SetGroup) patchOp()       {}
func (o SetGroup) prop() string { return o.Prop }

// RemoveProp deletes a property entirely. Empty value is not removal (S-7).
type RemoveProp struct {
	Prop string
}

func (RemoveProp) patchOp()       {}
func (o RemoveProp) prop() string { return o.Prop }

// SetParam sets a parameter on a property value.
type SetParam struct {
	Prop  string
	Index int
	Param string
	Value string
}

func (SetParam) patchOp()       {}
func (o SetParam) prop() string { return o.Prop }

// RemoveParam removes a parameter from a property value.
type RemoveParam struct {
	Prop  string
	Index int
	Param string
}

func (RemoveParam) patchOp()       {}
func (o RemoveParam) prop() string { return o.Prop }

var (
	ErrPatchImmutable = errors.New("model: property cannot be patched")
	ErrPatchEmpty     = errors.New("model: patch has no operations")
)

// nonPatchable lists properties that identify the record or its wire format.
var nonPatchable = map[string]bool{
	"UID":     true,
	"VERSION": true,
}

// NewPatch builds a patch from operations. Returns an error if any operation
// targets a non-patchable property.
func NewPatch(ops ...PatchOp) (*Patch, error) {
	if len(ops) == 0 {
		return nil, ErrPatchEmpty
	}
	for _, op := range ops {
		name := strings.ToUpper(op.prop())
		if nonPatchable[name] {
			return nil, fmt.Errorf("%w: %s", ErrPatchImmutable, name)
		}
	}
	return &Patch{ops: append([]PatchOp(nil), ops...)}, nil
}

// Ops returns a copy of the patch operations (for tests and validation).
func (p *Patch) Ops() []PatchOp {
	return append([]PatchOp(nil), p.ops...)
}

// payload is the in-memory parsed record. Providers populate it; only this
// package and providers may read or mutate raw bytes.
type payload struct {
	props map[string]propState
}

type propState struct {
	values []propValue
}

type propValue struct {
	text   string
	params map[string]string
	group  string
	// wire is the value's content line as the server sent it, unfolded. It
	// lives only while nobody has touched the value: every operation that
	// changes it clears wire, and an encoder writes a value that still has
	// one exactly as it came (S-7).
	wire string
}

// RawValue is one value of one property as it arrived from the far server:
// the text plus its parameters. It is the only way in — the parsed record
// itself stays unexported, so nothing above this package can rebuild an
// iCalendar or vCard object out of form fields (S-7).
type RawValue struct {
	Text string
	// Params holds every parameter under its upper-case name; a parameter
	// with several values has them joined by commas (ParamValues splits them).
	Params map[string]string
	// Group is the vCard group the value belongs to (item1.EMAIL): Apple
	// ties a label to a value by it. Empty in iCalendar.
	Group string
	// Wire is the content line the value arrived in, unfolded; empty for a
	// value Carrel made or changed. An encoder writes a value with a wire line
	// as that line, so what nobody edited goes back as it came.
	Wire string
}

// NewObject constructs an Object from parsed properties. Providers call it
// after fetching from DAV; property names are folded to upper case.
// ingest copies raw values in, property names folded to upper case.
func ingest(props map[string][]RawValue) map[string]propState {
	cp := make(map[string]propState, len(props))
	for name, values := range props {
		st := propState{values: make([]propValue, len(values))}
		for i, v := range values {
			st.values[i] = propValue{text: v.Text, group: v.Group, wire: v.Wire}
			if len(v.Params) > 0 {
				st.values[i].params = make(map[string]string, len(v.Params))
				for k, pv := range v.Params {
					st.values[i].params[strings.ToUpper(k)] = pv
				}
			}
		}
		cp[strings.ToUpper(name)] = st
	}
	return cp
}

func NewObject(path, etag string, props map[string][]RawValue) *Object {
	cp := ingest(props)
	return &Object{
		Path: path,
		ETag: etag,
		raw:  payload{props: cp},
	}
}

// Props returns the record for display: property names with their values and
// parameters, copied out. Callers get a view, never the record itself.
func (o *Object) Props() map[string][]RawValue {
	if o == nil {
		return nil
	}
	p := o.activePayload()
	if p == nil {
		return nil
	}
	out := make(map[string][]RawValue, len(p.props))
	for name, st := range p.props {
		values := make([]RawValue, len(st.values))
		for i, v := range st.values {
			values[i].Text, values[i].Group, values[i].Wire = v.text, v.group, v.wire
			if len(v.params) > 0 {
				values[i].Params = make(map[string]string, len(v.params))
				for k, pv := range v.params {
					values[i].Params[k] = pv
				}
			}
		}
		out[name] = values
	}
	return out
}

func (p propState) clone() propState {
	out := propState{values: make([]propValue, len(p.values))}
	for i, v := range p.values {
		out.values[i] = v.clone()
	}
	return out
}

func (v propValue) clone() propValue {
	out := propValue{text: v.text, group: v.group, wire: v.wire}
	if len(v.params) > 0 {
		out.params = make(map[string]string, len(v.params))
		for k, val := range v.params {
			out.params[k] = val
		}
	}
	return out
}

// Apply validates and applies the patch atomically, or returns an error without
// changing the object (S-7).
func (p *Patch) Apply(obj *Object) error {
	if obj == nil {
		return errors.New("model: nil object")
	}
	base := obj.activePayload()
	if base == nil {
		return errors.New("model: nil object")
	}
	next := base.clone()
	for _, op := range p.ops {
		if err := applyOp(next, op); err != nil {
			return err
		}
	}
	if obj.cal != nil {
		obj.cal.main = next.props
	} else {
		obj.raw = next
	}
	return nil
}

func (p payload) clone() payload {
	out := payload{props: make(map[string]propState, len(p.props))}
	for k, v := range p.props {
		out.props[k] = v.clone()
	}
	return out
}

func applyOp(p payload, op PatchOp) error {
	switch o := op.(type) {
	case SetText:
		key := strings.ToUpper(o.Prop)
		v := propValue{text: o.Value}
		if old := p.props[key].values; len(old) > 0 {
			v.group = old[0].group
		}
		p.props[key] = propState{values: []propValue{v}}
	case SetTextAt:
		key := strings.ToUpper(o.Prop)
		st, ok := p.props[key]
		if !ok || o.Index < 0 || o.Index >= len(st.values) {
			return fmt.Errorf("model: %s[%d]: no such value", key, o.Index)
		}
		if st.values[o.Index].text != o.Value {
			st.values[o.Index].text = o.Value
			st.values[o.Index].wire = ""
		}
		p.props[key] = st
	case RemoveValue:
		key := strings.ToUpper(o.Prop)
		st, ok := p.props[key]
		if !ok || o.Index < 0 || o.Index >= len(st.values) {
			return fmt.Errorf("model: %s[%d]: no such value", key, o.Index)
		}
		st.values = append(st.values[:o.Index], st.values[o.Index+1:]...)
		if len(st.values) == 0 {
			delete(p.props, key)
		} else {
			p.props[key] = st
		}
	case AddValue:
		key := strings.ToUpper(o.Prop)
		st := p.props[key]
		st.values = append(st.values, propValue{text: o.Value})
		p.props[key] = st
	case SetValues:
		// A value whose text is among the new ones, in the same order, is the
		// same value: it keeps its parameters, group and wire line, so
		// rewriting a list after one row changed does not rewrite the rest.
		key := strings.ToUpper(o.Prop)
		old := p.props[key].values
		vals := make([]propValue, len(o.Values))
		next := 0
		for i, v := range o.Values {
			vals[i] = propValue{text: v}
			for j := next; j < len(old); j++ {
				if old[j].text == v {
					vals[i] = old[j].clone()
					next = j + 1
					break
				}
			}
		}
		p.props[key] = propState{values: vals}
	case SetRaw:
		key := strings.ToUpper(o.Prop)
		if len(o.Values) == 0 {
			delete(p.props, key)
			break
		}
		p.props[key] = ingest(map[string][]RawValue{key: o.Values})[key]
	case SetGroup:
		key := strings.ToUpper(o.Prop)
		st, ok := p.props[key]
		if !ok || o.Index < 0 || o.Index >= len(st.values) {
			return fmt.Errorf("model: %s[%d]: no such value", key, o.Index)
		}
		if st.values[o.Index].group != o.Group {
			st.values[o.Index].group = o.Group
			st.values[o.Index].wire = ""
		}
		p.props[key] = st
	case RemoveProp:
		delete(p.props, strings.ToUpper(o.Prop))
	case SetParam:
		key := strings.ToUpper(o.Prop)
		st, ok := p.props[key]
		if !ok || o.Index < 0 || o.Index >= len(st.values) {
			return fmt.Errorf("model: %s[%d]: no such value", key, o.Index)
		}
		name := strings.ToUpper(o.Param)
		if cur, has := st.values[o.Index].params[name]; has && cur == o.Value {
			return nil
		}
		if st.values[o.Index].params == nil {
			st.values[o.Index].params = make(map[string]string)
		}
		st.values[o.Index].params[name] = o.Value
		st.values[o.Index].wire = ""
		p.props[key] = st
	case RemoveParam:
		key := strings.ToUpper(o.Prop)
		st, ok := p.props[key]
		if !ok || o.Index < 0 || o.Index >= len(st.values) {
			return fmt.Errorf("model: %s[%d]: no such value", key, o.Index)
		}
		name := strings.ToUpper(o.Param)
		if _, has := st.values[o.Index].params[name]; has {
			delete(st.values[o.Index].params, name)
			st.values[o.Index].wire = ""
		}
		p.props[key] = st
	default:
		return fmt.Errorf("model: unknown patch operation %T", op)
	}
	return nil
}

// PropText returns property values for tests and loss comparison.
func (o *Object) PropText(prop string) []string {
	p := o.activePayload()
	if p == nil {
		return nil
	}
	st, ok := p.props[strings.ToUpper(prop)]
	if !ok {
		return nil
	}
	out := make([]string, len(st.values))
	for i, v := range st.values {
		out[i] = v.text
	}
	return out
}
