package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/teambition/rrule-go"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// An edit never rebuilds a record from its form (S-7). The form shows values;
// what differs between the values it was read with and the values it came back
// with becomes patch operations on the record the server holds. A field that
// did not change produces nothing, so normalisation in the form cannot rewrite
// what another client wrote.

// formValues is a form as values: one entry per field, rows in order. A time
// field is date, clock, all-day; a list field ends with the empty "add" row.
type formValues map[string][]string

type fieldType int

const (
	fieldText fieldType = iota
	fieldArea
	fieldMulti
	fieldTime
	fieldDone
	fieldPercent
	fieldReminders
	// fieldName is Given and Family in one field: two inputs, one property.
	// N is structured — Family;Given;Middle;Prefix;Suffix — and the three
	// components no input offers come back untouched (U-6.1, S-7).
	fieldName
	// fieldList is a text list typed with commas between items: CATEGORIES,
	// one line on the way out, the items' parameters kept.
	fieldList
	// fieldBirthday is BDAY: typed as DD.MM.YYYY, or DD.MM without a year
	// (s-contacts-3), written back in the form it came in (basic or
	// extended).
	fieldBirthday
	// fieldAddress is ADR as five inputs a row: street, city, region,
	// postcode, country. PO box and extended are kept as they were.
	fieldAddress
	// fieldChoice is one value of a fixed list, written as it is: STATUS.
	// A value another client wrote is kept as a choice.
	fieldChoice
	// fieldZone is the zone DTSTART and DTEND are written in: their TZID,
	// "UTC" or "floating". The form's times stay in the instance zone (C-10).
	fieldZone
	// fieldRepeat is RRULE, edited as a whole series (S-11): Frequency,
	// Every, On, Until, Count, and the rule itself. See repeatValues.
	fieldRepeat
	// fieldRelated is the first RELATED-TO: picked from the records this
	// session has loaded for the collection, or typed as a UID (S-11).
	fieldRelated
	// fieldAttendee adds one ATTENDEE to a new event: the person a timeline
	// creates it for (S-15.4). It shows nothing of what the event holds —
	// attendees are EventEditPage.Attendees — and travels hidden.
	fieldAttendee
)

type editField struct {
	name     string
	label    string
	prop     string
	typ      fieldType
	required bool
	allDay   bool // fieldTime: the field has its own all-day box
	typed    bool // fieldMulti: each row has its label, the TYPE parameter
	// optional fields a record leaves empty go into a set of their own on
	// its full form, "Show N empty fields"; the client folds it.
	optional bool
	// timeOptional: a date without a time is a day (a note's DTSTART), and
	// there is no all-day box.
	timeOptional bool
	choices      []fieldOption
	hint         string // shown under the field, when the rule needs saying
	// set is the rubric the field stands in on the full form (s-tasks-3,
	// s-agenda-3): FieldSet.Legend; empty is the head of the form.
	set string
	// timeLabel names the time input of a fieldTime; label names its date.
	timeLabel string
	// blank is what an absent property reads as in a fieldChoice: a task
	// without STATUS needs action. The form shows it, and a save that leaves
	// it so writes nothing (S-7).
	blank string
}

// fieldOption is one value a fieldChoice offers; sep draws a line before it,
// where the list frame beside the screen draws one.
type fieldOption struct {
	value, label string
	sep          bool
}

// zoneChoices are the zones the form offers besides the one the event came
// in, UTC and the instance's own.
var zoneChoices = []string{
	"Europe/London", "Europe/Dublin", "Europe/Lisbon", "Europe/Paris", "Europe/Berlin", "Europe/Madrid", "Europe/Rome",
	"Europe/Amsterdam", "Europe/Stockholm", "Europe/Warsaw", "Europe/Prague", "Europe/Vienna", "Europe/Athens",
	"Europe/Helsinki", "Europe/Kyiv", "Europe/Istanbul", "Europe/Minsk", "Europe/Moscow", "Europe/Samara",
	"Asia/Yekaterinburg", "Asia/Omsk", "Asia/Novosibirsk", "Asia/Krasnoyarsk", "Asia/Irkutsk", "Asia/Yakutsk",
	"Asia/Vladivostok", "Asia/Magadan", "Asia/Kamchatka", "Asia/Tbilisi", "Asia/Yerevan", "Asia/Baku",
	"Asia/Almaty", "Asia/Tashkent", "Asia/Dubai", "Asia/Kolkata", "Asia/Bangkok", "Asia/Shanghai", "Asia/Singapore",
	"Asia/Tokyo", "Asia/Seoul", "Australia/Perth", "Australia/Sydney", "Pacific/Auckland",
	"America/St_Johns", "America/Halifax", "America/New_York", "America/Chicago", "America/Denver",
	"America/Phoenix", "America/Los_Angeles", "America/Anchorage", "Pacific/Honolulu",
	"America/Mexico_City", "America/Bogota", "America/Sao_Paulo", "America/Argentina/Buenos_Aires",
	"Africa/Cairo", "Africa/Johannesburg", "Africa/Lagos", "Africa/Nairobi",
}

// zoneOf is the zone a DTSTART is written in: its TZID, "UTC", "floating",
// or "" for a day, which has none.
func zoneOf(cur []model.RawValue) string {
	if len(cur) == 0 {
		return ""
	}
	v := cur[0]
	switch {
	case v.Params["VALUE"] == "DATE" || len(strings.TrimSpace(v.Text)) == len("20060102"):
		return ""
	case v.Params["TZID"] != "":
		return v.Params["TZID"]
	case strings.HasSuffix(v.Text, "Z"):
		return "UTC"
	}
	return "floating"
}

// zoneOptions are the zone the event came in, marked so, then UTC, the
// instance zone and zoneChoices.
func zoneOptions(held, selected, instance string) []view.Option {
	label := func(z string) string {
		if z == "floating" {
			return "Floating — the same clock time anywhere"
		}
		return z
	}
	// The zone it came in stands first, and a line under it (s-agenda-8).
	opts := []view.Option{{Value: held, Label: text(label(held) + " · as sent"), Selected: selected == held}}
	seen := map[string]bool{held: true}
	for _, z := range append([]string{"UTC", instance}, zoneChoices...) {
		if seen[z] {
			continue
		}
		seen[z] = true
		opts = append(opts, view.Option{Value: z, Label: text(label(z)), Selected: z == selected, Separator: len(opts) == 1})
	}
	return opts
}

// zoneOffered reports whether a zone may be chosen: the one held, or one the
// list offers and the zone database knows.
func zoneOffered(z, held, instance string) (*time.Location, bool) {
	if z == held {
		return nil, true
	}
	if z != "UTC" && z != instance && !slices.Contains(zoneChoices, z) {
		return nil, false
	}
	loc, err := time.LoadLocation(z)
	return loc, err == nil
}

// carriedFields are what a sheet or a new form may be opened with, by the
// address: a relation and an attendee.
var carriedFields = []string{"related", "attendee"}

func hasField(k view.Kind, name string) bool {
	for _, f := range editFields[k] {
		if f.name == name {
			return true
		}
	}
	return false
}

// carried puts into values what the address brings for the fields this kind
// has.
func carried(c *collectionCtx, r *http.Request, values formValues) {
	for _, name := range carriedFields {
		if v := strings.TrimSpace(r.URL.Query().Get(name)); v != "" && hasField(c.spec.kind, name) {
			values[name] = []string{v}
		}
	}
}

