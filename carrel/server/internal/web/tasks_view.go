package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// What the frames of the tasks draw beyond a list of rows (s-tasks-1,
// s-tasks-2; the client's scan of the tasks): the fields of a task in the
// order of its card and of its panel, the overdue said in words, the line
// under the panel's attachments, and the record open in the panel beside a
// list without scripts.

// openRow answers ?open={a row's address} on a list without scripts (C-6.4,
// s-tasks-1, s-notes-1): the record of that row stands in the panel, read,
// and on marks its row; Close is the list without it. With scripts the
// panel is a fragment at the record's address (C-7.2).
func (s *Server) openRow(r *http.Request, sess *session.Session, base string, spec kindSpec, items []listed, on func(href string)) *view.Details {
	open, _ := r.Context().Value(openKey{}).(string)
	if open == "" {
		return nil
	}
	for _, it := range items {
		uid := listedUID(spec.kind, it)
		if s.route(it.from.recordPath(spec.segment, uid)) != open {
			continue
		}
		c, ok := s.collectionOf(sess, spec, it.from.conn.ID, it.from.col.Path)
		if !ok {
			return nil
		}
		on(open)
		d := s.recordDetails(c, uid, it.obj)
		d.Close = s.route(base)
		if r.URL.RawQuery != "" {
			d.Close += "?" + r.URL.RawQuery
		}
		// A linked row of a combined list opens the linked record
		// (s-tasks-1, s-notes-1; note 123 p. 10).
		if combinedBase(base) && linkedState(it) == view.RecordLinked {
			d = s.linkedDetails(r.Context(), sess, spec, it, d)
		}
		return &d
	}
	return nil
}

// taskRowsOn marks the rows of an address on: the row open in the panel.
func taskRowsOn(groups []view.TaskGroup) func(string) {
	return func(href string) {
		for gi := range groups {
			for ri := range groups[gi].Rows {
				if groups[gi].Rows[ri].Href == href {
					groups[gi].Rows[ri].On = true
				}
			}
		}
	}
}

// noteRowsOn marks the rows of an address on: the note open in the panel.
func noteRowsOn(groups []view.NoteGroup) func(string) {
	return func(href string) {
		for gi := range groups {
			for ri := range groups[gi].Rows {
				if groups[gi].Rows[ri].Href == href {
					groups[gi].Rows[ri].On = true
				}
			}
		}
	}
}

// filterByWord keeps the notes or the tasks whose title or text holds the
// word («Filter by word»: s-notes-1, s-tasks-1), as the client sifts them
// with scripts.
func filterByWord(items []listed, filter string) []listed {
	needle := strings.ToLower(strings.TrimSpace(filter))
	if needle == "" {
		return items
	}
	var out []listed
	for _, it := range items {
		title := strings.ToLower(cal.Text(it.pick("SUMMARY"), "SUMMARY"))
		body := strings.ToLower(cal.Description(it.obj))
		if strings.Contains(title, needle) || strings.Contains(body, needle) {
			out = append(out, it)
		}
	}
	return out
}

// notebookSubtitle is the line under a notebook's title: what the list of
// any kind says, and how the notes lie on the server — «… · 3 notes ·
// VJOURNAL» (s-notes-1).
func notebookSubtitle(line string) string {
	return line + " · VJOURNAL"
}

// oneNotebookSubtitle is that line of one notebook: its connection, how many
// notes and how they lie, then that the server gave no write (states-30:
// «nextcloud · 2 notes · VJOURNAL · the server granted no write access»); a
// notebook that did not answer says only where it is (states-29).
func oneNotebookSubtitle(c *collectionCtx, notes int, read bool) string {
	line := connLabel(c.conn)
	if read {
		line = notebookSubtitle(line + " · " + plural(notes, recordNouns[c.spec.kind]))
	}
	return line + noWriteTail(c)
}

// combinedNotebooks is the line under the combined notebook (s-notes-1): how
// many of the notebooks are ticked and, once the poll is over, how many
// notes — no account: the frame names none.
func combinedNotebooks(rail view.Rail, spec kindSpec, seen *int) string {
	ticked, _, count := combinedParts(rail, spec, seen)
	if count == "" {
		return ticked
	}
	return ticked + " · " + count
}

// countTaskGroups puts each group's count by its rubric, the overdue one an
// alarm (s-tasks-1): the client does not count (C-6.1).
func countTaskGroups(groups []view.TaskGroup) {
	for i := range groups {
		groups[i].Count = len(groups[i].Rows)
		groups[i].Overdue = groups[i].Rubric == "Overdue"
	}
}

// sourcesCount is what the column of the source says of a linked row in a
// combined view: how many sources it has, «2 sources»; the stamps are its
// hint (Sources, s-tasks-1, s-notes-1).
func sourcesCount(it listed) view.Stamp {
	return view.Stamp(plural(len(it.also)+1, "source"))
}

// overdueBy says how late a task is, as its card and form write it: «overdue
// by 3 days», «overdue by 5 hours»; a day due is late from the day after.
// Empty when it is not late, or done.
func overdueBy(tf timeFmt, due cal.Timing, done bool) string {
	if done || due.Start.IsZero() {
		return ""
	}
	now := tf.now
	if due.AllDay {
		days := int(dayOf(now, tf.tz).Sub(dayOf(due.Start, tf.tz)).Hours() / 24)
		if days < 1 {
			return ""
		}
		return "overdue by " + plural(days, "day")
	}
	late := now.Sub(due.Start)
	switch {
	case late <= 0:
		return ""
	case late >= 24*time.Hour:
		return "overdue by " + plural(int(late/(24*time.Hour)), "day")
	case late >= time.Hour:
		return "overdue by " + plural(int(late/time.Hour), "hour")
	}
	return "overdue by " + plural(max(1, int(late/time.Minute)), "minute")
}

