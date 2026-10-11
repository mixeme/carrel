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
	"carrel.dev/carrel/server/render"
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
	// "UTC" or "floating". The form's times are in it (formZone): an event
	// is edited in its own zone (s-agenda-3, the owner's decision of
	// 02.10.2026), and the clock typed is the clock in the zone chosen.
	fieldZone
	// fieldRepeat is RRULE, edited as a whole series (S-11): Frequency,
	// Every, On, Until, Count, and the rule itself. See repeatValues.
	fieldRepeat
	// fieldRelated is every RELATED-TO of the record (the owner's decision
	// of 09.10.2026): each picked from the records this session has loaded,
	// or typed — a title of a loaded record or a UID — into the line
	// «related.add» (S-11, s-tasks-3, -10).
	fieldRelated
	// fieldAttendee is the attendees of an event (Typecase task 77,
	// s-agenda-3): their addresses, «mailto:…», the organiser first. The full
	// form draws them as EventEditPage.Attendees and posts each kept one and
	// the line typed in (attendee.add); the quick sheet carries the person a
	// timeline creates the event for (S-15.4) hidden. See attendees.go.
	fieldAttendee
)

type editField struct {
	name     string
	label    string
	prop     string
	typ      fieldType
	required bool
	allDay   bool // fieldTime: the field has its own all-day box
	// taskDay: fieldTime of a task, whose times share one all-day box,
	// «allday», standing before the first of them (s-tasks-3).
	taskDay bool
	typed   bool // fieldMulti: each row has its label, the TYPE parameter
	// mono: the frames draw the field's input monospaced (is-mono, C-6.8
	// FormRow.Mono) — a phone number (s-contacts-3, s-notes-8); dates,
	// times, a birthday and a percent are so by their kind.
	mono bool
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
	// add is the button under the rows of a repeating field, «Add phone»
	// (C-6.8 InputAdd): the form of a record draws no empty row beforehand
	// (s-contacts-3).
	add string
	// newRow is the label of the one empty row the full form of a new
	// record starts the field with, «mobile» under Phone numbers
	// (s-contacts-27); empty — no row, only the button.
	newRow string
	// quick is the label a field has on the quick sheet where it differs
	// from the full form's (s-notes-8, s-notes-9): Title; a time's date,
	// Date; the one row of a repeating field, which draws neither its label
	// list nor the button — Phone, Email, Remind.
	quick string
	// quickTime labels the time of a fieldTime on the quick sheet: Start,
	// End (s-notes-9).
	quickTime string
	// quickNoDate: the quick sheet draws no date of this time — the end is
	// on the day of the start, or on the next when it is earlier (s-notes-9).
	// The full form reads its time typed without a date the same way (S-11).
	quickNoDate bool
	// quickSample is the sample in the empty row of the quick sheet: «+7 …»,
	// «name@example.org» (s-notes-8); of a time, its date's, and
	// quickTimeSample its time's: «YYYY-MM-DD», «--:--» (s-notes-10).
	quickSample     string
	quickTimeSample string
}

