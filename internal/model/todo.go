// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// Task status values used by the interface (RFC 5545 §3.8.1.11).
const (
	TaskNeedsAction = "NEEDS-ACTION"
	TaskInProcess   = "IN-PROCESS"
	TaskCompleted   = "COMPLETED"
	TaskCancelled   = "CANCELLED"
)

// Todo is the display view of a VTODO (§10). One-way, like Event and Note.
type Todo struct {
	UID             string
	Summary         string
	Description     string
	Location        string
	Status          string
	Categories      []string
	Start           time.Time
	Due             time.Time
	Completed       time.Time
	DueDateOnly     bool
	Priority        int
	PercentComplete int
	// Modified is LAST-MODIFIED, read for the "Recently changed" sort of
	// 2.6.B2. It is already in knownTodoProps — excluded from Other because
	// it was always meant to be read on its own rather than shown raw — so
	// giving it a field costs nothing the parser was not already doing.
	Modified    time.Time
	Related     []Relation
	Attachments []Attachment
	Other       []Property
}

var knownTodoProps = map[string]bool{
	ical.PropUID:             true,
	ical.PropSummary:         true,
	ical.PropDescription:     true,
	ical.PropLocation:        true,
	ical.PropStatus:          true,
	ical.PropCategories:      true,
	ical.PropDateTimeStart:   true,
	ical.PropDue:             true,
	ical.PropCompleted:       true,
	ical.PropPercentComplete: true,
	ical.PropPriority:        true,
	ical.PropDateTimeStamp:   true,
	ical.PropCreated:         true,
	ical.PropLastModified:    true,
	ical.PropSequence:        true,
	ical.PropClass:           true,
	ical.PropRelatedTo:       true,
	ical.PropAttach:          true,
}

// Todo returns the display view of a task object.
func (o *Object) Todo(loc *time.Location) (Todo, error) {
	if o == nil || o.kind != KindICal {
		return Todo{}, ErrNotICal
	}
	comp := o.primaryComponent()
	if comp == nil || strings.ToUpper(comp.Name) != ical.CompToDo {
		return Todo{}, ErrNotTodo
	}
	if loc == nil {
		loc = time.Local
	}
	task := Todo{
		UID:         o.UID(),
		Summary:     icalPropText(comp.Props, ical.PropSummary),
		Description: icalPropText(comp.Props, ical.PropDescription),
		Location:    icalPropText(comp.Props, ical.PropLocation),
		Status:      strings.ToUpper(icalPropRaw(comp.Props, ical.PropStatus)),
		Related:     relationsFrom(comp.Props),
		Attachments: attachmentsFrom(comp.Props),
	}
	if cats := comp.Props.Get(ical.PropCategories); cats != nil {
		task.Categories = splitComma(cats.Value)
	}
	if start, err := comp.Props.DateTime(ical.PropDateTimeStart, loc); err == nil {
		task.Start = start
	}
	if p := comp.Props.Get(ical.PropDue); p != nil {
		if due, err := p.DateTime(loc); err == nil {
			task.Due = due
		}
		task.DueDateOnly = p.ValueType() == ical.ValueDate
	}
	if done, err := comp.Props.DateTime(ical.PropCompleted, loc); err == nil {
		task.Completed = done
	}
	task.Priority = atoiOr(icalPropRaw(comp.Props, ical.PropPriority), 0)
	task.PercentComplete = atoiOr(icalPropRaw(comp.Props, ical.PropPercentComplete), 0)
	if modified, err := comp.Props.DateTime(ical.PropLastModified, loc); err == nil {
		task.Modified = modified
	}
	for _, name := range o.Names() {
		if knownTodoProps[name] {
			continue
		}
		task.Other = append(task.Other, Property{Name: name, Values: o.Property(name)})
	}
	return task, nil
}

// DisplayTitle is SUMMARY, or a fallback when the task has none.
func (t Todo) DisplayTitle() string {
	if s := strings.TrimSpace(t.Summary); s != "" {
		return s
	}
	if line := firstLine(t.Description); line != "" {
		return line
	}
	if t.UID != "" {
		return t.UID
	}
	return "(untitled task)"
}

// Done reports whether the task is finished.
func (t Todo) Done() bool {
	return t.Status == TaskCompleted || (!t.Completed.IsZero() && t.Status == "")
}

// Open reports whether the task still wants doing: neither completed nor
// cancelled.
func (t Todo) Open() bool { return !t.Done() && t.Status != TaskCancelled }

// Overdue reports whether an open task's DUE has passed.
func (t Todo) Overdue(now time.Time) bool {
	return t.Open() && !t.Due.IsZero() && t.Due.Before(now)
}

// SortKey orders a task list: open before closed, then by due date, with tasks
// carrying no due date after those that do.
func (t Todo) SortKey() (int, time.Time) {
	bucket := 0
	switch {
	case t.Done():
		bucket = 3
	case t.Status == TaskCancelled:
		bucket = 2
	case t.Due.IsZero():
		bucket = 1
	}
	return bucket, t.Due
}

func atoiOr(text string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return fallback
	}
	return n
}
