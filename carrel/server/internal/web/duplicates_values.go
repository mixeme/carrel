package web

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The questions of a group (S-16; from-typecase note 117 p. 1) are asked
// only where its members disagree, the same way for linking and merging. A
// value one member holds alone is a radio of its property — the name, the
// organisation, the photo, the time («When», whole: start, end and
// repetition), the place, a task's due date and priority, a note's text —,
// a value of a property held many times a tick of «what to keep» — the
// phones, by their type, the addresses, the tags —, every one ticked until
// the person unticks it. What the person chose is the linked record: the
// card of the entity marks those values with the collection they came
// from, and a copy names the fields where it holds something else
// («Phone, email differ»).

// recValue is one value of a record as a question compares it.
type recValue struct {
	// Key tells two values apart: the value itself, a photo's digest, a
	// time as written (cal.TimeKey)
	Key string `json:"k"`
	// Label is what the question says of it; Key when empty
	Label string `json:"l,omitempty"`
}

func (v recValue) label() string {
	if v.Label != "" {
		return v.Label
	}
	return v.Key
}

// recValues are a record's values per question: the property, and for a
// phone its type after askType — «TEL\x1fPhone · work».
type recValues map[string][]recValue

const askType = "\x1f"

// askedProps are the questions of a kind in the order the card asks them
// (s-dup-1: «Display name», «Organisation», the phones, «Email», «Photo»;
// «When», «Where»; «Due», «Priority»; «Text», «Tags»). TEL stands for every
// phone question.
var askedProps = map[view.Kind][]string{
	// Every field of the card is asked where the copies differ, one a copy
	// lacks with «— · ‹copies›» (Дб7 of every field; note 123 p. 3): the
	// title, the addresses by their type, the birthday, the note too.
	view.KindContact: {"FN", "ORG", "TEL", "EMAIL", "TITLE", "ADR", "BDAY", "PHOTO", "CATEGORIES", "NOTE"},
	view.KindEvent:   {"WHEN", "LOCATION", "CATEGORIES"},
	// A task holds a place as an event does, and is asked about it the same
	// way (Р21 of scans/kinds-parity.md).
	view.KindTask: {"DUE", "PRIORITY", "LOCATION", "CATEGORIES"},
	view.KindNote: {"TEXT", "CATEGORIES"},
}

// manyProps are asked with ticks: what to keep of them.
var manyProps = map[string]bool{"TEL": true, "EMAIL": true, "ADR": true, "CATEGORIES": true}

// typedProps are asked a question per type of value: «Phone · work»,
// «Address · work».
var typedProps = map[string]string{"TEL": "Phone", "ADR": "Address"}

// askedLabels name a question as the card does.
var askedLabels = map[string]string{
	"FN": "Display name", "ORG": "Organisation", "TITLE": "Title", "BDAY": "Birthday", "NOTE": "Note",
	"PHOTO": "Photo", "EMAIL": "Email", "CATEGORIES": "Tags",
	"WHEN": "When", "LOCATION": "Where", "DUE": "Due", "PRIORITY": "Priority", "TEXT": "Text",
}

// differWords name a question in «Phone, email differ» (s-contacts-33,
// s-tasks-17, s-notes-32, s-agenda-25).
var differWords = map[string]string{
	"FN": "name", "ORG": "organisation", "TITLE": "title", "ADR": "address", "BDAY": "birthday", "NOTE": "note",
	"PHOTO": "photo", "TEL": "phone", "EMAIL": "email",
	"CATEGORIES": "tags", "WHEN": "when", "LOCATION": "where", "DUE": "due", "PRIORITY": "priority", "TEXT": "text",
}

// byHolder are the questions whose value says nothing to read — a photo, a
// note's text —: an option is told by where it lives (s-dup-1, «nextcloud ·
// Записи»).
var byHolder = map[string]bool{"PHOTO": true, "TEXT": true}

// propOf is the property of a question: «TEL» of «TEL\x1fPhone · work».
func propOf(id string) string {
	p, _, _ := strings.Cut(id, askType)
	return p
}

// askedLabel is a question's label: «Phone · work» for a phone.
func askedLabel(id string) string {
	if _, typed, ok := strings.Cut(id, askType); ok {
		return typed
	}
	return askedLabels[id]
}