// fieldOption is one value a fieldChoice offers; sep draws a line before it,
// where the list frame beside the screen draws one.
type fieldOption struct {
	value, label string
	sep          bool
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

// formZone is the zone an event's form shows and reads its times in: the
// zone its start is written in (s-agenda-3: 12:00–13:00 of an event from
// Asia/Yekaterinburg, as sent) — a TZID the zone database knows, or UTC. A
// floating time, a day and a TZID the database does not know are read as a
// wall clock in the instance zone, and so shown.
func formZone(zone string, tz *time.Location) *time.Location {
	switch zone {
	case "", "floating":
		return tz
	case "UTC":
		return time.UTC
	}
	if loc, err := time.LoadLocation(zone); err == nil {
		return loc
	}
	return tz
}

// floatingLabel is the choice that is not a zone but its absence: the same
// clock time anywhere (s-agenda-8). Chosen, it drops TZID (S-11).
const floatingLabel = "Floating — the same clock time anywhere"

// zoneOptions are the list of s-agenda-8: the zone the event came in under
// the heading As sent, a line, every zone of zoneNames, a line and Floating,
// always last — once, when the event came floating.
func zoneOptions(held, selected string) []view.Option {
	label := func(z string) view.Text {
		if z == "floating" {
			return floatingLabel
		}
		return text(z)
	}
	opts := make([]view.Option, 0, len(zoneNames)+2)
	opts = append(opts, view.Option{Value: held, Label: label(held), Selected: selected == held, Group: "As sent"})
	for _, z := range zoneNames {
		if z != held {
			opts = append(opts, view.Option{Value: z, Label: text(z), Selected: z == selected, Separator: len(opts) == 1})
		}
	}
	if held != "floating" {
		opts = append(opts, view.Option{Value: "floating", Label: floatingLabel, Selected: selected == "floating", Separator: true})
	}
	return opts
}

// newZoneOptions are the zones a new event offers: every zone of zoneNames
// with the instance zone chosen (C-10), a line and Floating, last. Nothing
// came «As sent».
func newZoneOptions(selected string) []view.Option {
	opts := make([]view.Option, 0, len(zoneNames)+1)
	for _, z := range zoneNames {
		opts = append(opts, view.Option{Value: z, Label: text(z), Selected: z == selected})
	}
	return append(opts, view.Option{Value: "floating", Label: floatingLabel, Selected: selected == "floating", Separator: true})
}

// instanceZone is the zone a new event is written in unless another is
// chosen: the instance's (C-10), by its name in the list; one that has no
// name there — the system's own, unnamed — is written in UTC, the same
// instant.
func (s *Server) instanceZone() string {
	name := s.tz().String()
	if _, found := slices.BinarySearch(zoneNames, name); found {
		return name
	}
	return "UTC"
}

// zoneOffered reports whether a zone may be chosen: the one held, Floating,
// or one of zoneNames the zone database knows. The location is nil for the
// held zone and for Floating.
func zoneOffered(z, held string) (*time.Location, bool) {
	if z == held || z == "floating" {
		return nil, true
	}
	if _, found := slices.BinarySearch(zoneNames, z); !found {
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

// relatedKinds order the list of Related to where two records share a
// moment: the order of the frame (s-tasks-10).
var relatedKinds = map[view.Kind]int{view.KindEvent: 0, view.KindTask: 1, view.KindNote: 2}

// relatedOptions are the list of Related to (s-tasks-10): the records this
// session has loaded, of every collection of the catalog — events, tasks and
// notes, each with its collection's mark and «kind · day», the day the short
// way of a list — newest first, and last, after a line, «By UID…», the value
// typed by hand under the same name, with the line under it (s-tasks-11).
// The record itself is not offered. The ones related now are chosen; those
// outside the loaded lists stand first, in the record's order: a card a
// timeline wrote a note for as its name and «contact», anything else as its
// UID — and unloaded says so. No «Nothing»: an empty field is no relation.
// Nothing is asked of the servers.
func (s *Server) relatedOptions(c *collectionCtx, self string, selected []string) (opts []view.Option, unloaded bool) {
	blob := s.catalogOf(c.sess)
	type loaded struct {
		indexEntry
		colour view.Colour
	}
	var list []loaded
	cards := map[string]loaded{}
	seen := map[string]bool{self: true, "": true}
	eachIndexed(c.sess, blob, func(col sealed.Collection, e indexEntry) {
		if seen[e.UID] {
			return
		}
		seen[e.UID] = true
		l := loaded{e, catalogColour(col)}
		switch {
		case e.Kind != view.KindContact:
			list = append(list, l)
		case slices.Contains(selected, e.UID):
			cards[e.UID] = l
		}
	})
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if (a.At == 0) != (b.At == 0) {
			return a.At != 0
		}
		if a.At != b.At {
			return a.At > b.At
		}
		if relatedKinds[a.Kind] != relatedKinds[b.Kind] {
			return relatedKinds[a.Kind] < relatedKinds[b.Kind]
		}
		return a.Title < b.Title
	})
	option := func(l loaded) view.Option {
		meta := string(l.Kind)
		if l.At != 0 {
			meta += " · " + c.tf.shortDay(time.Unix(l.At, 0))
		}
		return view.Option{Value: l.UID, Label: text(l.Title), Colour: l.colour, Meta: text(meta), Selected: slices.Contains(selected, l.UID)}
	}
	for _, uid := range selected {
		switch card, ok := cards[uid]; {
		case ok:
			// A card is its name and «contact», one pill (s-tasks-11).
			opts = append(opts, view.Option{Value: card.UID, Label: text(card.Title + " · contact"), Colour: card.colour, Selected: true})
		case uid != "" && !seen[uid]:
			opts = append(opts, view.Option{Value: uid, Label: text(uid), Selected: true})
			seen[uid], unloaded = true, true
		}
	}
	for _, l := range list {
		opts = append(opts, option(l))
	}
	return append(opts, view.Option{Label: "By UID…", Separator: true, Typed: true,
		Hint: "By UID: a record that is not among the loaded ones."}), unloaded
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

// ruleUntil is the day a rule's UNTIL ends the series on, as a person counts
// it (the client's scan of the calendar, Г3): the last day an occurrence can
// start on. RFC 5545 §3.3.10 bounds the start by UNTIL inclusively. A date
// UNTIL is that day. A UTC instant is its day in tz — unless its clock there
// is before the series' time of day, in which case no occurrence of that day
// reaches it and the day before is the last: UNTIL=20261228T235959Z of a
// series on Mondays at 12:00 in Yekaterinburg is 29 December, 02:59 in
// Moscow, and the series ends on Monday 28 December. start is DTSTART; a
// zero start compares nothing. What the form writes back — the end of the
// day in tz — reads as that day again. ok is false without an UNTIL.
func ruleUntil(raw string, start time.Time, tz *time.Location) (time.Time, bool) {
	u := parseRule(raw).get("UNTIL")
	if len(u) == len("20060102") {
		d, err := time.Parse("20060102", u)
		return d, err == nil
	}
	at, err := time.Parse("20060102T150405Z", u)
	if err != nil || tz == nil {
		return time.Time{}, false
	}
	local := at.In(tz)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, tz)
	if !start.IsZero() {
		s := start.In(tz)
		if clock := time.Date(day.Year(), day.Month(), day.Day(), s.Hour(), s.Minute(), s.Second(), 0, tz); local.Before(clock) {
			day = day.AddDate(0, 0, -1)
		}
	}
	return day, true
}

// repeatValues are an RRULE as the form shows it: frequency, every, the plain
// days of BYDAY in the form's order, until as the last day of the series in
// tz (ruleUntil; start is DTSTART), count, and the rule itself.
func repeatValues(raw string, tz *time.Location, start time.Time) []string {
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
	if day, ok := ruleUntil(raw, start, tz); ok {
		out[repUntil] = day.Format("2006-01-02")
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
// typedRule is Advanced RRULE as the form sent it: empty is no rule typed,
// and the rule the form was shown with — its hidden «.base» — stands, so the
// fields change it and nothing is read as a rule taken away (s-agenda-3).
func typedRule(r *http.Request, name string) string {
	if v := strings.TrimSpace(r.FormValue(name + ".rule")); v != "" {
		return v
	}
	return strings.TrimSpace(r.FormValue(name + ".base"))
}

// held is the rule the record has: the sample of Advanced RRULE, which shows
// empty until a rule other than it is typed (s-agenda-3), and the hidden
// «.base» the form sends back with it (typedRule).
func repeatSet(f editField, vals []string, held string, errText view.Text) view.FieldSet {
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
		{Name: f.name + ".interval", Label: "Every", Kind: view.InputNumber, Value: text(v[repInterval]), Mono: true},
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
	// Count and Advanced RRULE as s-agenda-3 draws them: no hint under
	// Count (both at once is refused with its reason), and the advanced rule
	// empty with the rule the series has as its sample — typed, it wins over
	// the fields above.
	typed := strings.TrimSpace(v[repRule])
	if typed == held {
		typed = ""
	}
	set.Rows = append(set.Rows, days,
		view.FormRow{Name: f.name + ".until", Label: "Until", Kind: view.InputDate, Value: text(v[repUntil]), Mono: true},
		view.FormRow{Name: f.name + ".count", Label: "Count", Kind: view.InputNumber, Value: text(v[repCount])},
		view.FormRow{Name: f.name + ".rule", Label: "Advanced RRULE", Kind: view.InputText, Mono: true, Value: text(typed), Placeholder: text(held),
			Hint: "The advanced value takes precedence when set."},
	)
	if held != "" {
		set.Rows = append(set.Rows, view.FormRow{Name: f.name + ".base", Kind: view.InputHidden, Value: text(held)})
	}
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
		// No hint under it (s-contacts-3): the order Display name is cut in is
		// said on the quick sheet (NewSheet.Note, s-notes-8).
		{name: "fn", label: "Display name", prop: "FN", typ: fieldText},
		{name: "n", label: "Name", prop: "N", typ: fieldName},
		// The nickname and the birthday stand in the head empty too
		// (s-contacts-3, s-contacts-4).
		{name: "nickname", label: "Nickname", prop: "NICKNAME", typ: fieldText},
		// Who the person is, then where they work and when they were born,
		// then how to reach them, then the rest: the order of s-contacts-3.
		{name: "org", label: "Organisation", prop: "ORG", typ: fieldText, optional: true},
		{name: "title", label: "Title", prop: "TITLE", typ: fieldText, optional: true},
		{name: "bday", label: "Birthday", prop: "BDAY", typ: fieldBirthday,
			hint: "Type it: 14.03.1987, or 14.03 without the year."},
		{name: "tel", label: "Number", prop: "TEL", typ: fieldMulti, typed: true, mono: true, set: "Phone numbers", add: "Add phone", newRow: "cell", quick: "Phone", quickSample: "+7 …"},
		{name: "email", label: "Address", prop: "EMAIL", typ: fieldMulti, typed: true, set: "Email addresses", add: "Add email", quick: "Email", quickSample: "name@example.org"},
		// Addresses after the e-mail (s-contacts-3), folded away with their
		// button when the card has none.
		{name: "adr", label: "Address", prop: "ADR", typ: fieldAddress, typed: true, set: "Addresses", optional: true, add: "Add address"},
		{name: "categories", label: "Tags", prop: "CATEGORIES", typ: fieldList, optional: true, set: "Other"},
		{name: "note", label: "Note", prop: "NOTE", typ: fieldArea, optional: true, set: "Other"},
	},
	view.KindEvent: {
		{name: "summary", label: "Summary", prop: "SUMMARY", typ: fieldText, quick: "Title"},
		{name: "location", label: "Location", prop: "LOCATION", typ: fieldText},
		{name: "status", label: "Status", prop: "STATUS", typ: fieldChoice, choices: eventStatuses},
		// No hint under Tags (s-agenda-3).
		{name: "categories", label: "Tags", prop: "CATEGORIES", typ: fieldList},
		{name: "description", label: "Description", prop: "DESCRIPTION", typ: fieldArea},
		{name: "attendee", label: "Attendees", prop: "ATTENDEE", typ: fieldAttendee},
		{name: "start", label: "Start date", timeLabel: "Start time", prop: "DTSTART", typ: fieldTime, required: true, allDay: true, set: "When",
			quick: "Date", quickTime: "Start"},
		{name: "end", label: "End date", timeLabel: "End time", prop: "DTEND", typ: fieldTime, set: "When", quickTime: "End", quickNoDate: true},
		{name: "tz", label: "Time zone", prop: "DTSTART", typ: fieldZone, set: "When"},
		{name: "reminder", label: "Reminders", typ: fieldReminders, set: "Reminders", add: "Add reminder", quick: "Remind"},
		{name: "repeat", label: "Repeats", prop: "RRULE", typ: fieldRepeat, set: "Repeats"},
	},
	view.KindTask: {
		{name: "summary", label: "Summary", prop: "SUMMARY", typ: fieldText},
		{name: "description", label: "Description", prop: "DESCRIPTION", typ: fieldArea},
		{name: "start", label: "Start date", timeLabel: "Start time", prop: "DTSTART", typ: fieldTime, taskDay: true, set: "When"},
		{name: "due", label: "Due date", timeLabel: "Due time", prop: "DUE", typ: fieldTime, taskDay: true, set: "When",
			quickSample: "YYYY-MM-DD", quickTimeSample: "--:--"},
		{name: "status", label: "Status", prop: "STATUS", typ: fieldChoice, choices: taskStatuses, blank: "NEEDS-ACTION", set: "Progress"},
		{name: "priority", label: "Priority", prop: "PRIORITY", typ: fieldChoice, choices: priorities, set: "Progress"},
		{name: "percent", label: "Percent complete", prop: "PERCENT-COMPLETE", typ: fieldPercent, set: "Progress"},
		{name: "done", label: "Done", prop: "STATUS", typ: fieldDone, set: "Progress"},
		{name: "reminder", label: "Reminders", typ: fieldReminders, set: "Reminders", add: "Add reminder", quick: "Remind"},
		{name: "location", label: "Location", prop: "LOCATION", typ: fieldText, set: "Other"},
		// No hint under Tags (s-tasks-3), as the event's and the note's.
		{name: "categories", label: "Tags", prop: "CATEGORIES", typ: fieldList, set: "Other"},
		{name: "related", label: "Related to", prop: "RELATED-TO", typ: fieldRelated, set: "Other"},
	},
	view.KindNote: {
		{name: "summary", label: "Title", prop: "SUMMARY", typ: fieldText},
		{name: "description", label: "Body", prop: "DESCRIPTION", typ: fieldArea},
		{name: "start", label: "Date", prop: "DTSTART", typ: fieldTime, timeOptional: true},
		// No hint under Tags (s-notes-20), as the event's.
		{name: "categories", label: "Tags", prop: "CATEGORIES", typ: fieldList},
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

// quickNotes are the lines under the quick sheet's fields: what it does not
// ask and why (s-notes-8, s-notes-9, s-notes-10).
var quickNotes = map[view.Kind]view.Text{
	view.KindContact: "The first word is Given, the rest Family — «Пётр Ковалёв», not «Ковалёв Пётр». Everything else is filled in on the card.",
	view.KindEvent:   "Repeats, guests and more live on the card. A reminder is set here because that is why most events are created.",
	view.KindTask:    "No time means the whole day. A task with no due date is a task all the same — both may stay empty.",
}

// caretField is the field of a record's form the caret stands in when it is
// shown, full or quick: the name of a contact (s-contacts-3, s-notes-8), the
// summary of an event and a task (s-agenda-3, s-notes-9, s-tasks-3,
// s-notes-10), a note's text (s-notes-5, s-notes-20). A field that came back
// with a reason takes it instead (s-contacts-18).
var caretField = map[view.Kind]string{
	view.KindContact: "fn",
	view.KindEvent:   "summary",
	view.KindTask:    "summary",
	view.KindNote:    "description",
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

// rowLabels are the labels a phone, an e-mail or an address row is offered
// (TYPE), the lists of s-contacts-12 and s-contacts-13 and «home, work…» of an
// address (s-contacts-3): vCard types, which another client names in its own
// words. «mobile» is TYPE=cell and «main» TYPE=main, as other
// clients write them. A label another client wrote is offered too, so keeping
// it is a choice.
var rowLabels = map[string][]fieldOption{
	"TEL": {{value: "cell", label: "mobile"}, {value: "work", label: "work"}, {value: "home", label: "home"},
		{value: "main", label: "main"}, {value: "fax", label: "fax"}, {value: "other", label: "other"}},
	"EMAIL": {{value: "work", label: "work"}, {value: "home", label: "home"}, {value: "other", label: "other"}},
	"ADR":   {{value: "home", label: "home"}, {value: "work", label: "work"}, {value: "other", label: "other"}},
}

// addressParts are the inputs of an address row: ADR components 2 to 6
// (RFC 6350 §6.3.1).
var addressParts = []struct {
	name, label string
	mono        bool // drawn monospaced (s-contacts-3)
}{
	{"street", "Street", false}, {"city", "City", false}, {"region", "Region", false}, {"code", "Postcode", true}, {"country", "Country", false},
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
	// An event's times are shown in its own zone; a task's in the
	// instance's: it has no zone to choose.
	at := tz
	if hasField(c.spec.kind, "tz") {
		at = formZone(zoneOf(props["DTSTART"]), tz)
	}
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
			var start time.Time
			if tm, err := cal.TimingOf(obj, tz); err == nil {
				start = tm.Start
			}
			vals[f.name] = repeatValues(firstValue(obj, f.prop), tz, start)
		case fieldRelated:
			vals[f.name] = cal.RelatedTo(obj)
		case fieldAttendee:
			vals[f.name] = attendeeValues(obj)
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
			var rows, labels []string
			for _, v := range props[f.prop] {
				p := strings.SplitN(v.Text, ";", 7)
				for len(p) < 7 {
					p = append(p, "")
				}
				rows = append(rows, addressRow(p[2:]))
				labels = append(labels, v.Params["TYPE"])
			}
			vals[f.name] = append(rows, "")
			vals[f.name+".type"] = append(labels, "")
		case fieldTime:
			tm, err := cal.TimingOf(obj, tz)
			if f.prop == "DUE" {
				tm, err = cal.DueOf(obj, tz)
			}
			if err != nil {
				return nil, nil, err
			}
			vals[f.name] = timeRow(f, tm, at)
		case fieldDone:
			vals[f.name] = []string{""}
			if taskDone(obj) {
				vals[f.name] = []string{"on"}
			}
		case fieldPercent:
			// None is «0» (s-tasks-3, s-tasks-2 «0%»): saved as shown it
			// writes nothing — the version is measured by the same value.
			vals[f.name] = []string{strings.TrimSpace(firstValue(obj, f.prop))}
			if vals[f.name][0] == "" {
				vals[f.name][0] = "0"
			}
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
	if c.spec.kind == view.KindTask {
		vals[taskDayBox] = []string{taskDayOf(vals)}
	}
	return vals, alarms, nil
}

// taskDayBox is the name of a task's one «All day» (s-tasks-3).
const taskDayBox = "allday"

// taskDayOf is what a task's one box shows: whether its start is a day, or,
// without a start, its due.
func taskDayOf(vals formValues) string {
	for _, f := range editFields[view.KindTask] {
		if row := triple(vals[f.name]); f.taskDay && row[0] != "" {
			return row[2]
		}
	}
	return ""
}

// oneTaskDay is a task's one box against its two times (S-7). Ticked or
// cleared, it is what both times become. Left as the form showed it, each
// time keeps its own kind: another client's task whose start is a day and
// whose due a time is not rewritten by a save that did not touch the box.
// have is the version the change is measured against.
func oneTaskDay(have, want formValues) {
	box, ok := want[taskDayBox]
	if !ok {
		return
	}
	for _, f := range editFields[view.KindTask] {
		if !f.taskDay {
			continue
		}
		w, h := triple(want[f.name]), triple(have[f.name])
		switch {
		case w[0] == "":
			w[2] = ""
		case first(box) == taskDayOf(have) && h[0] != "":
			w[2] = h[2]
		default:
			w[2] = first(box)
		}
		want[f.name] = w[:]
	}
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
	if tm.AllDay && (f.allDay || f.taskDay || f.timeOptional) {
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
			vals[f.name] = repeatValues("", nil, time.Time{})
		case fieldChoice:
			vals[f.name] = []string{f.blank}
		case fieldZone:
			// A new event is written in the instance zone unless another is
			// chosen (from-typecase note 19).
			vals[f.name] = []string{s.instanceZone()}
		case fieldAttendee:
			vals[f.name] = []string{}
		case fieldPercent:
			vals[f.name] = []string{"0"} // as a task without one shows it
		default:
			vals[f.name] = []string{""}
		}
	}
	if c.spec.kind == view.KindTask {
		vals[taskDayBox] = []string{""}
	}
	return vals
}

// newEventValues are what the full form of a new event starts with
// (s-agenda-15, Typecase task 77): the next whole hour, an hour long, in the
// zone chosen — the instance's; one reminder, First, 10 minutes before;
// Every 1, nothing repeating. They are values, not samples: a new record is
// measured against the empty form (createRecord), so saved as shown they are
// written.
func (s *Server) newEventValues(values formValues) {
	now := s.now().In(formZone(first(values["tz"]), s.tz()))
	start := time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, 0, 0, 0, now.Location())
	end := start.Add(time.Hour)
	values["start"] = []string{start.Format("2006-01-02"), start.Format("15:04"), ""}
	values["end"] = []string{end.Format("2006-01-02"), end.Format("15:04"), ""}
	values["reminder"] = []string{"-PT10M", ""}
	if rep := values["repeat"]; len(rep) > repInterval {
		rep[repInterval] = "1"
	}
}

func checkbox(r *http.Request, name string) string {
	if v := r.FormValue(name); v != "" && v != "off" {
		return "on"
	}
	return ""
}

// indexedRows reads name.0, name.1, … as far as they go, and ends them with
// the empty slot the values of a repeating field end with. A row added by
// «Add …» (C-6.8 InputAdd) takes the next number, so it is read here too.
func indexedRows(r *http.Request, name string) []string {
	var rows []string
	for i := 0; ; i++ {
		key := name + "." + strconv.Itoa(i)
		if _, ok := r.Form[key]; !ok {
			break
		}
		rows = append(rows, strings.TrimSpace(r.FormValue(key)))
	}
	return append(rows, "")
}

func (s *Server) submittedValues(c *collectionCtx, r *http.Request) formValues {
	vals := formValues{}
	for _, f := range editFields[c.spec.kind] {
		switch f.typ {
		case fieldText, fieldPercent, fieldList, fieldBirthday, fieldChoice, fieldZone:
			vals[f.name] = []string{strings.TrimSpace(r.FormValue(f.name))}
		case fieldAttendee:
			// Every kept one under the one name; the line typed in only
			// when the form drew the field (attendeeOps).
			r.FormValue(f.name) // parses the form
			vals[f.name] = []string{}
			for _, v := range r.Form[f.name] {
				if v = strings.TrimSpace(v); v != "" {
					vals[f.name] = append(vals[f.name], v)
				}
			}
			if _, drawn := r.Form[f.name+".add"]; drawn {
				vals[f.name+".add"] = []string{strings.TrimSpace(r.FormValue(f.name + ".add"))}
			}
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
				vals[f.name+".type"] = append(labels, "")
			}
		case fieldRelated:
			_ = r.FormValue(f.name) // parses the form
			vals[f.name] = s.relatedTyped(c, r.Form[f.name], r.FormValue(f.name+".add"))
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
				strings.TrimSpace(r.FormValue(f.name + ".count")), typedRule(r, f.name),
			}
		case fieldAddress:
			part := func(prefix string) string {
				var parts []string
				for _, p := range addressParts {
					parts = append(parts, strings.TrimSpace(r.FormValue(prefix+"."+p.name)))
				}
				return addressRow(parts)
			}
			var rows, labels []string
			for i := 0; ; i++ {
				prefix := fmt.Sprintf("%s.%d", f.name, i)
				if _, ok := r.Form[prefix+".street"]; !ok {
					break
				}
				rows = append(rows, part(prefix))
				labels = append(labels, strings.TrimSpace(r.FormValue(prefix+".type")))
			}
			vals[f.name] = append(rows, "")
			vals[f.name+".type"] = append(labels, "")
		case fieldTime:
			row := []string{strings.TrimSpace(r.FormValue(f.name + ".date")), strings.TrimSpace(r.FormValue(f.name + ".time")), ""}
			if f.allDay {
				row[2] = checkbox(r, f.name+".allday")
			}
			if f.taskDay {
				vals[taskDayBox] = []string{checkbox(r, taskDayBox)}
				if row[0] != "" {
					row[2] = first(vals[taskDayBox])
				}
			}
			if f.timeOptional && row[0] != "" && row[1] == "" {
				row[2] = "on"
			}
			vals[f.name] = row
		case fieldDone:
			vals[f.name] = []string{checkbox(r, f.name)}
		}
	}
	for _, name := range r.Form[removedKey] {
		removeRow(c.spec.kind, vals, name)
	}
	sameDayEnds(c.spec.kind, vals)
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
	for _, f := range editFields[c.spec.kind] {
		if f.typed {
			want[f.name+".type"] = keptLabels(f.prop, want[f.name+".type"], have[f.name+".type"])
		}
	}
	startRow := triple(want["start"])
	startChanged := !sameRows(want["start"], have["start"])
	// A new zone rewrites both times at the clock the form names, in that
	// zone, even when neither row changed: the form is in the event's own
	// zone (formZone), and what it shows is what is written. A new event is
	// written in the zone chosen — the instance's unless another was (the
	// quick sheet chooses none) — the same way (from-typecase note 19).
	var zone string
	held, chosen := first(have["tz"]), first(want["tz"])
	fresh := hasField(c.spec.kind, "tz") && len(props["DTSTART"]) == 0
	if fresh {
		held = ""
		if chosen == "" {
			chosen = s.instanceZone()
		}
	}
	if (fresh || (held != "" && chosen != held)) && startRow[2] != "on" {
		loc, ok := zoneOffered(chosen, held)
		switch {
		case !ok || chosen == "":
			ch.errors["tz"] = "Choose a time zone from the list."
		default:
			zone = chosen
			if loc != nil && zone != "UTC" {
				ch.zone = loc
			}
		}
	}
	// The rows are read in the zone they are written in: the one chosen,
	// else the one held. A task has no zone and reads the instance's.
	clock := tz
	if hasField(c.spec.kind, "tz") {
		clock = formZone(held, tz)
		if zone != "" {
			clock = formZone(zone, tz)
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
			labels := want[f.name+".type"]
			if sameRows(w, h) && sameLabels(labels, have[f.name+".type"]) {
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
			ch.ops = append(ch.ops, multiOps(f.prop, cur, rows, labels)...)
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
			ch.ops = append(ch.ops, s.attendeeOps(c, obj, have, want, ch.errors)...)
		case fieldRelated:
			// The relations kept stay in the record's order, the new ones
			// follow in the form's; the same relations in another order
			// are no change.
			var want []string
			for _, v := range h {
				if slices.Contains(w, v) {
					want = append(want, v)
				}
			}
			for _, v := range w {
				if !slices.Contains(h, v) {
					want = append(want, v)
				}
			}
			if slices.Equal(want, h) {
				continue
			}
			ch.ops = append(ch.ops, valueOps(f.prop, props[f.prop], want)...)
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
			if f.name == "end" && c.spec.kind == view.KindEvent && endless(row, startRow, props) {
				// An event without an end lasts no time at all, or the one
				// day of its date (RFC 5545 §3.6.1), and iCloud refuses it
				// with 404 (docs/servers.md, SV-53). So the end is written
				// out: the start itself, and for a date the day after it
				// (shift below). The same event, on every server.
				row = startRow
				w = row[:]
			}
			if f.required && row[0] == "" {
				ch.errors[f.name] = f.label + " is required."
				continue
			}
			at, err := rowTime(row, allDay, clock)
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
			ops := timeOps(f.prop, props[f.prop], row, allDay, shift, clock, zone)
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
// each other's work. A task's one «All day» is laid on its two times here
// too (oneTaskDay). have is the version the change is measured against.
func fillNames(kind view.Kind, have, want formValues) {
	if kind == view.KindTask {
		oneTaskDay(have, want)
	}
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

// endless reports an event the form would write without an end, when the end
// is the person's to give: a new event, or one whose end is being cleared. An
// event another client wrote without an end, and edited here in something
// else, stays as it came (S-7); one with DURATION keeps it. A start the form
// cannot read gets its own error, and no end is made of it.
func endless(end, start [3]string, props map[string][]model.RawValue) bool {
	if end[0] != "" || start[0] == "" || (start[2] != "on" && start[1] == "") || len(props["DURATION"]) > 0 {
		return false
	}
	return len(props["DTSTART"]) == 0 || len(props["DTEND"]) > 0
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

// valueOps rewrites a property that stands several times by its values: a
// value kept keeps the parameters it had, wherever it stands now
// (RELTYPE=SIBLING stays on the relation it was on when the one before it
// goes), a new one comes without parameters, and what is not wanted goes.
func valueOps(prop string, cur []model.RawValue, want []string) []model.PatchOp {
	if len(want) == 0 {
		if len(cur) > 0 {
			return []model.PatchOp{model.RemoveProp{Prop: prop}}
		}
		return nil
	}
	ops := []model.PatchOp{model.SetValues{Prop: prop, Values: want}}
	for i, v := range want {
		for _, had := range cur {
			if strings.TrimSpace(had.Text) != v {
				continue
			}
			for _, k := range sortedKeys(had.Params) {
				ops = append(ops, model.SetParam{Prop: prop, Index: i, Param: k, Value: had.Params[k]})
			}
			break
		}
	}
	return ops
}

// relatedTyped are the relations a form asks for: the ones ticked, and the
// one typed into the line «related.add» — a UID, or the title of a record
// this session has loaded, which is that record —, each once, in the form's
// order.
func (s *Server) relatedTyped(c *collectionCtx, ticked []string, typed string) []string {
	out := []string{}
	add := func(v string) {
		if v = strings.TrimSpace(v); v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	for _, v := range ticked {
		add(v)
	}
	if typed = strings.TrimSpace(typed); typed != "" {
		uid := typed
		eachIndexed(c.sess, s.catalogOf(c.sess), func(_ sealed.Collection, e indexEntry) {
			if uid == typed && e.Kind != view.KindContact && e.Title == typed {
				uid = e.UID
			}
		})
		add(uid)
	}
	return out
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

// notLabels are TYPE values that say something other than a label: the
// vCard 3 default of an e-mail (INTERNET) and of a phone (VOICE), X400, the
// preference (PREF), a phone that takes messages (MSG), and the postal kinds
// of an address (DOM, INTL, POSTAL, PARCEL). They are no label — not
// offered as another client's — and stay in the card as they are (S-7).
var notLabels = map[string]bool{
	"internet": true, "x400": true, "pref": true, "voice": true, "msg": true,
	"dom": true, "intl": true, "postal": true, "parcel": true,
}

// typeParts splits a TYPE into the parts that are a label and the rest.
func typeParts(raw string) (labels, rest []string) {
	for _, p := range strings.Split(raw, ",") {
		switch p = strings.TrimSpace(p); {
		case p == "":
		case notLabels[strings.ToLower(p)]:
			rest = append(rest, p)
		default:
			labels = append(labels, p)
		}
	}
	return labels, rest
}

// shownLabel is the label a TYPE shows as chosen: its first part that is one
// of the rowLabels of prop, as the card writes it — WORK,PREF is WORK, «work»
// — else its label parts as they are, another client's, else none.
func shownLabel(prop, raw string) (value string, known bool) {
	labels, _ := typeParts(raw)
	for _, p := range labels {
		for _, l := range rowLabels[prop] {
			if strings.EqualFold(l.value, p) {
				return p, true
			}
		}
	}
	return strings.Join(labels, ","), false
}

// labelOptions are the rowLabels of prop and the row's own label when it is
// none of them. A label is matched whatever its case, and a matched one keeps
// the value as the card holds it: TYPE=CELL is «mobile», offered once. A row
// without a label — none, or only what is no label (notLabels) — selects
// none. Another client's label stands under a line as it is
// (s-contacts-22): not turned into a type, not lost.
func labelOptions(prop, selected string) []view.Option {
	var opts []view.Option
	chosen, known := shownLabel(prop, selected)
	for _, l := range rowLabels[prop] {
		o := view.Option{Value: l.value, Label: text(l.label)}
		if known && strings.EqualFold(l.value, chosen) {
			o.Value, o.Selected = chosen, true
		}
		opts = append(opts, o)
	}
	if !known && chosen != "" {
		opts = append(opts, view.Option{Value: chosen, Label: text(chosen), Selected: true, Separator: true})
	}
	return opts
}

// keptLabels are the labels a save writes: a row whose chosen label is the
// one it showed keeps its TYPE exactly as the card holds it, and a label
// chosen anew replaces only the label, keeping what is no label beside it
// (WORK,PREF made home is home,PREF). have are the TYPEs of the rows the
// form had, want the labels posted.
func keptLabels(prop string, want, have []string) []string {
	out := slices.Clone(want)
	for i := range min(len(want), len(have)) {
		shown, _ := shownLabel(prop, have[i])
		if strings.EqualFold(want[i], have[i]) || strings.EqualFold(strings.TrimSpace(want[i]), shown) {
			out[i] = have[i]
			continue
		}
		_, rest := typeParts(have[i])
		if l := strings.TrimSpace(want[i]); l != "" {
			rest = append([]string{l}, rest...)
		}
		out[i] = strings.Join(rest, ",")
	}
	return out
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

// apply patches obj and, when the reminder set changed, rewrites it. A
// series whose start the patch changed takes its exceptions with it
// (cal.RetimeExceptions).
func (c *collectionCtx) apply(obj *model.Object, ch change) (*model.Object, error) {
	if len(ch.ops) > 0 {
		started := obj.Props()["DTSTART"]
		patch, err := model.NewPatch(ch.ops...)
		if err != nil {
			return nil, err
		}
		if err := patch.Apply(obj); err != nil {
			return nil, err
		}
		if obj.IsCalendar() && len(started) > 0 {
			if obj, _, err = cal.RetimeExceptions(obj, started[0]); err != nil {
				return nil, err
			}
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
	// unfold: the form came with ?fields=all — «Show N empty fields»
	// without scripts — and folds nothing; its posts keep it so.
	unfold bool
	// formUID: the UID a new contact's form carries once a photo is chosen
	// for it (photo_new.go); Save creates the card under it. typedUID: the
	// UID the typing alone is kept under, the photo cancelled or removed —
	// the next Add keeps it
	formUID  string
	typedUID string
}

func (s *Server) editAction(c *collectionCtx, st editState) string {
	if st.isNew {
		return c.listPath() + "/new"
	}
	return c.recordPath(st.uid) + "/edit"
}

func (s *Server) editPage(c *collectionCtx, st editState) any {
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
	sh.Top.Refresh = s.recordRefresh(c, s.editAction(c, st))
	sh.Notices = append(sh.Notices, st.notices...)
	if len(st.errors) > 0 {
		sh.Notices = append(sh.Notices, view.Notice{Role: view.NoticeAlert, Title: "Nothing was saved", Body: "Correct the marked fields and save again."})
	}
	form := s.recordForm(c, st)
	// Attaching is a write of its own (S-20). A record not yet created
	// keeps its files in a draft under the UID it will have, and its Attach
	// sends the form (attach_new.go, s-agenda-15).
	attaching := st.obj != nil && !st.isNew && !c.col.ReadOnly && c.cal != nil
	var draft attachDraft
	newAttach := st.isNew && !c.col.ReadOnly && c.cal != nil
	if newAttach {
		draft, _ = loadAttachDraft(c, st.formUID)
		if st.formUID != "" && len(form.Sets) > 0 {
			form.Sets[0].Rows = append(form.Sets[0].Rows, view.FormRow{Name: "uid", Kind: view.InputHidden, Value: text(st.formUID)})
		}
	}
	if attaching && c.spec.kind != view.KindNote {
		form.Actions = append(form.Actions, s.attachAction(c, st.uid, st.etag, "edit"))
	}
	if newAttach && c.spec.kind != view.KindNote {
		form.Actions = append(form.Actions, s.newAttachAction(c, st.formUID))
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
		// The head: the person and the book (s-contacts-3).
		name := strings.TrimSpace(first(st.values["fn"]))
		if st.isNew || name == "" {
			name = "New contact"
		}
		if st.isNew && st.formUID != "" && len(form.Sets) > 0 {
			form.Sets[0].Rows = append(form.Sets[0].Rows, view.FormRow{Name: "uid", Kind: view.InputHidden, Value: text(st.formUID)})
		}
		page := view.ContactEditPage{Shell: sh, UID: st.uid, Name: text(name), Subtitle: view.Text(c.stamp()), Form: form, Loss: st.loss, ReadOnly: c.col.ReadOnly}
		page.Photo = s.photoBlock(c, st)
		return page
	case view.KindEvent:
		// The head: the event and its calendar (s-agenda-3). The reminders
		// and the rule are the form's rows; the rule's words stand on the
		// card, not here.
		name := strings.TrimSpace(first(st.values["summary"]))
		if st.isNew || name == "" {
			name = "New event"
		}
		page := view.EventEditPage{Shell: sh, UID: st.uid, Name: text(name), Subtitle: view.Text(c.stamp()), Form: form, Loss: st.loss, Delete: del,
			Attendees: s.attendeeField(c, st)}
		if newAttach {
			page.Attach = s.draftRows(c, draft)
		}
		if st.obj != nil {
			if cal.HasRecurrence(st.obj) && del != nil {
				// The line stands under Delete, once.
				page.DeleteNote = seriesNote
			}
			page.Attach = s.attachRows(c, st.uid, st.obj, st.etag, "edit")
			// «Kept as the server sent them» (s-agenda-3): what Carrel does
			// not know, as it came.
			for _, name := range cal.EventForeign(st.obj) {
				page.Foreign = append(page.Foreign, view.Field{Label: text(name), Value: text(strings.Join(st.obj.PropText(name), ", ")), Mono: true})
			}
		}
		return page
	case view.KindTask:
		// The head: the task, its list and how late it is; Source in the
		// form (s-tasks-3).
		name := strings.TrimSpace(first(st.values["summary"]))
		if st.isNew || name == "" {
			name = "New task"
		}
		page := view.TaskEditPage{Shell: sh, UID: st.uid, Name: text(name), Subtitle: taskSubtitle(c, st.obj), Form: form, Source: c.source(), Loss: st.loss, Delete: del}
		if st.obj != nil && !st.isNew {
			page.Attach = s.attachRows(c, st.uid, st.obj, st.etag, "edit")
			page.Source.Path, page.Source.Compared = text(st.obj.Path), s.eventCompared(c, st.uid, st.obj)
		}
		if newAttach {
			page.Attach = s.draftRows(c, draft)
		}
		return page
	default:
		page := s.noteEditPage(c, st, sh, form, attaching, del)
		if newAttach {
			// The note's Attach stands in the bar and the column, as on
			// one that exists (s-notes-20).
			a := s.newAttachAction(c, st.formUID)
			page.Doc.Bar = append(page.Doc.Bar, a)
			page.Meta.Actions = shown(&a)
			page.Meta.Attach = s.draftRows(c, draft)
		}
		return page
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
	if !st.isNew {
		page.Source.UID = text(st.uid) // «UID …» of the column's Source (s-notes-20)
	}
	page.Doc = view.NoteDoc{
		Mode: view.NoteEditing, Title: text(first(st.values["summary"])), Body: text(body),
		Rendered: cal.RenderMarkdown(body), ReadOnly: c.col.ReadOnly,
		Bar: []view.Action{
			// Save sends the document's form (Submits): without scripts the
			// title, the text and the fields go with it (НД1, U-8).
			{ID: "record.save", Label: "Save", Href: s.postQuery(s.editAction(c, st), url.Values{"etag": {st.etag}, "readat": {strconv.FormatInt(st.readAt.Unix(), 10)}}), Method: "POST", Primary: true, Enabled: true, Submits: true},
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
	// The document is the form (НД1, U-8): its hidden rows — the token and
	// the version —, the title and the text by the names the server reads.
	// The rest of the form is Fields (C-6), sent with it.
	doc := view.Form{Action: form.Action, Method: form.Method, Target: form.Target, Sets: []view.FieldSet{{}}, Errors: form.Errors, Actions: []view.Action{}}
	var title, texts []view.FormRow
	for _, set := range form.Sets {
		for _, row := range set.Rows {
			switch {
			case row.Kind == view.InputHidden:
				doc.Sets[0].Rows = append(doc.Sets[0].Rows, row)
			case row.Name == "summary":
				title = append(title, row)
			case row.Name == "description":
				texts = append(texts, row)
			default:
				page.Doc.Fields = append(page.Doc.Fields, row)
			}
		}
	}
	doc.Sets[0].Rows = append(append(doc.Sets[0].Rows, title...), texts...)
	page.Doc.Form = &doc
	if exists {
		page.Rail.Neighbours = s.noteNeighbours(c, st.uid)
		page.Doc.Foreign = foreignLine(cal.NoteForeign(st.obj))
		page.Meta.Attach = s.attachRows(c, st.uid, st.obj, st.etag, "edit")
		page.Meta.Linked = st.linked
		page.Meta.Actions = shown(attach, del)
	}
	return page
}

// foreignLine is the line under the text of a note's edit (s-notes-20): the
// first property Carrel does not know and how many more, said by the server
// — the client counts nothing —, the name monospaced. Nothing unknown, no
// line.
func foreignLine(names []string) view.Runs {
	switch len(names) {
	case 0:
		return nil
	case 1:
		return join("A property Carrel does not know — ", monoRun(names[0]), " — is kept unchanged and not shown here.")
	}
	return join("Properties Carrel does not know — ", monoRun(names[0]), " and "+countWord(len(names)-1)+" more — are kept unchanged and not shown here.")
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
	folded := 0
	full := len(fields) == len(editFields[c.spec.kind])
	removed := st.values[removedKey]
	for _, name := range removed {
		// Rows Remove took out without scripts (C-6.8 InputRemove), named
		// again with every post until Save.
		sets[0].Rows = append(sets[0].Rows, view.FormRow{Name: removedKey, Kind: view.InputHidden, Value: text(name)})
	}
	// quickDay is the all-day box of the quick sheet, which stands after
	// the last of its times (s-notes-9).
	var quickDay *view.FormRow
	for _, f := range fields {
		vals, errText := st.values[f.name], text(st.errors[f.name])
		blank := !slices.ContainsFunc(vals, func(v string) bool { return strings.TrimSpace(v) != "" }) &&
			!slices.ContainsFunc(removed, func(n string) bool { return strings.HasPrefix(n, f.name+".") })
		hide := f.optional && full && st.obj != nil && blank && errText == "" && !st.unfold
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
			label := f.label
			if !full && f.quick != "" {
				label = f.quick
			}
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(label), Kind: view.InputText, Value: text(first(vals)), Hint: text(f.hint), Error: errText, Required: f.required})
		case fieldArea:
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputArea, Value: text(first(vals)), Error: errText})
		case fieldList:
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputText, Value: text(first(vals)), Hint: text(f.hint), Error: errText})
		case fieldRepeat:
			held := ""
			if st.obj != nil {
				held = strings.TrimSpace(firstValue(st.obj, f.prop))
			}
			set.Rows = append(set.Rows, repeatSet(f, vals, held, errText).Rows...)
		case fieldAttendee:
			// The full form draws the field (EventEditPage.Attendees); the
			// quick sheet carries whom it is made for, hidden.
			if !full {
				for _, v := range attendeeAddresses(vals) {
					set.Rows = append(set.Rows, view.FormRow{Name: f.name, Kind: view.InputHidden, Value: text("mailto:" + v), Error: errText})
				}
			}
		case fieldRelated:
			// Every relation of the record (s-tasks-3, -10): each chosen
			// option a value under the field's name, picked from what the
			// session has loaded, or typed after «By UID…» under the same
			// name — a UID, monospaced (s-tasks-11); and the line one more
			// is typed into, which stands whether there are relations or
			// not (Typecase's task 142) — a title or a UID.
			opts, unloaded := s.relatedOptions(c, st.uid, vals)
			row := view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputSelect, Multiple: true, Value: text(first(vals)),
				Options: opts, Placeholder: "UID of the record", Error: errText, Suggest: s.relatedSuggest(c, st), Mono: true}
			if unloaded {
				row.Hint = "A record that is not loaded here: its UID."
			}
			row.Add = &view.FormRow{Name: f.name + ".add", Label: "Add a link", Kind: view.InputText,
				Placeholder: "Title or UID", Suggest: s.relatedSuggest(c, st), Mono: true}
			set.Rows = append(set.Rows, row)
		case fieldZone:
			held, chosen := zoneOf(props[f.prop]), first(vals)
			switch {
			case st.obj == nil:
				// A new event: the instance zone chosen — also when the
				// quick sheet, which draws no zone, opens the card — and the
				// list without «As sent» (from-typecase note 19).
				if chosen == "" {
					chosen = s.instanceZone()
				}
				set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputSelect, Value: text(chosen),
					Options: newZoneOptions(chosen), Placeholder: "Find a zone", Error: errText})
			case held != "":
				row := view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputSelect, Value: text(chosen),
					Options: zoneOptions(held, chosen), Placeholder: "Find a zone", Error: errText}
				if chosen == held {
					// «as sent» after the zone the event came with
					// (s-agenda-3).
					row.ValueAside = "as sent"
				}
				set.Rows = append(set.Rows, row)
			default:
				// A day has no zone.
			}
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
				Placeholder: "DD.MM.YYYY", Hint: text(f.hint), Error: errText, Mono: true})
		case fieldName:
			row := pair(vals)
			set.Rows = append(set.Rows,
				view.FormRow{Name: f.name + ".given", Label: "Given", Kind: view.InputText, Value: text(row[0]), Error: errText},
				view.FormRow{Name: f.name + ".family", Label: "Family", Kind: view.InputText, Value: text(row[1])})
		case fieldMulti:
			// Each row is its Label, then the value (s-contacts-3); the rows
			// the record has, then the button that adds one. The quick sheet
			// has one row, bare (s-notes-8).
			kind := view.InputText
			if f.prop == "EMAIL" {
				kind = view.InputEmail
			}
			rows := shownRows(vals, full)
			labels := st.values[f.name+".type"]
			for i, v := range rows {
				name := fmt.Sprintf("%s.%d", f.name, i)
				if slices.Contains(removed, name) {
					set.Rows = append(set.Rows, removedRows(f, name)...)
					continue
				}
				row := view.FormRow{Name: name, Label: text(f.label), Kind: kind, Value: text(v), Mono: f.mono}
				if !full {
					row.Label, row.Placeholder = text(f.quick), text(f.quickSample)
				} else if f.typed {
					label := first(labels[min(i, len(labels)):])
					set.Rows = append(set.Rows, labelRow(f, name, label))
				}
				if i == 0 {
					row.Error = errText
				}
				set.Rows = append(set.Rows, row)
				if full {
					set.Rows = append(set.Rows, removeButton(name))
				}
			}
			if full {
				tpl := view.FormRow{Name: f.name + ".{n}", Label: text(f.label), Kind: kind, Mono: f.mono}
				add := addButton(f, tpl, removeButton(f.name+".{n}"))
				if f.typed {
					add.Template = []view.FormRow{labelRow(f, f.name+".{n}", ""), tpl, removeButton(f.name + ".{n}")}
				}
				set.Rows = append(set.Rows, add)
			}
		case fieldAddress:
			// Each row is its label, picked as a phone's is, the five parts
			// and Remove (s-contacts-3).
			labels := st.values[f.name+".type"]
			for i, v := range shownRows(vals, full) {
				prefix := fmt.Sprintf("%s.%d", f.name, i)
				if slices.Contains(removed, prefix) {
					set.Rows = append(set.Rows, removedRows(f, prefix)...)
					continue
				}
				label := labelRow(f, prefix, first(labels[min(i, len(labels)):]))
				if i == 0 {
					label.Error = errText
				}
				set.Rows = append(set.Rows, label)
				parts := addressRowParts(v)
				for j, row := range addressRows(prefix) {
					row.Value = text(parts[j])
					set.Rows = append(set.Rows, row)
				}
				set.Rows = append(set.Rows, removeButton(prefix))
			}
			tpl := append([]view.FormRow{labelRow(f, f.name+".{n}", "")}, addressRows(f.name+".{n}")...)
			set.Rows = append(set.Rows, addButton(f, append(tpl, removeButton(f.name+".{n}"))...))
		case fieldTime:
			row := triple(vals)
			// The all-day box leads its field, as in s-agenda-3; a task's
			// one box leads the first of its times (s-tasks-3). The quick
			// sheet has it after the times (s-notes-9), and its own labels.
			if f.allDay && full {
				set.Rows = append(set.Rows, view.FormRow{Name: f.name + ".allday", Label: "All day", Kind: view.InputCheck, Value: text(row[2])})
			}
			// The quick sheet has no box of a task: a date without a time
			// is the whole day (s-notes-10, quickTaskDay).
			if f.taskDay && full && !slices.ContainsFunc(set.Rows, func(r view.FormRow) bool { return r.Name == taskDayBox }) {
				// It switches both times off while it stands (s-tasks-3,
				// C-6.8 Disables).
				set.Rows = append(set.Rows, view.FormRow{Name: taskDayBox, Label: "All day", Kind: view.InputCheck, Value: text(first(st.values[taskDayBox])),
					Disables: taskTimes()})
			}
			date := view.FormRow{Name: f.name + ".date", Label: text(f.label), Kind: view.InputDate, Value: text(row[0]), Error: errText, Required: f.required, Mono: true}
			clock := view.FormRow{Name: f.name + ".time", Label: text(timeLabel(f)), Kind: view.InputTime, Value: text(row[1]), Mono: true}
			if f.taskDay && full && first(st.values[taskDayBox]) == "on" {
				clock.ReadOnly = true
			}
			switch {
			case full:
				set.Rows = append(set.Rows, date, clock)
			case f.quickNoDate:
				clock.Label, clock.Error = text(f.quickTime), errText
				set.Rows = append(set.Rows, clock)
			default:
				if f.quick != "" {
					date.Label = text(f.quick)
				}
				if f.quickTime != "" {
					clock.Label = text(f.quickTime)
				}
				date.Placeholder, clock.Placeholder = text(f.quickSample), text(f.quickTimeSample)
				set.Rows = append(set.Rows, date, clock)
			}
			if f.allDay && !full {
				quickDay = &view.FormRow{Name: f.name + ".allday", Label: "All day", Kind: view.InputCheck, Value: text(row[2])}
			}
			if f.quickNoDate && quickDay != nil {
				set.Rows = append(set.Rows, *quickDay)
				quickDay = nil
			}
		case fieldDone:
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputCheck, Value: text(first(vals))})
		case fieldPercent:
			set.Rows = append(set.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputNumber, Value: text(first(vals)), Error: errText, Mono: true})
		case fieldReminders:
			// The first reminder, then the others, then the button that adds
			// one (s-agenda-3: First, Add reminder). The quick sheet has one
			// row and no button (s-notes-9: Remind).
			rows := shownRows(vals, full)
			for i, v := range rows {
				row := view.FormRow{Name: fmt.Sprintf("%s.%d", f.name, i), Label: text(reminderLabel(i)), Kind: view.InputSelect, Value: text(v), Options: reminderOptions(v, st.alarms, i, tf)}
				if !full {
					row.Label = text(f.quick)
				}
				if i == 0 {
					row.Error = errText
				}
				set.Rows = append(set.Rows, row)
			}
			if full {
				add := addButton(f, view.FormRow{Name: f.name + ".{n}", Label: text(reminderLabel(len(rows))), Kind: view.InputSelect, Options: reminderOptions("", nil, -1, tf)})
				if len(rows) == 0 {
					add.Error = errText
				}
				set.Rows = append(set.Rows, add)
			}
		}
		if hide {
			empty = append(empty, set.Rows[before:]...)
			set.Rows = set.Rows[:before]
			folded++
		}
	}
	if quickDay != nil {
		sets[0].Rows = append(sets[0].Rows, *quickDay)
	}
	// A rubric whose fields all went into the fold is not drawn: a card
	// without categories and a note has no Other. The fold is the last
	// thing of the form, after every rubric, and counts the fields in it: a
	// card without addresses folds «Add address» too (s-contacts-3).
	sets = append(sets[:1], slices.DeleteFunc(sets[1:], func(fs view.FieldSet) bool { return len(fs.Rows) == 0 })...)
	if len(empty) > 0 {
		legend := fmt.Sprintf("Show %d empty fields", folded)
		if folded == 1 {
			legend = "Show 1 empty field"
		}
		// Without scripts the fold is a link to the same form with every
		// field in its rubric.
		unfold := &view.Action{ID: "form.unfold", Label: text(legend), Href: s.route(s.editAction(c, st) + "?fields=all"), Method: "GET", Enabled: true}
		sets = append(sets, view.FieldSet{Legend: text(legend), Folded: true, Unfold: unfold, Rows: empty})
	}
	action := s.editAction(c, st)
	if st.unfold {
		action += "?fields=all"
	}
	form := view.Form{
		Action: s.route(action), Method: "POST", Sets: sets,
		Actions: []view.Action{{ID: "record.save", Label: "Save", Method: "POST", Primary: true, Enabled: !c.col.ReadOnly}, s.cancelAction(c, st)},
	}
	names := make([]string, 0, len(st.errors))
	for name := range st.errors {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		form.Errors = append(form.Errors, view.FieldError{Name: name, Message: text(st.errors[name])})
	}
	return focused(form, caretField[c.spec.kind])
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

