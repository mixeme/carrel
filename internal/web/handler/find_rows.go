// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/fanout"
	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// The row builders below are the only place that decides what a merged record
// looks like. They run on a source's own goroutine, inside the fan-out, so a row
// is finished before it is ever handed to a template (§16).

// dayGroup is the grouping every dated row uses: one heading per day, with the
// key sortable and the label readable.
func dayGroup(at time.Time, loc *time.Location) (string, string) {
	if at.IsZero() {
		return "zzzz", "No date"
	}
	local := at.In(loc)
	label := local.Format("Monday, 2 January 2006")
	today := time.Now().In(loc)
	switch {
	case sameDay(local, today):
		label = "Today · " + local.Format("2 January")
	case sameDay(local, today.AddDate(0, 0, 1)):
		label = "Tomorrow · " + local.Format("2 January")
	case sameDay(local, today.AddDate(0, 0, -1)):
		label = "Yesterday · " + local.Format("2 January")
	}
	return local.Format("20060102"), label
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func sortKey(at time.Time, tiebreak string) string {
	if at.IsZero() {
		return "9999" + tiebreak
	}
	return at.UTC().Format("20060102T150405") + "|" + tiebreak
}

func sourceSubtitle(src fanout.Source) string {
	if src.AccountLabel == "" {
		return src.CollectionLabel
	}
	return src.CollectionLabel + " · " + src.AccountLabel
}

func occurrenceItem(src fanout.Source, occ model.Occurrence, loc *time.Location) fanout.Item {
	key, label := dayGroup(occ.Start, loc)
	row := resultRow{
		Kind: "event", Title: displayOr(occ.Summary, "(no title)"),
		Subtitle: sourceSubtitle(src), GroupKey: key, GroupLabel: label,
		Sort: sortKey(occ.Start, occ.Summary), Account: src.AccountLabel,
		Collection: src.CollectionLabel, Color: src.Color,
		TimeLabel: occurrenceTime(occ, loc),
	}
	if occ.Location != "" {
		row.Subtitle = occ.Location + " · " + row.Subtitle
	}
	return fanout.Item{SourceID: src.ID, Key: row.Sort, Data: row}
}

func occurrenceTime(occ model.Occurrence, loc *time.Location) string {
	if occ.AllDay {
		return "All day"
	}
	label := occ.Start.In(loc).Format("15:04")
	if !occ.End.IsZero() && occ.End.After(occ.Start) {
		label += "–" + occ.End.In(loc).Format("15:04")
	}
	return label
}

// eventItem is the search-result form of an event: the same row, but keyed on
// the event's own start rather than on an expanded occurrence.
func eventItem(src fanout.Source, event model.Event, url string, loc *time.Location) fanout.Item {
	key, label := dayGroup(event.Start, loc)
	row := resultRow{
		Kind: "event", Title: event.DisplayTitle(), Subtitle: sourceSubtitle(src),
		GroupKey: key, GroupLabel: label, Sort: sortKey(event.Start, event.Summary),
		URL: url, Account: src.AccountLabel, Collection: src.CollectionLabel,
		Color: src.Color, Tags: event.Categories,
	}
	if !event.Start.IsZero() {
		if event.AllDay {
			row.TimeLabel = "All day"
		} else {
			row.TimeLabel = event.Start.In(loc).Format("15:04")
		}
	}
	return fanout.Item{SourceID: src.ID, Key: row.Sort, Data: row}
}

func todoItem(src fanout.Source, todo model.Todo, loc *time.Location) fanout.Item {
	key, label := dayGroup(todo.Due, loc)
	row := resultRow{
		Kind: "task", Title: todo.DisplayTitle(), Subtitle: sourceSubtitle(src),
		GroupKey: key, GroupLabel: label, Sort: sortKey(todo.Due, todo.Summary),
		Account: src.AccountLabel, Collection: src.CollectionLabel,
		Color: src.Color, Tags: todo.Categories,
		Done: todo.Done(), Overdue: todo.Overdue(time.Now()),
	}
	switch {
	case todo.Due.IsZero():
		row.TimeLabel = "No due date"
	case todo.DueDateOnly:
		row.TimeLabel = "Due"
	default:
		row.TimeLabel = "Due " + todo.Due.In(loc).Format("15:04")
	}
	return fanout.Item{SourceID: src.ID, Key: row.Sort, Data: row}
}

func noteItem(src fanout.Source, note model.Note, loc *time.Location) fanout.Item {
	key, label := dayGroup(note.Date, loc)
	row := resultRow{
		Kind: "note", Title: note.DisplayTitle(), Subtitle: note.Excerpt(120),
		GroupKey: key, GroupLabel: label, Sort: sortKey(note.Date, note.Summary),
		Account: src.AccountLabel, Collection: src.CollectionLabel,
		Color: src.Color, Tags: note.Categories,
	}
	if row.Subtitle == "" {
		row.Subtitle = sourceSubtitle(src)
	}
	if !note.Date.IsZero() && !note.DateOnly {
		row.TimeLabel = note.Date.In(loc).Format("15:04")
	}
	return fanout.Item{SourceID: src.ID, Key: row.Sort, Data: row}
}

// contactItem groups people by initial, which is the natural key §14 asks for
// when the merged list is a directory rather than an agenda.
func contactItem(src fanout.Source, contact model.Contact, url string) fanout.Item {
	name := contact.DisplayName()
	row := resultRow{
		Kind: "contact", Title: displayOr(name, "(no name)"),
		Subtitle: contactSubtitle(contact, src), URL: url,
		GroupKey: initialOf(name), GroupLabel: initialOf(name),
		Sort: strings.ToLower(name), Account: src.AccountLabel,
		Collection: src.CollectionLabel, Color: src.Color,
		Tags: contact.Categories,
	}
	return fanout.Item{SourceID: src.ID, Key: row.Sort + "|" + src.ID, Data: row}
}

func contactSubtitle(contact model.Contact, src fanout.Source) string {
	parts := make([]string, 0, 3)
	for _, org := range contact.Organization {
		if org = strings.TrimSpace(org); org != "" {
			parts = append(parts, org)
			break
		}
	}
	if emails := contact.NormalizedEmails(); len(emails) > 0 {
		parts = append(parts, emails[0])
	}
	parts = append(parts, sourceSubtitle(src))
	return strings.Join(parts, " · ")
}

func initialOf(name string) string {
	for _, r := range strings.TrimSpace(name) {
		return strings.ToUpper(string(r))
	}
	return "#"
}
