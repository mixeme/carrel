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

// At is the reminder at another time, counted from where it was — the start,
// or the end with RELATED=END (a task's due date): what it does — its
// action, its sound, its text, whom it writes to — stays (Б28 of the audit
// of 11.10.2026: a reminder whose time was changed came back as a plain
// display one). That it was acknowledged does not: it was, at the time it
// had. A duration given to an absolute reminder counts from the start.
func (a Alarm) At(trigger string) Alarm {
	if a.comp == nil {
		return NewAlarm(trigger)
	}
	c := &ical.Component{Name: a.comp.Name, Props: ical.Props{}, Children: a.comp.Children}
	for name, props := range a.comp.Props {
		if name != "ACKNOWLEDGED" {
			c.Props[name] = append([]ical.Prop(nil), props...)
		}
	}
	trig := ical.NewProp(ical.PropTrigger)
	trig.Value = trigger
	end := a.RelatedEnd && !a.Absolute
	if end {
		trig.Params.Set(ical.ParamRelated, "END")
	}
	c.Props.Set(trig)
	return Alarm{Trigger: trigger, RelatedEnd: end, Description: a.Description, comp: c}
}

// Alarms reads the VALARM set of the main component.
func Alarms(obj *model.Object) ([]Alarm, error) {
	main, err := mainComponent(obj)
	if err != nil {
		return nil, err
	}
	return componentAlarms(main), nil
}

func componentAlarms(main *ical.Component) []Alarm {
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
	return out
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
	cal, err := decode(obj.CalendarDoc())
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

// alarmKey is what makes a reminder the reminder it is to the person: what
// it does, and when — the offset it names, from the start or from the end,
// or the instant. How a server spells it, and what it adds of its own (a
// UID, when it was last acknowledged), is not part of it.
func alarmKey(a Alarm) string {
	action := ""
	if a.comp != nil {
		action = strings.ToUpper(strings.TrimSpace(propText(a.comp.Props, ical.PropAction)))
	}
	when := strings.ToUpper(strings.TrimSpace(a.Trigger))
	if d, ok := a.Offset(); ok && when != "" {
		when = d.String()
	}
	from := "start"
	switch {
	case a.Absolute:
		from = "at"
	case a.RelatedEnd:
		from = "end"
	}
	return action + "\x00" + from + "\x00" + when
}

// MissingAlarms counts reminders not kept whole, each as often as it stands.
// The action and trigger are compared semantically; the rest includes the
// message, recipients, sound, repetition and properties from other clients.
func MissingAlarms(sent, got *model.Object) int {
	want, err := Alarms(sent)
	if err != nil {
		return 0
	}
	have, _ := Alarms(got)
	return missingAlarms(want, have)
}

func missingAlarms(want, have []Alarm) int {
	// One received alarm can account for one sent alarm only. Find a full
	// matching rather than taking the first fit: a server may add properties
	// to one of several alarms with the same action and trigger.
	edges := make([][]int, len(want))
	for i, a := range want {
		for j, b := range have {
			if alarmKey(a) == alarmKey(b) && model.CompareAfterPut("", alarmContents(a), alarmContents(b), nil) == nil {
				edges[i] = append(edges[i], j)
			}
		}
	}
	owner := make([]int, len(have))
	for i := range owner {
		owner[i] = -1
	}
	var match func(int, []bool) bool
	match = func(i int, seen []bool) bool {
		for _, j := range edges[i] {
			if seen[j] {
				continue
			}
			seen[j] = true
			if owner[j] < 0 || match(owner[j], seen) {
				owner[j] = i
				return true
			}
		}
		return false
	}
	missing := 0
	for i := range want {
		if !match(i, make([]bool, len(have))) {
			missing++
		}
	}
	return missing
}

func alarmContents(a Alarm) *model.Object {
	props := propsToRaw(a.comp.Props, nil)
	// The first two are compared by alarmKey. Identity and acknowledgement
	// belong to the server; model.CompareAfterPut also ignores its stamps.
	for _, name := range []string{"ACTION", "TRIGGER", "UID", "ACKNOWLEDGED"} {
		delete(props, name)
	}
	return model.NewObject("", "", props)
}

// AlarmLoss adds «VALARM» to the list of what a server did not keep when a
// reminder of sent is gone from got or is no longer the reminder it was: the
// comparison of a write reads the properties of the record, and a reminder
// is a component inside it.
func AlarmLoss(lost []string, sent, got *model.Object) []string {
	if MissingAlarms(sent, got) == 0 {
		return lost
	}
	for _, p := range lost {
		if p == ical.CompAlarm {
			return lost
		}
	}
	return append(lost, ical.CompAlarm)
}