// reminderLabel names the reminder row i: the first is First (s-agenda-3).
func reminderLabel(i int) string {
	if i == 0 {
		return "First"
	}
	return "Reminder"
}

// shownRows are the rows a repeating field draws: its values without the
// empty slot they end with. The quick sheet draws one row even when the
// record has none (s-notes-8, s-notes-9).
func shownRows(vals []string, full bool) []string {
	rows := vals[:max(len(vals)-1, 0)]
	if !full && len(rows) == 0 {
		rows = []string{""}
	}
	return rows
}

// labelRow is the Label of a phone or an e-mail row (s-contacts-12, -13),
// before its value.
func labelRow(f editField, name, label string) view.FormRow {
	shown, _ := shownLabel(f.prop, label)
	return view.FormRow{Name: name + ".type", Label: "Label", Kind: view.InputSelect, Value: text(shown), Options: labelOptions(f.prop, label)}
}

// removedKey carries the rows Remove took out without scripts (C-6.8
// InputRemove): each names one, «tel.0», and goes back with every post until
// Save. The row itself travels empty and hidden under its number, so the
// rows after it keep theirs, and Save takes out exactly its value (S-7).
const removedKey = "removed"

// removeButton is Remove at the end of the row name (s-contacts-3).
func removeButton(name string) view.FormRow {
	return view.FormRow{Name: "remove", Label: "Remove", Kind: view.InputRemove, Value: text(name)}
}