// carriedQuery is what of the address goes on to the sheet or form it leads
// to.
func carriedQuery(r *http.Request) string {
	q := url.Values{}
	for _, name := range carriedFields {
		if v := strings.TrimSpace(r.URL.Query().Get(name)); v != "" {
			q.Set(name, v)
		}
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// timeLabel names the time input of a time field.
func timeLabel(f editField) string {
	if f.timeLabel != "" {
		return f.timeLabel
	}
	if f.timeOptional {
		return "Time"
	}
	return f.label + " at"
}

// relatedOptions are "nothing", the record related now, and every record of
// the loaded lists of this collection but the record itself.
func relatedOptions(index []indexEntry, self, selected string) []view.Option {
	opts := []view.Option{{Value: "", Label: "Nothing", Selected: selected == ""}}
	seen := map[string]bool{self: true, "": true}
	title := map[string]string{}
	for _, e := range index {
		title[e.UID] = e.Title
	}
	if selected != "" && selected != self {
		label := selected + " · not in the lists loaded for this collection"
		if t, ok := title[selected]; ok {
			label = t
		}
		opts = append(opts, view.Option{Value: selected, Label: text(label), Selected: true})
		seen[selected] = true
	}
	for _, e := range index {
		if seen[e.UID] {
			continue
		}
		seen[e.UID] = true
		opts = append(opts, view.Option{Value: e.UID, Label: text(e.Title + " · " + string(e.Kind))})
	}
	return opts
}

// repeatDays are the days of On, in the order the form shows them.
var repeatDays = []struct{ code, label string }{
	{"MO", "Monday"}, {"TU", "Tuesday"}, {"WE", "Wednesday"}, {"TH", "Thursday"}, {"FR", "Friday"}, {"SA", "Saturday"}, {"SU", "Sunday"},
}

// The values of a repeat field, in order.
const (
	repFreq = iota
	repInterval
	repOn
	repUntil
	repCount
	repRule
)

// ruleParts are the parts of an RRULE in the order they are written.
type ruleParts [][2]string

func parseRule(raw string) ruleParts {
	var out ruleParts
	for _, part := range strings.Split(strings.TrimSpace(raw), ";") {
		if k, v, ok := strings.Cut(part, "="); ok {
			out = append(out, [2]string{strings.ToUpper(strings.TrimSpace(k)), strings.TrimSpace(v)})
		}
	}
	return out
}

func (r ruleParts) get(k string) string {
	for _, p := range r {
		if p[0] == k {
			return p[1]
		}
	}
	return ""
}

// set replaces a part where it stands, appends a new one, and takes one out
// when v is empty.
func (r ruleParts) set(k, v string) ruleParts {
	var out ruleParts
	done := false
	for _, p := range r {
		if p[0] != k {
			out = append(out, p)
			continue
		}
		if v != "" && !done {
			out = append(out, [2]string{k, v})
		}
		done = true
	}
	if !done && v != "" {
		out = append(out, [2]string{k, v})
	}
	return out
}

func (r ruleParts) String() string {
	parts := make([]string, len(r))
	for i, p := range r {
		parts[i] = p[0] + "=" + p[1]
	}
	return strings.Join(parts, ";")
}

// repeatValues are an RRULE as the form shows it: frequency, every, the plain
// days of BYDAY in the form's order, until as a day in tz, count, and the rule
// itself.
func repeatValues(raw string, tz *time.Location) []string {
	r := parseRule(raw)
	out := make([]string, 6)
	out[repFreq], out[repCount], out[repRule] = r.get("FREQ"), r.get("COUNT"), strings.TrimSpace(raw)
	if out[repFreq] != "" {
		out[repInterval] = r.get("INTERVAL")
		if out[repInterval] == "" {
			out[repInterval] = "1"
		}
	}
	days := map[string]bool{}
	for _, d := range strings.Split(r.get("BYDAY"), ",") {
		days[strings.ToUpper(d)] = true
	}
	var on []string
	for _, d := range repeatDays {
		if days[d.code] {
			on = append(on, d.code)
		}
	}
	out[repOn] = strings.Join(on, ",")
	if u := r.get("UNTIL"); len(u) == len("20060102") {
		if t, err := time.Parse("20060102", u); err == nil {
			out[repUntil] = t.Format("2006-01-02")
		}
	} else if t, err := time.Parse("20060102T150405Z", u); err == nil && tz != nil {
		out[repUntil] = t.In(tz).Format("2006-01-02")
	}
	return out
}

// repeatRule is the RRULE a changed form asks for; "" is none. A changed raw
// rule wins and is written as typed. Otherwise only the simple parts that
// changed are written into the rule as it was, in their places, so what the
// fields do not show — WKST, BYMONTHDAY, a numbered BYDAY — stays.
func repeatRule(want, have []string, allDay bool, tz *time.Location) (string, error) {
	pad := func(v []string) []string { return append(append([]string(nil), v...), make([]string, 6)...)[:6] }
	want, have = pad(want), pad(have)
	var rule string
	if want[repRule] != have[repRule] {
		rule = strings.TrimSpace(want[repRule])
	} else if want[repFreq] == "" {
		return "", nil
	} else {
		r := parseRule(have[repRule])
		if want[repFreq] != have[repFreq] {
			r = r.set("FREQ", strings.ToUpper(want[repFreq]))
		}
		if want[repInterval] != have[repInterval] {
			v := want[repInterval]
			if n, err := strconv.Atoi(v); v != "" && (err != nil || n < 1) {
				return "", errors.New("Every is a number of 1 or more.")
			}
			if v == "1" {
				v = ""
			}
			r = r.set("INTERVAL", v)
		}
		if want[repOn] != have[repOn] {
			r = r.set("BYDAY", want[repOn])
		}
		if want[repCount] != have[repCount] {
			if n, err := strconv.Atoi(want[repCount]); want[repCount] != "" && (err != nil || n < 1) {
				return "", errors.New("Count is a number of 1 or more.")
			}
			r = r.set("COUNT", want[repCount])
		}
		if want[repUntil] != have[repUntil] {
			v := ""
			if want[repUntil] != "" {
				day, err := time.ParseInLocation("2006-01-02", want[repUntil], tz)
				if err != nil {
					return "", errors.New("Enter the last day as YYYY-MM-DD.")
				}
				v = day.Format("20060102")
				if !allDay {
					// UNTIL of a timed series is UTC (RFC 5545 §3.3.10): the
					// end of that day where the form's times are.
					v = day.AddDate(0, 0, 1).Add(-time.Second).UTC().Format("20060102T150405Z")
				}
			}
			r = r.set("UNTIL", v)
		}
		rule = r.String()
	}
	if rule == "" {
		return "", nil
	}
	r := parseRule(rule)
	if r.get("COUNT") != "" && r.get("UNTIL") != "" {
		return "", errors.New("A series ends after a number of times or on a day, not both.")
	}
	if _, err := rrule.StrToROption(rule); err != nil || r.get("FREQ") == "" {
		return "", errors.New("The rule is not one RFC 5545 reads: " + rule)
	}
	return rule, nil
}

// repeatFields describe a rule the way the form's fields hold it, for the
// recurrence block beside the form.
func repeatFields(v []string) []view.Field {
	if v[repFreq] == "" {
		return nil
	}
	out := []view.Field{{Label: "Frequency", Value: text(strings.ToLower(v[repFreq]))}, {Label: "Every", Value: text(v[repInterval])}}
	if v[repOn] != "" {
		var days []string
		for _, d := range repeatDays {
			if strings.Contains(v[repOn], d.code) {
				days = append(days, d.label)
			}
		}
		out = append(out, view.Field{Label: "On", Value: text(strings.Join(days, ", "))})
	}
	switch {
	case v[repUntil] != "":
		out = append(out, view.Field{Label: "Until", Value: text(v[repUntil])})
	case v[repCount] != "":
		out = append(out, view.Field{Label: "Count", Value: text(v[repCount])})
	}
	return out
}

// repeatSet is the Repeats field set of the event form.
func repeatSet(f editField, vals []string, errText view.Text) view.FieldSet {
	v := append(append([]string(nil), vals...), make([]string, 6)...)[:6]
	// The list of s-agenda-10: a line under «Does not repeat».
	freqs := []view.Option{{Value: "", Label: "Does not repeat"}, {Value: "DAILY", Label: "Daily", Separator: true}, {Value: "WEEKLY", Label: "Weekly"},
		{Value: "MONTHLY", Label: "Monthly"}, {Value: "YEARLY", Label: "Yearly"}}
	seen := false
	for i := range freqs {
		freqs[i].Selected = freqs[i].Value == v[repFreq]
		seen = seen || freqs[i].Selected
	}
	if !seen {
		freqs = append(freqs, view.Option{Value: v[repFreq], Label: text(v[repFreq]), Selected: true})
	}
	set := view.FieldSet{Legend: text(f.label), Rows: []view.FormRow{
		{Name: f.name + ".freq", Label: "Frequency", Kind: view.InputSelect, Value: text(v[repFreq]), Options: freqs, Error: errText},
		{Name: f.name + ".interval", Label: "Every", Kind: view.InputNumber, Value: text(v[repInterval])},
	}}
	// On is one row of the days, MO to SU, as the frame draws it: each day
	// ticked goes as a value of repeat.on.
	on := map[string]bool{}
	for _, d := range strings.Split(v[repOn], ",") {
		on[d] = true
	}
	days := view.FormRow{Name: f.name + ".on", Label: "On", Kind: view.InputCheck}
	for _, d := range repeatDays {
		days.Options = append(days.Options, view.Option{Value: d.code, Label: text(d.code), Selected: on[d.code]})
	}
	set.Rows = append(set.Rows, days,
		view.FormRow{Name: f.name + ".until", Label: "Until", Kind: view.InputDate, Value: text(v[repUntil])},
		view.FormRow{Name: f.name + ".count", Label: "Count", Kind: view.InputNumber, Value: text(v[repCount]), Hint: "Until or Count, not both."},
		view.FormRow{Name: f.name + ".rule", Label: "Advanced RRULE", Kind: view.InputText, Value: text(v[repRule]),
			Hint: "Changed here, the rule is written as typed and wins over the fields above. Exceptions and moved instances are not edited: the series is edited as a whole."},
	)
	return set
}

// The lists below are the list frames beside their screens (from-typecase
// note 6), in their order, with their lines.

// taskStatuses are the STATUS values of a task (RFC 5545 §3.8.1.11,
// s-tasks-8). There is no «no status»: a task without one needs action.
var taskStatuses = []fieldOption{{value: "NEEDS-ACTION", label: "Needs action"}, {value: "IN-PROCESS", label: "In process"},
	{value: "COMPLETED", label: "Completed"}, {value: "CANCELLED", label: "Cancelled"}}

// priorities are PRIORITY as three steps (RFC 5545 §3.8.1.9: 1 is the
// highest, 9 the lowest), each with its number, which the card and other
// clients show (s-tasks-9); another number stays a choice.
var priorities = []fieldOption{{value: "", label: "No priority"}, {value: "1", label: "High · 1", sep: true},
	{value: "5", label: "Medium · 5"}, {value: "9", label: "Low · 9"}}

// eventStatuses are the STATUS values of an event (RFC 5545 §3.8.1.11,
// s-agenda-7). Most events carry none, and that is not Confirmed.
var eventStatuses = []fieldOption{{value: "", label: "No status"}, {value: "TENTATIVE", label: "Tentative", sep: true},
	{value: "CONFIRMED", label: "Confirmed"}, {value: "CANCELLED", label: "Cancelled"}}

// statusLabel is a STATUS as its list names it; a value no list offers is
// shown as written.
func statusLabel(v string) string {
	for _, c := range append(append([]fieldOption(nil), taskStatuses...), eventStatuses...) {
		if c.value != "" && strings.EqualFold(c.value, v) {
			return c.label
		}
	}
	return strings.ToLower(v)
}

// editFields are the fields of each form in the order of its frame: the task
// and the event are laid out in rubrics (set), s-tasks-3 and s-agenda-3.
// What a frame does not draw and the owner kept — the task's start time, its
// all-day boxes, Done and reminders — stands in the rubric it belongs to
// (to-typecase task 13).
var editFields = map[view.Kind][]editField{
	view.KindContact: {
		{name: "fn", label: "Display name", prop: "FN", typ: fieldText,
			hint: "Left empty, it is made from Given and Family. Fill it alone and it is cut the other way: the first word is the given name, the rest the family name."},
		{name: "n", label: "Name", prop: "N", typ: fieldName},
		{name: "nickname", label: "Nickname", prop: "NICKNAME", typ: fieldText, optional: true},
		// Who the person is, then where they work and when they were born,
		// then how to reach them: the order of s-contacts-3.
		{name: "org", label: "Organisation", prop: "ORG", typ: fieldText, optional: true},
		{name: "title", label: "Title", prop: "TITLE", typ: fieldText, optional: true},
		{name: "bday", label: "Birthday", prop: "BDAY", typ: fieldBirthday, optional: true,
			hint: "Type it: 14.03.1987, or 14.03 without the year."},
		{name: "tel", label: "Phone", prop: "TEL", typ: fieldMulti, typed: true},
		{name: "email", label: "Email", prop: "EMAIL", typ: fieldMulti, typed: true},
		{name: "adr", label: "Address", prop: "ADR", typ: fieldAddress, set: "Addresses"},
		{name: "categories", label: "Categories", prop: "CATEGORIES", typ: fieldList, optional: true, hint: "Separated by commas."},
		{name: "note", label: "Note", prop: "NOTE", typ: fieldArea, optional: true},
	},
	view.KindEvent: {
		{name: "summary", label: "Summary", prop: "SUMMARY", typ: fieldText},
		{name: "location", label: "Location", prop: "LOCATION", typ: fieldText},
		{name: "status", label: "Status", prop: "STATUS", typ: fieldChoice, choices: eventStatuses},
		{name: "categories", label: "Categories", prop: "CATEGORIES", typ: fieldList, hint: "Separated by commas."},
		{name: "description", label: "Description", prop: "DESCRIPTION", typ: fieldArea},
		{name: "attendee", label: "Attendee", prop: "ATTENDEE", typ: fieldAttendee},
		{name: "start", label: "Start date", timeLabel: "Start time", prop: "DTSTART", typ: fieldTime, required: true, allDay: true, set: "When"},
		{name: "end", label: "End date", timeLabel: "End time", prop: "DTEND", typ: fieldTime, set: "When"},
		{name: "tz", label: "Time zone", prop: "DTSTART", typ: fieldZone, set: "When"},
		{name: "reminder", label: "Reminders", typ: fieldReminders, set: "Reminders"},
		{name: "repeat", label: "Repeats", prop: "RRULE", typ: fieldRepeat, set: "Repeats"},
	},
	view.KindTask: {
		{name: "summary", label: "Summary", prop: "SUMMARY", typ: fieldText},
		{name: "description", label: "Description", prop: "DESCRIPTION", typ: fieldArea},
		{name: "start", label: "Start date", timeLabel: "Start time", prop: "DTSTART", typ: fieldTime, allDay: true, set: "When"},
		{name: "due", label: "Due date", timeLabel: "Due time", prop: "DUE", typ: fieldTime, allDay: true, set: "When"},
		{name: "status", label: "Status", prop: "STATUS", typ: fieldChoice, choices: taskStatuses, blank: "NEEDS-ACTION", set: "Progress"},
		{name: "priority", label: "Priority", prop: "PRIORITY", typ: fieldChoice, choices: priorities, set: "Progress"},
		{name: "percent", label: "Percent complete", prop: "PERCENT-COMPLETE", typ: fieldPercent, set: "Progress"},
		{name: "done", label: "Done", prop: "STATUS", typ: fieldDone, set: "Progress"},
		{name: "reminder", label: "Reminders", typ: fieldReminders, set: "Reminders"},
		{name: "location", label: "Location", prop: "LOCATION", typ: fieldText, set: "Other"},
		{name: "categories", label: "Categories", prop: "CATEGORIES", typ: fieldList, hint: "Separated by commas.", set: "Other"},
		{name: "related", label: "Related to", prop: "RELATED-TO", typ: fieldRelated, set: "Other"},
	},
	view.KindNote: {
		{name: "summary", label: "Title", prop: "SUMMARY", typ: fieldText},
		{name: "description", label: "Body", prop: "DESCRIPTION", typ: fieldArea},
		{name: "start", label: "Date", prop: "DTSTART", typ: fieldTime, timeOptional: true},
		{name: "categories", label: "Tags", prop: "CATEGORIES", typ: fieldList, hint: "Separated by commas."},
		{name: "related", label: "Related to", prop: "RELATED-TO", typ: fieldRelated},
	},
}

// quickFieldNames are what the quick sheet holds: the few things without which
// the record does not exist, and the one or two people jot down on the move
// (U-6.16). They name fields of editFields rather than describing their own,
// so the sheet and the full form cannot drift apart.
var quickFieldNames = map[view.Kind][]string{
	view.KindContact: {"fn", "tel", "email"},
	view.KindEvent:   {"summary", "start", "end", "reminder"},
	view.KindTask:    {"summary", "due"},
	view.KindNote:    {"description"},
}

func quickFields(k view.Kind) []editField {
	var out []editField
	for _, name := range quickFieldNames[k] {
		for _, f := range editFields[k] {
			if f.name == name {
				out = append(out, f)
			}
		}
	}
	return out
}

// rowLabels are the labels a phone or an e-mail row is offered (TYPE), the
// lists of s-contacts-12 and s-contacts-13: vCard types, which another client
// names in its own words. «mobile» is TYPE=cell and «main» TYPE=main, as other
// clients write them. A label another client wrote is offered too, so keeping
// it is a choice.
var rowLabels = map[string][]fieldOption{
	"TEL": {{value: "cell", label: "mobile"}, {value: "work", label: "work"}, {value: "home", label: "home"},
		{value: "main", label: "main"}, {value: "fax", label: "fax"}, {value: "other", label: "other"}},
	"EMAIL": {{value: "work", label: "work"}, {value: "home", label: "home"}, {value: "other", label: "other"}},
}

// addressParts are the inputs of an address row: ADR components 2 to 6
// (RFC 6350 §6.3.1).
var addressParts = []struct{ name, label string }{
	{"street", "Street"}, {"city", "City"}, {"region", "Region"}, {"code", "Postcode"}, {"country", "Country"},
}

// addressRow is the five shown parts of an ADR value, one string for
// comparison: "" when every part is empty.
func addressRow(parts []string) string {
	if strings.TrimSpace(strings.Join(parts, "")) == "" {
		return ""
	}
	return strings.Join(parts, "\x1f")
}

func addressRowParts(row string) [5]string {
	var out [5]string
	copy(out[:], strings.Split(row, "\x1f"))
	return out
}

// reminderPresets are the triggers the form offers, the steps of s-agenda-9.
// A trigger another client wrote is offered too, so keeping it is always a
// choice.
var reminderPresets = []fieldOption{{value: "PT0S", label: "At the time"}, {value: "-PT5M", label: "5 minutes before"},
	{value: "-PT10M", label: "10 minutes before"}, {value: "-PT30M", label: "30 minutes before"},
	{value: "-PT1H", label: "1 hour before"}, {value: "-P1D", label: "1 day before"}}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

func triple(v []string) [3]string {
	var out [3]string
	copy(out[:], v)
	return out
}

// pair is a two-slot field: Given and Family, in that order.
func pair(v []string) [2]string {
	var out [2]string
	copy(out[:], v)
	return out
}

// sameRows compares rows, ignoring trailing empty ones.
func sameRows(a, b []string) bool {
	trim := func(v []string) []string {
		for len(v) > 0 && strings.TrimSpace(v[len(v)-1]) == "" {
			v = v[:len(v)-1]
		}
		return v
	}
	a, b = trim(a), trim(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// currentValues reads the form values of a record, and its reminders.
func (s *Server) currentValues(c *collectionCtx, obj *model.Object) (formValues, []cal.Alarm, error) {
	tz := s.tz()
	vals := formValues{}
	props := obj.Props()
	var alarms []cal.Alarm
	for _, f := range editFields[c.spec.kind] {
		switch f.typ {
		case fieldText, fieldArea:
			vals[f.name] = []string{c.text(obj, f.prop)}
		case fieldChoice:
			v := strings.TrimSpace(firstValue(obj, f.prop))
			if v == "" {
				v = f.blank
			}
			vals[f.name] = []string{v}
		case fieldZone:
			vals[f.name] = []string{zoneOf(props[f.prop])}
		case fieldRepeat:
			vals[f.name] = repeatValues(firstValue(obj, f.prop), tz)
		case fieldRelated:
			vals[f.name] = []string{strings.TrimSpace(firstValue(obj, f.prop))}
		case fieldAttendee:
			vals[f.name] = []string{""}
		case fieldList:
			vals[f.name] = []string{strings.Join(categoriesOf(c.spec.kind, obj), ", ")}
		case fieldBirthday:
			var params map[string]string
			if cur := props[f.prop]; len(cur) > 0 {
				params = cur[0].Params
			}
			vals[f.name] = []string{birthdayTyped(c.text(obj, f.prop), params)}
		case fieldName:
			p := nameParts(c.text(obj, f.prop))
			vals[f.name] = []string{p[1], p[0]}
		case fieldMulti:
			var rows, labels []string
			for _, v := range props[f.prop] {
				rows = append(rows, v.Text)
				labels = append(labels, v.Params["TYPE"])
			}
			vals[f.name] = append(rows, "")
			if f.typed {
				vals[f.name+".type"] = append(labels, "")
			}
		case fieldAddress:
			var rows []string
			for _, v := range props[f.prop] {
				p := strings.SplitN(v.Text, ";", 7)
				for len(p) < 7 {
					p = append(p, "")
				}
				rows = append(rows, addressRow(p[2:]))
			}
			vals[f.name] = append(rows, "")
		case fieldTime:
			tm, err := cal.TimingOf(obj, tz)
			if f.prop == "DUE" {
				tm, err = cal.DueOf(obj, tz)
			}
			if err != nil {
				return nil, nil, err
			}
			vals[f.name] = timeRow(f, tm, tz)
		case fieldDone:
			vals[f.name] = []string{""}
			if taskDone(obj) {
				vals[f.name] = []string{"on"}
			}
		case fieldPercent:
			vals[f.name] = []string{strings.TrimSpace(firstValue(obj, f.prop))}
		case fieldReminders:
			var err error
			if alarms, err = cal.Alarms(obj); err != nil {
				return nil, nil, err
			}
			rows := make([]string, 0, len(alarms)+1)
			for _, a := range alarms {
				rows = append(rows, a.Trigger)
			}
			vals[f.name] = append(rows, "")
		}
	}
	return vals, alarms, nil
}

func timeRow(f editField, tm cal.Timing, tz *time.Location) []string {
	at := tm.Start
	if f.name == "end" {
		at = tm.End
		if tm.AllDay && !at.IsZero() {
			// DTEND of a day is exclusive; the form shows the last day.
			at = at.AddDate(0, 0, -1)
		}
	}
	if at.IsZero() {
		return []string{"", "", ""}
	}
	row := []string{at.In(tz).Format("2006-01-02"), "", ""}
	if !tm.AllDay {
		row[1] = at.In(tz).Format("15:04")
	}
	if tm.AllDay && (f.allDay || f.timeOptional) {
		row[2] = "on"
	}
	return row
}

func (s *Server) emptyValues(c *collectionCtx) formValues {
	vals := formValues{}
	for _, f := range editFields[c.spec.kind] {
		switch f.typ {
		case fieldTime:
			vals[f.name] = []string{"", "", ""}
		case fieldName:
			vals[f.name] = []string{"", ""}
		case fieldRepeat:
			vals[f.name] = repeatValues("", nil)
		case fieldChoice:
			vals[f.name] = []string{f.blank}
		default:
			vals[f.name] = []string{""}
		}
	}
	return vals
}

func checkbox(r *http.Request, name string) string {
	if v := r.FormValue(name); v != "" && v != "off" {
		return "on"
	}
	return ""
}

// indexedRows reads name.0, name.1, … and name.new.
func indexedRows(r *http.Request, name string) []string {
	var rows []string
	for i := 0; ; i++ {
		key := name + "." + strconv.Itoa(i)
		if _, ok := r.Form[key]; !ok {
			break
		}
		rows = append(rows, strings.TrimSpace(r.FormValue(key)))
	}
	return append(rows, strings.TrimSpace(r.FormValue(name+".new")))
}

func (s *Server) submittedValues(c *collectionCtx, r *http.Request) formValues {
	vals := formValues{}
	for _, f := range editFields[c.spec.kind] {
		switch f.typ {
		case fieldText, fieldPercent, fieldList, fieldBirthday, fieldChoice, fieldZone, fieldAttendee:
			vals[f.name] = []string{strings.TrimSpace(r.FormValue(f.name))}
		case fieldArea:
			vals[f.name] = []string{strings.TrimRight(strings.ReplaceAll(r.FormValue(f.name), "\r\n", "\n"), "\n ")}
		case fieldName:
			vals[f.name] = []string{strings.TrimSpace(r.FormValue(f.name + ".given")), strings.TrimSpace(r.FormValue(f.name + ".family"))}
		case fieldMulti, fieldReminders:
			vals[f.name] = indexedRows(r, f.name)
			if f.typed {
				var labels []string
				for i := 0; i < len(vals[f.name])-1; i++ {
					labels = append(labels, strings.TrimSpace(r.FormValue(fmt.Sprintf("%s.%d.type", f.name, i))))
				}
				vals[f.name+".type"] = append(labels, strings.TrimSpace(r.FormValue(f.name+".new.type")))
			}
		case fieldRelated:
			v := strings.TrimSpace(r.FormValue(f.name + ".uid"))
			if v == "" {
				v = strings.TrimSpace(r.FormValue(f.name))
			}
			vals[f.name] = []string{v}
		case fieldRepeat:
			freq := strings.TrimSpace(r.FormValue(f.name + ".freq")) // parses the form
			var on []string
			for _, d := range repeatDays {
				if slices.Contains(r.Form[f.name+".on"], d.code) {
					on = append(on, d.code)
				}
			}
			vals[f.name] = []string{
				freq, strings.TrimSpace(r.FormValue(f.name + ".interval")),
				strings.Join(on, ","), strings.TrimSpace(r.FormValue(f.name + ".until")),
				strings.TrimSpace(r.FormValue(f.name + ".count")), strings.TrimSpace(r.FormValue(f.name + ".rule")),
			}
		case fieldAddress:
			part := func(prefix string) string {
				var parts []string
				for _, p := range addressParts {
					parts = append(parts, strings.TrimSpace(r.FormValue(prefix+"."+p.name)))
				}
				return addressRow(parts)
			}
			var rows []string
			for i := 0; ; i++ {
				prefix := fmt.Sprintf("%s.%d", f.name, i)
				if _, ok := r.Form[prefix+".street"]; !ok {
					break
				}
				rows = append(rows, part(prefix))
			}
			vals[f.name] = append(rows, part(f.name+".new"))
		case fieldTime:
			row := []string{strings.TrimSpace(r.FormValue(f.name + ".date")), strings.TrimSpace(r.FormValue(f.name + ".time")), ""}
			if f.allDay {
				row[2] = checkbox(r, f.name+".allday")
			}
			if f.timeOptional && row[0] != "" && row[1] == "" {
				row[2] = "on"
			}
			vals[f.name] = row
		case fieldDone:
			vals[f.name] = []string{checkbox(r, f.name)}
		}
	}
	return vals
}

// change is what a form asks of a record.
type change struct {
	ops           []model.PatchOp
	alarms        []cal.Alarm
	alarmsChanged bool
	// zone is a zone the record's times moved to, whose VTIMEZONE the
	// document must carry; zoneYear is the year it is taken for.
	zone     *time.Location
	zoneYear int
	errors   map[string]string
}

// buildChange turns the fields that differ between have and want into patch
// operations on obj. cur is obj's reminder set.
func (s *Server) buildChange(c *collectionCtx, obj *model.Object, cur []cal.Alarm, have, want formValues, now time.Time) change {
	tz := s.tz()
	ch := change{errors: map[string]string{}}
	props := obj.Props()
	escape := c.spec.kind != view.KindContact
	fillNames(c.spec.kind, have, want)
	startRow := triple(want["start"])
	startChanged := !sameRows(want["start"], have["start"])
	// A new zone rewrites both times at the instants the form names, even
	// when neither row changed.
	var zone string
	if held := first(have["tz"]); held != "" && first(want["tz"]) != held && startRow[2] != "on" {
		loc, ok := zoneOffered(first(want["tz"]), held, tz.String())
		if !ok {
			ch.errors["tz"] = "Choose a time zone from the list."
		} else {
			zone = first(want["tz"])
			if loc != nil && zone != "UTC" {
				ch.zone = loc
			}
		}
	}
	var startAt, endAt time.Time

	for _, f := range editFields[c.spec.kind] {
		w, h := want[f.name], have[f.name]
		switch f.typ {
		case fieldText, fieldArea:
			v := first(w)
			if f.required && strings.TrimSpace(v) == "" {
				ch.errors[f.name] = f.label + " is required."
				continue
			}
			if v == first(h) {
				continue
			}
			ch.ops = append(ch.ops, textOps(f.prop, props[f.prop], v, escape)...)
		case fieldMulti:
			var labels []string
			if f.typed {
				labels = want[f.name+".type"]
			}
			if sameRows(w, h) && (!f.typed || sameLabels(labels, have[f.name+".type"])) {
				continue
			}
			ch.ops = append(ch.ops, multiOps(f.prop, props[f.prop], w, labels)...)
		case fieldAddress:
			if sameRows(w, h) {
				continue
			}
			cur := props[f.prop]
			rows := make([]string, len(w))
			for i, row := range w {
				if row == "" {
					continue
				}
				full := make([]string, 7)
				if i < len(cur) {
					copy(full, strings.SplitN(cur[i].Text, ";", 7))
				}
				for j, p := range addressRowParts(row) {
					full[2+j] = strings.ReplaceAll(p, ";", "")
				}
				rows[i] = strings.Join(full, ";")
			}
			ch.ops = append(ch.ops, multiOps(f.prop, cur, rows, nil)...)
		case fieldList:
			items := listItems(first(w))
			if sameRows(items, listItems(first(h))) {
				continue
			}
			if !escape {
				ch.ops = append(ch.ops, textOps(f.prop, props[f.prop], strings.Join(items, ","), false)...)
				continue
			}
			for i := range items {
				items[i] = cal.EscapeText(items[i])
			}
			ch.ops = append(ch.ops, textOps(f.prop, props[f.prop], strings.Join(items, ","), false)...)
		case fieldRepeat:
			if sameRows(w, h) {
				continue
			}
			rule, err := repeatRule(w, h, zoneOf(props["DTSTART"]) == "" && len(props["DTSTART"]) > 0, tz)
			if err != nil {
				ch.errors[f.name] = err.Error()
				continue
			}
			ch.ops = append(ch.ops, textOps(f.prop, props[f.prop], rule, false)...)
		case fieldChoice:
			// An absent property and its blank are one state (S-7): a task
			// without STATUS saved with Needs action gains none.
			v, held := first(w), first(h)
			if v == "" {
				v = f.blank
			}
			if held == "" {
				held = f.blank
			}
			if v == held {
				continue
			}
			if !offered(f.choices, v, held) {
				ch.errors[f.name] = "Choose a " + strings.ToLower(f.label) + " from the list."
				continue
			}
			if f.prop == "STATUS" && c.spec.kind == view.KindTask {
				if first(want["done"]) != first(have["done"]) {
					continue // the Done box said it, and says COMPLETED with it
				}
				switch {
				case v == "COMPLETED" && len(props["COMPLETED"]) == 0:
					ch.ops = append(ch.ops, model.SetText{Prop: "COMPLETED", Value: now.UTC().Format("20060102T150405Z")})
				case v != "COMPLETED" && len(props["COMPLETED"]) > 0:
					ch.ops = append(ch.ops, model.RemoveProp{Prop: "COMPLETED"})
				}
			}
			ch.ops = append(ch.ops, textOps(f.prop, props[f.prop], v, false)...)
		case fieldAttendee:
			v := strings.ToLower(strings.TrimPrefix(first(w), "mailto:"))
			if v == "" {
				continue
			}
			if !strings.Contains(v, "@") || strings.ContainsAny(v, " \t;:,<>") {
				ch.errors[f.name] = "An attendee is an e-mail address."
				continue
			}
			ch.ops = append(ch.ops, model.AddValue{Prop: f.prop, Value: "mailto:" + v})
		case fieldRelated:
			if first(w) == first(h) {
				continue
			}
			rows := []string{first(w)}
			for i, v := range props[f.prop] {
				if i > 0 {
					rows = append(rows, v.Text)
				}
			}
			ch.ops = append(ch.ops, multiOps(f.prop, props[f.prop], rows, nil)...)
		case fieldBirthday:
			v := first(w)
			if v == first(h) || sameBirthday(v, first(h)) {
				continue
			}
			written, err := birthdayWritten(v, props[f.prop], firstValue(obj, "VERSION"))
			if err != nil {
				ch.errors[f.name] = err.Error()
				continue
			}
			ch.ops = append(ch.ops, textOps(f.prop, props[f.prop], written, false)...)
			if _, yearless, _ := birthdayDigits(v); written != "" && !yearless && len(props[f.prop]) > 0 && props[f.prop][0].Params[appleOmitYear] != "" {
				// A full date: the marker would hide the year typed.
				ch.ops = append(ch.ops, model.RemoveParam{Prop: f.prop, Index: 0, Param: appleOmitYear})
			}
		case fieldName:
			if sameRows(w, h) {
				continue
			}
			row := pair(w)
			ch.ops = append(ch.ops, nameOps(f.prop, props[f.prop], row[0], row[1])...)
		case fieldTime:
			row := triple(w)
			allDay := row[2] == "on"
			if f.name == "end" {
				allDay = startRow[2] == "on"
			}
			if f.required && row[0] == "" {
				ch.errors[f.name] = f.label + " is required."
				continue
			}
			at, err := rowTime(row, allDay, tz)
			if err != nil {
				ch.errors[f.name] = err.Error()
				continue
			}
			switch f.name {
			case "start":
				startAt = at
			case "end":
				endAt = at
			}
			if sameRows(w, h) && !(f.name == "end" && startChanged && row[0] != "") && (zone == "" || row[0] == "") {
				continue
			}
			shift := 0
			if f.name == "end" && allDay {
				shift = 1
			}
			ops := timeOps(f.prop, props[f.prop], row, allDay, shift, tz, zone)
			if f.name == "start" && ch.zone != nil {
				ch.zoneYear = at.Year()
			}
			if f.name == "end" && len(props["DURATION"]) > 0 && row[0] != "" {
				ops = append([]model.PatchOp{model.RemoveProp{Prop: "DURATION"}}, ops...)
			}
			ch.ops = append(ch.ops, ops...)
		case fieldDone:
			wantDone, hadDone := first(w) == "on", first(h) == "on"
			if wantDone == hadDone {
				continue
			}
			if wantDone {
				ch.ops = append(ch.ops,
					model.SetText{Prop: "STATUS", Value: "COMPLETED"},
					model.SetText{Prop: "COMPLETED", Value: now.UTC().Format("20060102T150405Z")})
				continue
			}
			ch.ops = append(ch.ops, model.SetText{Prop: "STATUS", Value: "NEEDS-ACTION"})
			if len(props["COMPLETED"]) > 0 {
				ch.ops = append(ch.ops, model.RemoveProp{Prop: "COMPLETED"})
			}
		case fieldPercent:
			v := first(w)
			if v == first(h) {
				continue
			}
			if v == "" {
				if len(props[f.prop]) > 0 {
					ch.ops = append(ch.ops, model.RemoveProp{Prop: f.prop})
				}
				continue
			}
			if n, err := strconv.Atoi(v); err != nil || n < 0 || n > 100 {
				ch.errors[f.name] = "Enter a number from 0 to 100."
				continue
			}
			ch.ops = append(ch.ops, model.SetText{Prop: f.prop, Value: v})
		case fieldReminders:
			ch.alarms = cur
			if sameRows(w, h) {
				continue
			}
			allowed := make(map[string]bool)
			for _, p := range reminderPresets {
				allowed[p.value] = true
			}
			for _, a := range cur {
				allowed[a.Trigger] = true
			}
			var next []cal.Alarm
			for i, v := range w {
				if v == "" {
					continue
				}
				if !allowed[v] {
					ch.errors[f.name] = "Choose a reminder from the list."
					break
				}
				if i < len(cur) && cur[i].Trigger == v {
					next = append(next, cur[i])
				} else {
					next = append(next, cal.NewAlarm(v))
				}
			}
			ch.alarms, ch.alarmsChanged = next, true
		}
	}
	if !startAt.IsZero() && !endAt.IsZero() && endAt.Before(startAt) {
		ch.errors["end"] = "The end is before the start."
	}
	if c.spec.kind == view.KindTask && ch.alarmsChanged && len(ch.alarms) > 0 &&
		triple(want["due"])[0] == "" && len(props["DTSTART"]) == 0 {
		ch.errors["reminder"] = "Reminders need a due date."
	}
	return ch
}

// fillNames is U-6.1: the display name and the two parts of N fill each
// other in, and both directions happen before the fields are walked — so what
// reaches the server is what the screen would show, and whoever filled one
// side is never told the other is missing. One switch, so the two cannot undo
// each other's work. have is the version the change is measured against.
func fillNames(kind view.Kind, have, want formValues) {
	if kind == view.KindContact {
		fn := strings.TrimSpace(first(want["fn"]))
		n := pair(want["n"])
		given, family := strings.TrimSpace(n[0]), strings.TrimSpace(n[1])
		switch {
		case fn == "" && given+family != "":
			want["fn"] = []string{strings.TrimSpace(given + " " + family)}
		case fn != "" && given == "" && family == "" && fn != strings.TrimSpace(first(have["fn"])):
			// Only when the display name itself was changed. Editing a phone
			// on a card that carries no N must not quietly grow one: what the
			// person did not touch stays as it was (S-7).
			// Cut left to right, the way the quick sheet cuts it (U-6.16):
			// the first word is the given name, the rest the family name.
			given, family = fn, ""
			if i := strings.IndexByte(fn, ' '); i > 0 {
				given, family = fn[:i], strings.TrimSpace(fn[i+1:])
			}
			want["n"] = []string{given, family}
		}
	}
}

// rowTime is the instant a time row names, for validation. An empty row is
// the zero time.
func rowTime(row [3]string, allDay bool, tz *time.Location) (time.Time, error) {
	if row[0] == "" {
		return time.Time{}, nil
	}
	if allDay {
		t, err := time.ParseInLocation("2006-01-02", row[0], tz)
		if err != nil {
			return time.Time{}, errors.New("Enter the date as YYYY-MM-DD.")
		}
		return t, nil
	}
	if row[1] == "" {
		return time.Time{}, errors.New("Enter a time, or mark it as all day.")
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", row[0]+" "+row[1], tz)
	if err != nil {
		return time.Time{}, errors.New("Enter the date as YYYY-MM-DD and the time as HH:MM.")
	}
	return t, nil
}

// textOps sets one value and keeps the parameters it was written with. An
// empty value removes the property: empty is not removal (S-7).
func textOps(prop string, cur []model.RawValue, v string, escape bool) []model.PatchOp {
	if v == "" {
		if len(cur) > 0 {
			return []model.PatchOp{model.RemoveProp{Prop: prop}}
		}
		return nil
	}
	if escape {
		v = cal.EscapeText(v)
	}
	ops := []model.PatchOp{model.SetText{Prop: prop, Value: v}}
	if len(cur) > 0 {
		for _, k := range sortedKeys(cur[0].Params) {
			ops = append(ops, model.SetParam{Prop: prop, Index: 0, Param: k, Value: cur[0].Params[k]})
		}
	}
	return ops
}

// listItems are the items of a list field as typed: split on commas,
// trimmed, the empty ones dropped.
func listItems(v string) []string {
	var out []string
	for _, item := range strings.Split(v, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// birthdayShown is BDAY as the form shows it: YYYY-MM-DD, or --MM-DD for a
// day without a year (RFC 6350 §4.3.1). A date-time shows its date; what is
// not a date at all (vCard 4 allows text) is shown as it is.
func birthdayShown(raw string, params map[string]string) string {
	v := strings.TrimSpace(raw)
	if omit := params[appleOmitYear]; omit != "" && strings.HasPrefix(v, omit) {
		// Apple's day without a year, as sabre keeps --MMDD: the year is a
		// placeholder the marker names.
		v = "--" + strings.TrimLeft(strings.TrimPrefix(v, omit), "-")
	}
	if i := strings.IndexByte(v, 'T'); i > 0 {
		v = v[:i]
	}
	digits := strings.ReplaceAll(strings.TrimPrefix(v, "--"), "-", "")
	switch {
	case strings.HasPrefix(v, "--") && len(digits) == 4 && isDigits(digits):
		return "--" + digits[:2] + "-" + digits[2:]
	case len(digits) == 8 && isDigits(digits):
		return digits[:4] + "-" + digits[4:6] + "-" + digits[6:]
	}
	return raw
}

// birthdayTyped is BDAY as the form holds it: typed text, DD.MM.YYYY or DD.MM
// without a year (s-contacts-3). What is not a date is shown as it is.
func birthdayTyped(raw string, params map[string]string) string {
	shown := birthdayShown(raw, params)
	digits, yearless, ok := birthdayDigits(shown)
	switch {
	case !ok:
		return shown
	case yearless:
		return digits[2:] + "." + digits[:2]
	}
	return digits[6:] + "." + digits[4:6] + "." + digits[:4]
}

// sameBirthday reports two typings of one day: 14.03.1987 and 1987-03-14.
func sameBirthday(a, b string) bool {
	da, ya, oka := birthdayDigits(a)
	db, yb, okb := birthdayDigits(b)
	return oka && okb && da == db && ya == yb
}

// birthdayDigits reads a typed birthday: DD.MM.YYYY or DD.MM, as the form
// asks, and YYYY-MM-DD or --MM-DD, as vCard writes and the form showed
// before. The answer is YYYYMMDD, or MMDD without a year, of a day that
// exists — 29 February without a year does.
func birthdayDigits(typed string) (digits string, yearless, ok bool) {
	v := strings.TrimSpace(typed)
	if parts := strings.Split(strings.TrimSuffix(v, "."), "."); len(parts) == 2 || len(parts) == 3 {
		pad := func(s string) string {
			if len(s) == 1 {
				return "0" + s
			}
			return s
		}
		day, month := pad(parts[0]), pad(parts[1])
		digits, yearless = month+day, true
		if len(parts) == 3 {
			if len(parts[2]) != 4 {
				return "", false, false
			}
			digits, yearless = parts[2]+month+day, false
		}
	} else {
		yearless = strings.HasPrefix(v, "--")
		digits = strings.ReplaceAll(strings.TrimPrefix(v, "--"), "-", "")
	}
	want, check := 8, digits
	if yearless {
		want, check = 4, "2000"+digits
	}
	if len(digits) != want || !isDigits(digits) {
		return "", false, false
	}
	if _, err := time.Parse("20060102", check); err != nil {
		return "", false, false
	}
	return digits, yearless, true
}

// appleOmitYear marks the year of BDAY as a placeholder: Apple writes a day
// without a year so, and Baikal and Nextcloud (sabre) store --MMDD that way.
const appleOmitYear = "X-APPLE-OMIT-YEAR"

// birthdayWritten turns what was typed into BDAY in the form the stored one
// has: basic (18151210, --0412) or extended (1815-12-10, --04-12). Without a
// stored one, vCard 3 gets extended and vCard 4 basic, as each writes it.
func birthdayWritten(typed string, cur []model.RawValue, version string) (string, error) {
	if typed == "" {
		return "", nil
	}
	digits, yearless, ok := birthdayDigits(typed)
	if !ok {
		return "", errors.New("Type the date as 14.03.1987, or 14.03 without the year.")
	}
	extended := strings.TrimSpace(version) == "3.0"
	if len(cur) > 0 {
		old := strings.TrimPrefix(strings.TrimSpace(cur[0].Text), "--")
		if i := strings.IndexByte(old, 'T'); i > 0 {
			old = old[:i]
		}
		extended = strings.Contains(old, "-")
	}
	out := digits
	if extended {
		if yearless {
			out = digits[:2] + "-" + digits[2:]
		} else {
			out = digits[:4] + "-" + digits[4:6] + "-" + digits[6:]
		}
	}
	if yearless {
		out = "--" + out
		if len(cur) > 0 && len(cur[0].Params[appleOmitYear]) == 4 {
			// Written the way the card holds such a day: the marker's year.
			out = cur[0].Params[appleOmitYear] + strings.TrimPrefix(out, "--")
			if extended {
				out = cur[0].Params[appleOmitYear] + "-" + digits[:2] + "-" + digits[2:]
			}
		}
	}
	return out, nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// nameParts splits N into the five components RFC 6350 gives it: family,
// given, middle, prefix, suffix. The property travels as one value, and
// go-vcard escapes a backslash, a newline and a comma but never the semicolon
// (card.TestEncodeKeepsStructuredName), so splitting and joining happens here
// and nowhere else.
func nameParts(v string) [5]string {
	var out [5]string
	for i, p := range strings.SplitN(v, ";", 5) {
		out[i] = p
	}
	return out
}

// nameOps writes Given and Family into N and leaves the three components no
// field offers — middle name, prefix, suffix — exactly as they were (S-7). A
// semicolon typed into either box is dropped: nothing escapes it on the way
// out, so it would arrive as a separator and shift every component after it.
// The property goes only when nothing is left in it at all: empty is not
// removal, but a name emptied of every part is.
func nameOps(prop string, cur []model.RawValue, given, family string) []model.PatchOp {
	var parts [5]string
	if len(cur) > 0 {
		parts = nameParts(cur[0].Text)
	}
	parts[0] = strings.ReplaceAll(family, ";", "")
	parts[1] = strings.ReplaceAll(given, ";", "")
	if strings.TrimSpace(strings.Join(parts[:], "")) == "" {
		if len(cur) > 0 {
			return []model.PatchOp{model.RemoveProp{Prop: prop}}
		}
		return nil
	}
	ops := []model.PatchOp{model.SetText{Prop: prop, Value: strings.Join(parts[:], ";")}}
	if len(cur) > 0 {
		for _, k := range sortedKeys(cur[0].Params) {
			ops = append(ops, model.SetParam{Prop: prop, Index: 0, Param: k, Value: cur[0].Params[k]})
		}
	}
	return ops
}

// multiOps rewrites a list property row by row: a kept row keeps its
// parameters (TYPE=work stays on the address it was on), an emptied row
// disappears, the add row comes without parameters. labels, when given, are
// the TYPE of each row as the form chose it: empty takes the label off.
func multiOps(prop string, cur []model.RawValue, rows, labels []string) []model.PatchOp {
	var texts []string
	var from, rowOf []int
	for i, v := range rows {
		if v = strings.TrimSpace(v); v == "" {
			continue
		}
		texts = append(texts, v)
		rowOf = append(rowOf, i)
		if i < len(cur) {
			from = append(from, i)
		} else {
			from = append(from, -1)
		}
	}
	if len(texts) == 0 {
		if len(cur) > 0 {
			return []model.PatchOp{model.RemoveProp{Prop: prop}}
		}
		return nil
	}
	ops := []model.PatchOp{model.SetValues{Prop: prop, Values: texts}}
	for i, src := range from {
		if src >= 0 {
			for _, k := range sortedKeys(cur[src].Params) {
				if labels != nil && k == "TYPE" {
					continue
				}
				ops = append(ops, model.SetParam{Prop: prop, Index: i, Param: k, Value: cur[src].Params[k]})
			}
		}
		if labels == nil {
			continue
		}
		if label := first(labels[min(rowOf[i], len(labels)):]); label != "" {
			ops = append(ops, model.SetParam{Prop: prop, Index: i, Param: "TYPE", Value: label})
		} else {
			ops = append(ops, model.RemoveParam{Prop: prop, Index: i, Param: "TYPE"})
		}
	}
	return ops
}

// offered reports whether v is one of the choices or the value the record
// holds.
func offered(choices []fieldOption, v, held string) bool {
	if v == held {
		return true
	}
	for _, c := range choices {
		if c.value == v {
			return true
		}
	}
	return false
}

// choiceOptions are the choices and, after them, the value the record holds
// when it is none of them.
func choiceOptions(choices []fieldOption, selected, held string) []view.Option {
	var opts []view.Option
	seen := false
	for _, c := range choices {
		opts = append(opts, view.Option{Value: c.value, Label: text(c.label), Selected: c.value == selected, Separator: c.sep})
		seen = seen || c.value == held
	}
	if held != "" && !seen {
		opts = append(opts, view.Option{Value: held, Label: text(held), Selected: held == selected})
	}
	return opts
}

// sameLabels compares the labels of the rows the form had.
func sameLabels(a, b []string) bool {
	for i := 0; i < max(len(a), len(b)); i++ {
		if first(a[min(i, len(a)):]) != first(b[min(i, len(b)):]) {
			return false
		}
	}
	return true
}

// labelOptions are the rowLabels of prop and the row's own label when it is
// none of them. A label is matched whatever its case, and a matched one keeps
// the value as the card holds it, so a save that leaves it changes nothing:
// TYPE=CELL is «mobile», offered once. A row without a label selects none.
func labelOptions(prop, selected string) []view.Option {
	var opts []view.Option
	seen := selected == ""
	for _, l := range rowLabels[prop] {
		o := view.Option{Value: l.value, Label: text(l.label)}
		if selected != "" && strings.EqualFold(l.value, selected) {
			o.Value, o.Selected, seen = selected, true, true
		}
		opts = append(opts, o)
	}
	if !seen {
		opts = append(opts, view.Option{Value: selected, Label: text(strings.ToLower(strings.ReplaceAll(selected, ",", ", "))), Selected: true})
	}
	return opts
}

// timeOps writes a date or date-time in the form it had: a TZID stays with
// its zone, UTC stays UTC, a floating time stays floating — unless zone
// names another one to write it in (a TZID, "UTC" or "floating"). A day is a
// DATE; shiftDays turns the last day of an all-day event into its exclusive
// DTEND. The row is already validated by rowTime.
func timeOps(prop string, cur []model.RawValue, row [3]string, allDay bool, shiftDays int, tz *time.Location, zone string) []model.PatchOp {
	if row[0] == "" {
		if len(cur) > 0 {
			return []model.PatchOp{model.RemoveProp{Prop: prop}}
		}
		return nil
	}
	var old model.RawValue
	if len(cur) > 0 {
		old = cur[0]
	}
	params := make(map[string]string)
	for k, v := range old.Params {
		if k != "VALUE" && k != "TZID" {
			params[k] = v
		}
	}
	var value string
	if allDay {
		day, _ := time.ParseInLocation("2006-01-02", row[0], tz)
		value = day.AddDate(0, 0, shiftDays).Format("20060102")
		params["VALUE"] = "DATE"
	} else {
		at, _ := time.ParseInLocation("2006-01-02 15:04", row[0]+" "+row[1], tz)
		wasDate := old.Params["VALUE"] == "DATE" || len(old.Text) == len("20060102")
		switch zone {
		case "UTC":
			old = model.RawValue{Text: "Z"}
			delete(params, "TZID")
		case "floating":
			old = model.RawValue{Text: "local"}
			delete(params, "TZID")
		case "":
			zone = old.Params["TZID"]
		}
		switch {
		case zone != "" && zone != "UTC" && zone != "floating":
			params["TZID"] = zone
			if loc, err := time.LoadLocation(zone); err == nil {
				value = at.In(loc).Format("20060102T150405")
			} else {
				// A zone Go does not know was read as a wall clock in the
				// instance zone; it is written back the same way.
				value = at.Format("20060102T150405")
			}
		case old.Text == "" || wasDate || strings.HasSuffix(old.Text, "Z"):
			value = at.UTC().Format("20060102T150405Z")
		default:
			value = at.Format("20060102T150405")
		}
	}
	ops := []model.PatchOp{model.SetText{Prop: prop, Value: value}}
	for _, k := range sortedKeys(params) {
		ops = append(ops, model.SetParam{Prop: prop, Index: 0, Param: k, Value: params[k]})
	}
	return ops
}

// apply patches obj and, when the reminder set changed, rewrites it.
func (c *collectionCtx) apply(obj *model.Object, ch change) (*model.Object, error) {
	if len(ch.ops) > 0 {
		patch, err := model.NewPatch(ch.ops...)
		if err != nil {
			return nil, err
		}
		if err := patch.Apply(obj); err != nil {
			return nil, err
		}
	}
	if ch.alarmsChanged {
		var err error
		if obj, err = cal.SetAlarms(obj, ch.alarms); err != nil {
			return nil, err
		}
	}
	if ch.zone != nil {
		return cal.EnsureTimezone(obj, ch.zone, ch.zoneYear)
	}
	return obj, nil
}

// editState is everything an edit page shows.
type editState struct {
	obj     *model.Object // the record as read; nil for a new one
	uid     string
	values  formValues
	alarms  []cal.Alarm
	errors  map[string]string
	etag    string
	readAt  time.Time
	isNew   bool
	loss    *view.LossReport
	notices []view.Notice
	// notebook is the Notebook a note's edit chose (s-notes-20): the list
	// address of the notebook; empty is its own.
	notebook string
	// linked is the same note in other notebooks (S-16), for the column of
	// its edit.
	linked []view.Field
}

func (s *Server) editAction(c *collectionCtx, st editState) string {
	if st.isNew {
		return c.listPath() + "/new"
	}
	return c.recordPath(st.uid) + "/edit"
}

func (s *Server) editPage(c *collectionCtx, st editState) any {
	tf := c.tf
	title := "Edit " + c.spec.noun
	if st.isNew {
		title = "New " + c.spec.noun
	}
	sh := s.buildShell(c.sess, title)
	back := s.cardBack(c, st.uid)
	if st.isNew || c.spec.kind == view.KindNote {
		// A note is edited on its page (s-notes-20): the way out is the
		// notebook's list, as from its reading.
		back = s.listBack(c)
	}
	sh.Rail = s.recordRail(c, back)
	sh.Notices = append(sh.Notices, st.notices...)
	if len(st.errors) > 0 {
		sh.Notices = append(sh.Notices, view.Notice{Role: view.NoticeAlert, Title: "Nothing was saved", Body: "Correct the marked fields and save again."})
	}
	form := s.recordForm(c, st)
	// Attaching is a write of its own (S-20): a record not yet created has
	// nowhere to take a file.
	attaching := st.obj != nil && !st.isNew && !c.col.ReadOnly && c.cal != nil
	if attaching && c.spec.kind != view.KindNote {
		form.Actions = append(form.Actions, s.attachAction(c, st.uid, st.etag, "edit"))
	}
	// The foot of the form of a record that exists is its deletion, with
	// the line the frame writes under it (from-typecase note 2: s-agenda-3,
	// s-tasks-3); Save and Cancel stay one pair, in the form.
	var del *view.Action
	if st.obj != nil && !st.isNew && !c.col.ReadOnly {
		del = s.recordOps(c.origin(), c.spec, st.uid, st.etag).del
	}
	switch c.spec.kind {
	case view.KindContact:
		page := view.ContactEditPage{Shell: sh, UID: st.uid, Form: form, Loss: st.loss, ReadOnly: c.col.ReadOnly}
		page.Photo = s.photoBlock(c, st)
		return page
	case view.KindEvent:
		page := view.EventEditPage{Shell: sh, UID: st.uid, Form: form, Reminders: reminderRows(st.alarms, tf), Loss: st.loss, Delete: del}
		if st.obj != nil {
			if cal.HasRecurrence(st.obj) {
				page.Recurrence = s.recurrenceBlock(st.obj, tf)
				page.Recurrence.Fields = append(repeatFields(repeatValues(cal.RawRRULE(st.obj), tf.tz)), page.Recurrence.Fields...)
				// The line stands under Delete here, and once.
				page.Recurrence.Note = ""
				if del != nil {
					page.DeleteNote = seriesNote
				}
			}
			for _, a := range cal.Attendees(st.obj) {
				page.Attendees = append(page.Attendees, view.AttendeeRow{Name: text(a.Name), Email: text(a.Email), Status: text(partstatLabel(a.Status))})
			}
			page.Attach = s.attachRows(c, st.uid, st.obj, st.etag, "edit")
		}
		return page
	case view.KindTask:
		page := view.TaskEditPage{Shell: sh, UID: st.uid, Form: form, Done: first(st.values["done"]) == "on", Reminders: reminderRows(st.alarms, tf), Loss: st.loss, Delete: del}
		if del != nil {
			page.DeleteNote = taskDeleteNote
		}
		if st.obj != nil {
			page.Attach = s.attachRows(c, st.uid, st.obj, st.etag, "edit")
		}
		if n, err := strconv.Atoi(first(st.values["percent"])); err == nil {
			page.Percent = &n
		}
		return page
	default:
		return s.noteEditPage(c, st, sh, form, attaching, del)
	}
}

// noteEditPage is the page of a note in edit (s-notes-20, from-typecase note
// 9): the shell stays — «Back to list», the notebooks and the neighbours —,
// and so does the column beside the text, less Tags and Related, which are
// fields under the title now: attachments, the same note in other notebooks,
// the source and the deletion. The text column is the form: title, Notebook,
// Date, Time, Tags, Related to, the text, and the properties Carrel does not
// know under it. Focus (s-notes-4) is this page without the rail and the
// column — the client's.
func (s *Server) noteEditPage(c *collectionCtx, st editState, sh view.Shell, form view.Form, attaching bool, del *view.Action) view.NotePage {
	body := first(st.values["description"])
	page := view.NotePage{Shell: sh, UID: st.uid, Source: c.source(), Loss: st.loss}
	page.Doc = view.NoteDoc{
		Mode: view.NoteEditing, Title: text(first(st.values["summary"])), Body: text(body),
		Rendered: cal.RenderMarkdown(body), ReadOnly: c.col.ReadOnly,
		Bar: []view.Action{
			{ID: "record.save", Label: "Save", Href: s.postQuery(s.editAction(c, st), url.Values{"etag": {st.etag}, "readat": {strconv.FormatInt(st.readAt.Unix(), 10)}}), Method: "POST", Enabled: true},
			s.cancelAction(c, st),
		},
	}
	var attach *view.Action
	if attaching {
		a := s.attachAction(c, st.uid, st.etag, "edit")
		attach = &a
		page.Doc.Bar = append(page.Doc.Bar, a)
	}
	exists := st.obj != nil && !st.isNew
	if exists {
		// Where the note lies is chosen here too: saving into another
		// notebook moves it there (S-18).
		page.Doc.Fields = append(page.Doc.Fields, s.notebookRow(c, st.notebook))
	}
	// The title and the body are the document; the version travels in
	// the address of Save. The rest of the form is Fields (C-6).
	for _, set := range form.Sets {
		for _, row := range set.Rows {
			if row.Kind != view.InputHidden && row.Name != "summary" && row.Name != "description" {
				page.Doc.Fields = append(page.Doc.Fields, row)
			}
		}
	}
	if exists {
		page.Rail.Neighbours = s.noteNeighbours(c, st.uid)
		page.Doc.Foreign = cal.NoteForeign(st.obj)
		page.Meta.Attach = s.attachRows(c, st.uid, st.obj, st.etag, "edit")
		page.Meta.Linked = st.linked
		page.Meta.Actions = shown(attach, del)
	}
	return page
}

// notebookRow is the Notebook of a note's edit (s-notes-20, s-notes-15):
// every live notebook, the note's own chosen until another is; one that
// cannot be written is shown closed with the reason. A value is the
// notebook's list address.
func (s *Server) notebookRow(c *collectionCtx, chosen string) view.FormRow {
	if chosen == "" {
		chosen = c.listPath()
	}
	row := view.FormRow{Name: "notebook", Label: "Notebook", Kind: view.InputSelect, Value: text(chosen)}
	blob := s.catalogOf(c.sess)
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || !col.Has(c.spec.kind) {
				continue
			}
			o := origin{conn: conn, col: col}
			at := o.listPath(c.spec.segment)
			opt := view.Option{Value: at, Label: view.Text(o.stamp()), Colour: catalogColour(col), Selected: at == chosen}
			if col.ReadOnly || conn.Feed {
				opt.Disabled, opt.Reason = true, "Read-only"
			}
			row.Options = append(row.Options, opt)
		}
	}
	return row
}

// notebookTarget is the notebook a note's edit chose, or nil when the value
// names none the catalog holds for notes.
func (s *Server) notebookTarget(c *collectionCtx, value string) *collectionCtx {
	blob := s.catalogOf(c.sess)
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if (origin{conn: conn, col: col}).listPath(c.spec.segment) == value {
				return s.moveTarget(c, conn.ID, col.Path)
			}
		}
	}
	return nil
}

// taskDeleteNote is the line under Delete task (s-tasks-3). The frame writes
// it in Russian; this is the same in English (to-typecase task 8).
const taskDeleteNote = "Deleting goes to the server at once, and nothing can undo it."

func (s *Server) cancelAction(c *collectionCtx, st editState) view.Action {
	href := c.recordPath(st.uid)
	if st.isNew {
		href = c.listPath()
	}
	return view.Action{ID: "record.cancel", Label: "Cancel", Href: s.route(href), Method: "GET", Enabled: true}
}

func (s *Server) recordForm(c *collectionCtx, st editState) view.Form {
	return s.recordFormOf(c, st, editFields[c.spec.kind])
}

// recordFormOf draws the fields it is given. The quick sheet draws a few of
// the same ones (U-6.16), by name and not by a list of its own, so the two
// screens cannot drift apart.
func (s *Server) recordFormOf(c *collectionCtx, st editState, fields []editField) view.Form {
	tf := c.tf
	// The head of the form comes first; each rubric stands where its first
	// field does (editFields).
	sets := []view.FieldSet{{Rows: []view.FormRow{
		csrfField(c.sess),
		{Name: "etag", Kind: view.InputHidden, Value: text(st.etag)},
		{Name: "readat", Kind: view.InputHidden, Value: text(strconv.FormatInt(st.readAt.Unix(), 10))},
	}}}
	var props map[string][]model.RawValue
	if st.obj != nil {
		props = st.obj.Props()
	}
	var empty []view.FormRow
	full := len(fields) == len(editFields[c.spec.kind])
	for _, f := range fields {
		vals, errText := st.values[f.name], text(st.errors[f.name])
		hide := f.optional && full && st.obj != nil && strings.TrimSpace(first(vals)) == "" && errText == ""
		// The quick sheet is too small for rubrics: its few fields stand
		// together, and only the reminders keep their own set (U-6.16).
		legend := f.set
		if !full && f.typ != fieldReminders {
			legend = ""
		}
		at := slices.IndexFunc(sets, func(fs view.FieldSet) bool { return string(fs.Legend) == legend })
		if at < 0 {
			at = len(sets)
			sets = append(sets, view.FieldSet{Legend: text(legend)})
		}
		set := &sets[at]
		before := len(set.Rows)
		switch f.typ {
		case fieldText:
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputText, Value: text(first(vals)), Hint: text(f.hint), Error: errText, Required: f.required})
		case fieldArea:
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputArea, Value: text(first(vals)), Error: errText})
		case fieldList:
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputText, Value: text(first(vals)), Hint: text(f.hint), Error: errText})
		case fieldRepeat:
			set.Rows = append(set.Rows, repeatSet(f, vals, errText).Rows...)
		case fieldAttendee:
			if v := first(vals); v != "" {
				set.Rows = append(set.Rows, view.FormRow{Name: f.name, Kind: view.InputHidden, Value: text(v), Error: errText})
			}
		case fieldRelated:
			set.Rows = append(set.Rows,
				view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputSelect, Value: text(first(vals)),
					Options: relatedOptions(c.index(), st.uid, first(vals)), Error: errText},
				view.FormRow{Name: f.name + ".uid", Label: "Or enter a UID by hand", Kind: view.InputText,
					Hint: "Filled, it wins over the choice above. Only the records already loaded for this collection are offered: nothing is asked of the servers."})
		case fieldZone:
			held := zoneOf(props[f.prop])
			if held == "" {
				continue // a day has no zone; a new event is written in UTC
			}
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputSelect, Value: text(first(vals)),
				Options: zoneOptions(held, first(vals), s.tz().String()), Error: errText})
		case fieldChoice:
			var held string
			if st.obj != nil {
				held = strings.TrimSpace(firstValue(st.obj, f.prop))
			}
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputSelect, Value: text(first(vals)),
				Options: choiceOptions(f.choices, first(vals), held), Error: errText})
		case fieldBirthday:
			// Typed, not picked (s-contacts-3): a date input cannot hold a
			// day without a year, and the day is quicker typed than found.
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputText, Value: text(first(vals)),
				Placeholder: "DD.MM.YYYY", Hint: text(f.hint), Error: errText})
		case fieldName:
			row := pair(vals)
			set.Rows = append(set.Rows,
				view.FormRow{Name: f.name + ".given", Label: "Given", Kind: view.InputText, Value: text(row[0]), Error: errText},
				view.FormRow{Name: f.name + ".family", Label: "Family", Kind: view.InputText, Value: text(row[1])})
		case fieldMulti:
			kind := view.InputText
			if f.prop == "EMAIL" {
				kind = view.InputEmail
			}
			for i, v := range vals {
				row := view.FormRow{Name: fmt.Sprintf("%s.%d", f.name, i), Label: text(f.label), Kind: kind, Value: text(v)}
				if i == len(vals)-1 {
					row.Name, row.Label = f.name+".new", text("Add "+strings.ToLower(f.label))
				} else if i < len(props[f.prop]) {
					row.Label = text(typedLabel(f.label, props[f.prop][i].Params))
				}
				if i == 0 {
					row.Error = errText
				}
				set.Rows = append(set.Rows, row)
				if f.typed {
					label := first(st.values[f.name+".type"][min(i, len(st.values[f.name+".type"])):])
					name := fmt.Sprintf("%s.%d.type", f.name, i)
					if i == len(vals)-1 {
						name = f.name + ".new.type"
					}
					set.Rows = append(set.Rows, view.FormRow{Name: name, Label: text(f.label + " label"), Kind: view.InputSelect, Value: text(label), Options: labelOptions(f.prop, label)})
				}
			}
		case fieldAddress:
			for i, v := range vals {
				prefix, label := fmt.Sprintf("%s.%d", f.name, i), f.label
				if i == len(vals)-1 {
					prefix, label = f.name+".new", "Add an address"
				} else if i < len(props[f.prop]) {
					label = typedLabel(f.label, props[f.prop][i].Params)
				}
				parts := addressRowParts(v)
				for j, p := range addressParts {
					row := view.FormRow{Name: prefix + "." + p.name, Label: text(p.label), Kind: view.InputText, Value: text(parts[j])}
					if j == 0 {
						row.Label = text(label + " · " + p.label)
						row.Error = errText
					}
					set.Rows = append(set.Rows, row)
				}
			}
		case fieldTime:
			row := triple(vals)
			// The all-day box leads its field, as in s-agenda-3.
			if f.allDay {
				set.Rows = append(set.Rows, view.FormRow{Name: f.name + ".allday", Label: "All day", Kind: view.InputCheck, Value: text(row[2])})
			}
			set.Rows = append(set.Rows,
				view.FormRow{Name: f.name + ".date", Label: text(f.label), Kind: view.InputDate, Value: text(row[0]), Error: errText, Required: f.required},
				view.FormRow{Name: f.name + ".time", Label: text(timeLabel(f)), Kind: view.InputTime, Value: text(row[1])})
		case fieldDone:
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputCheck, Value: text(first(vals))})
		case fieldPercent:
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputNumber, Value: text(first(vals)), Error: errText})
		case fieldReminders:
			// The first reminder, then the others, then a row that adds one
			// (s-agenda-3: First, Add reminder); without any, the adding row
			// is the first.
			for i, v := range vals {
				row := view.FormRow{Name: fmt.Sprintf("reminder.%d", i), Label: "Reminder", Kind: view.InputSelect, Value: text(v), Options: reminderOptions(v, st.alarms, i, tf)}
				if i == len(vals)-1 {
					row.Name, row.Label = "reminder.new", "Add reminder"
				}
				if i == 0 {
					row.Label, row.Error = "First", errText
				}
				set.Rows = append(set.Rows, row)
			}
		}
		if hide {
			empty = append(empty, set.Rows[before:]...)
			set.Rows = set.Rows[:before]
		}
	}
	if len(empty) > 0 {
		legend := fmt.Sprintf("Show %d empty fields", len(empty))
		if len(empty) == 1 {
			legend = "Show 1 empty field"
		}
		sets = append(sets, view.FieldSet{Legend: text(legend), Rows: empty})
	}
	form := view.Form{
		Action: s.route(s.editAction(c, st)), Method: "POST", Sets: sets,
		Actions: []view.Action{{ID: "record.save", Label: "Save", Method: "POST", Enabled: !c.col.ReadOnly}, s.cancelAction(c, st)},
	}
	names := make([]string, 0, len(st.errors))
	for name := range st.errors {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		form.Errors = append(form.Errors, view.FieldError{Name: name, Message: text(st.errors[name])})
	}
	return form
}

