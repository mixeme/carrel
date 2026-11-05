package web

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/view"
)

// The question before the save of a series (the owner's decision of
// 11.10.2026, А1 of the audit; Typecase's note 56, s-agenda-22…-24). An edit
// that shortens a series, gives it another rule or stops it repeating may
// take the day of an occurrence changed by hand: nothing would show it any
// more, and other programs drop it. Save asks first, in one sheet over the
// edit: keep each as an event of its own, with what was changed in it, or
// remove them; Cancel goes back to the edit and writes nothing. A day left
// out that the series no longer has goes without a word. The occurrences an
// edit did not take the day of — those without one before it — are not
// asked about.

// leftField is the name the sheet's answers are sent under.
const leftField = "left"

// seriesBefore is what a series was before an edit: the occurrences it had
// already left without a day, its rule and its start.
type seriesBefore struct {
	left  []cal.Left
	rule  string
	start time.Time
}

// readSeries is what the record as read is as a series.
func readSeries(obj *model.Object, loc *time.Location) seriesBefore {
	var b seriesBefore
	b.left, _ = cal.LeftWithoutDay(obj, loc)
	b.rule = cal.RawRRULE(obj)
	if tm, err := cal.TimingOf(obj, loc); err == nil {
		b.start = tm.Start
	}
	return b
}

// leftByEdit are the occurrences of sent changed by hand that have no day in
// its series and had one before the edit.
func leftByEdit(before seriesBefore, sent *model.Object, loc *time.Location) []cal.Left {
	now, err := cal.LeftWithoutDay(sent, loc)
	if err != nil || len(now) == 0 {
		return nil
	}
	var out []cal.Left
	for _, l := range now {
		if !slices.ContainsFunc(before.left, func(b cal.Left) bool { return b.At.Equal(l.At) }) {
			out = append(out, l)
		}
	}
	return out
}

// leftNouns name what a series of a kind repeats, one and many.
var leftNouns = map[view.Kind][2]string{
	view.KindEvent: {"meeting", "meetings"},
	view.KindTask:  {"task", "tasks"},
}

// seriesClock is the time words of a series' sheet: in the series' own zone,
// as its form shows its times (formZone).
func seriesClock(c *collectionCtx, obj *model.Object) timeFmt {
	tf := c.tf
	tf.tz = formZone(zoneOf(obj.Props()[zoneProp(obj.Props())]), c.tf.tz)
	return tf
}

// leftWhen is where an occurrence stands: its day and, unless it is a whole
// day, its time — «26 January, 12:00».
func leftWhen(tf timeFmt, at time.Time, day bool) string {
	when := strings.TrimSuffix(tf.date(at), " "+strconv.Itoa(at.In(tf.tz).Year()))
	if !day {
		when += ", " + tf.clock(at)
	}
	return when
}

// leftLines are the occurrences left without their day, each where it stands
// and its title, and how it differs from the series (s-agenda-22).
func leftLines(tf timeFmt, kind view.Kind, title string, left []cal.Left) []view.AskLine {
	noun := leftNouns[kind][0]
	var out []view.AskLine
	for _, l := range left {
		at := l.At
		if !l.Start.IsZero() {
			at = l.Start
		}
		name := l.Summary
		if name == "" {
			name = title
		}
		line := view.AskLine{Text: text(leftWhen(tf, at, l.AllDay) + " — " + name)}
		switch {
		case l.Moved:
			line.Aside = text("— it was the " + noun + " of " + leftWhen(tf, l.At, l.AllDay))
		case l.Retitled:
			line.Aside = "— its title was changed"
		default:
			line.Aside = "— it was changed by hand"
		}
		out = append(out, line)
	}
	return out
}

