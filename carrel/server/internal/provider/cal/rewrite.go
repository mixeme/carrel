package cal

import (
	"bytes"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// addressParams are the parameters of ORGANIZER and ATTENDEE that name a
// person by address (RFC 5545 §3.2.18, §3.2.4, §3.2.5). The last two hold a
// list.
var addressParams = []string{ical.ParamSentBy, ical.ParamDelegatedFrom, ical.ParamDelegatedTo}

// RewriteAddresses points every ORGANIZER and ATTENDEE of a record that names
// an address in pairs — as its value or in SENT-BY, DELEGATED-FROM,
// DELEGATED-TO — at the address it is paired with (S-15.4). pairs maps an
// old address, lower-cased and without mailto:, to the new one.
//
// Every component of the record's UID is rewritten: the main one and every
// overridden occurrence beside it, which carries its own ORGANIZER and
// ATTENDEE — a person added to one Thursday is named nowhere else. The patch
// language reaches only the main component (architecture.md), so this is a
// document edit, as SetAlarms is. What else a line holds — CN, PARTSTAT,
// ROLE, RSVP — stays, and so does the scheme as it was written: MAILTO: stays
// MAILTO:. obj is not changed; n is how many addresses were replaced, and 0
// hands obj back.
func RewriteAddresses(obj *model.Object, pairs map[string]string) (*model.Object, int, error) {
	if _, err := mainComponent(obj); err != nil {
		return nil, 0, err
	}
	doc, err := decode(obj.CalendarDoc())
	if err != nil {
		return nil, 0, err
	}
	main := findMain(doc)
	main.Props = rawToProps(obj.Props())
	uid := propText(main.Props, ical.PropUID)
	to := make(map[string]string, len(pairs))
	for from, into := range pairs {
		to[strings.ToLower(strings.TrimSpace(from))] = into
	}
	n := 0
	for _, c := range doc.Children {
		if c.Name != main.Name {
			continue
		}
		if c != main {
			if c.Props.Get(ical.PropRecurrenceID) == nil {
				continue
			}
			if u := propText(c.Props, ical.PropUID); u != "" && u != uid {
				continue
			}
		}
		n += rewriteProps(c.Props, to)
	}
	if n == 0 {
		return obj, 0, nil
	}
	var buf bytes.Buffer
	if err := encodeCalendar(&buf, doc); err != nil {
		return nil, 0, err
	}
	out, err := Parse(obj.Path, obj.ETag, bytes.NewReader(model.FoldLines(buf.Bytes())))
	if err != nil {
		return nil, 0, err
	}
	return out, n, nil
}

func rewriteProps(props ical.Props, to map[string]string) int {
	n := 0
	for _, name := range []string{ical.PropOrganizer, ical.PropAttendee} {
		for i := range props[name] {
			p := &props[name][i]
			if v, ok := swapAddress(p.Value, to); ok {
				p.Value = v
				n++
			}
			for pname, values := range p.Params {
				if !isAddressParam(pname) {
					continue
				}
				for j, v := range values {
					parts := strings.Split(v, ",")
					changed := false
					for k, part := range parts {
						if w, ok := swapAddress(part, to); ok {
							parts[k] = w
							changed = true
							n++
						}
					}
					if changed {
						values[j] = strings.Join(parts, ",")
					}
				}
			}
		}
	}
	return n
}

func isAddressParam(name string) bool {
	for _, p := range addressParams {
		if strings.EqualFold(name, p) {
			return true
		}
	}
	return false
}

// swapAddress is v with its address replaced, the scheme kept as written.
func swapAddress(v string, to map[string]string) (string, bool) {
	trimmed := strings.TrimSpace(v)
	addr := CalAddress(trimmed)
	into, ok := to[strings.ToLower(addr)]
	if !ok || addr == "" {
		return v, false
	}
	return trimmed[:len(trimmed)-len(addr)] + into, true
}

// Ended reports a record every occurrence of which began before now: a
// meeting that is history (S-15.4). A series without an end never ends; an
// overridden occurrence moved past now keeps the record alive. A record
// without a start — a task with no dates — has not ended.
func Ended(obj *model.Object, now time.Time) bool {
	comp, err := loadMain(obj)
	if err != nil || comp == nil {
		return false
	}
	startProp := ical.PropDateTimeStart
	if comp.Props.Get(startProp) == nil && comp.Name == ical.CompToDo {
		startProp = ical.PropDue
	}
	p := comp.Props.Get(startProp)
	if p == nil {
		return false
	}
	start, err := propTime(p, time.UTC)
	if err != nil {
		return false
	}
	opt, err := comp.Props.RecurrenceRule()
	if err != nil {
		return false
	}
	if opt != nil && opt.Until.IsZero() && opt.Count == 0 {
		return false
	}
	if ov, err := Overrides(obj, time.UTC); err == nil {
		for _, o := range ov {
			if !o.Timing.Start.Before(now) {
				return false
			}
		}
	}
	set, err := seriesOf(comp, time.UTC)
	switch {
	case err != nil:
		return false
	case set == nil:
		return start.Before(now)
	}
	return set.After(now, true).IsZero()
}
