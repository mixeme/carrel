package web

import (
	"bytes"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"

	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/view"
)

// A save is the edit and nothing else (S-7.3, the owner's decision of
// 11.10.2026). The patch model keeps what a patch does not name (S-7) and
// the re-read says what the server lost (S-7.1); neither sees a patch that
// is itself wrong — a field the person never touched rewritten, a value the
// form did not show dropped with the one it did. So before anything goes to
// the server the save is made in memory and looked at twice:
//
//   - the form read from the result is the form the person sent;
//   - the lines of the record that changed belong to the fields whose shown
//     value changed, and to what those fields are said to take with them.
//
// A save that fails either is not sent: it is Carrel's fault, and it costs
// the person an edit they still have on the screen rather than data nobody
// will miss for months.

// editFault is a save Carrel would not make.
type editFault struct {
	// fields are the labels of the fields that would not read back as sent.
	fields []string
	// props are the properties that would change though no field that owns
	// them did, each once, as «NAME» or «NAME in COMPONENT».
	props []string
	// lines are the content lines in question, «- » gone and «+ » come, for
	// the details of the notice.
	lines []string
}

// editAlso is what a field takes with it besides its own property: the
// consequences S-7.3 names. Keyed by kind and field name.
var editAlso = map[view.Kind]map[string][]string{
	view.KindEvent: {
		// A series takes its exceptions with it (S-7.1); a start that moved
		// keeps the length of an event that is written by its length.
		// and an event is always written with an end (S-11).
		// The days added go as the days left out do, and the last day of the
		// series stays the day the form shows (S-11): UNTIL is in the rule.
		"start":    {"EXDATE", "RDATE", "RRULE", "DURATION", "DTEND"},
		"end":      {"DURATION"},
		"tz":       {"DTEND", "EXDATE", "RDATE", "RRULE"},
		"attendee": {"ORGANIZER"},
	},
	view.KindTask: {
		"done":   {"COMPLETED"},
		"status": {"COMPLETED"},
		// The zone rewrites the due date with the start (Р6), and a series
		// as an event's.
		"tz": {"DUE", "EXDATE", "RDATE", "RRULE"},
	},
}

// retimes are the fields whose change moves a series: its exceptions are
// renamed (RECURRENCE-ID) and the zone it is written in may come with it
// (VTIMEZONE).
var retimes = []string{"start", "tz"}

// wireLine is one content line of a record: where it stands, its name, and
// which of its name it is there.
type wireLine struct {
	scope string // "" the record itself; "VALARM", "exception", "VTIMEZONE", "VCALENDAR"
	name  string
	nth   int
	text  string
}

// wire is the record as it would be sent.
func (c *collectionCtx) wire(obj *model.Object) ([]byte, error) {
	var buf bytes.Buffer
	var err error
	if c.card != nil {
		err = card.Encode(&buf, obj)
	} else {
		err = cal.Encode(&buf, obj)
	}
	return buf.Bytes(), err
}

// lineName is the property a content line writes: its name without a vCard
// group, in upper case.
func lineName(line string) string {
	name := line
	if i := strings.IndexAny(name, ";:"); i >= 0 {
		name = name[:i]
	}
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}
	return strings.ToUpper(strings.TrimSpace(name))
}