// seriesChange is the line «What you changed», in the fewest words that say
// it (Typecase's note 74, 184 p. 3): an end that moved — «The series ends on
// 18 January — it ran weekly until 28 December» —, another pace — «The
// series repeats every two weeks — it repeated weekly» —, or «The series no
// longer repeats».
func seriesChange(tf timeFmt, was seriesBefore, sent *model.Object) view.AskLine {
	raw := cal.RawRRULE(sent)
	var start time.Time
	if tm, err := cal.TimingOf(sent, tf.tz); err == nil {
		start = tm.Start
	}
	before := ""
	if was.rule != "" {
		before = seriesPace(was.rule) + seriesEnd(tf, was.rule, was.start)
	}
	var line view.AskLine
	switch {
	case raw == "":
		line.Text = "The series no longer repeats"
		if before != "" {
			line.Aside = text("— it repeated " + before)
		}
	case was.rule != "" && withoutEnd(raw) == withoutEnd(was.rule):
		line.Text = "The series no longer ends"
		if until, ok := ruleUntil(raw, start, tf.tz); ok {
			line.Text = text("The series ends on " + tf.untilDay(until))
		} else if n := parseRule(raw).get("COUNT"); n != "" {
			line.Text = text("The series ends after " + n + " times")
		}
		line.Aside = text("— it ran " + before)
	default:
		line.Text = text("The series repeats " + seriesPace(raw))
		if before != "" {
			line.Aside = text("— it repeated " + seriesPace(was.rule))
		}
	}
	return line
}

// seriesPace is how often a rule repeats, in the words of the question:
// «weekly», «every two weeks».
func seriesPace(raw string) string {
	r := parseRule(raw)
	units := map[string]string{"YEARLY": "year", "MONTHLY": "month", "WEEKLY": "week", "DAILY": "day", "HOURLY": "hour"}
	unit, ok := units[r.get("FREQ")]
	if !ok {
		return "by its own rule"
	}
	n, _ := strconv.Atoi(r.get("INTERVAL"))
	if n <= 1 {
		if unit == "day" {
			return "daily"
		}
		return unit + "ly"
	}
	return "every " + countWord(n) + " " + unit + "s"
}

// seriesEnd is a rule's end after its pace: « until 28 December», « 8
// times», "" for none.
func seriesEnd(tf timeFmt, raw string, start time.Time) string {
	if until, ok := ruleUntil(raw, start, tf.tz); ok {
		return " until " + tf.untilDay(until)
	}
	if n := parseRule(raw).get("COUNT"); n != "" {
		return ", " + n + " times"
	}
	return ""
}

// withoutEnd is a rule with its end taken out, to tell a moved end from
// another pace.
func withoutEnd(raw string) string {
	var keep []string
	for _, p := range parseRule(raw) {
		if p[0] != "UNTIL" && p[0] != "COUNT" {
			keep = append(keep, p[0]+"="+p[1])
		}
	}
	sort.Strings(keep)
	return strings.Join(keep, ";")
}

