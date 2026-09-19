package cal

import (
	"bytes"
	"fmt"
	"time"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// A TZID names a VTIMEZONE of the same document (RFC 5545 §3.2.19): a record
// whose time is moved to another zone carries that zone's definition with
// it. Carrel builds it from the zone database the server runs with, for the
// year the record starts in, with the yearly rule of its changes.

// EnsureTimezone gives obj's document a VTIMEZONE for loc when it has none
// of that TZID, and returns the record to Update. Like SetAlarms it is a
// document edit: the other components stay, and the properties already
// patched on obj are carried over. year is the year the definition is taken
// from.
func EnsureTimezone(obj *model.Object, loc *time.Location, year int) (*model.Object, error) {
	if _, err := mainComponent(obj); err != nil {
		return nil, err
	}
	cal, err := decode(obj.CalendarDoc())
	if err != nil {
		return nil, err
	}
	for _, child := range cal.Children {
		if child.Name == ical.CompTimezone && child.Props.Get(ical.PropTimezoneID) != nil &&
			child.Props.Get(ical.PropTimezoneID).Value == loc.String() {
			return obj, nil
		}
	}
	main := findMain(cal)
	main.Props = rawToProps(obj.Props())
	cal.Children = append([]*ical.Component{Timezone(loc, year)}, cal.Children...)
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, err
	}
	return model.NewCalendarObject(obj.Path, obj.ETag, main.Name, obj.Props(), buf.Bytes()), nil
}

// Timezone is a VTIMEZONE for loc as it stands in year: one STANDARD when the
// zone does not change its offset that year, otherwise one observance per
// change with its yearly rule (FREQ=YEARLY;BYMONTH;BYDAY).
func Timezone(loc *time.Location, year int) *ical.Component {
	tz := ical.NewComponent(ical.CompTimezone)
	setRaw(tz.Props, ical.PropTimezoneID, loc.String())
	changes := offsetChanges(loc, year)
	if len(changes) == 0 {
		at := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
		name, off := at.Zone()
		tz.Children = append(tz.Children, observance(ical.CompTimezoneStandard, "19700101T000000", off, off, name, ""))
		return tz
	}
	for _, at := range changes {
		before := at.Add(-time.Second)
		_, from := before.Zone()
		name, to := at.Zone()
		kind := ical.CompTimezoneStandard
		if at.IsDST() {
			kind = ical.CompTimezoneDaylight
		}
		// DTSTART is the wall clock the change happens at, in the offset
		// before it (RFC 5545 §3.6.5).
		wall := at.In(time.FixedZone("", from)).Format("20060102T150405")
		tz.Children = append(tz.Children, observance(kind, wall, from, to, name, yearlyRule(at.In(time.FixedZone("", from)))))
	}
	return tz
}

func observance(kind, start string, from, to int, name, rule string) *ical.Component {
	c := ical.NewComponent(kind)
	setRaw(c.Props, ical.PropDateTimeStart, start)
	setRaw(c.Props, ical.PropTimezoneOffsetFrom, utcOffset(from))
	setRaw(c.Props, ical.PropTimezoneOffsetTo, utcOffset(to))
	if name != "" {
		setRaw(c.Props, ical.PropTimezoneName, name)
	}
	if rule != "" {
		setRaw(c.Props, ical.PropRecurrenceRule, rule)
	}
	return c
}

// setRaw sets a value as it is written, with no VALUE parameter: each of
// these has its type by default.
func setRaw(props ical.Props, name, value string) {
	p := ical.NewProp(name)
	p.Value = value
	props.Set(p)
}

// utcOffset is an offset as RFC 5545 §3.3.14 writes it: +0300, -0430.
func utcOffset(sec int) string {
	sign := '+'
	if sec < 0 {
		sign, sec = '-', -sec
	}
	if sec%60 != 0 {
		return fmt.Sprintf("%c%02d%02d%02d", sign, sec/3600, sec/60%60, sec%60)
	}
	return fmt.Sprintf("%c%02d%02d", sign, sec/3600, sec/60%60)
}

// yearlyRule names the day of a change the way zones name it: the n-th or
// the last weekday of its month.
func yearlyRule(day time.Time) string {
	days := []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}
	n := (day.Day()-1)/7 + 1
	if day.AddDate(0, 0, 7).Month() != day.Month() {
		n = -1
	}
	return fmt.Sprintf("FREQ=YEARLY;BYMONTH=%d;BYDAY=%d%s", int(day.Month()), n, days[day.Weekday()])
}

// offsetChanges are the instants loc changes its offset in year, found a day
// at a time and then to the second.
func offsetChanges(loc *time.Location, year int) []time.Time {
	var out []time.Time
	start := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
	end := time.Date(year+1, 1, 1, 0, 0, 0, 0, loc)
	_, prev := start.Zone()
	for t := start; t.Before(end); t = t.Add(24 * time.Hour) {
		next := t.Add(24 * time.Hour)
		if _, off := next.Zone(); off != prev {
			lo, hi := t, next
			for hi.Sub(lo) > time.Second {
				mid := lo.Add(hi.Sub(lo) / 2)
				if _, o := mid.Zone(); o == prev {
					lo = mid
				} else {
					hi = mid
				}
			}
			out = append(out, hi)
			prev = off
		}
	}
	return out
}
