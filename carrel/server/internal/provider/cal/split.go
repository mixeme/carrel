package cal

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// Cutting a file into records and copying records out (S-19). An imported
// stream holds whatever the other program wrote: several VCALENDAR objects, a
// series with the instances that override it, the time zones they name, and
// components that are no record at all.

// ProdID is what Carrel signs a calendar document it assembles with.
const ProdID = "-//Carrel//Carrel//EN"

// Stray is a component of an imported stream that is no record Carrel keeps.
// It is named in the report of the import, never dropped in silence (S-19).
type Stray struct {
	Component string // VFREEBUSY, VEVENT, …
	UID       string
	Summary   string
	Reason    string
	// Broken — a record of a kind Carrel keeps that could not be read: a
	// time or a rule it cannot read, said with its line (s-io-7: «line 88:
	// cannot read DUE «2026-13-01»»). The file's other records are read all
	// the same.
	Broken bool
}

// Chunk is one top-level component of a document, as it came: an export
// writes one envelope of its own and copies these bytes into it.
type Chunk struct {
	Name string
	TZID string // the zone it defines, when it is a VTIMEZONE
	Body []byte
}

// chunk is one top-level component while the file is being read; at is the
// index of its first line in the file, for the line a refusal names.
type chunk struct {
	name  string
	lines []string
	at    int
}

// prop is the value of one of the component's own properties, raw.
func (c chunk) prop(name string) string {
	for _, line := range logicalLines(ownLines(c.lines)) {
		if n, value, ok := propOf(line); ok && n == name {
			return value
		}
	}
	return ""
}

// Split reads a stream of iCalendar documents into one object per record:
// every VEVENT, VTODO and VJOURNAL master with the instances that override it
// and the time zones it names. A record without UID or DTSTAMP — both
// required (RFC 5545 §3.6) — is given them here; no server would take it
// otherwise.
func Split(doc []byte, now time.Time) ([]*model.Object, []Stray, error) {
	lines := rawLines(doc)
	var objs []*model.Object
	var strays []Stray
	found := false
	for i := 0; i < len(lines); {
		name, ok := beginName(lines[i])
		if !ok || name != ical.CompCalendar {
			i++
			continue
		}
		found = true
		end := matchingEnd(lines, i)
		got, bad, err := splitCalendar(lines[i+1:end], i+1, now)
		if err != nil {
			return nil, nil, err
		}
		objs, strays = append(objs, got...), append(strays, bad...)
		i = end + 1
	}
	if !found {
		return nil, nil, fmt.Errorf("cal: the file holds no VCALENDAR")
	}
	return objs, strays, nil
}

// Components splits a calendar document into the components under its
// VCALENDAR envelope. Nothing is decoded, so nothing is lost on the way out.
func Components(doc []byte) []Chunk {
	lines := rawLines(doc)
	var out []Chunk
	for i := 0; i < len(lines); {
		name, ok := beginName(lines[i])
		if !ok || name != ical.CompCalendar {
			i++
			continue
		}
		end := matchingEnd(lines, i)
		_, comps := partition(lines[i+1:end], i+1)
		for _, c := range comps {
			out = append(out, Chunk{
				Name: c.name, TZID: c.prop(ical.PropTimezoneID),
				Body: []byte(strings.Join(c.lines, "\r\n") + "\r\n"),
			})
		}
		i = end + 1
	}
	return out
}

// SetUID gives a record a new identity. Every component of the document gets
// it at once: an instance points at the series it overrides by UID, and a
// series whose instances kept the old one would be two records, not one.
func SetUID(obj *model.Object, uid string) (*model.Object, error) {
	if obj == nil || !obj.IsCalendar() {
		return nil, fmt.Errorf("cal: not a calendar object")
	}
	out, replaced := replaceProp(rawLines(obj.CalendarDoc()), ical.PropUID, uid)
	if replaced == 0 {
		return nil, fmt.Errorf("cal: the record has no UID to change")
	}
	return Parse(obj.Path, obj.ETag, strings.NewReader(strings.Join(out, "\r\n")+"\r\n"))
}

// group is one record of an imported file: its master component and the
// instances that override it.
type group struct {
	uid       string
	master    *chunk
	overrides []chunk
}