// askSeries is the question of an edit that leaves left without their day
// (s-agenda-22, -23): over the edit as typed, the form sending all it sent
// again with the answer.
func (s *Server) askSeries(r *http.Request, c *collectionCtx, uid string, was seriesBefore, sent *model.Object, left []cal.Left, st editState) view.AskSheet {
	nouns := leftNouns[c.spec.kind]
	title := titleOr(cal.Text(sent, "SUMMARY"), "Untitled "+c.spec.noun)
	one := len(left) == 1
	sheet := view.AskSheet{Kind: c.spec.kind, Icon: "i-refresh", Subject: text(title + " · not saved yet")}
	tf := seriesClock(c, sent)
	block := view.AskBlock{Title: text("Left without their day · " + strconv.Itoa(len(left))), Lines: leftLines(tf, c.spec.kind, title, left)}
	keep := "Keep as events"
	if one {
		sheet.Title = text("Keep the " + nouns[0] + " that was moved by hand?")
		block.Title = "Left without its day · 1"
		sheet.Note = "Kept, it becomes an event of its own, with what was changed in it. Removed, it goes: the series no longer has that day."
		keep = "Keep as an event"
	} else {
		sheet.Title = text("Keep the " + nouns[1] + " that were moved by hand?")
		sheet.Note = "Kept, each becomes an event of its own, with what was changed in it. Removed, they go: the series no longer has those days."
	}
	if c.spec.kind == view.KindTask {
		keep = strings.Replace(keep, "an event", "a task", 1)
		keep = strings.Replace(keep, "events", "tasks", 1)
		sheet.Note = text(strings.ReplaceAll(string(sheet.Note), "an event of its own", "a task of its own"))
	}
	sheet.Blocks = []view.AskBlock{{Title: "What you changed", Lines: []view.AskLine{seriesChange(tf, was, sent)}}, block}
	// Everything the edit sent goes with the answer, hidden.
	address := c.recordPath(uid) + "/edit"
	rows := []view.FormRow{csrfField(c.sess)}
	keys := make([]string, 0, len(r.PostForm))
	for k := range r.PostForm {
		if k != csrfFormField && k != leftField {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range r.PostForm[k] {
			rows = append(rows, view.FormRow{Name: k, Kind: view.InputHidden, Value: text(v)})
		}
	}
	sheet.Form = view.Form{Action: s.route(address), Method: "POST", Sets: []view.FieldSet{{Rows: rows}}}
	answer := func(id, label, value string, primary bool) view.Action {
		return view.Action{ID: id, Label: text(label), Method: "POST", Primary: primary, Enabled: true, Submits: true,
			Href: s.postQuery(address, url.Values{leftField: {value}})}
	}
	sheet.Actions = []view.Action{answer("series.keep", keep, "keep", true), answer("series.remove", "Remove", "remove", false), answer("series.cancel", "Cancel", "cancel", false)}
	if under := s.editUnder(c, st); under != nil {
		sheet.Under = under
	}
	return sheet
}

// editUnder is the edit under a question, as typed.
func (s *Server) editUnder(c *collectionCtx, st editState) *view.Under {
	switch p := s.editPage(c, st).(type) {
	case view.EventEditPage:
		return &view.Under{EventEdit: &p}
	case view.TaskEditPage:
		return &view.Under{TaskEdit: &p}
	}
	return nil
}

// apartFailed is an occurrence that could not be written as an event of its
// own: it stays in the series, as it was.
type apartFailed struct {
	left cal.Left
	err  error
}

// writeApart writes each occurrence of sent standing for a day in left as an
// event of its own in the record's collection. kept are the days written,
// made the paths of the events made — taken back if the series then cannot
// be written.
func (s *Server) writeApart(ctx context.Context, c *collectionCtx, sent *model.Object, left []cal.Left) (kept []time.Time, made []string, failed []apartFailed) {
	for _, l := range left {
		uid, err := model.NewUID()
		if err != nil {
			failed = append(failed, apartFailed{l, err})
			continue
		}
		ev, ok, err := cal.Apart(sent, l.At, uid, c.tf.tz)
		if err == nil && !ok {
			continue
		}
		if err == nil && apartRefused != nil {
			err = apartRefused(l.At)
		}
		if err == nil {
			path := strings.TrimSuffix(c.col.Path, "/") + "/" + uid + c.spec.ext
			ev.Path = path
			if err = c.create(ctx, path, ev); err == nil {
				kept, made = append(kept, l.At), append(made, path)
				continue
			}
		}
		failed = append(failed, apartFailed{l, err})
	}
	return kept, made, failed
}

// apartRefused, set by a test alone, refuses the event apart of the day it
// names, as a server that did not take it (s-agenda-24).
var apartRefused func(at time.Time) error

// takeBack removes the events writeApart made, when the series they were
// taken from could not be written: they are in it still.
func (s *Server) takeBack(ctx context.Context, c *collectionCtx, made []string) {
	for _, p := range made {
		_ = c.remove(ctx, p, "")
	}
}

// askFailed is the sheet after the answer «keep» where some occurrence could
// not be written apart (s-agenda-24): what was written, what was not and
// why; Try again writes those again, Leave it in the series leaves them as
// they were. The events written apart are named where they stand — their
// own time, not the day of the series they stood for (s-agenda-24: «26
// January, 12:00 and 16 February, 14:30»); left are the occurrences asked
// about.
func (s *Server) askFailed(r *http.Request, c *collectionCtx, uid string, sent *model.Object, kept []time.Time, failed []apartFailed, left []cal.Left) view.AskSheet {
	title := titleOr(cal.Text(sent, "SUMMARY"), "Untitled "+c.spec.noun)
	nouns := leftNouns[c.spec.kind]
	tf := seriesClock(c, sent)
	sheet := view.AskSheet{Kind: c.spec.kind, Icon: "i-refresh", Title: text("Keep the " + nouns[1] + " that were moved by hand?"), Subject: text(title + " · saved in part")}
	written := []view.AskLine{{Text: "The series", Aside: "— saved"}}
	if len(kept) > 0 {
		var days []string
		for _, at := range kept {
			where, allDay := at, false
			for _, l := range left {
				if l.At.Equal(at) {
					allDay = l.AllDay
					if !l.Start.IsZero() {
						where = l.Start
					}
				}
			}
			days = append(days, leftWhen(tf, where, allDay))
		}
		word := "separate events"
		if c.spec.kind == view.KindTask {
			word = "separate tasks"
		}
		if len(kept) == 1 {
			word = strings.TrimSuffix(word, "s")
		}
		written = append(written, view.AskLine{Text: text(strconv.Itoa(len(kept)) + " " + word), Aside: text("— " + strings.Join(days, " and "))})
	}
	var lefts []cal.Left
	var lines []view.AskLine
	rows := []view.FormRow{csrfField(c.sess), {Name: "etag", Kind: view.InputHidden, Value: text(sent.ETag)}}
	for _, f := range failed {
		lefts = append(lefts, f.left)
		rows = append(rows, view.FormRow{Name: "day", Kind: view.InputHidden, Value: text(f.left.At.UTC().Format(time.RFC3339))})
	}
	for i, l := range leftLines(tf, c.spec.kind, title, lefts) {
		l.Aside = text("— " + failureWords(failed[i].err))
		lines = append(lines, l)
	}
	sheet.Blocks = []view.AskBlock{{Title: "Written", Lines: written}, {Title: text("Not written · " + strconv.Itoa(len(failed))), Lines: lines}}
	first := failed[0].left
	which := "The " + nouns[0] + " of " + leftWhen(tf, first.At, true)
	if len(failed) > 1 {
		which = strconv.Itoa(len(failed)) + " " + nouns[1]
	}
	sheet.Alert = text(which + " was not made a separate event: it stays in the series as it was before you saved.")
	if len(failed) > 1 {
		sheet.Alert = text(which + " were not made separate events: they stay in the series as they were before you saved.")
	}
	sheet.Note = "Nothing is lost: what could not be written apart is still in the series."
	address := c.recordPath(uid) + "/edit"
	sheet.Form = view.Form{Action: s.route(address), Method: "POST", Sets: []view.FieldSet{{Rows: rows}}}
	sheet.Actions = []view.Action{
		{ID: "series.retry", Label: "Try again", Method: "POST", Primary: true, Enabled: true, Submits: true, Href: s.postQuery(address, url.Values{"op": {"apart"}})},
		{ID: "series.leave", Label: "Leave it in the series", Method: "GET", Enabled: true, Href: s.route(c.recordPath(uid))},
	}
	sheet.Under = s.underOf(r, "", c.recordPath(uid))
	return sheet
}

// failureWords is why a server did not take a write, as a sheet says it:
// its status, or the error.
func failureWords(err error) string {
	if err == nil {
		return "the server did not take it"
	}
	return "the server answered " + err.Error()
}

// postApart is Try again of s-agenda-24: the occurrences named by their days
// written apart again from the series as the server holds it now, and taken
// out of it once written.
func (s *Server) postApart(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string) {
	ctx := r.Context()
	obj, err := c.find(ctx, uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.cardAnswer(r, c, uid, nil, s.unavailableNotice(c, c.recordPath(uid), err)))
		return
	}
	all, err := cal.LeftWithoutDay(obj, c.tf.tz)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.cardAnswer(r, c, uid, obj, readNotice(obj.Path, err)))
		return
	}
	r.FormValue("day") // parses the form
	var left []cal.Left
	for _, v := range r.Form["day"] {
		at, err := time.Parse(time.RFC3339, v)
		if err != nil {
			continue
		}
		if i := slices.IndexFunc(all, func(l cal.Left) bool { return l.At.Equal(at) }); i >= 0 {
			left = append(left, all[i])
		}
	}
	if len(left) == 0 {
		s.redirect(w, r, c.recordPath(uid))
		return
	}
	kept, made, failed := s.writeApart(ctx, c, obj, left)
	if len(kept) > 0 {
		sent, err := cal.DropLeft(obj, kept, c.tf.tz)
		if err == nil {
			sent.ETag = obj.ETag
			err = c.update(ctx, sent)
		}
		if err != nil {
			s.takeBack(ctx, c, made)
			s.writePage(w, r, http.StatusOK, s.cardAnswer(r, c, uid, obj, s.writeFailure(c, "PUT", obj.Path, err)))
			return
		}
		sent.ETag = ""
		c.noteWrite(sent.Path, sent)
		obj = sent
	}
	if len(failed) > 0 {
		s.writePage(w, r, http.StatusOK, s.askFailed(r, c, uid, obj, kept, failed, left))
		return
	}
	s.redirect(w, r, c.recordPath(uid))
}
