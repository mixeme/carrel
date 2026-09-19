package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

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
	// fieldBirthday is BDAY: a date, or --MM-DD without a year, written back
	// in the form it came in (basic or extended).
	fieldBirthday
	// fieldAddress is ADR as five inputs a row: street, city, region,
	// postcode, country. PO box and extended are kept as they were.
	fieldAddress
)

type editField struct {
	name     string
	label    string
	prop     string
	typ      fieldType
	required bool
	allDay   bool   // fieldTime: the field has its own all-day box
	typed    bool   // fieldMulti: each row has its label, the TYPE parameter
	hint     string // shown under the field, when the rule needs saying
}

var editFields = map[view.Kind][]editField{
	view.KindContact: {
		{name: "fn", label: "Display name", prop: "FN", typ: fieldText,
			hint: "Left empty, it is made from Given and Family. Fill it alone and it is cut the other way: the first word is the given name, the rest the family name."},
		{name: "n", label: "Name", prop: "N", typ: fieldName},
		{name: "nickname", label: "Nickname", prop: "NICKNAME", typ: fieldText},
		{name: "email", label: "Email", prop: "EMAIL", typ: fieldMulti, typed: true},
		{name: "tel", label: "Phone", prop: "TEL", typ: fieldMulti, typed: true},
		{name: "adr", label: "Address", prop: "ADR", typ: fieldAddress},
		{name: "org", label: "Organisation", prop: "ORG", typ: fieldText},
		{name: "title", label: "Title", prop: "TITLE", typ: fieldText},
		{name: "bday", label: "Birthday", prop: "BDAY", typ: fieldBirthday,
			hint: "--MM-DD for a birthday without a year."},
		{name: "categories", label: "Categories", prop: "CATEGORIES", typ: fieldList, hint: "Separated by commas."},
		{name: "note", label: "Note", prop: "NOTE", typ: fieldArea},
	},
	view.KindEvent: {
		{name: "summary", label: "Title", prop: "SUMMARY", typ: fieldText},
		{name: "start", label: "Starts", prop: "DTSTART", typ: fieldTime, required: true, allDay: true},
		{name: "end", label: "Ends", prop: "DTEND", typ: fieldTime},
		{name: "location", label: "Location", prop: "LOCATION", typ: fieldText},
		{name: "description", label: "Description", prop: "DESCRIPTION", typ: fieldArea},
		{name: "reminder", label: "Reminders", typ: fieldReminders},
	},
	view.KindTask: {
		{name: "summary", label: "Title", prop: "SUMMARY", typ: fieldText},
		{name: "due", label: "Due", prop: "DUE", typ: fieldTime, allDay: true},
		{name: "done", label: "Done", prop: "STATUS", typ: fieldDone},
		{name: "percent", label: "Percent complete", prop: "PERCENT-COMPLETE", typ: fieldPercent},
		{name: "description", label: "Description", prop: "DESCRIPTION", typ: fieldArea},
		{name: "reminder", label: "Reminders", typ: fieldReminders},
	},
	view.KindNote: {
		{name: "summary", label: "Title", prop: "SUMMARY", typ: fieldText},
		{name: "description", label: "Body", prop: "DESCRIPTION", typ: fieldArea},
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

// rowLabels are the labels a phone or an address row is offered (TYPE). A
// label another client wrote is offered too, so keeping it is a choice.
var rowLabels = []string{"home", "work", "cell", "voice", "fax", "other"}

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

// reminderPresets are the triggers the form offers. A trigger another client
// wrote is offered too, so keeping it is always a choice.
var reminderPresets = []string{"PT0S", "-PT5M", "-PT10M", "-PT15M", "-PT30M", "-PT1H", "-PT2H", "-P1D", "-P2D", "-P1W"}

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
		case fieldList:
			vals[f.name] = []string{strings.Join(categoriesOf(c.spec.kind, obj), ", ")}
		case fieldBirthday:
			vals[f.name] = []string{birthdayShown(c.text(obj, f.prop))}
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
	if tm.AllDay && f.allDay {
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
		case fieldText, fieldPercent, fieldList, fieldBirthday:
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
	errors        map[string]string
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
		case fieldBirthday:
			v := first(w)
			if v == first(h) {
				continue
			}
			written, err := birthdayWritten(v, props[f.prop], firstValue(obj, "VERSION"))
			if err != nil {
				ch.errors[f.name] = err.Error()
				continue
			}
			ch.ops = append(ch.ops, textOps(f.prop, props[f.prop], written, false)...)
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
				ch.errors[f.name] = f.label + " needs a date."
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
			if sameRows(w, h) && !(f.name == "end" && startChanged && row[0] != "") {
				continue
			}
			shift := 0
			if f.name == "end" && allDay {
				shift = 1
			}
			ops := timeOps(f.prop, props[f.prop], row, allDay, shift, tz)
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
				allowed[p] = true
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
func birthdayShown(raw string) string {
	v := strings.TrimSpace(raw)
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

// birthdayWritten turns what was typed into BDAY in the form the stored one
// has: basic (18151210, --0412) or extended (1815-12-10, --04-12). Without a
// stored one, vCard 3 gets extended and vCard 4 basic, as each writes it.
func birthdayWritten(typed string, cur []model.RawValue, version string) (string, error) {
	if typed == "" {
		return "", nil
	}
	yearless := strings.HasPrefix(typed, "--")
	digits := strings.ReplaceAll(strings.TrimPrefix(typed, "--"), "-", "")
	layout, want := "20060102", 8
	if yearless {
		layout, want = "0102", 4
	}
	check := digits
	if yearless {
		check = "2000" + digits // 29 February is a birthday too
		layout = "20060102"
	}
	if len(digits) != want || !isDigits(digits) {
		return "", errors.New("Enter the date as YYYY-MM-DD, or --MM-DD without a year.")
	}
	if _, err := time.Parse(layout, check); err != nil {
		return "", errors.New("Enter the date as YYYY-MM-DD, or --MM-DD without a year.")
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

// sameLabels compares the labels of the rows the form had.
func sameLabels(a, b []string) bool {
	for i := 0; i < max(len(a), len(b)); i++ {
		if first(a[min(i, len(a)):]) != first(b[min(i, len(b)):]) {
			return false
		}
	}
	return true
}

// labelOptions are rowLabels, "no label", and the row's own label when it
// is none of them.
func labelOptions(selected string) []view.Option {
	opts := []view.Option{{Value: "", Label: "No label", Selected: selected == ""}}
	seen := selected == ""
	for _, l := range rowLabels {
		opts = append(opts, view.Option{Value: l, Label: text(l), Selected: l == selected})
		seen = seen || l == selected
	}
	if !seen {
		opts = append(opts, view.Option{Value: selected, Label: text(strings.ToLower(strings.ReplaceAll(selected, ",", ", "))), Selected: true})
	}
	return opts
}

// timeOps writes a date or date-time in the form it had: a TZID stays with
// its zone, UTC stays UTC, a floating time stays floating. A day is a DATE;
// shiftDays turns the last day of an all-day event into its exclusive DTEND.
// The row is already validated by rowTime.
func timeOps(prop string, cur []model.RawValue, row [3]string, allDay bool, shiftDays int, tz *time.Location) []model.PatchOp {
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
		switch zone := old.Params["TZID"]; {
		case zone != "":
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
		return cal.SetAlarms(obj, ch.alarms)
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
	if st.isNew {
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
	switch c.spec.kind {
	case view.KindContact:
		page := view.ContactEditPage{Shell: sh, UID: st.uid, Form: form, Loss: st.loss, ReadOnly: c.col.ReadOnly}
		page.Photo = s.photoBlock(c, st)
		return page
	case view.KindEvent:
		page := view.EventEditPage{Shell: sh, UID: st.uid, Form: form, Reminders: reminderRows(st.alarms, tf), Loss: st.loss}
		if st.obj != nil {
			if cal.HasRecurrence(st.obj) {
				page.Recurrence = s.recurrenceBlock(st.obj, tf)
			}
			for _, a := range cal.Attendees(st.obj) {
				page.Attendees = append(page.Attendees, view.AttendeeRow{Name: text(a.Name), Email: text(a.Email), Status: text(partstatLabel(a.Status))})
			}
			page.Attach = s.attachRows(c, st.uid, st.obj, st.etag, "edit")
		}
		return page
	case view.KindTask:
		page := view.TaskEditPage{Shell: sh, UID: st.uid, Form: form, Done: first(st.values["done"]) == "on", Reminders: reminderRows(st.alarms, tf), Loss: st.loss}
		if st.obj != nil {
			page.Attach = s.attachRows(c, st.uid, st.obj, st.etag, "edit")
		}
		if n, err := strconv.Atoi(first(st.values["percent"])); err == nil {
			page.Percent = &n
		}
		return page
	default:
		body := first(st.values["description"])
		doc := view.NoteDoc{
			Mode: view.NoteEditing, Title: text(first(st.values["summary"])), Body: text(body),
			Rendered: cal.RenderMarkdown(body), ReadOnly: c.col.ReadOnly,
			Bar: []view.Action{
				{ID: "record.save", Label: "Save", Href: s.postQuery(s.editAction(c, st), url.Values{"etag": {st.etag}, "readat": {strconv.FormatInt(st.readAt.Unix(), 10)}}), Method: "POST", Enabled: true},
				s.cancelAction(c, st),
			},
		}
		if attaching {
			doc.Bar = append(doc.Bar, s.attachAction(c, st.uid, st.etag, "edit"))
		}
		// NoteDoc has no shell: a loss or a notice needs the page around it.
		if st.loss != nil || len(sh.Notices) > 0 {
			return view.NotePage{Shell: sh, UID: st.uid, Doc: doc, Source: c.source(), Loss: st.loss}
		}
		return doc
	}
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
	main := view.FieldSet{Rows: []view.FormRow{
		csrfField(c.sess),
		{Name: "etag", Kind: view.InputHidden, Value: text(st.etag)},
		{Name: "readat", Kind: view.InputHidden, Value: text(strconv.FormatInt(st.readAt.Unix(), 10))},
	}}
	var props map[string][]model.RawValue
	if st.obj != nil {
		props = st.obj.Props()
	}
	var sets, extra []view.FieldSet
	var reminders *view.FieldSet
	for _, f := range fields {
		vals, errText := st.values[f.name], text(st.errors[f.name])
		switch f.typ {
		case fieldText:
			main.Rows = append(main.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputText, Value: text(first(vals)), Hint: text(f.hint), Error: errText, Required: f.required})
		case fieldArea:
			main.Rows = append(main.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputArea, Value: text(first(vals)), Error: errText})
		case fieldList:
			main.Rows = append(main.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputText, Value: text(first(vals)), Hint: text(f.hint), Error: errText})
		case fieldBirthday:
			// A date input cannot hold a day without a year: such a value is
			// shown as text, as it is.
			kind := view.InputDate
			if strings.HasPrefix(first(vals), "--") {
				kind = view.InputText
			}
			main.Rows = append(main.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: kind, Value: text(first(vals)), Hint: text(f.hint), Error: errText})
		case fieldName:
			row := pair(vals)
			main.Rows = append(main.Rows,
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
				main.Rows = append(main.Rows, row)
				if f.typed {
					label := first(st.values[f.name+".type"][min(i, len(st.values[f.name+".type"])):])
					name := fmt.Sprintf("%s.%d.type", f.name, i)
					if i == len(vals)-1 {
						name = f.name + ".new.type"
					}
					main.Rows = append(main.Rows, view.FormRow{Name: name, Label: text(f.label + " label"), Kind: view.InputSelect, Value: text(label), Options: labelOptions(label)})
				}
			}
		case fieldAddress:
			set := view.FieldSet{Legend: "Addresses"}
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
			extra = append(extra, set)
		case fieldTime:
			row := triple(vals)
			main.Rows = append(main.Rows,
				view.FormRow{Name: f.name + ".date", Label: text(f.label), Kind: view.InputDate, Value: text(row[0]), Error: errText, Required: f.required},
				view.FormRow{Name: f.name + ".time", Label: text(f.label + " at"), Kind: view.InputTime, Value: text(row[1])})
			if f.allDay {
				main.Rows = append(main.Rows, view.FormRow{Name: f.name + ".allday", Label: "All day", Kind: view.InputCheck, Value: text(row[2])})
			}
		case fieldDone:
			main.Rows = append(main.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputCheck, Value: text(first(vals))})
		case fieldPercent:
			main.Rows = append(main.Rows, view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputNumber, Value: text(first(vals)), Error: errText})
		case fieldReminders:
			set := view.FieldSet{Legend: text(f.label)}
			for i, v := range vals {
				row := view.FormRow{Name: fmt.Sprintf("reminder.%d", i), Label: "Reminder", Kind: view.InputSelect, Value: text(v), Options: reminderOptions(v, st.alarms, i, tf)}
				if i == len(vals)-1 {
					row.Name, row.Label = "reminder.new", "Add a reminder"
				}
				if i == 0 {
					row.Error = errText
				}
				set.Rows = append(set.Rows, row)
			}
			reminders = &set
		}
	}
	sets = append(sets, main)
	sets = append(sets, extra...)
	if reminders != nil {
		sets = append(sets, *reminders)
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

func reminderOptions(selected string, alarms []cal.Alarm, row int, tf timeFmt) []view.Option {
	opts := []view.Option{{Value: "", Label: "No reminder", Selected: selected == ""}}
	seen := false
	for _, p := range reminderPresets {
		opts = append(opts, view.Option{Value: p, Label: text(tf.reminder(cal.NewAlarm(p))), Selected: p == selected})
		seen = seen || p == selected
	}
	if selected != "" && !seen {
		label := selected
		if row < len(alarms) && alarms[row].Trigger == selected {
			label = tf.reminder(alarms[row])
		}
		opts = append(opts, view.Option{Value: selected, Label: text(label), Selected: true})
	}
	return opts
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
		obj: obj, uid: uid, values: vals, alarms: alarms, etag: obj.ETag, readAt: time.Now(),
	}))
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
	case op == "detach" && c.cal != nil:
		s.postDetach(w, r, c, uid)
		return
	case op != "":
		s.notFound(w, r)
		return
	}
	etag := strings.TrimSpace(r.FormValue("etag"))
	readAt := unixForm(r.FormValue("readat"))
	want := s.submittedValues(c, r)
	st := editState{uid: uid, values: want, etag: etag, readAt: readAt}

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
		s.saveDraft(c, uid, etag, readAt, changedValues(editFields[c.spec.kind], base, typed))
		s.redirect(w, r, c.recordPath(uid)+"/conflict")
		return
	case err != nil:
		st.notices = []view.Notice{s.writeFailure(c, "PUT", obj.Path, err)}
		s.writePage(w, r, http.StatusOK, s.editPage(c, st))
		return
	}
	s.finishWrite(w, r, c, uid, sent, c.recordPath(uid))
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
		obj: got, uid: uid, values: vals, alarms: alarms, etag: got.ETag, readAt: time.Now(), loss: report, notices: notices,
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
		s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: s.emptyValues(c), isNew: true, readAt: time.Now()}))
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
				s.redirect(w, r, c.listPath()+"/new/quick")
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
			if WantsFragment(r) && FragmentTarget(r) == TargetSheet {
				_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
				return
			}
			s.writePage(w, r, http.StatusOK, sheet)
			return
		}
	}
	s.notFound(w, r)
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
	sheet := s.quickSheet(c, editState{values: s.emptyValues(c), isNew: true, readAt: time.Now()})
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
		s.writePage(w, r, http.StatusOK, s.editPage(c, editState{values: want, isNew: true, readAt: time.Now()}))
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
	st := editState{values: want, isNew: true, readAt: time.Now()}
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