// wireLines reads a document into its content lines. A calendar's lines are
// told apart by where they stand: the record itself, its alarms, a
// component of its own (an occurrence changed by hand — «exception» —, a
// zone), and the envelope.
func wireLines(doc []byte) []wireLine {
	lines := model.LogicalLines(doc)
	var out []wireLine
	var stack []string
	top := "" // the scope of the child of VCALENDAR being read
	seen := map[string]int{}
	for i, line := range lines {
		name := lineName(line)
		switch name {
		case "BEGIN":
			comp := strings.ToUpper(strings.TrimSpace(line[strings.IndexByte(line, ':')+1:]))
			stack = append(stack, comp)
			if len(stack) == 2 && stack[0] == "VCALENDAR" {
				top = ""
				switch {
				case comp == "VTIMEZONE":
					top = comp
				case ownRecurrenceID(lines[i+1:]):
					top = "exception"
				}
			}
			// The frame of a component inside the record is part of it; the
			// record's own frame and its envelope's are not lines of it.
			if scope := scopeOf(stack, top); scope != "" && len(stack) > 1 {
				out = append(out, wireLine{scope: scope, name: name, text: line})
			}
			continue
		case "END":
			if scope := scopeOf(stack, top); scope != "" && len(stack) > 1 {
				out = append(out, wireLine{scope: scope, name: name, text: line})
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		scope := scopeOf(stack, top)
		key := scope + "\x00" + name
		out = append(out, wireLine{scope: scope, name: name, nth: seen[key], text: line})
		seen[key]++
	}
	return out
}

// scopeOf names where a line stands by the components open around it.
func scopeOf(stack []string, top string) string {
	switch {
	case len(stack) == 0:
		return ""
	case stack[0] != "VCALENDAR":
		// A vCard: one component, and what stands inside it is its own.
		return ""
	case len(stack) == 1:
		return "VCALENDAR"
	case top != "":
		return top
	case len(stack) == 2:
		return ""
	}
	return stack[len(stack)-1]
}

// ownRecurrenceID reports whether the component whose lines begin at rest
// carries a RECURRENCE-ID of its own, not one of a component inside it.
func ownRecurrenceID(rest []string) bool {
	depth := 0
	for _, line := range rest {
		switch lineName(line) {
		case "BEGIN":
			depth++
		case "END":
			if depth == 0 {
				return false
			}
			depth--
		case "RECURRENCE-ID":
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

// wireDiff are the lines of before that after no longer has, and the lines
// after has gained, each line counted as often as it stands.
func wireDiff(before, after []wireLine) (gone, come []wireLine) {
	count := map[string]int{}
	key := func(l wireLine) string { return l.scope + "\x00" + l.text }
	for _, l := range after {
		count[key(l)]++
	}
	for _, l := range before {
		if count[key(l)] > 0 {
			count[key(l)]--
			continue
		}
		gone = append(gone, l)
	}
	count = map[string]int{}
	for _, l := range before {
		count[key(l)]++
	}
	for _, l := range after {
		if count[key(l)] > 0 {
			count[key(l)]--
			continue
		}
		come = append(come, l)
	}
	return gone, come
}

// wholeField reports a field that shows every value of its property; the
// others show the first, and a save by them may touch no other.
func wholeField(f editField) bool {
	switch f.typ {
	case fieldMulti, fieldAddress, fieldList, fieldRelated, fieldAttendee:
		return true
	}
	return false
}

// shows reports whether the form's field f shows the line l of its property:
// the attendees are the mail addresses, and a line that names a room or a
// phone is not among them.
func shows(f editField, l wireLine) bool {
	if f.typ != fieldAttendee {
		return true
	}
	line, ok := model.ParseLine(l.text)
	return ok && dupes.NormEmail(cal.CalAddress(line.Value)) != ""
}

// checkEdit looks at a save made in memory before it is sent (S-7.3): was is
// the record as it stood, on the wire; sent is the record patched; have the
// form was was read into, want the form the person sent, as buildChange left
// it. nil is a save that is the edit.
func (s *Server) checkEdit(c *collectionCtx, was []byte, sent *model.Object, have, want formValues, restored map[string][]model.RawValue) *editFault {
	fault := &editFault{}
	whole := func() *editFault {
		fault.fields = append(fault.fields, "the record as a whole")
		return fault
	}
	// The record is read from what would be sent, not from the patched
	// object: the form of the result is the form of those bytes.
	now, err := c.wire(sent)
	if err != nil {
		return whole()
	}
	read, err := c.parse(sent.Path, sent.ETag, now)
	if err != nil {
		return whole()
	}
	after, _, err := s.currentValues(c, read)
	if err != nil {
		return whole()
	}
	fields := editFields[c.spec.kind]
	typed := ""
	if add, drawn := want[attendeeName+".add"]; drawn {
		typed, _ = s.typedAttendee(c.sess, first(add))
	}
	_, invited := invitedBy(c, read)
	organiser := ""
	if c.cal != nil {
		organiser = dupes.NormEmail(cal.Organiser(read).Email)
	}
	for _, f := range fields {
		if !readsAs(f, have, want, after, typed, organiser, invited) {
			fault.fields = append(fault.fields, f.label)
		}
	}

	// A field is changed when what the person sent is not what the form
	// showed them — measured the same way, so a field they left alone is
	// left alone however its value is spelt in the record.
	changed := map[string]bool{}
	for _, f := range fields {
		if !readsAs(f, have, want, have, typed, organiser, invited) {
			changed[f.name] = true
		}
	}
	// A field that shows the first of several values and was emptied shows
	// the next one after the save: the one the person saw is gone, and the
	// others were never the form's to remove.
	held := map[string]int{}
	for _, l := range wireLines(was) {
		if l.scope == "" {
			held[l.name]++
		}
	}
	for i := 0; i < len(fault.fields); i++ {
		for _, f := range fields {
			if f.label == fault.fields[i] && (f.typ == fieldText || f.typ == fieldArea) && strings.TrimSpace(first(want[f.name])) == "" && held[f.prop] > 1 {
				fault.fields = append(fault.fields[:i], fault.fields[i+1:]...)
				i--
				break
			}
		}
	}
	// owners are the fields that may change a property of the record itself.
	owners := map[string][]editField{}
	for _, f := range fields {
		if f.prop != "" {
			owners[f.prop] = append(owners[f.prop], f)
		}
	}
	also := map[string]bool{}
	for name, props := range editAlso[c.spec.kind] {
		if changed[name] {
			for _, p := range props {
				also[p] = true
			}
		}
	}
	// A property a conflict puts back may change, and reads back as the
	// version opened held it (Рв6).
	props := read.Props()
	for _, name := range slices.Sorted(maps.Keys(restored)) {
		also[name] = true
		if !sameRaw(props[name], restored[name]) {
			fault.fields = append(fault.fields, name)
		}
	}
	if first(want[taskNext]) == "on" {
		// A repeating task moved on to its next time counts one time fewer.
		also["RRULE"] = true
	}
	retimed := false
	for _, name := range retimes {
		retimed = retimed || changed[name]
	}

	gone, come := wireDiff(wireLines(was), wireLines(now))
	allowed := func(l wireLine, added bool) bool {
		switch l.scope {
		case "":
			if isEditStamp(l.name) || also[l.name] {
				return true
			}
			for _, f := range owners[l.name] {
				if changed[f.name] && (wholeField(f) || l.nth == 0) && (added || shows(f, l)) {
					return true
				}
			}
			return false
		case "VALARM":
			return changed["reminder"]
		case "exception":
			return l.name == "RECURRENCE-ID" && retimed
		case "VTIMEZONE":
			// A zone comes with the times written in it; none ever goes.
			return added && retimed
		}
		return false
	}
	named := map[string]bool{}
	note := func(l wireLine, added bool) {
		if allowed(l, added) {
			return
		}
		name := l.name
		if l.scope != "" {
			name += " in " + l.scope
			if l.scope == "exception" {
				name = l.name + " of an occurrence changed by hand"
			}
		}
		if !named[name] {
			named[name] = true
			fault.props = append(fault.props, name)
		}
		sign := "- "
		if added {
			sign = "+ "
		}
		fault.lines = append(fault.lines, sign+l.text)
	}
	for _, l := range gone {
		note(l, false)
	}
	for _, l := range come {
		note(l, true)
	}
	if len(fault.fields) == 0 && len(fault.props) == 0 {
		return nil
	}
	sort.Strings(fault.props)
	return fault
}

// isEditStamp names what every write of a record stamps itself with.
func isEditStamp(name string) bool {
	switch name {
	case "DTSTAMP", "LAST-MODIFIED", "REV":
		return true
	}
	return false
}

// readsAs reports whether field f of the saved record (after) reads as the
// person sent it (want). The two are compared as the form shows them, not
// as they are spelt: a form posts a text trimmed, an empty percent is none,
// a label chosen is the label shown. have is what the form was read from;
// typed is the address the line of the attendees named, organiser the saved
// record's, invited — a record whose list only its organiser changes.
func readsAs(f editField, have, want, after formValues, typed, organiser string, invited bool) bool {
	w, a := want[f.name], after[f.name]
	switch f.typ {
	case fieldText:
		return strings.TrimSpace(first(w)) == strings.TrimSpace(first(a))
	case fieldArea:
		return shownArea(first(w)) == shownArea(first(a))
	case fieldChoice:
		wv, av := strings.TrimSpace(first(w)), strings.TrimSpace(first(a))
		if wv == "" {
			wv = f.blank
		}
		if av == "" {
			av = f.blank
		}
		return wv == av
	case fieldZone:
		// A day has no zone — an event's start, a task's one box —, and the
		// form of one that had none offers the instance's without asking
		// for it.
		if triple(after["start"])[2] == "on" || first(after[taskDayBox]) == "on" || first(w) == "" || first(w) == first(have[f.name]) {
			return true
		}
		return first(w) == first(a)
	case fieldRepeat:
		return sameRepeat(have[f.name], w, a)
	case fieldRelated:
		return sameSet(w, a)
	case fieldAttendee:
		if invited {
			return true
		}
		asked, got := attendeeAddresses(w), attendeeAddresses(a)
		if _, drawn := want[f.name+".add"]; !drawn {
			// A form that did not draw the field only adds what it carries.
			asked = attendeeAddresses(append(slices.Clone(have[f.name]), asked...))
		}
		if typed != "" && !slices.Contains(asked, typed) {
			asked = append(asked, typed)
		}
		for _, addr := range asked {
			if !slices.Contains(got, addr) {
				return false
			}
		}
		// Whoever is there besides is the organiser: the one it had, who
		// stays, or the person themself on a record that gained its first
		// attendees.
		for _, addr := range got {
			if !slices.Contains(asked, addr) && addr != organiser {
				return false
			}
		}
		return true
	case fieldList:
		return slices.Equal(listItems(first(w)), listItems(first(a)))
	case fieldBirthday:
		return first(w) == first(a) || sameBirthday(first(w), first(a))
	case fieldName:
		wp, ap := pair(w), pair(a)
		clean := func(v string) string { return strings.TrimSpace(strings.ReplaceAll(v, ";", "")) }
		return clean(wp[0]) == clean(ap[0]) && clean(wp[1]) == clean(ap[1])
	case fieldMulti, fieldAddress:
		wr, wl := filledRows(w, want[f.name+".type"])
		ar, al := filledRows(a, after[f.name+".type"])
		if f.typ == fieldAddress {
			for i := range wr {
				wr[i] = strings.ReplaceAll(wr[i], ";", "")
			}
			for i := range ar {
				ar[i] = strings.ReplaceAll(ar[i], ";", "")
			}
		}
		if !slices.Equal(wr, ar) {
			return false
		}
		if f.typ == fieldMulti && !f.typed {
			return true
		}
		for i := range wl {
			ws, _ := shownLabel(f.prop, wl[i])
			as, _ := shownLabel(f.prop, al[i])
			if !strings.EqualFold(ws, as) {
				return false
			}
		}
		return true
	case fieldTime:
		wr, ar := triple(w), triple(a)
		day := ar[2] == "on" || triple(after["start"])[2] == "on" && f.name == "end"
		if wr[0] == "" {
			// No end given: none written, or the start itself (endless).
			return ar[0] == "" || f.name == "end" && ar[0] == triple(after["start"])[0] && ar[1] == triple(after["start"])[1]
		}
		if wr[0] != ar[0] || wr[2] != ar[2] {
			return false
		}
		return day || wr[1] == ar[1]
	case fieldDone:
		return first(w) == first(a)
	case fieldPercent:
		wv, av := first(w), first(a)
		if wv == "" {
			wv = "0"
		}
		if av == "" {
			av = "0"
		}
		return wv == av
	case fieldReminders:
		wr, _ := filledRows(w, nil)
		ar, _ := filledRows(a, nil)
		return slices.Equal(wr, ar)
	}
	return true
}

// shownArea is a text of several lines as its field shows and posts it: the
// line ends of one kind, nothing after the last line.
func shownArea(v string) string {
	return strings.TrimRight(strings.ReplaceAll(v, "\r\n", "\n"), "\n \t")
}

// filledRows are the rows of a repeating field that hold something, trimmed,
// each with its label.
func filledRows(rows, labels []string) (out, outLabels []string) {
	for i, v := range rows {
		if v = strings.TrimSpace(v); v == "" {
			continue
		}
		out = append(out, v)
		outLabels = append(outLabels, first(labels[min(i, len(labels)):]))
	}
	return out, outLabels
}

func sameSet(a, b []string) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	sort.Strings(x)
	sort.Strings(y)
	return slices.Equal(slices.Compact(x), slices.Compact(y))
}

// sameRepeat compares the rule a form asked for with the rule the saved
// record shows. A rule typed is the rule; otherwise the simple fields are
// what the person saw and set.
func sameRepeat(have, want, after []string) bool {
	pad := func(v []string) []string { return append(append([]string(nil), v...), make([]string, 6)...)[:6] }
	have, want, after = pad(have), pad(want), pad(after)
	if want[repRule] != have[repRule] {
		return sameRuleText(want[repRule], after[repRule])
	}
	if want[repFreq] == "" {
		return after[repFreq] == ""
	}
	interval := func(v string) string {
		if v == "" {
			return "1"
		}
		return v
	}
	return strings.EqualFold(want[repFreq], after[repFreq]) && interval(want[repInterval]) == interval(after[repInterval]) &&
		want[repOn] == after[repOn] && want[repUntil] == after[repUntil] && want[repCount] == after[repCount]
}

// editTamper, set by a test alone, spoils a change on its way to the record:
// the one way to show that a patch which is not the edit is never sent.
var editTamper func(*change)

// applyChecked is apply with the save looked at before it is sent (S-7.3):
// a fault is a save that must not be written.
func (s *Server) applyChecked(c *collectionCtx, obj *model.Object, ch change, have, want formValues) (*model.Object, *editFault, error) {
	was, err := c.wire(obj)
	if err != nil {
		return nil, nil, err
	}
	if editTamper != nil {
		editTamper(&ch)
	}
	sent, err := c.apply(obj, ch)
	if err != nil {
		return nil, nil, err
	}
	fault := s.checkEdit(c, was, sent, have, want, ch.restored)
	if fault != nil {
		// Carrel's own fault, and the one place it is known: the log names
		// the fields and properties, never what they hold.
		slog.Error("web: a save that is not the edit was not sent", "kind", string(c.spec.kind),
			"fields", strings.Join(fault.fields, ", "), "properties", strings.Join(fault.props, ", "))
	}
	return sent, fault, nil
}

// faultNotice says a save was not made and that the fault is Carrel's (the
// owner's decision of 11.10.2026): what would not read back as entered, what
// would change untouched, and the lines themselves for whoever reports it.
func faultNotice(f *editFault) view.Notice {
	var said []string
	if len(f.fields) > 0 {
		said = append(said, "the record would not read back as you entered it ("+strings.Join(f.fields, ", ")+")")
	}
	if len(f.props) > 0 {
		said = append(said, "it would also change what you did not touch ("+strings.Join(f.props, ", ")+")")
	}
	n := view.Notice{
		Role:  view.NoticeAlert,
		Title: "Carrel could not save this edit",
		Body: text("Nothing was saved, and the record is as it was. This is a fault in Carrel, not in what you entered or in the server: " +
			strings.Join(said, ", and ") + ". The form keeps what you entered."),
	}
	if len(f.lines) > 0 {
		d := &view.Diagnostic{Summary: "What the save would have changed besides"}
		for _, l := range f.lines {
			d.Lines = append(d.Lines, text(l))
		}
		n.Details = d
	}
	return n
}

// sameRuleText compares two rules as the sets of parts they are.
func sameRuleText(a, b string) bool {
	parts := func(v string) []string {
		var out []string
		for _, p := range parseRule(v) {
			out = append(out, p[0]+"="+strings.ToUpper(p[1]))
		}
		sort.Strings(out)
		return out
	}
	return slices.Equal(parts(a), parts(b))
}