// removedRows are the inputs of a removed row, empty and hidden.
func removedRows(f editField, name string) []view.FormRow {
	var names []string
	if f.typed {
		names = append(names, name+".type")
	}
	if f.typ == fieldAddress {
		for _, p := range addressParts {
			names = append(names, name+"."+p.name)
		}
	} else {
		names = append(names, name)
	}
	rows := make([]view.FormRow, 0, len(names))
	for _, n := range names {
		rows = append(rows, view.FormRow{Name: n, Kind: view.InputHidden})
	}
	return rows
}

// removeRow empties the row name of a repeating field of the kind — a
// phone, an e-mail, an address — and names it among the removed. A name no
// such row has changes nothing.
func removeRow(kind view.Kind, values formValues, name string) {
	field, num, ok := strings.Cut(name, ".")
	i, err := strconv.Atoi(num)
	if !ok || err != nil || i < 0 || strconv.Itoa(i) != num {
		return
	}
	for _, f := range editFields[kind] {
		if f.name != field || !f.typed || i >= len(values[f.name])-1 {
			continue
		}
		values[f.name][i] = ""
		if labels := values[f.name+".type"]; i < len(labels) {
			labels[i] = ""
		}
		if !slices.Contains(values[removedKey], name) {
			values[removedKey] = append(values[removedKey], name)
		}
	}
}