// reminderOptions are the steps of s-agenda-9, a reminder another client
// wrote after them when it is none of the steps, and «No reminder» under a
// line.
func reminderOptions(selected string, alarms []cal.Alarm, row int, tf timeFmt) []view.Option {
	var opts []view.Option
	seen := false
	for _, p := range reminderPresets {
		opts = append(opts, view.Option{Value: p.value, Label: text(p.label), Selected: p.value == selected})
		seen = seen || p.value == selected
	}
	if selected != "" && !seen {
		label := selected
		if row < len(alarms) && alarms[row].Trigger == selected {
			label = tf.reminder(alarms[row])
		}
		opts = append(opts, view.Option{Value: selected, Label: text(label), Selected: true})
	}
	return append(opts, view.Option{Value: "", Label: "No reminder", Selected: selected == "", Separator: true})
}

func (s *Server) getKindEdit(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, true)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	uid := r.PathValue("uid")
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid)+"/edit", err)))
		return
	}
	vals, alarms, err := s.currentValues(c, obj)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, readNotice(obj.Path, err)))
		return
	}
	s.rememberBody(c, obj)
	s.writePage(w, r, http.StatusOK, s.editPage(c, editState{
		obj: obj, uid: uid, values: vals, alarms: alarms, etag: obj.ETag, readAt: s.now(),
		linked: s.noteLinked(r.Context(), c, uid, obj),
	}))
}