// valuesOf are the values of a record the questions of its kind compare.
func valuesOf(tf timeFmt, k view.Kind, o *model.Object) recValues {
	out := recValues{}
	add := func(id string, v recValue) {
		if strings.TrimSpace(v.Key) == "" {
			return
		}
		for _, have := range out[id] {
			if have.Key == v.Key {
				return
			}
		}
		out[id] = append(out[id], v)
	}
	digest := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:8])
	}
	for _, tag := range categoriesOf(k, o) {
		add("CATEGORIES", recValue{Key: tag})
	}
	switch k {
	case view.KindContact:
		add("FN", recValue{Key: strings.TrimSpace(firstValue(o, "FN"))})
		add("ORG", recValue{Key: structured(firstValue(o, "ORG"))})
		if p := firstValue(o, "PHOTO"); p != "" {
			add("PHOTO", recValue{Key: digest(p)})
		}
		for _, v := range o.Props()["TEL"] {
			add("TEL"+askType+typedLabel("Phone", v.Params), recValue{Key: dupes.ValueKey("TEL", v.Text), Label: strings.TrimSpace(v.Text)})
		}
		for _, v := range o.PropText("EMAIL") {
			add("EMAIL", recValue{Key: dupes.ValueKey("EMAIL", v), Label: strings.TrimSpace(v)})
		}
		add("TITLE", recValue{Key: strings.TrimSpace(firstValue(o, "TITLE"))})
		for _, v := range o.Props()["ADR"] {
			if a := structured(v.Text); a != "" {
				add("ADR"+askType+typedLabel("Address", v.Params), recValue{Key: strings.ToLower(a), Label: a})
			}
		}
		if v := o.Props()["BDAY"]; len(v) > 0 {
			add("BDAY", recValue{Key: strings.TrimSpace(v[0].Text), Label: birthdayShown(v[0].Text, v[0].Params)})
		}
		add("NOTE", recValue{Key: strings.TrimSpace(firstValue(o, "NOTE"))})
	case view.KindEvent:
		add("WHEN", recValue{Key: cal.TimeKey(o), Label: tf.whenShort(o)})
		add("LOCATION", recValue{Key: strings.TrimSpace(cal.Text(o, "LOCATION"))})
	case view.KindTask:
		if due, err := cal.DueOf(o, tf.tz); err == nil && !due.Start.IsZero() {
			add("DUE", recValue{Key: cal.TimeKey(o), Label: tf.shortDue(due)})
		}
		if n, err := strconv.Atoi(strings.TrimSpace(firstValue(o, "PRIORITY"))); err == nil && n > 0 {
			add("PRIORITY", recValue{Key: strconv.Itoa(n)})
		}
		add("LOCATION", recValue{Key: strings.TrimSpace(cal.Text(o, "LOCATION"))})
	case view.KindNote:
		if d := strings.TrimSpace(cal.Description(o)); d != "" {
			add("TEXT", recValue{Key: digest(d)})
		}
	}
	return out
}

// whenShort is a time as a question offers it (s-dup-1): «Mon 10:00 –
// 11:00, weekly» for a series, the day for one occurrence, «all day».
func (f timeFmt) whenShort(o *model.Object) string {
	tm, err := cal.TimingOf(o, f.tz)
	if err != nil || tm.Start.IsZero() {
		return "No time"
	}
	raw := cal.RawRRULE(o)
	start := tm.Start.In(f.tz)
	day := f.shortDay(start)
	if raw != "" {
		day = start.Format("Mon")
	}
	out := day
	switch {
	case tm.AllDay:
		out += ", all day"
	case !tm.End.IsZero() && f.dayKey(tm.End) == f.dayKey(tm.Start):
		out += " " + f.clock(tm.Start) + " – " + f.clock(tm.End)
	default:
		out += " " + f.clock(tm.Start)
	}
	if raw != "" {
		words, _ := f.ruleWords(raw, true)
		// «Weekly on Mon» of a series that starts on a Monday says the day
		// twice: the frequency is enough.
		words = strings.TrimSuffix(words, " on "+start.Format("Mon"))
		out += ", " + strings.ToLower(words[:1]) + words[1:]
	}
	return out
}

// groupMember is a member of a group as its questions see it.
type groupMember struct {
	ref sealed.RecordRef
	// col is the name of its collection («Команда»), stamp where it lives
	// whole («radicale · vps · Команда»)
	col    string
	colour view.Colour
	stamp  view.Stamp
	values recValues
	// loaded: its values are known; a member whose list this session has
	// not read offers nothing
	loaded bool
}