// addressRows are the five inputs of an address row under prefix.
func addressRows(prefix string) []view.FormRow {
	rows := make([]view.FormRow, 0, len(addressParts))
	for _, p := range addressParts {
		rows = append(rows, view.FormRow{Name: prefix + "." + p.name, Label: text(p.label), Kind: view.InputText, Mono: p.mono})
	}
	return rows
}

// addButton is «Add …» of a repeating field (C-6.8 InputAdd): the rows one
// press adds, and without scripts the form posted with add=<field>.
func addButton(f editField, template ...view.FormRow) view.FormRow {
	return view.FormRow{Name: "add", Label: text(f.add), Kind: view.InputAdd, Value: text(f.name), Template: template}
}

// addedRow gives the repeating field name one more empty row, as the path
// without scripts asks (C-7): the form comes back with it, nothing written.
// A name no repeating field of the kind has adds nothing.
func addedRow(kind view.Kind, values formValues, name string) {
	for _, f := range editFields[kind] {
		if f.name != name || f.add == "" {
			continue
		}
		grow := func(key string) {
			v := values[key]
			values[key] = append(v[:max(len(v)-1, 0):max(len(v)-1, 0)], "", "")
		}
		grow(f.name)
		if f.typed {
			grow(f.name + ".type")
		}
	}
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
		linked: s.noteLinked(r.Context(), c, uid, obj), unfold: r.URL.Query().Get("fields") == "all",
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
	st := editState{uid: uid, values: want, etag: etag, readAt: readAt, unfold: r.URL.Query().Get("fields") == "all"}
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
	if add := r.FormValue("add"); add != "" && !markDone {
		// «Add …» pressed without scripts (C-6.8 InputAdd): the form as
		// typed, one row more, and nothing written.
		addedRow(c.spec.kind, st.values, add)
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
		return
	}
	if remove := r.FormValue("remove"); remove != "" && !markDone {
		// Remove without scripts (C-6.8 InputRemove): the form as typed,
		// that row out of it, and nothing written.
		removeRow(c.spec.kind, st.values, remove)
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
		return
	}
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
		labelled := f.typed && !sameLabels(keptLabels(f.prop, want[f.name+".type"], base[f.name+".type"]), base[f.name+".type"])
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
	loss := withExceptions(compare(c.conn.ID, sent, got, c.sess.Losses()), sent, got)
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
	case render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet:
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, s.newSheet(c, pick), nil)
	case pick:
		s.writePage(w, r, http.StatusOK, s.newSheet(c, true))
	default:
		values := s.newFormValues(c)
		carried(c, r, values)
		st := editState{values: values, isNew: true, readAt: s.now()}
		// The form of files attached before the record (attach_new.go), as
		// it was typed when Attach sent it.
		if d, ok := newDraftUID(c, r.URL.Query().Get("uid")); ok {
			st.formUID = d.UID
			if d.Values != nil {
				st.values = d.Values
			}
		}
		if c.card != nil {
			// The form of a photo chosen before the card (photo_new.go),
			// as it was typed when a photo button sent it.
			st.formUID = chosenUID(c, r.URL.Query().Get("uid"))
			if d, ok := newPhotoDraft(c, issuedUID(r.URL.Query().Get("uid"))); ok && d.Values != nil {
				st.values = d.Values
				if len(d.Data) == 0 {
					st.typedUID = d.UID
				}
			}
		}
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
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
			if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
				_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
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
		{ID: "connection.add", Label: "Connect account", Href: s.connectHref(), Method: "GET", Primary: true, Enabled: true},
		{ID: "record.cancel", Label: "Cancel", Href: s.route("/app/" + kind), Method: "GET", Enabled: true},
	}}
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
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
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
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
	quickTaskDay(c.spec.kind, want)
	if r.URL.Query().Get("op") == "open" {
		s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: want, isNew: true, readAt: s.now()}))
		return
	}
	s.createRecord(w, r, c, want, "")
}