// noteLinked is the same note in other notebooks (S-16) — the column of its
// edit keeps it (s-notes-20). Other kinds show it on the card only.
func (s *Server) noteLinked(ctx context.Context, c *collectionCtx, uid string, obj *model.Object) []view.Field {
	if c.spec.kind != view.KindNote {
		return nil
	}
	if _, _, rows, ok := s.linkedCalendar(ctx, c, uid, obj); ok {
		return rows
	}
	return nil
}

// postKindEdit writes a patch with If-Match of the version the form was read
// at (S-8). A 412 keeps what the person changed in DraftRejectedEdit and
// opens the conflict screen; nothing is resolved automatically.
func (s *Server) postKindEdit(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, true)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	uid := r.PathValue("uid")
	// An attachment is a write of its own on this address (S-20). Its file is
	// read from the body as it comes: nothing may parse the form before.
	switch op := r.URL.Query().Get("op"); {
	case op == "attach" && c.cal != nil:
		s.postAttach(w, r, c, uid)
		return
	case op == "attachfile" && c.cal != nil:
		s.postAttachFile(w, r, c, uid)
		return
	case op == "detach" && c.cal != nil:
		s.postDetach(w, r, c, uid)
		return
	case op == "done" && c.spec.kind == view.KindTask:
		// Mark done (review Р-10): the Done box alone, the rest of the
		// record as the server holds it; below, want is have with it. A
		// client may post the action's query as its body: an attachment
		// never comes that way, so only then is the body read for op.
	case op == "" && r.FormValue("op") == "done" && c.spec.kind == view.KindTask:
	case op != "" || r.FormValue("op") != "":
		s.notFound(w, r)
		return
	}
	etag := strings.TrimSpace(r.FormValue("etag"))
	readAt := unixForm(r.FormValue("readat"))
	want := s.submittedValues(c, r)
	st := editState{uid: uid, values: want, etag: etag, readAt: readAt}
	// A note's Notebook other than its own: the edit is saved, then the
	// note moves there (s-notes-20, S-18).
	moving := false
	if c.spec.kind == view.KindNote {
		st.notebook = strings.TrimSpace(r.FormValue("notebook"))
		moving = st.notebook != "" && st.notebook != c.listPath()
	}

	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		st.notices = []view.Notice{s.unavailableNotice(c, c.recordPath(uid)+"/edit", err)}
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
		return
	}
	have, alarms, err := s.currentValues(c, obj)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, readNotice(obj.Path, err)))
		return
	}
	st.obj, st.alarms = obj, alarms
	st.linked = s.noteLinked(r.Context(), c, uid, obj)
	markDone := r.URL.Query().Get("op") == "done" || r.URL.Query().Get("op") == "" && r.FormValue("op") == "done"
	if markDone {
		want = formValues{}
		for k, v := range have {
			want[k] = append([]string(nil), v...)
		}
		want["done"] = []string{checkbox(r, "done")}
		st.values = want
	}
	if etag == "" {
		st.notices = []view.Notice{{Role: view.NoticeAlert, Title: "Nothing was saved", Body: "The form does not say which version it edits. Open the record again."}}
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
		return
	}

	typed := formValues{}
	for k, v := range want {
		typed[k] = append([]string(nil), v...)
	}
	ch := s.buildChange(c, obj, alarms, have, want, time.Now())
	if len(ch.errors) > 0 {
		st.errors = ch.errors
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
		return
	}
	if len(ch.ops) == 0 && !ch.alarmsChanged {
		if moving {
			s.moveRecord(w, r, c, s.notebookTarget(c, st.notebook), uid, obj, false, "Nothing was moved")
			return
		}
		s.redirect(w, r, c.recordPath(uid))
		return
	}
	// What the person changed is measured against the version the form was
	// read at. Without it in the cache, against the server's version — then
	// the conflict screen may also offer what only the server changed.
	base := have
	if read, ok := s.readVersion(c, obj.Path, etag); ok {
		if rv, _, err := s.currentValues(c, read); err == nil {
			base = rv
		}
	}

	// The patch is applied to the record read: what it held is taken first.
	wasEmails := cardEmails(obj)
	sent, err := c.apply(obj, ch)
	if err != nil {
		st.notices = []view.Notice{s.writeFailure(c, "PUT", obj.Path, err)}
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
		return
	}
	sent.ETag = etag
	err = c.update(r.Context(), sent)
	switch {
	case dav.IsStatus(err, http.StatusPreconditionFailed):
		// The draft is what the person changed against what they read: the
		// names fill each other in against that version, not the server's,
		// or a display name renamed elsewhere would put an N in the draft
		// nobody typed.
		fillNames(c.spec.kind, base, typed)
		s.saveDraft(c, uid, etag, readAt, changedValues(editFields[c.spec.kind], base, typed), st.notebook)
		s.redirect(w, r, c.recordPath(uid)+"/conflict")
		return
	case err != nil:
		st.notices = []view.Notice{s.writeFailure(c, "PUT", obj.Path, err)}
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
		return
	}
	back := c.recordPath(uid)
	if markDone {
		back = doneBack(c, uid, r.FormValue("from"))
	}
	if moving {
		s.saveAndMove(w, r, c, s.notebookTarget(c, st.notebook), uid, sent, back)
		return
	}
	// An address replaced on a card is asked about before the card is shown
	// again (S-15.4): records that named the old one drop out of the
	// person's timeline. What the save lost comes first, as always, and the
	// question is then not asked.
	if c.spec.kind == view.KindContact {
		if sheet, token := s.offerRewrite(c, uid, wasEmails, sent); sheet != "" {
			s.finishWriteWith(w, r, c, uid, sent, sheet, func(loss *model.Loss, _ *model.Object) ([]view.Action, []view.Notice) {
				if loss != nil {
					dropRewritePlan(c.sess, token)
				}
				return nil, nil
			})
			return
		}
	}
	s.finishWrite(w, r, c, uid, sent, back)
}