// splitCalendar makes the records of one VCALENDAR body. Components of one
// UID travel together, and the envelope of the file travels with each record.
func splitCalendar(body []string, base int, now time.Time) ([]*model.Object, []Stray, error) {
	envelope, comps := partition(body, base)
	zones := map[string]chunk{}
	groups := map[string]*group{}
	var order []string
	var strays []Stray

	for _, c := range comps {
		switch {
		case c.name == ical.CompTimezone:
			if tzid := c.prop(ical.PropTimezoneID); tzid != "" {
				if _, seen := zones[tzid]; !seen {
					zones[tzid] = c
				}
			}
		case isMainKind(c.name):
			uid := c.prop(ical.PropUID)
			key := c.name + "\x00" + uid
			if uid == "" {
				key = fmt.Sprintf("%s\x00%d", c.name, len(order))
			}
			g := groups[key]
			if g == nil {
				g = &group{uid: uid}
				groups[key], order = g, append(order, key)
			}
			if c.prop(ical.PropRecurrenceID) != "" {
				g.overrides = append(g.overrides, c)
				continue
			}
			if g.master != nil {
				// Two masters under one UID: the second is its own record,
				// and the import gives it an identity of its own.
				key = fmt.Sprintf("%s\x00%d", c.name, len(order))
				g = &group{uid: uid}
				groups[key], order = g, append(order, key)
			}
			g.master = &chunk{name: c.name, lines: c.lines, at: c.at}
		default:
			strays = append(strays, Stray{
				Component: c.name, UID: c.prop(ical.PropUID), Summary: textOf(c.prop(ical.PropSummary)),
				Reason: "Carrel keeps no records of this kind.",
			})
		}
	}

	var objs []*model.Object
	for _, key := range order {
		g := groups[key]
		if g.master == nil {
			for _, c := range g.overrides {
				strays = append(strays, Stray{
					Component: c.name, UID: g.uid, Summary: textOf(c.prop(ical.PropSummary)),
					Reason: "One occurrence of a series this file does not hold.",
				})
			}
			continue
		}
		// A record that cannot be read is refused alone, with the line that
		// says why (Typecase's notes 111 p. 5, 113; s-io-7); the file's
		// other records are read all the same.
		err := checkTimes(*g.master)
		for _, c := range g.overrides {
			if err == nil {
				err = checkTimes(c)
			}
		}
		var obj *model.Object
		if err == nil {
			obj, err = g.object(envelope, zones, now)
		}
		if err != nil {
			strays = append(strays, Stray{
				Component: g.master.name, UID: g.uid, Summary: textOf(g.master.prop(ical.PropSummary)),
				Reason: err.Error(), Broken: true,
			})
			continue
		}
		objs = append(objs, obj)
	}
	return objs, strays, nil
}

// object assembles the record's own document: the envelope of the file it
// came from, the time zones its components name, and those components, line
// for line.
func (g *group) object(envelope []string, zones map[string]chunk, now time.Time) (*model.Object, error) {
	comps := append([]chunk{*g.master}, g.overrides...)
	var named []string
	seen := map[string]bool{}
	for _, c := range comps {
		for _, tzid := range timezonesOf(c.lines) {
			if _, ok := zones[tzid]; ok && !seen[tzid] {
				seen[tzid], named = true, append(named, tzid)
			}
		}
	}
	sort.Strings(named)

	out := append([]string{"BEGIN:" + ical.CompCalendar}, envelopeProps(envelope)...)
	for _, tzid := range named {
		out = append(out, zones[tzid].lines...)
	}
	for _, c := range comps {
		out = append(out, fill(c, g.uid, now)...)
	}
	out = append(out, "END:"+ical.CompCalendar)
	return Parse("", "", strings.NewReader(strings.Join(out, "\r\n")+"\r\n"))
}

// fill adds what every record must carry and the file left out: a UID — the
// group's, so instances stay with their series — and a DTSTAMP.
func fill(c chunk, uid string, now time.Time) []string {
	var add []string
	if !hasProp(c.lines, ical.PropUID) {
		if uid == "" {
			uid, _ = model.NewUID()
		}
		add = append(add, ical.PropUID+":"+uid)
	}
	if !hasProp(c.lines, ical.PropDateTimeStamp) {
		add = append(add, ical.PropDateTimeStamp+":"+now.UTC().Format("20060102T150405Z"))
	}
	if len(add) == 0 {
		return c.lines
	}
	out := append([]string{c.lines[0]}, add...)
	return append(out, c.lines[1:]...)
}

// envelopeProps are the calendar-level properties of the file, plus the two
// the encoder insists on when it carried neither.
func envelopeProps(envelope []string) []string {
	out := append([]string(nil), envelope...)
	if !hasLineProp(out, ical.PropVersion) {
		out = append(out, ical.PropVersion+":2.0")
	}
	if !hasLineProp(out, ical.PropProductID) {
		out = append(out, ical.PropProductID+":"+ProdID)
	}
	return out
}

// hasLineProp reports a property among lines, whatever their nesting.
func hasLineProp(lines []string, name string) bool {
	for _, line := range logicalLines(lines) {
		if n, _, ok := propOf(line); ok && n == name {
			return true
		}
	}
	return false
}

// partition cuts the top-level components out of a component body; what is
// left are its own property lines.
func partition(body []string, base int) (props []string, comps []chunk) {
	for i := 0; i < len(body); {
		name, ok := beginName(body[i])
		if !ok {
			if !isEnd(body[i]) {
				props = append(props, body[i])
			}
			i++
			continue
		}
		stop := matchingEnd(body, i) + 1
		if stop > len(body) {
			stop = len(body)
		}
		comps = append(comps, chunk{name: name, lines: body[i:stop], at: base + i})
		i = stop
	}
	return props, comps
}

func isMainKind(name string) bool {
	for _, k := range mainKinds {
		if name == k {
			return true
		}
	}
	return false
}

// textOf unescapes a TEXT value for the report.
func textOf(raw string) string {
	p := ical.NewProp(ical.PropSummary)
	p.Value = raw
	if s, err := p.Text(); err == nil {
		return s
	}
	return raw
}

// timezonesOf are the zones the TZID parameters of a component name.
func timezonesOf(lines []string) []string {
	var out []string
	for _, line := range logicalLines(lines) {
		colon, semi := strings.IndexByte(line, ':'), strings.IndexByte(line, ';')
		if semi < 0 || colon < 0 || semi > colon {
			continue
		}
		for _, p := range strings.Split(line[semi+1:colon], ";") {
			k, v, ok := strings.Cut(p, "=")
			if ok && strings.EqualFold(strings.TrimSpace(k), ical.ParamTimezoneID) {
				out = append(out, strings.Trim(strings.TrimSpace(v), `"`))
			}
		}
	}
	return out
}
