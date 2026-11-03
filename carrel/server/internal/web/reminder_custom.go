package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/view"
)

// A reminder of its own (Typecase's task 156, s-agenda-19; the owner's
// decision of 12.10.2026): «Custom…» in a reminder's list opens how long
// before and in what — minutes, hours, days, weeks —, and a record of a
// whole day also at what hour («the day before at 09:00» is 1, days before,
// 09:00). A reminder another client wrote at an offset none of the steps
// is comes into the form as that same row, «Custom…» with its numbers,
// rather than as one more item of the list. The row's value stays the
// trigger: «custom» is only what the list sends, and the form's numbers
// become the trigger as the form is read.

// reminderCustom is the value of «Custom…» in a reminder's list.
const reminderCustom = "custom"

// reminderUnits are the units of «Custom…», as the frame lists them.
var reminderUnits = []struct {
	value, label string
	d            time.Duration
}{
	{"minutes", "minutes before", time.Minute}, {"hours", "hours before", time.Hour},
	{"days", "days before", 24 * time.Hour}, {"weeks", "weeks before", 7 * 24 * time.Hour},
}

// customOf is a trigger as «Custom…» shows it: how long before, in what,
// and, for a whole day, at what hour of the day before. False for an
// absolute trigger, one after the start, and one that is not whole minutes.
func customOf(trigger string, allDay bool) (count, unit, at string, ok bool) {
	d, ok := cal.NewAlarm(trigger).Offset()
	if !ok || d > 0 || d%time.Minute != 0 {
		return "", "", "", false
	}
	before := -d
	if allDay && before%(24*time.Hour) != 0 {
		// The day before at an hour: the whole days before the day's
		// midnight, and the hour of the last of them.
		days := before/(24*time.Hour) + 1
		clock := days*24*time.Hour - before
		return strconv.Itoa(int(days)), "days", fmt.Sprintf("%02d:%02d", int(clock/time.Hour), int(clock%time.Hour/time.Minute)), true
	}
	for i := len(reminderUnits) - 1; i >= 0; i-- {
		u := reminderUnits[i]
		if before%u.d == 0 {
			return strconv.Itoa(int(before / u.d)), u.value, "", true
		}
	}
	return "", "", "", false
}

// customTrigger is the trigger of the numbers of «Custom…»; false with what
// is wrong.
func customTrigger(count, unit, at string, allDay bool) (string, string) {
	n, err := strconv.Atoi(strings.TrimSpace(count))
	if err != nil || n < 0 {
		return "", "How long: enter a whole number, 0 or more."
	}
	var per time.Duration
	for _, u := range reminderUnits {
		if u.value == unit {
			per = u.d
		}
	}
	if per == 0 {
		return "", "Unit: choose minutes, hours, days or weeks before."
	}
	d := -time.Duration(n) * per
	if allDay && strings.TrimSpace(at) != "" {
		clock, err := time.Parse("15:04", strings.TrimSpace(at))
		if err != nil {
			return "", "At: enter a time as HH:MM, for example 09:00."
		}
		d += time.Duration(clock.Hour())*time.Hour + time.Duration(clock.Minute())*time.Minute
	}
	return icalDuration(d), ""
}

// icalDuration writes a duration the way TRIGGER holds one: «-P2D»,
// «-PT15H», «-P1W», «PT0S».
func icalDuration(d time.Duration) string {
	sign := ""
	if d < 0 {
		sign, d = "-", -d
	}
	if d == 0 {
		return "PT0S"
	}
	const day = 24 * time.Hour
	if d%(7*day) == 0 {
		return fmt.Sprintf("%sP%dW", sign, d/(7*day))
	}
	out := sign + "P"
	if days := d / day; days > 0 {
		out += strconv.Itoa(int(days)) + "D"
	}
	if rest := d % day; rest > 0 {
		out += "T"
		if h := rest / time.Hour; h > 0 {
			out += strconv.Itoa(int(h)) + "H"
		}
		if m := rest % time.Hour / time.Minute; m > 0 {
			out += strconv.Itoa(int(m)) + "M"
		}
	}
	return out
}

// isPreset says whether a trigger is one of the steps of the list.
func isPreset(trigger string) bool {
	for _, p := range reminderPresets {
		if p.value == trigger {
			return true
		}
	}
	return false
}