// notMoved is the title of every answer where a note's edit was saved and
// the note stayed in its notebook.
const notMoved = "Saved, but not moved"

// saveAndMove finishes a note's edit that chose another notebook: the edit
// is on the server already, and the note moves by the rules of S-18. What
// the save lost is reported first, and the note stays where it is until that
// has been seen; a move that fails leaves it there too. Either way the
// answer says the edit was saved and the note was not moved.
func (s *Server) saveAndMove(w http.ResponseWriter, r *http.Request, c *collectionCtx, dest *collectionCtx, uid string, sent *model.Object, back string) {
	got, err := c.find(r.Context(), uid)
	if err != nil {
		sent.ETag = ""
		c.noteWrite(sent.Path, sent)
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, view.Notice{Role: view.NoticeAlert, Title: notMoved,
			Body: text("The " + c.spec.noun + " could not be read back after saving, so it stayed where it was. Move it from its menu.")}))
		return
	}
	// Asked without the session's tally: the report below asks again, and
	// counts it then.
	if model.CompareAfterPut(c.conn.ID, sent, got, nil) != nil {
		s.reportWrite(w, r, c, sent, back, written{uid: uid, got: got}, func(*model.Loss, *model.Object) ([]view.Action, []view.Notice) {
			return nil, []view.Notice{{Role: view.NoticeAlert, Title: notMoved,
				Body: text("The server did not keep everything this edit wrote, so the " + c.spec.noun + " stayed where it was. Move it from its menu once you have looked.")}}
		})
		return
	}
	c.noteWrite(got.Path, got)
	s.moveRecord(w, r, c, dest, uid, got, false, notMoved)
}

