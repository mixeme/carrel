// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// matchTerms is what a timeline poll looks for: normalised emails first, then
// the display name as a text fallback (§23.9).
type matchTerms struct {
	emails []string
	name   string
}

func newMatchTerms(subject timelineSubject) matchTerms {
	emails := make([]string, 0, len(subject.terms))
	for _, term := range subject.terms {
		if norm := model.NormalizeEmail(term); norm != "" {
			emails = append(emails, norm)
			continue
		}
	}
	name := strings.TrimSpace(subject.name)
	if name == "" && len(subject.terms) > 0 {
		name = strings.TrimSpace(subject.terms[len(subject.terms)-1])
	}
	return matchTerms{emails: emails, name: name}
}

func (t matchTerms) fromStrings(terms []string) matchTerms {
	var emails []string
	var name string
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		if norm := model.NormalizeEmail(term); norm != "" {
			emails = append(emails, norm)
		} else if name == "" {
			name = term
		}
	}
	return matchTerms{emails: emails, name: name}
}

func eventMatchLabel(ev model.Event, terms matchTerms) string {
	if label := attendeeMatchLabel(ev.Attendees, terms.emails); label != "" {
		return label
	}
	if textMention(ev.Summary, ev.Description, terms) {
		if strings.TrimSpace(ev.Summary) != "" && fieldMentions(ev.Summary, terms) {
			return "mentioned in the title"
		}
		return "mentioned in the text"
	}
	return ""
}

func todoMatchLabel(todo model.Todo, terms matchTerms) string {
	var parts []string
	switch {
	case strings.TrimSpace(todo.Summary) != "" && fieldMentions(todo.Summary, terms):
		parts = append(parts, "mentioned in the title")
	case textMention(todo.Summary, todo.Description, terms):
		parts = append(parts, "mentioned in the text")
	}
	if todo.Overdue(time.Now()) {
		parts = append(parts, "overdue")
	}
	return strings.Join(parts, " · ")
}

func noteMatchLabel(note model.Note, terms matchTerms) string {
	if note.Mentions(terms.emails) {
		return "mentioned in the text"
	}
	if textMention(note.Summary, note.Description, terms) {
		if strings.TrimSpace(note.Summary) != "" && fieldMentions(note.Summary, terms) {
			return "mentioned in the title"
		}
		return "mentioned in the text"
	}
	return ""
}

func contactSearchLabel(contact model.Contact, query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	q := strings.ToLower(query)
	for _, email := range contact.Emails {
		if strings.Contains(strings.ToLower(email.Value), q) {
			return email.Value
		}
	}
	for _, phone := range contact.Phones {
		if strings.Contains(strings.ToLower(phone.Value), q) {
			return phone.Value
		}
	}
	for _, org := range contact.Organization {
		if strings.Contains(strings.ToLower(org), q) {
			return org
		}
	}
	return ""
}

func attendeeMatchLabel(attendees []model.Attendee, emails []string) string {
	if len(emails) == 0 {
		return ""
	}
	want := make(map[string]bool, len(emails))
	for _, email := range emails {
		want[email] = true
	}
	for _, attendee := range attendees {
		norm := model.NormalizeEmail(attendee.URI)
		if norm == "" || !want[norm] {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(attendee.PartStat)) {
		case "ACCEPTED":
			return "attendee · accepted"
		case "DECLINED":
			return "attendee · declined"
		case "TENTATIVE":
			return "attendee · tentative"
		case "NEEDS-ACTION", "":
			return "attendee · no reply"
		default:
			return "attendee · " + strings.ToLower(attendee.PartStat)
		}
	}
	return ""
}

func textMention(summary, description string, terms matchTerms) bool {
	if fieldMentions(summary, terms) || fieldMentions(description, terms) {
		return true
	}
	for _, email := range terms.emails {
		hay := strings.ToLower(summary + "\n" + description)
		if strings.Contains(hay, email) {
			return true
		}
	}
	return false
}

func fieldMentions(field string, terms matchTerms) bool {
	field = strings.ToLower(strings.TrimSpace(field))
	if field == "" {
		return false
	}
	if terms.name != "" && strings.Contains(field, strings.ToLower(terms.name)) {
		return true
	}
	for _, email := range terms.emails {
		if strings.Contains(field, email) {
			return true
		}
	}
	return false
}