// question is one disagreement of a group.
type question struct {
	id   string
	opts []answerOption
	// chosen is the option of a radio; kept the ticks of a property held
	// many times
	chosen int
	kept   []bool
	// shared are the values of a property held many times that every
	// member holds: nobody is asked about them, and they stay
	shared []recValue
}

func (q question) prop() string { return propOf(q.id) }
func (q question) many() bool   { return manyProps[q.prop()] }

// answerOption is one value a question offers.
type answerOption struct {
	value   recValue
	none    bool // the member holds no value: choosing it leaves the property out (Дб7)
	holders []groupMember
	// number is what the option sends: choice.<PROP>=number, from 1,
	// counted across the questions of one property (s-dup-1)
	number int
}

// text is the option's label: the value and the collections holding it
// («+7 495 120-45-77 · Команда», «— · Дела»), or where it lives when the
// value says nothing — a photo, a note's text —, held or not (s-dup-1:
// «nextcloud · Архив» of a copy without a photo; note 123 p. 3).
func (o answerOption) text(q question) string {
	if byHolder[q.prop()] {
		return string(o.holders[0].stamp)
	}
	v := "—"
	if !o.none {
		v = o.value.label()
	}
	var cols []string
	for _, h := range o.holders {
		if !slices.Contains(cols, h.col) {
			cols = append(cols, h.col)
		}
	}
	return v + " · " + strings.Join(cols, ", ")
}

func (o answerOption) holds(ref sealed.RecordRef) bool {
	return slices.ContainsFunc(o.holders, func(h groupMember) bool { return h.ref == ref })
}

// questionsOf are the questions of a group among its loaded members, with
// the answers the person gave (prefs, dropped) or what stays without one:
// the first value held, every value ticked.
func questionsOf(k view.Kind, members []groupMember, prefs map[string]sealed.RecordRef, dropped map[string][]string) []question {
	var loaded []groupMember
	for _, m := range members {
		if m.loaded {
			loaded = append(loaded, m)
		}
	}
	if len(loaded) < 2 {
		return nil
	}
	var out []question
	numbers := map[string]int{}
	for _, prop := range askedProps[k] {
		ids := []string{prop}
		if typedProps[prop] != "" {
			ids = nil
			for _, m := range loaded {
				for id := range m.values {
					if propOf(id) == prop && !slices.Contains(ids, id) {
						ids = append(ids, id)
					}
				}
			}
			slices.Sort(ids)
		}
		for _, id := range ids {
			q := question{id: id}
			if manyProps[prop] {
				q = manyQuestion(id, loaded, dropped[prop])
			} else {
				q = oneQuestion(id, loaded, prefs[id])
			}
			if len(q.opts) == 0 {
				continue
			}
			for i := range q.opts {
				numbers[prop]++
				q.opts[i].number = numbers[prop]
			}
			out = append(out, q)
		}
	}
	return out
}

// oneQuestion asks a property held once when the members hold it
// differently; a member without a value offers «—» (Дб7).
func oneQuestion(id string, members []groupMember, pref sealed.RecordRef) question {
	q := question{id: id}
	held := false
	for _, m := range members {
		vals := m.values[id]
		at := slices.IndexFunc(q.opts, func(o answerOption) bool {
			return (len(vals) == 0 && o.none) || (len(vals) > 0 && !o.none && o.value.Key == vals[0].Key)
		})
		if at >= 0 {
			q.opts[at].holders = append(q.opts[at].holders, m)
			continue
		}
		o := answerOption{none: len(vals) == 0, holders: []groupMember{m}}
		if len(vals) > 0 {
			o.value, held = vals[0], true
		}
		q.opts = append(q.opts, o)
	}
	if !held || len(q.opts) < 2 {
		return question{id: id}
	}
	q.chosen = slices.IndexFunc(q.opts, func(o answerOption) bool { return !o.none })
	if at := slices.IndexFunc(q.opts, func(o answerOption) bool { return o.holds(pref) }); at >= 0 {
		q.chosen = at
	}
	return q
}

