// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/fanout"
	"gitea.mixdep.ru/mix/carrel/internal/merge"
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
func contactItem(src fanout.Source, contact model.Contact, url string, marks contactMarks) fanout.Item {
	name := contact.DisplayName()
	row := resultRow{
		Kind: "contact", Title: displayOr(name, "(no name)"),
		Subtitle: contactSubtitle(contact, src), URL: url,
		GroupKey: initialOf(name), GroupLabel: initialOf(name),
		Sort: strings.ToLower(name), Account: src.AccountLabel,
		Collection: src.CollectionLabel, Color: src.Color,
		Tags: contact.Categories, DupCount: 1,
		Print: merge.FingerprintContact(contact), DupGroup: marks.Group,
		DupIgnored: marks.Ignored, Emails: valuesOf(contact.Emails),
		Phones: valuesOf(contact.Phones),
	}
	return fanout.Item{SourceID: src.ID, Key: row.Sort + "|" + src.ID, Data: row}
}

func valuesOf(values []model.LabeledValue) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if text := strings.TrimSpace(v.Value); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// collapseDuplicates folds the rows of one group into a single row with the
// badge of §15, keeping the position of its first record.
//
// A group is either one the person linked — a stored decision, already stamped on
// the rows — or one detected now from the fingerprints the rows carry. A pair
// decided against is never grouped, which is what makes "not duplicates" outlast
// the session it was clicked in (§21).
// dupDisplay is what a merged list needs to fold a group into one row: the score
// to do it at, and where the badge on it leads.
type dupDisplay struct {
	Threshold int
	URL       string
}

func collapseDuplicates(rows []resultRow, dup dupDisplay) []resultRow {
	if len(rows) < 2 {
		return rows
	}
	var pairs [][2]int
	linked := make(map[string][]int)
	for i, row := range rows {
		if row.DupGroup != "" {
			linked[row.DupGroup] = append(linked[row.DupGroup], i)
		}
	}
	for _, members := range linked {
		for i := 1; i < len(members); i++ {
			pairs = append(pairs, [2]int{members[0], members[i]})
		}
	}

	prints := make([]merge.Fingerprint, len(rows))
	for i, row := range rows {
		prints[i] = row.Print
	}
	detected := merge.Clusters(prints, merge.Options{
		Threshold: dup.Threshold,
		Skip: func(a, b int) bool {
			return sharesGroup(rows[a].DupIgnored, rows[b].DupIgnored)
		},
	})
	for _, cluster := range detected {
		for i := 1; i < len(cluster.Indexes); i++ {
			pairs = append(pairs, [2]int{cluster.Indexes[0], cluster.Indexes[i]})
		}
	}
	if len(pairs) == 0 {
		return rows
	}

	sets := merge.Sets(len(rows), pairs)
	setOf := make([]int, len(rows))
	for at, set := range sets {
		for _, member := range set {
			setOf[member] = at
		}
	}
	out := make([]resultRow, 0, len(rows))
	emitted := make([]bool, len(sets))
	for i, row := range rows {
		at := setOf[i]
		if len(sets[at]) < 2 {
			out = append(out, row)
			continue
		}
		if emitted[at] {
			continue
		}
		emitted[at] = true
		out = append(out, mergeRows(rows, sets[at], dup.URL))
	}
	return out
}

// mergeRows builds the one row a group is shown as: the leading record's name,
// the union of the repeatable fields, and the members underneath it.
func mergeRows(rows []resultRow, members []int, dupURL string) resultRow {
	row := rows[members[0]]
	row.DupCount = len(members)
	row.DupURL = dupURL
	row.Members = make([]resultRow, 0, len(members))
	row.DupLinked = true
	group := rows[members[0]].DupGroup
	emails, phones := stringSet{}, stringSet{}
	for _, at := range members {
		member := rows[at]
		if member.DupGroup == "" || member.DupGroup != group {
			row.DupLinked = false
		}
		emails.add(member.Emails...)
		phones.add(member.Phones...)
		member.DupCount = 1
		member.Members = nil
		row.Members = append(row.Members, member)
	}
	row.Emails, row.Phones = emails.values, phones.values
	row.Subtitle = collapsedSubtitle(row)
	return row
}

func collapsedSubtitle(row resultRow) string {
	parts := make([]string, 0, 3)
	if len(row.Phones) > 0 {
		parts = append(parts, strings.Join(row.Phones, ", "))
	}
	if len(row.Emails) > 0 {
		parts = append(parts, strings.Join(row.Emails, ", "))
	}
	sources := make([]string, 0, len(row.Members))
	for _, member := range row.Members {
		sources = append(sources, member.Collection)
	}
	parts = append(parts, strings.Join(sources, " + "))
	return strings.Join(parts, " · ")
}

func sharesGroup(a, b []string) bool {
	for _, left := range a {
		for _, right := range b {
			if left != "" && left == right {
				return true
			}
		}
	}
	return false
}

// stringSet keeps the first spelling of every value and the order they arrived
// in, which is what a merged field of §15 shows.
type stringSet struct {
	seen   map[string]bool
	values []string
}

func (s *stringSet) add(values ...string) {
	if s.seen == nil {
		s.seen = make(map[string]bool, len(values))
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || s.seen[key] {
			continue
		}
		s.seen[key] = true
		s.values = append(s.values, value)
	}
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