// sameDayEnds gives a time the quick sheet draws without its date the day of
// the time before it (s-notes-9: one Date, then Start and End): the same day,
// or the next when the clock is earlier than the one before. The full form
// draws the End date, and an End time typed with that date left empty is read
// by the same rule (S-11): before, the time was dropped and the event ended as
// it began.
func sameDayEnds(kind view.Kind, vals formValues) {
	var prev [3]string
	for _, f := range editFields[kind] {
		if f.typ != fieldTime {
			continue
		}
		row := triple(vals[f.name])
		if f.quickNoDate && row[0] == "" && row[1] != "" && prev[0] != "" {
			row[0] = prev[0]
			if prev[1] != "" && row[1] < prev[1] {
				if d, err := time.Parse("2006-01-02", prev[0]); err == nil {
					row[0] = d.AddDate(0, 0, 1).Format("2006-01-02")
				}
			}
			vals[f.name] = row[:]
		}
		prev = row
	}
}

// quickTaskDay is the task's box the quick sheet does not draw (s-notes-10,
// owner's decision 28.09.2026): a due date without a time is due the whole
// day. No date is no due at all, and a date with a time is that moment.
func quickTaskDay(kind view.Kind, vals formValues) {
	if kind != view.KindTask {
		return
	}
	due := triple(vals["due"])
	day := ""
	if due[0] != "" && due[1] == "" {
		day = "on"
	}
	vals[taskDayBox] = []string{day}
	for _, f := range editFields[kind] {
		if row := triple(vals[f.name]); f.taskDay && row[0] != "" {
			row[2] = day
			vals[f.name] = row[:]
		}
	}
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
		{ID: "record.save", Label: text("Save " + c.spec.noun), Method: "POST", Primary: true, Enabled: !c.col.ReadOnly},
		{ID: "record.open", Label: "Open the card", Href: s.route(quick + "?op=open"), Method: "POST", Enabled: true},
		s.cancelAction(c, st),
	}
	sheet.Form = form
	sheet.Note = quickNotes[c.spec.kind]
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
	next := view.Action{ID: "record.new", Label: text("New " + c.spec.noun), Href: own, Method: "GET", Primary: true, Enabled: true}
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
	// A step of a new form's attachments (attach_new.go): its file is read
	// from the body as it comes, nothing may parse the form before.
	if op := r.URL.Query().Get("op"); op != "" && c.cal != nil {
		s.postNewAttach(w, r, c, op)
		return
	}
	want := s.submittedValues(c, r)
	// The UID of a contact's form with a photo chosen (photo_new.go), of a
	// new record's form with files attached (attach_new.go).
	var formUID string
	if c.card != nil {
		formUID = chosenUID(c, r.FormValue("uid"))
	} else if d, ok := newDraftUID(c, r.FormValue("uid")); ok {
		formUID = d.UID
	}
	if add := r.FormValue("add"); add != "" {
		// «Add …» without scripts: the form back with one row more, and no
		// record until Save (C-6.8 InputAdd).
		addedRow(c.spec.kind, want, add)
		s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: want, isNew: true, readAt: s.now(), formUID: formUID}))
		return
	}
	if remove := r.FormValue("remove"); remove != "" {
		removeRow(c.spec.kind, want, remove)
		s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: want, isNew: true, readAt: s.now(), formUID: formUID}))
		return
	}
	if c.card != nil && formUID == "" && r.FormValue("uid") != "" {
		// The form shows a photo the session no longer holds: saving
		// without it is the person's choice, not a silent one.
		s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: want, isNew: true, readAt: s.now(), notices: []view.Notice{lostPhotoNotice}}))
		return
	}
	s.createRecord(w, r, c, want, formUID)
}