// doneAction is Mark done or Mark not done of a task, at the version etag.
// from says where to come back: "" the card, "list" its list, "all" the
// combined list of tasks.
func (s *Server) doneAction(edit, etag string, done bool, from string) view.Action {
	a := view.Action{ID: "task.done", Label: "Mark done", Method: "POST", Enabled: true}
	q := url.Values{"op": {"done"}, "etag": {etag}, "done": {"on"}}
	if done {
		a.Label, q["done"] = "Mark not done", []string{"off"}
	}
	if from != "" {
		q.Set("from", from)
	}
	a.Href = s.postQuery(edit, q)
	return a
}

func doneBack(c *collectionCtx, uid, from string) string {
	switch from {
	case "list":
		return c.listPath()
	case "all":
		return "/app/tasks"
	}
	return c.recordPath(uid)
}

func changedValues(fields []editField, base, want formValues) formValues {
	out := formValues{}
	for _, f := range fields {
		labelled := f.typed && !sameLabels(want[f.name+".type"], base[f.name+".type"])
		if !sameRows(want[f.name], base[f.name]) || labelled {
			out[f.name] = want[f.name]
			if f.typed {
				out[f.name+".type"] = want[f.name+".type"]
			}
		}
	}
	return out
}

func unixForm(v string) time.Time {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.Unix(n, 0)
}

