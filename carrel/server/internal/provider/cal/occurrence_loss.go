package cal

import (
	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// occurrenceKept compares the occurrence itself, not only the instant its
// RECURRENCE-ID names. UID and server stamps may change on creation; the
// person's fields, unknown properties and nested reminders must survive.
func occurrenceKept(sent, got *ical.Component) bool {
	if sent == nil || got == nil {
		return false
	}
	return model.CompareAfterCreate("", occurrenceObject(sent), occurrenceObject(got), nil) == nil &&
		missingAlarms(componentAlarms(sent), componentAlarms(got)) == 0
}

func occurrenceObject(c *ical.Component) *model.Object {
	props := propsToRaw(c.Props, nil)
	// A server may return a zoned date-time in UTC. Retain other parameters
	// (notably RECURRENCE-ID's RANGE); a DATE remains different from a time.
	for _, name := range []string{"RECURRENCE-ID", "DTSTART", "DTEND", "DUE"} {
		for i, v := range props[name] {
			if s, ok := readStamp(v.Text, v.Params); ok {
				if s.day {
					v.Text = s.instant().Format("20060102")
				} else if s.loc != nil {
					v.Text = s.instant().UTC().Format("20060102T150405Z")
				}
				if s.day || s.loc != nil {
					delete(v.Params, "TZID")
				}
				delete(v.Params, "VALUE")
				props[name][i] = v
			}
		}
	}
	// Keep the existing model comparison of equivalent event ends (an
	// omitted zero-length DTEND) available for the occurrence as well.
	return model.NewCalendarObject("", "", c.Name, props, nil)
}
