package cal

import (
	"bytes"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// Alarm is one VALARM of the main component (S-11: reminders are nested in
// the record, not frozen siblings). An alarm read from the record keeps its
// component, so writing the set back leaves it exactly as the other client
// wrote it — actions, sounds, X- properties and all.
type Alarm struct {
	Trigger     string // TRIGGER as written: "-PT15M", or a date-time
	RelatedEnd  bool   // RELATED=END: counted from the end
	Absolute    bool   // VALUE=DATE-TIME
	Description string
	comp        *ical.Component
}

// Offset is the TRIGGER duration; false for an absolute trigger or one that
// does not parse.
func (a Alarm) Offset() (time.Duration, bool) {
	if a.Absolute {
		return 0, false
	}
	p := ical.NewProp(ical.PropTrigger)
	p.Value = a.Trigger
	p.SetValueType(ical.ValueDuration)
	d, err := p.Duration()
	return d, err == nil
}

// NewAlarm is a display reminder relative to the start.
func NewAlarm(trigger string) Alarm {
	c := ical.NewComponent(ical.CompAlarm)
	c.Props.SetText(ical.PropAction, "DISPLAY")
	trig := ical.NewProp(ical.PropTrigger)
	trig.Value = trigger
	c.Props.Set(trig)
	c.Props.SetText(ical.PropDescription, "Reminder")
	return Alarm{Trigger: trigger, Description: "Reminder", comp: c}
}

// Alarms reads the VALARM set of the main component.
func Alarms(obj *model.Object) ([]Alarm, error) {
	main, err := mainComponent(obj)
	if err != nil {
		return nil, err
	}
	var out []Alarm
	for _, child := range main.Children {
		if child.Name != ical.CompAlarm {
			continue
		}
		a := Alarm{Description: propText(child.Props, ical.PropDescription), comp: child}
		if trig := child.Props.Get(ical.PropTrigger); trig != nil {
			a.Trigger = trig.Value
			a.RelatedEnd = strings.EqualFold(trig.Params.Get(ical.ParamRelated), "END")
			a.Absolute = trig.ValueType() == ical.ValueDateTime
		}
		out = append(out, a)
	}
	return out, nil
}

// SetAlarms rewrites the VALARM set of the main component and returns the
// record to Update. The patch signature is untouched (architecture.md): the
// set is a document edit, not a new PatchOp. Other children of the main
// component, overrides and VTIMEZONE stay as they are, and properties already
// patched on obj are carried over.
func SetAlarms(obj *model.Object, alarms []Alarm) (*model.Object, error) {
	if _, err := mainComponent(obj); err != nil {
		return nil, err
	}
	cal, err := ical.NewDecoder(bytes.NewReader(obj.CalendarDoc())).Decode()
	if err != nil {
		return nil, err
	}
	main := findMain(cal)
	var children []*ical.Component
	for _, child := range main.Children {
		if child.Name != ical.CompAlarm {
			children = append(children, child)
		}
	}
	for _, a := range alarms {
		c := a.comp
		if c == nil {
			c = NewAlarm(a.Trigger).comp
		}
		children = append(children, c)
	}
	main.Children = children
	main.Props = rawToProps(obj.Props())

	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, err
	}
	return model.NewCalendarObject(obj.Path, obj.ETag, main.Name, obj.Props(), buf.Bytes()), nil
}