func attachmentMatchLabel(parentTitle string) string {
	title := strings.TrimSpace(parentTitle)
	if title == "" {
		return "attachment"
	}
	return "attached to «" + title + "»"
}

func filterRowsByTab(req findRequest, rows []resultRow) []resultRow {
	tab := strings.ToLower(strings.TrimSpace(req.Tab))
	if tab == "" || tab == "all" {
		return rows
	}
	out := make([]resultRow, 0, len(rows))
	for _, row := range rows {
		if rowMatchesTab(req.Mode, tab, row.Kind) {
			out = append(out, row)
		}
	}
	return out
}

func rowMatchesTab(mode findMode, tab, kind string) bool {
	switch tab {
	case "contacts", "contact":
		return kind == "contact"
	case "events", "event":
		return kind == "event"
	case "tasks", "task":
		return kind == "task"
	case "notes", "note":
		return kind == "note"
	case "files", "file":
		return kind == "file"
	default:
		return true
	}
}

func appendAttachmentRows(rows []resultRow) []resultRow {
	if len(rows) == 0 {
		return rows
	}
	out := make([]resultRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
		if row.Kind == "file" || len(row.Files) == 0 {
			continue
		}
		for _, name := range row.Files {
			fileRow := row
			fileRow.Kind = "file"
			fileRow.Title = name
			fileRow.MatchLabel = attachmentMatchLabel(row.Title)
			fileRow.Subtitle = ""
			fileRow.URL = ""
			fileRow.Files = nil
			fileRow.Sort = row.Sort + "|file|" + name
			out = append(out, fileRow)
		}
	}
	return out
}

// KindTabs builds the segment control for a fan-out screen (§1.8).
func (v findView) KindTabs() []kindTab {
	counts := map[string]int{"all": 0}
	for _, group := range v.Groups {
		for _, row := range group.Rows {
			counts["all"]++
			counts[row.Kind]++
		}
	}
	var specs []struct{ label, value string }
	switch v.Mode {
	case modeSearch:
		specs = []struct{ label, value string }{
			{"All", "all"}, {"Contacts", "contacts"}, {"Events", "events"},
			{"Tasks", "tasks"}, {"Notes", "notes"},
		}
	case modeTimeline:
		specs = []struct{ label, value string }{
			{"All", "all"}, {"Events", "events"}, {"Tasks", "tasks"},
			{"Notes", "notes"}, {"Files", "files"},
		}
	case modeDuplicates:
		specs = []struct{ label, value string }{
			{"All", "all"}, {"Contacts", "contacts"}, {"Events", "events"},
			{"Tasks", "tasks"}, {"Notes", "notes"},
		}
	default:
		return nil
	}
	active := strings.ToLower(strings.TrimSpace(v.Request.Tab))
	if active == "" {
		active = "all"
	}
	tabs := make([]kindTab, 0, len(specs))
	for _, spec := range specs {
		count := counts[spec.value]
		if spec.value == "contacts" {
			count = counts["contact"]
		}
		tab := kindTab{
			Label:  spec.label,
			Value:  spec.value,
			Count:  count,
			Active: spec.value == active,
		}
		req := v.Request
		if spec.value == "all" {
			req.Tab = ""
		} else {
			req.Tab = spec.value
		}
		tab.URL = v.tabURL(req)
		tabs = append(tabs, tab)
	}
	return tabs
}

func (v findView) tabURL(req findRequest) string {
	switch v.Mode {
	case modeSearch:
		return v.Base + "/app/search?" + req.values().Encode()
	case modeDuplicates:
		q := req.values().Encode()
		if q == "" {
			return v.Base + "/app/duplicates"
		}
		return v.Base + "/app/duplicates?" + q
	case modeTimeline:
		return v.Base + "/app/contacts/" + req.Account + "/" + EncodeCollectionPath(req.Collection) + "/" + urlPathEscape(req.UID) + "?" + req.values().Encode()
	default:
		return ""
	}
}