// manyQuestion asks what to keep of a property held many times: the values
// not every member holds, ticked unless the person unticked them.
func manyQuestion(id string, members []groupMember, dropped []string) question {
	q := question{id: id}
	var all []answerOption
	for _, m := range members {
		for _, v := range m.values[id] {
			at := slices.IndexFunc(all, func(o answerOption) bool { return o.value.Key == v.Key })
			if at < 0 {
				all = append(all, answerOption{value: v})
				at = len(all) - 1
			}
			all[at].holders = append(all[at].holders, m)
		}
	}
	for _, o := range all {
		if len(o.holders) == len(members) {
			q.shared = append(q.shared, o.value)
			continue
		}
		q.opts = append(q.opts, o)
		q.kept = append(q.kept, !slices.Contains(dropped, o.value.Key))
	}
	return q
}

// shownValues are the values a linked record shows of a question: the
// option chosen, or the values kept and those every member holds.
func (q question) shownValues() []recValue {
	if !q.many() {
		if o := q.opts[q.chosen]; !o.none {
			return []recValue{o.value}
		}
		return nil
	}
	out := slices.Clone(q.shared)
	for i, o := range q.opts {
		if q.kept[i] {
			out = append(out, o.value)
		}
	}
	return out
}

// shownFrom is where a value the linked record shows came from, when it was
// a choice — the collection the card names after it (s-contacts-33): the
// option chosen or kept that holds it; "" for a value nobody was asked
// about.
func (q question) shownFrom(key string) string {
	for i, o := range q.opts {
		if o.none || o.value.Key != key {
			continue
		}
		if (q.many() && q.kept[i]) || (!q.many() && i == q.chosen) {
			return o.holders[0].col
		}
	}
	return ""
}

// differing names the questions where values hold something else than
// shown: «Phone, email differ», one singular field «Priority differs»
// (from-typecase note 117 p. 4). "" when they hold the same.
func differing(qs []question, shown func(question) []recValue, values recValues) string {
	var words []string
	for _, q := range qs {
		w := differWords[q.prop()]
		if slices.Contains(words, w) || sameKeys(shown(q), values[q.id]) {
			continue
		}
		// A field the copy does not hold at all says nothing of it — the
		// title, an address, the birthday, a note of one copy (note 123 p.
		// 12: «Phone, email differ»); a phone or an address of mail it lacks
		// does.
		if len(values[q.id]) == 0 && q.prop() != "TEL" && q.prop() != "EMAIL" {
			continue
		}
		words = append(words, w)
	}
	if len(words) == 0 {
		return ""
	}
	out := strings.Join(words, ", ")
	verb := " differ"
	if len(words) == 1 && words[0] != "tags" {
		verb = " differs"
	}
	return strings.ToUpper(out[:1]) + out[1:] + verb
}

func sameKeys(a, b []recValue) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		if !slices.ContainsFunc(b, func(w recValue) bool { return w.Key == v.Key }) {
			return false
		}
	}
	return true
}

// answersOf read the card's answers to its questions: a radio's number
// (choice.<PROP>=n) names the member whose value stays, a tick not sent is
// a value dropped.
func answersOf(r *http.Request, qs []question) (map[string]sealed.RecordRef, map[string][]string) {
	prefs := map[string]sealed.RecordRef{}
	dropped := map[string][]string{}
	_ = readForm(r)
	for _, q := range qs {
		sent := r.Form["choice."+q.prop()]
		if q.many() {
			if _, ok := dropped[q.prop()]; !ok {
				dropped[q.prop()] = []string{}
			}
			for _, o := range q.opts {
				if !slices.Contains(sent, strconv.Itoa(o.number)) {
					dropped[q.prop()] = append(dropped[q.prop()], o.value.Key)
				}
			}
			continue
		}
		for _, o := range q.opts {
			if slices.Contains(sent, strconv.Itoa(o.number)) {
				prefs[q.id] = o.holders[0].ref
			}
		}
	}
	return prefs, dropped
}

// mergeRowsOf are the questions as the card shows them (view.MergeRow).
func mergeRowsOf(qs []question) []view.MergeRow {
	var out []view.MergeRow
	for _, q := range qs {
		row := view.MergeRow{Property: text(askedLabel(q.id)), Name: "choice." + q.prop(), Many: q.many(), Chosen: q.chosen}
		for i, o := range q.opts {
			row.Options = append(row.Options, text(o.text(q)))
			row.Values = append(row.Values, strconv.Itoa(o.number))
			if q.many() && q.kept[i] {
				row.Kept = append(row.Kept, i)
			}
		}
		out = append(out, row)
	}
	return out
}