// createRecord writes a new record. The full form and the quick sheet both
// come here: two screens, one way of saving (U-6.16). A contact's form with
// a photo chosen names its UID (formUID): the card is created under it, with
// the photo in it, and the draft goes (S-13, photo_new.go).
func (s *Server) createRecord(w http.ResponseWriter, r *http.Request, c *collectionCtx, want formValues, formUID string) {
	st := editState{values: want, isNew: true, readAt: s.now(), formUID: formUID}
	uid, err := formUID, error(nil)
	if uid == "" {
		uid, err = model.NewUID()
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	portrait, err := s.newCardPhoto(c, formUID)
	if err != nil {
		clearPhotoDraft(c, formUID)
		st.formUID = ""
		st.notices = []view.Notice{s.photoRefusal(err)}
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
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
	if err == nil && portrait != nil {
		patch, perr := model.NewPatch(card.PhotoOps(sent, portrait)...)
		if err = perr; err == nil {
			err = patch.Apply(sent)
		}
	}
	// The files attached before the record (attach_new.go).
	draft, drafted := attachDraft{}, false
	if c.cal != nil {
		draft, drafted = loadAttachDraft(c, formUID)
	}
	if err == nil && drafted {
		err = applyDraft(sent, draft)
	}
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
	if formUID != "" {
		clearPhotoDraft(c, formUID)
		clearAttachDraft(c, formUID)
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

// withExceptions makes the loss of a write say what became of a series'
// exceptions (the owner's word of 11.10.2026: warn when the server loses or
// breaks something). The comparison of the write reads the series alone and
// by what is written; a day left out or an occurrence changed apart is known
// by the instant it names (cal.ExceptionLoss). A server that dropped one is
// named by «EXDATE» or «RECURRENCE-ID» in the report even when it kept every
// property of the series; one that only wrote the same days another way has
// lost nothing.
func withExceptions(loss *model.Loss, sent, got *model.Object) *model.Loss {
	if sent == nil || got == nil || !sent.IsCalendar() || !got.IsCalendar() {
		return loss
	}
	var before []string
	if loss != nil {
		before = loss.Properties
	}
	after := cal.ExceptionLoss(before, sent, got)
	switch {
	case len(after) == 0:
		return nil
	case loss == nil:
		return &model.Loss{Properties: after}
	}
	if len(after) > len(before) {
		// Something new to say: the report is not one already seen.
		loss.Aggregated = false
	}
	loss.Properties = after
	return loss
}
