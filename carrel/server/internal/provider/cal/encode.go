package cal

import (
	"bytes"
	"io"

	"github.com/emersion/go-ical"
)

// A component may hold more than one RRULE: RFC 5545 says it SHOULD NOT,
// and some clients write two. go-ical refuses to write such a VEVENT («want
// at most one "RRULE" property»), so an event another client wrote so
// could not be saved by any edit (the check of the record's audit,
// 14.11.2026). Carrel writes every rule as it came: while go-ical writes
// the document, the second and later rules of a component stand under a
// name of Carrel's own, and the lines go out under RRULE again.

// extraRule is that name; no server ever sees it.
const extraRule = "X-CARREL-EXTRA-RRULE"

// encodeCalendar writes cal as go-ical does, its components' second and
// later rules included. cal is left as it was.
func encodeCalendar(w io.Writer, cal *ical.Calendar) error {
	type held struct {
		c     *ical.Component
		rules []ical.Prop
	}
	var moved []held
	var walk func(c *ical.Component)
	walk = func(c *ical.Component) {
		if rules := c.Props[ical.PropRecurrenceRule]; len(rules) > 1 {
			moved = append(moved, held{c, rules})
			extra := make([]ical.Prop, 0, len(rules)-1)
			for _, p := range rules[1:] {
				p.Name = extraRule
				extra = append(extra, p)
			}
			c.Props[ical.PropRecurrenceRule] = rules[:1]
			c.Props[extraRule] = extra
		}
		for _, child := range c.Children {
			walk(child)
		}
	}
	walk(cal.Component)
	defer func() {
		for _, m := range moved {
			m.c.Props[ical.PropRecurrenceRule] = m.rules
			delete(m.c.Props, extraRule)
		}
	}()
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return err
	}
	out := buf.Bytes()
	if len(moved) > 0 {
		out = bytes.ReplaceAll(out, []byte("\r\n"+extraRule), []byte("\r\n"+ical.PropRecurrenceRule))
	}
	_, err := w.Write(out)
	return err
}