// formAllDay says whether the record the form holds is a whole day: an
// event's All day, a task's one box. The times are read before the
// reminders (editFields), so the values already hold them.
func formAllDay(kind view.Kind, vals formValues) bool {
	switch kind {
	case view.KindEvent:
		return triple(vals["start"])[2] == "on"
	case view.KindTask:
		return first(vals[taskDayBox]) == "on"
	}
	return false
}

// customRead turns the rows of the reminder field name that chose
// «Custom…» into their triggers. The numbers are read only for such a row —
// a row of a step sends them hidden without a script, and they say nothing
// then. A row whose numbers do not make a trigger stays «custom», and what
// was typed is kept under name.custom for the form to come back with; the
// triggers the numbers made are named under name.made — a trigger sent
// as the row's value itself is still only a step or one the record has.
// «At» is read wherever it was sent: the numbers mean what they said in the
// form they were typed in, and a whole day made a timed event in the same
// form keeps the moment its reminder rang at («1, days, 23:45» is a quarter
// of an hour before).
func customRead(vals formValues, name string, r *http.Request, allDay bool) {
	rows := vals[name]
	raw := make([]string, len(rows))
	var made []string
	for i, v := range rows {
		if v != reminderCustom {
			continue
		}
		key := fmt.Sprintf("%s.%d", name, i)
		count, unit, at := strings.TrimSpace(r.FormValue(key+".count")), r.FormValue(key+".unit"), strings.TrimSpace(r.FormValue(key+".at"))
		if trig, bad := customTrigger(count, unit, at, allDay || at != ""); bad == "" {
			rows[i] = trig
			made = append(made, trig)
			continue
		}
		raw[i] = count + "\x1f" + unit + "\x1f" + at
	}
	vals[name+".custom"], vals[name+".made"] = raw, made
}

// customNumbers are the numbers a reminder row shows under «Custom…»: what
// the form sent when it came back with a reason (raw), else the trigger's,
// else none — a row of a step holds its fields empty, and hidden.
func customNumbers(v, raw string, allDay bool) (count, unit, at string, custom bool) {
	if v == reminderCustom {
		p := strings.SplitN(raw, "\x1f", 3)
		for len(p) < 3 {
			p = append(p, "")
		}
		return p[0], p[1], p[2], true
	}
	if v == "" || isPreset(v) {
		unit = "minutes"
		if allDay {
			unit = "days"
		}
		return "", unit, "", false
	}
	count, unit, at, ok := customOf(v, allDay)
	return count, unit, at, ok
}

// customRows are the fields «Custom…» of the reminder row name opens: How
// long, Unit and, for a whole day, At — hidden while a step is chosen.
func customRows(name, count, unit, at string, allDay, shown bool) []view.FormRow {
	units := make([]view.Option, 0, len(reminderUnits))
	for _, u := range reminderUnits {
		units = append(units, view.Option{Value: u.value, Label: text(u.label), Selected: u.value == unit})
	}
	rows := []view.FormRow{
		{Name: name + ".count", Label: "How long", Kind: view.InputNumber, Value: text(count), Mono: true, Hidden: !shown},
		{Name: name + ".unit", Label: "Unit", Kind: view.InputSelect, Value: text(unit), Options: units, Hidden: !shown},
	}
	if allDay {
		rows = append(rows, view.FormRow{Name: name + ".at", Label: "At", Kind: view.InputTime, Value: text(at), Mono: true, Hidden: !shown})
	}
	return rows
}

// customShows is what «Custom…» of the row name shows (Option.Shows).
func customShows(rows []view.FormRow) string {
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.Name)
	}
	return strings.Join(names, " ")
}

// reminderWords says a reminder the way a card does: a whole day's at an
// hour of a day before is «1 day before, 09:00» (s-agenda-19), the rest as
// the clock says it.
func (f timeFmt) reminderWords(a cal.Alarm, allDay bool) string {
	if allDay && !a.Absolute && !a.RelatedEnd {
		if count, unit, at, ok := customOf(a.Trigger, true); ok && at != "" && unit == "days" {
			n, _ := strconv.Atoi(count)
			return plural(n, "day") + " before, " + at
		}
	}
	return f.reminder(a)
}
