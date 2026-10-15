package dupes

import (
	"fmt"
	"sort"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
)

// A merge on the server (S-16) starts here: the body of the kept record is
// the base, and from the others comes what it lacks. Every property of every
// member is kept — this assembly does not know what a foreign property means,
// so it drops none — and where names collide the kept record wins. The only
// questions are a single-valued property whose values differ and the time,
// when the members' times differ (When); an answer is an Option index, and
// without an answer the kept record's value stays.

// singleValued are the properties a record holds once (RFC 6350 cardinality
// 1 and *1, RFC 5545 "MUST NOT occur more than once"), plus PHOTO and ORG,
// where two values would be one more question for every client after us.
// Every other property — the unknown and X- ones included — is a set.
var singleValued = map[string]bool{
	// vCard
	"FN": true, "N": true, "BDAY": true, "ANNIVERSARY": true, "GENDER": true,
	"KIND": true, "PHOTO": true, "ORG": true,
	// iCalendar main component
	"SUMMARY": true, "DESCRIPTION": true, "LOCATION": true, "CLASS": true,
	"GEO": true, "ORGANIZER": true, "PRIORITY": true, "STATUS": true,
	"TRANSP": true, "URL": true, "COLOR": true, "PERCENT-COMPLETE": true,
	"COMPLETED": true,
}

// ownOnly are not merged property by property. The identity and the stamps
// are the kept record's own. Time is one piece — DTSTART with its TZID, DTEND
// or DURATION, DUE and the repetition — and taking a part of it from another
// record would pair a time with a zone that record's document defines and
// the kept one may not: it is asked about as one question (When) and taken
// whole from the member chosen, with its zones and its rewritten occurrences
// (cal.TakeTime).
var ownOnly = map[string]bool{
	"UID": true, "VERSION": true, "PRODID": true, "REV": true,
	"DTSTAMP": true, "LAST-MODIFIED": true, "CREATED": true, "SEQUENCE": true,
	"DTSTART": true, "DTEND": true, "DURATION": true, "DUE": true,
	"RRULE": true, "RDATE": true, "EXDATE": true, "RECURRENCE-ID": true,
}

// When is the name of the question about time (А6 of the audit of
// 11.10.2026; the owner's decision of 12.10.2026: the person decides what to
// take from where). It is no property: an option's Value is the member's
// time as written (cal.TimeKey), and the screen says it in words.
const When = "WHEN"

// Leave is the answer to a single-valued property that leaves it out of the
// merged record: the person chose the member that holds none (Дб7, s-dup-1:
// «Organisation» — «—»).
const Leave = -1

// Option is one value offered for a single-valued property. From is the
// member it comes from: 0 is the kept record, i is others[i-1].
type Option struct {
	From  int
	Value model.RawValue
}

// Question is a single-valued property whose values differ. Options[0] is
// what stays without an answer.
type Question struct {
	Prop    string
	Options []Option
}

// Gain is a value the kept record receives.
type Gain struct {
	Prop  string
	From  int
	Value model.RawValue
}

// Plan is what a merge writes, before it is written: the confirmation lists
// it, because the list is the barrier, not the button.
type Plan struct {
	Questions []Question
	// Union names the set-valued properties more than one member holds: they
	// are joined, exact repeats dropped, and nobody is asked.
	Union []string
	Gains []Gain
	// Alarms is how many reminders of the others the kept event or task
	// takes, the ones it does not already have.
	Alarms int
	// Instances counts, per other member, the rewritten occurrences
	// (RECURRENCE-ID) that go with it when it is deleted: they belong to that
	// member's series, not to the kept one. The member whose time was chosen
	// is not counted: its occurrences come with its time.
	Instances map[int]int
	// Time is the member whose time the merged record has: 0, the kept one,
	// unless When was answered otherwise. Dropped counts the kept record's
	// own rewritten occurrences that went with the time it gave up.
	Time    int
	Dropped int
	// Left are the kept record's values of the properties answered Leave:
	// they are not in the merged record.
	Left []Gain
}

