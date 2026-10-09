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

// filterNotes keeps the notes whose title or text holds the word (s-notes-1,
// «Filter by word»), as the client sifts them with scripts.
func filterNotes(items []listed, filter string) []listed {
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
func combinedNotebooks(rail view.Rail, spec kindSpec, done bool) string {
	ticked, _, count := combinedParts(rail, spec, done)
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
// is, Starts, Complete and Priority monospaced, Reminders, Where, Link,
// Tags. The panel (s-tasks-1) has the card's order and no Reminders.
func taskFields(tf timeFmt, obj *model.Object, panel bool, tags []view.Tag) ([]view.Field, error) {
	status := taskStatuses[0].label
	if v := strings.TrimSpace(firstValue(obj, "STATUS")); v != "" {
		status = statusLabel(v)
	}
	out := []view.Field{{Label: "Status", Value: text(status)}}
	due, err := cal.DueOf(obj, tf.tz)
	if err == nil && !due.Start.IsZero() {
		f := view.Field{Label: "Due", Runs: view.Runs{{Text: text(tf.timeOrDate(due)), Mark: view.RunMono}}, Badge: tf.createdIn(due)}
		if late := overdueBy(tf, due, taskDone(obj)); late != "" {
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
	if !panel {
		alarms, _ := cal.Alarms(obj)
		if rows := reminderRows(alarms, tf); len(rows) > 0 {
			labels := make([]string, 0, len(rows))
			for _, r := range rows {
				labels = append(labels, string(r.Label))
			}
			out = append(out, view.Field{Label: "Reminders", Value: text(strings.Join(labels, ", "))})
		}
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

// taskAttachHint is the line under the attachments of a task's panel
// (s-tasks-1): up to the limit the file goes into the task, above it into
// the attachment folder (S-20) — the limit is the instance's.
func taskAttachHint(c *collectionCtx) view.Text {
	return text("The same block events and notes have: up to " + sizeText(attachLimit(c.col)) +
		" the file is written into the task itself, above that it goes to the attachments folder and the task keeps a link. Pasting and dropping work the same way.")
}