// taskFields are the fields of a task in the order of its card (s-tasks-2):
// Status — Needs action when it has none —, Due monospaced with how late it
// is (on the card), Starts, Complete and Priority monospaced, Reminders, Where, Link,
// Tags. The panel (s-tasks-1) has the card's order, Reminders too
// (Typecase 2823585).
func taskFields(tf timeFmt, obj *model.Object, panel bool, tags []view.Tag) ([]view.Field, error) {
	status := taskStatuses[0].label
	if v := strings.TrimSpace(firstValue(obj, "STATUS")); v != "" {
		status = statusLabel(v)
	}
	out := []view.Field{{Label: "Status", Value: text(status)}}
	due, err := cal.DueOf(obj, tf.tz)
	if err == nil && !due.Start.IsZero() {
		f := view.Field{Label: "Due", Runs: view.Runs{{Text: text(tf.timeOrDate(due)), Mark: view.RunMono}}, Badge: tf.createdIn(due)}
		// How late, on the card; the panel says the day only (s-tasks-20,
		// Typecase's note 125 p. 9) — its list's row says it is late.
		if late := overdueBy(tf, due, taskDone(obj)); late != "" && !panel {
			f.Runs = append(f.Runs, view.Run{Text: text(" · " + late), Mark: view.RunDanger})
		}
		out = append(out, f)
	}
	if start, err := cal.TimingOf(obj, tf.tz); err == nil && !start.Start.IsZero() {
		out = append(out, view.Field{Label: "Starts", Value: text(tf.timeOrDate(start))})
	}
	percent := 0
	if p := percentOf(obj); p != nil {
		percent = *p
	}
	complete := view.Field{Label: "Complete", Value: text(strconv.Itoa(percent) + "%"), Mono: true}
	var priority []view.Field
	if p := priorityOf(obj); p <= 9 {
		priority = []view.Field{{Label: "Priority", Value: text(strconv.Itoa(p)), Mono: true}}
	}
	out = append(append(out, complete), priority...)
	alarms, _ := cal.Alarms(obj)
	if rows := reminderRows(alarms, due.AllDay, tf); len(rows) > 0 {
		labels := make([]string, 0, len(rows))
		for _, r := range rows {
			labels = append(labels, string(r.Label))
		}
		out = append(out, view.Field{Label: "Reminders", Value: text(strings.Join(labels, " · "))})
	}
	if v := cal.Text(obj, "LOCATION"); v != "" {
		out = append(out, view.Field{Label: "Where", Value: text(v)})
	}
	if v := firstValue(obj, "URL"); v != "" {
		out = append(out, view.Field{Label: "Link", Value: text(v), Href: webHref(v)})
	}
	if len(tags) > 0 {
		out = append(out, view.Field{Label: "Tags", Tags: tags})
	}
	return out, err
}

// taskSubtitle is the line under a task's title (s-tasks-2, s-tasks-3):
// where it is, and how late it is, in alarm — the person who came by a link
// has seen no list. obj is the version stored; nil — a new task.
func taskSubtitle(c *collectionCtx, obj *model.Object) view.Runs {
	out := view.Runs{{Text: text(string(c.stamp()))}}
	if obj == nil {
		return out
	}
	if due, err := cal.DueOf(obj, c.tf.tz); err == nil {
		if late := overdueBy(c.tf, due, taskDone(obj)); late != "" {
			out[0].Text += " · "
			out = append(out, view.Run{Text: text(late), Mark: view.RunDanger})
		}
	}
	return out
}

// taskEditSubtitle is the line under the title of a task's edit
// (s-tasks-3, -14): only what its fields do not say — how late it is, in
// alarm. Where it lies is the choice in the head (Typecase's note 72).
func taskEditSubtitle(c *collectionCtx, obj *model.Object) view.Runs {
	if obj == nil {
		return nil
	}
	if due, err := cal.DueOf(obj, c.tf.tz); err == nil {
		if late := overdueBy(c.tf, due, taskDone(obj)); late != "" {
			return view.Runs{{Text: text(late), Mark: view.RunDanger}}
		}
	}
	return nil
}

// taskPaper is a task's row on paper (print-4; Typecase's note 132): the
// day it is due in the column, and under its title the time it is due,
// where it lies — a linked one's connection and lists, «nextcloud · Дела,
// Работа» —, and how much of it is done or, without that, how late it is.
func taskPaper(tf timeFmt, it listed, due cal.Timing, percent *int, done bool) (view.Text, view.Text) {
	var day string
	var line []string
	if !due.Start.IsZero() {
		day = tf.shortDay(due.Start)
		if !due.AllDay {
			line = append(line, tf.clock(due.Start))
		}
	}
	var hosts []string
	lists := map[string][]string{}
	for _, o := range append([]origin{it.from}, alsoOrigins(it)...) {
		host := connLabel(o.conn)
		if _, seen := lists[host]; !seen {
			hosts = append(hosts, host)
		}
		lists[host] = append(lists[host], collectionLabel(o.col))
	}
	for _, h := range hosts {
		line = append(line, h+" · "+strings.Join(lists[h], ", "))
	}
	switch {
	case percent != nil && *percent > 0:
		line = append(line, strconv.Itoa(*percent)+"%")
	case !due.Start.IsZero():
		if late := overdueBy(tf, due, done); late != "" {
			line = append(line, late)
		}
	}
	return text(day), text(strings.Join(line, " · "))
}

// alsoOrigins are where the other copies of a linked row lie.
func alsoOrigins(it listed) []origin {
	out := make([]origin, 0, len(it.also))
	for _, o := range it.also {
		out = append(out, o.from)
	}
	return out
}