// Merge builds the merged record from the kept one and the others, with
// choices answering the questions by index. Neither input is changed: the
// merged record is a new object with the kept one's path and version, ready
// to be written with If-Match.
func Merge(target *model.Object, others []*model.Object, choices map[string]int) (*model.Object, Plan, error) {
	var plan Plan
	if target == nil {
		return nil, plan, fmt.Errorf("dupes: no record to keep")
	}
	for _, o := range others {
		if o == nil || o.IsCalendar() != target.IsCalendar() {
			return nil, plan, fmt.Errorf("dupes: records of different kinds")
		}
	}
	base := target.Props()
	members := append([]map[string][]model.RawValue{base}, make([]map[string][]model.RawValue, len(others))...)
	for i, o := range others {
		members[i+1] = o.Props()
	}

	names := map[string]bool{}
	for _, m := range members {
		for name := range m {
			names[name] = true
		}
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)

	var ops []model.PatchOp
	for _, name := range sorted {
		if ownOnly[name] {
			continue
		}
		mine := base[name]
		if singleValued[name] && choices[name] == Leave {
			if _, answered := choices[name]; answered {
				if len(mine) > 0 {
					ops = append(ops, model.RemoveProp{Prop: name})
					plan.Left = append(plan.Left, Gain{Prop: name, From: 0, Value: mine[0]})
				}
				continue
			}
		}
		if singleValued[name] {
			var opts []Option
			for from, m := range members {
				vals := m[name]
				if len(vals) == 0 {
					continue
				}
				if !hasOption(opts, vals[0]) {
					opts = append(opts, Option{From: from, Value: vals[0]})
				}
			}
			if len(opts) == 0 {
				continue
			}
			if len(opts) > 1 {
				plan.Questions = append(plan.Questions, Question{Prop: name, Options: opts})
			}
			pick := opts[0]
			if i, ok := choices[name]; ok && i >= 0 && i < len(opts) {
				pick = opts[i]
			}
			if pick.From == 0 {
				continue
			}
			ops = append(ops, model.SetText{Prop: name, Value: pick.Value.Text})
			ops = append(ops, paramOps(name, 0, pick.Value.Params)...)
			plan.Gains = append(plan.Gains, Gain{Prop: name, From: pick.From, Value: pick.Value})
			continue
		}
		have := append([]model.RawValue(nil), mine...)
		holders := 0
		for _, m := range members {
			if len(m[name]) > 0 {
				holders++
			}
		}
		if holders > 1 {
			plan.Union = append(plan.Union, name)
		}
		for from, m := range members[1:] {
			for _, v := range m[name] {
				if hasValue(have, v) {
					continue
				}
				ops = append(ops, model.AddValue{Prop: name, Value: v.Text})
				ops = append(ops, paramOps(name, len(have), v.Params)...)
				have = append(have, v)
				plan.Gains = append(plan.Gains, Gain{Prop: name, From: from + 1, Value: v})
			}
		}
	}

	merged := clone(target)
	if len(ops) > 0 {
		patch, err := model.NewPatch(ops...)
		if err != nil {
			return nil, plan, err
		}
		if err := patch.Apply(merged); err != nil {
			return nil, plan, err
		}
	}
	if !target.IsCalendar() {
		return merged, plan, nil
	}

	// The time: one question when it differs, the kept record's without an
	// answer.
	times := []Option{{From: 0, Value: model.RawValue{Text: cal.TimeKey(target)}}}
	for i, o := range others {
		if v := (model.RawValue{Text: cal.TimeKey(o)}); !hasOption(times, v) {
			times = append(times, Option{From: i + 1, Value: v})
		}
	}
	if len(times) > 1 {
		plan.Questions = append(plan.Questions, Question{Prop: When, Options: times})
		if i, ok := choices[When]; ok && i > 0 && i < len(times) {
			plan.Time = times[i].From
			var err error
			if merged, plan.Dropped, err = cal.TakeTime(merged, others[plan.Time-1]); err != nil {
				return nil, plan, err
			}
		}
	}

	kept, err := cal.Alarms(merged)
	if err != nil {
		return nil, plan, err
	}
	all := append([]cal.Alarm(nil), kept...)
	for _, o := range others {
		theirs, err := cal.Alarms(o)
		if err != nil {
			return nil, plan, err
		}
		for _, a := range theirs {
			if !hasAlarm(all, a) {
				all = append(all, a)
				plan.Alarms++
			}
		}
	}
	if plan.Alarms > 0 {
		if merged, err = cal.SetAlarms(merged, all); err != nil {
			return nil, plan, err
		}
	}
	for i, o := range others {
		if i+1 == plan.Time {
			continue
		}
		inst, err := cal.Overrides(o, time.UTC)
		if err != nil {
			return nil, plan, err
		}
		if len(inst) > 0 {
			if plan.Instances == nil {
				plan.Instances = map[int]int{}
			}
			plan.Instances[i+1] = len(inst)
		}
	}
	return merged, plan, nil
}

// clone is a copy of a record that a patch may change.
func clone(o *model.Object) *model.Object {
	if o.IsCalendar() {
		return model.NewCalendarObject(o.Path, o.ETag, o.MainComponent(), o.Props(), o.CalendarDoc())
	}
	c := model.NewObject(o.Path, o.ETag, o.Props())
	c.Modified = o.Modified
	return c
}

func paramOps(prop string, index int, params map[string]string) []model.PatchOp {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]model.PatchOp, 0, len(keys))
	for _, k := range keys {
		out = append(out, model.SetParam{Prop: prop, Index: index, Param: k, Value: params[k]})
	}
	return out
}

func hasOption(opts []Option, v model.RawValue) bool {
	for _, o := range opts {
		if sameValue(o.Value, v) {
			return true
		}
	}
	return false
}

func hasValue(have []model.RawValue, v model.RawValue) bool {
	for _, h := range have {
		if sameValue(h, v) {
			return true
		}
	}
	return false
}

// sameValue is an exact repeat: the same text with the same parameters. A
// value written differently is another value, and a set keeps both.
func sameValue(a, b model.RawValue) bool {
	if a.Text != b.Text || len(a.Params) != len(b.Params) {
		return false
	}
	for k, v := range a.Params { // the model keeps parameter names upper case
		if w, ok := b.Params[k]; !ok || w != v {
			return false
		}
	}
	return true
}

func hasAlarm(have []cal.Alarm, a cal.Alarm) bool {
	for _, h := range have {
		if h.Trigger == a.Trigger && h.RelatedEnd == a.RelatedEnd && h.Absolute == a.Absolute {
			return true
		}
	}
	return false
}