// finishWrite re-reads what was written and compares (S-7.1). A loss is a
// notice on the edit page, said once per server and property; no loss is a
// redirect to done: the record, or its edit page after a photo. The loaded
// list takes the record as re-read, or as sent with no known version when the
// re-read failed.
func (s *Server) finishWrite(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, sent *model.Object, done string) {
	s.finishWriteWith(w, r, c, uid, sent, done, nil)
}

// afterWrite is told what a write left once it is read back — got nil when it
// could not be, loss nil when nothing was lost — and answers what the loss
// report offers and says beside it.
type afterWrite func(loss *model.Loss, got *model.Object) ([]view.Action, []view.Notice)

// finishWriteWith is finishWrite with a say in the loss report. after, when
// set, is called exactly once, whatever the outcome.
func (s *Server) finishWriteWith(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, sent *model.Object, done string, after afterWrite) {
	got, err := c.find(r.Context(), uid)
	s.reportWrite(w, r, c, sent, done, written{uid: uid, got: got, err: err}, after)
}

// written is what a write left on the server: the identity the record has
// there now, what was read back under it — nil when it could not be — and why.
// theirUID marks an identity the server chose itself, which is not a loss
// (S-7.1).
type written struct {
	uid      string
	got      *model.Object
	err      error
	theirUID bool
}

// reportWrite is the answer to a write once it has been read back: a redirect
// to done when nothing was lost, the loss report on the edit page when
// something was.
func (s *Server) reportWrite(w http.ResponseWriter, r *http.Request, c *collectionCtx, sent *model.Object, done string, wr written, after afterWrite) {
	tell := func(loss *model.Loss, got *model.Object) ([]view.Action, []view.Notice) {
		if after == nil {
			return nil, nil
		}
		return after(loss, got)
	}
	uid, got, err := wr.uid, wr.got, wr.err
	if err != nil {
		tell(nil, nil)
		sent.ETag = ""
		c.noteWrite(sent.Path, sent)
		s.redirect(w, r, done)
		return
	}
	c.noteWrite(got.Path, got)
	compare := model.CompareAfterPut
	if wr.theirUID {
		compare = model.CompareAfterCreate
	}
	loss := compare(c.conn.ID, sent, got, c.sess.Losses())
	actions, notices := tell(loss, got)
	if loss == nil {
		s.redirect(w, r, done)
		return
	}
	vals, alarms, err := s.currentValues(c, got)
	if err != nil {
		s.redirect(w, r, done)
		return
	}
	s.rememberBody(c, got)
	report := &view.LossReport{
		Note:       "The change was saved, but the server did not keep these properties. Other clients will not see them either.",
		Aggregated: loss.Aggregated,
		Actions:    actions,
	}
	for _, p := range loss.Properties {
		report.Properties = append(report.Properties, text(p))
	}
	s.writePage(w, r, http.StatusOK, s.editPage(c, editState{
		obj: got, uid: uid, values: vals, alarms: alarms, etag: got.ETag, readAt: s.now(), loss: report, notices: notices,
		linked: s.noteLinked(r.Context(), c, uid, got),
	}))
}

// getKindNew: as a sheet, the target collection stated; as a page, the empty
// edit form of the kind (C-5.4). From a combined view (?pick=1) there is no
// target to open a form for, so without the fragment header the sheet is the
// page: the choice is its options' links, as U-8 asks of a page without
// scripts.
func (s *Server) getKindNew(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, false)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	pick := r.URL.Query().Get("pick") != ""
	switch {
	case WantsFragment(r) && FragmentTarget(r) == TargetSheet:
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, s.newSheet(c, pick), nil)
	case pick:
		s.writePage(w, r, http.StatusOK, s.newSheet(c, true))
	default:
		values := s.emptyValues(c)
		carried(c, r, values)
		s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: values, isNew: true, readAt: s.now()}))
	}
}

// getKindQuickEntry is the way in from the shell, where no collection is known
// (U-6.16). It opens the sheet on the collection a record of this kind was
// last created in — the person's own last choice, not a guess. When there is
// none, or it is gone or read-only now, the sheet asks where first and shows
// no fields: filling in a form with nowhere to put it helps nobody.
func (s *Server) getKindQuickEntry(w http.ResponseWriter, r *http.Request, kind string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	spec, known := recordKinds[kind]
	if !known {
		s.notFound(w, r)
		return
	}
	blob := s.catalogOf(sess)
	writable := func(col sealed.Collection) bool {
		return !col.Gone && !col.ReadOnly && col.Has(spec.kind)
	}
	if last, remembered := blob.LastFor(spec.kind); remembered {
		if col, found := blob.Collection(last.AccountID, last.Path); found && writable(col) {
			if c, made := s.collectionOf(sess, spec, last.AccountID, last.Path); made {
				s.redirect(w, r, c.listPath()+"/new/quick"+carriedQuery(r))
				return
			}
		}
	}
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if !writable(col) {
				continue
			}
			c, made := s.collectionOf(sess, spec, conn.ID, col.Path)
			if !made {
				continue
			}
			sheet := s.newSheet(c, true)
			for i := range sheet.Target.Options {
				sheet.Target.Options[i].ID += carriedQuery(r)
			}
			if WantsFragment(r) && FragmentTarget(r) == TargetSheet {
				_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
				return
			}
			s.writePage(w, r, http.StatusOK, sheet)
			return
		}
	}
	// Nowhere to write (s-notes-6): no collection of the kind takes a
	// record. The sheet says so by having no target, and offers the way out
	// instead of an empty choice (U-6.16).
	sheet := view.NewSheet{Kind: spec.kind, Actions: []view.Action{
		{ID: "connection.add", Label: "Connect account", Href: s.connectHref(), Method: "GET", Enabled: true},
		{ID: "record.cancel", Label: "Cancel", Href: s.route("/app/" + kind), Method: "GET", Enabled: true},
	}}
	if WantsFragment(r) && FragmentTarget(r) == TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

// getKindQuick is the quick sheet (U-6.16): where the record goes, and the few
// fields without which it does not exist. The full creation form lives at
// /new; this sheet supplements it and does not replace it.
func (s *Server) getKindQuick(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, false)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	values := s.emptyValues(c)
	carried(c, r, values)
	sheet := s.quickSheet(c, editState{values: values, isNew: true, readAt: s.now()})
	if WantsFragment(r) && FragmentTarget(r) == TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

// postKindQuick saves the record, or carries what was typed into the full form
// without saving anything. Until Save is pressed there is no record: a button
// that opened the card and quietly created one would leave it behind for
// anyone who then changed their mind (U-6.16).
func (s *Server) postKindQuick(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, false)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	want := s.submittedValues(c, r)
	if r.URL.Query().Get("op") == "open" {
		s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: want, isNew: true, readAt: s.now()}))
		return
	}
	s.createRecord(w, r, c, want)
}

// quickSheet is the collection picker of newSheet with a form under it. The
// fields are the full form's own, by name (quickFields), and the form posts to
// the sheet's address; "Open the card" posts to the same address with op=open,
// so what was typed travels in the body and never in the address bar.
func (s *Server) quickSheet(c *collectionCtx, st editState) view.NewSheet {
	sheet := s.newSheet(c, false)
	form := s.recordFormOf(c, st, quickFields(c.spec.kind))
	for _, name := range carriedFields {
		if v := first(st.values[name]); v != "" && len(form.Sets) > 0 && hasField(c.spec.kind, name) {
			// What a sheet was opened for travels with it, not as a field of
			// it: New task from an event, a note or an event from a timeline.
			form.Sets[0].Rows = append(form.Sets[0].Rows, view.FormRow{Name: name, Kind: view.InputHidden, Value: text(v)})
		}
	}
	quick := c.listPath() + "/new/quick"
	form.Action = s.route(quick)
	form.Method = "POST"
	form.Actions = []view.Action{
		{ID: "record.save", Label: text("Save " + c.spec.noun), Method: "POST", Enabled: !c.col.ReadOnly},
		{ID: "record.open", Label: "Open the card", Href: s.route(quick + "?op=open"), Method: "POST", Enabled: true},
		s.cancelAction(c, st),
	}
	sheet.Form = form
	return sheet
}

// newSheet is where a new record goes: every live collection of the kind,
// read-only ones shown and closed with the reason. An option's ID is the
// address of its new-record page. Opened from one collection, that collection
// is chosen; opened from a combined view (?pick=1) nothing is, and the target
// is chosen, not guessed (S-14).
func (s *Server) newSheet(c *collectionCtx, pick bool) view.NewSheet {
	blob := s.catalogOf(c.sess)
	own := s.route(c.listPath() + "/new")
	sheet := view.NewSheet{Kind: c.spec.kind}
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || !col.Has(c.spec.kind) {
				continue
			}
			o := origin{conn: conn, col: col}
			opt := view.TargetOption{
				ID: s.route(o.listPath(c.spec.segment) + "/new"), Label: text(collectionLabel(col)),
				Stamp: o.stamp(), Colour: catalogColour(col), ReadOnly: col.ReadOnly,
			}
			if col.ReadOnly {
				opt.Reason = "Read-only"
			}
			sheet.Target.Options = append(sheet.Target.Options, opt)
		}
	}
	next := view.Action{ID: "record.new", Label: text("New " + c.spec.noun), Href: own, Method: "GET", Enabled: true}
	if pick {
		next.Enabled, next.Reason = false, text("Choose the collection the new "+c.spec.noun+" goes to.")
	} else {
		sheet.Target.Chosen = own
	}
	sheet.Actions = []view.Action{next}
	return sheet
}

// postKindNew creates with If-None-Match (S-8). The record starts as a
// skeleton — UID of 32 hex digits, file {uid}.vcf or {uid}.ics — and the form
// arrives as a patch on it.
func (s *Server) postKindNew(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, false)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	s.createRecord(w, r, c, s.submittedValues(c, r))
}

// createRecord writes a new record. The full form and the quick sheet both
// come here: two screens, one way of saving (U-6.16).
func (s *Server) createRecord(w http.ResponseWriter, r *http.Request, c *collectionCtx, want formValues) {
	st := editState{values: want, isNew: true, readAt: s.now()}
	uid, err := model.NewUID()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	now := time.Now()
	var obj *model.Object
	if c.card != nil {
		obj = card.NewObject(uid)
	} else if obj, err = cal.NewObject(c.spec.component, uid, now); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	ch := s.buildChange(c, obj, nil, s.emptyValues(c), want, now)
	if len(ch.errors) > 0 {
		st.errors = ch.errors
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
		return
	}
	path := strings.TrimSuffix(c.col.Path, "/") + "/" + uid + c.spec.ext
	sent, err := c.apply(obj, ch)
	if err == nil {
		sent.Path = path
		err = c.create(r.Context(), path, sent)
		// Refused because it is already there, on a write the guard had
		// sent once before: that is this record, arriving twice.
		// finishCreate reads the address back, so nothing is claimed that
		// the server does not hold.
		if landedOnTheFirstTry(err) {
			err = nil
		}
	}
	if err != nil {
		st.notices = []view.Notice{s.writeFailure(c, "PUT", path, err)}
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
		return
	}
	// Where a record was made is where the next one starts from, when the way
	// in named no collection (U-6.16). A failure to remember is not a failure
	// to save: the record is written either way.
	if key, unlocked := c.sess.DataKey(); unlocked {
		_ = s.users.SetLastUsed(c.sess.AccountID(), key,
			sealed.Selection{Kind: c.spec.kind, AccountID: c.col.AccountID, Path: c.col.Path})
	}
	s.finishCreate(w, r, c, uid, sent)
}

// finishCreate reads a new record back and sends the person to it. A server
// may name the resource itself and write its own UID over the one it was sent
// — Google does both with a contact (docs/servers.md, SV-25) — so a record
// that does not answer under the UID it was given is looked for at the address
// it was written to, and the identity found there is the one it has in Carrel
// (S-7.1). A record that is nowhere ends as a notice: sending a person to an
// address that does not open would be the worse answer.
func (s *Server) finishCreate(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, sent *model.Object) {
	ctx := r.Context()
	wr := written{uid: uid}
	wr.got, wr.err = c.find(ctx, uid)
	if notFoundErr(wr.err) {
		at, err := c.fetch(ctx, sent.Path)
		made := ""
		if err == nil {
			made = uidOf(c, at)
		}
		switch {
		case made == "" || made == uid:
			s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.namelessNotice(c)))
			return
		default:
			// The server keeps it under a name of its own too, so the address
			// comes from a read by that identity when there is one.
			wr = written{uid: made, got: at, theirUID: true}
			if kept, err := c.find(ctx, made); err == nil {
				wr.got = kept
			}
		}
	}
	s.reportWrite(w, r, c, sent, c.recordPath(wr.uid), wr, nil)
}

// namelessNotice is a record the server took and then did not show: not under
// the identity it was sent, not at the address it was written to. The
// collection is the one honest way on.
func (s *Server) namelessNotice(c *collectionCtx) view.Notice {
	return view.Notice{
		Role:  view.NoticeAlert,
		Title: text("Saved, but the server does not show it"),
		Body: text("The server took the " + c.spec.noun + " and answers neither at the address it was written to nor " +
			"under the identity it was given. Open " + collectionLabel(c.col) + " to see what is there."),
		Actions: []view.Action{{
			ID: "collection.open", Label: text("Open " + collectionLabel(c.col)),
			Href: s.route(c.listPath()), Method: "GET", Enabled: true,
		}},
	}
}

// postKindDelete deletes with If-Match of the version the page showed. A
// changed record is not deleted: the page says so and shows what is there now.
func (s *Server) postKindDelete(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, true)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	uid := r.PathValue("uid")
	etag := strings.TrimSpace(r.FormValue("etag"))
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.redirect(w, r, c.listPath())
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid), err)))
		return
	case etag == "":
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, view.Notice{
			Role: view.NoticeAlert, Title: "Nothing was deleted", Body: "The request did not say which version to delete.",
		}))
		return
	}
	err = c.remove(r.Context(), obj.Path, etag)
	switch {
	case dav.IsStatus(err, http.StatusPreconditionFailed):
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, view.Notice{
			Role:  view.NoticeAlert,
			Title: "Changed elsewhere, so nothing was deleted",
			Body:  "This record changed after you opened it. Check the current version below and delete again if you still mean to.",
		}))
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, s.writeFailure(c, "DELETE", obj.Path, err)))
	default:
		c.noteWrite(obj.Path, nil)
		s.redirect(w, r, c.listPath())
	}
}
