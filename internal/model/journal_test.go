// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"
	"testing"
	"time"
)

func TestNoteReadsJournal(t *testing.T) {
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		"BEGIN:VJOURNAL\r\nUID:note-1\r\nDTSTAMP:20260101T090000Z\r\n" +
		"DTSTART;VALUE=DATE:20260212\r\nSUMMARY:Standup\r\n" +
		"DESCRIPTION:Agreed to ship on Friday.\\nAda takes the migration.\r\n" +
		"CATEGORIES:work,meeting\r\nRELATED-TO;RELTYPE=PARENT:event-9\r\n" +
		"X-JTX-COLOR:blue\r\nEND:VJOURNAL\r\nEND:VCALENDAR\r\n"
	obj, err := ParseICal("/cal/note-1.ics", `"v1"`, []byte(body))
	if err != nil {
		t.Fatalf("ParseICal: %v", err)
	}
	if obj.Component() != "VJOURNAL" {
		t.Fatalf("Component() = %q, want VJOURNAL", obj.Component())
	}
	note, err := obj.Note(time.UTC)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if note.UID != "note-1" || note.Summary != "Standup" {
		t.Errorf("note identity = %q/%q", note.UID, note.Summary)
	}
	if !note.DateOnly {
		t.Error("a DTSTART of VALUE=DATE did not come back as a date-only note")
	}
	if got := note.Date.Format("2006-01-02"); got != "2026-02-12" {
		t.Errorf("Date = %s, want 2026-02-12", got)
	}
	if len(note.Categories) != 2 || note.Categories[0] != "work" {
		t.Errorf("Categories = %v", note.Categories)
	}
	if len(note.Related) != 1 || note.Related[0].UID != "event-9" || note.Related[0].RelType != RelTypeParent {
		t.Errorf("Related = %+v", note.Related)
	}
	// The unknown property must survive as something to render, and must not
	// be mistaken for one of the fields the form owns (§8).
	found := false
	for _, prop := range note.Other {
		if prop.Name == "X-JTX-COLOR" {
			found = true
		}
	}
	if !found {
		t.Errorf("X-JTX-COLOR is missing from Other: %+v", note.Other)
	}
}

func TestNoteTitleFallsBackToFirstLine(t *testing.T) {
	note := Note{Description: "Ring the plumber\nbefore Thursday", UID: "u"}
	if got := note.DisplayTitle(); got != "Ring the plumber" {
		t.Errorf("DisplayTitle() = %q, want the first line", got)
	}
	if got := note.Excerpt(40); got != "before Thursday" {
		t.Errorf("Excerpt() = %q, want what follows the title", got)
	}
	// With a summary of its own the whole body is the preview.
	titled := Note{Summary: "Plumber", Description: "Ring the plumber\nbefore Thursday"}
	if got := titled.Excerpt(80); got != "Ring the plumber before Thursday" {
		t.Errorf("Excerpt() = %q", got)
	}
	if got := (Note{UID: "only-uid"}).DisplayTitle(); got != "only-uid" {
		t.Errorf("DisplayTitle() = %q, want the UID", got)
	}
}

func TestNewJournalAndTodoRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name      string
		component string
		build     func(string) (*Object, error)
	}{
		{"journal", "VJOURNAL", NewJournal},
		{"todo", "VTODO", NewTodo},
		{"event", "VEVENT", NewEvent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj, err := tc.build("uid-" + tc.name)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if obj.Component() != tc.component {
				t.Fatalf("Component() = %q, want %q", obj.Component(), tc.component)
			}
			if obj.UID() != "uid-"+tc.name {
				t.Errorf("UID() = %q", obj.UID())
			}
			raw, err := obj.Marshal()
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if !strings.Contains(string(raw), "BEGIN:"+tc.component) {
				t.Errorf("marshalled body has no %s:\n%s", tc.component, raw)
			}
			// A body this package wrote must parse back into the same kind of
			// object, or a create would be followed by an unreadable card.
			back, err := ParseICal(obj.Path, "", raw)
			if err != nil {
				t.Fatalf("ParseICal: %v", err)
			}
			if back.Component() != tc.component {
				t.Errorf("re-parsed component = %q", back.Component())
			}
		})
	}
}

func TestTodoReadsTask(t *testing.T) {
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		"BEGIN:VTODO\r\nUID:task-1\r\nDTSTAMP:20260101T090000Z\r\n" +
		"SUMMARY:File the return\r\nDUE:20260410T170000Z\r\n" +
		"STATUS:IN-PROCESS\r\nPRIORITY:2\r\nPERCENT-COMPLETE:40\r\n" +
		"CATEGORIES:admin\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	obj, err := ParseICal("/cal/task-1.ics", "", []byte(body))
	if err != nil {
		t.Fatalf("ParseICal: %v", err)
	}
	task, err := obj.Todo(time.UTC)
	if err != nil {
		t.Fatalf("Todo: %v", err)
	}
	if task.Summary != "File the return" || task.Status != TaskInProcess {
		t.Errorf("task = %+v", task)
	}
	if task.Priority != 2 || task.PercentComplete != 40 {
		t.Errorf("priority/percent = %d/%d", task.Priority, task.PercentComplete)
	}
	if task.DueDateOnly {
		t.Error("a DUE with a time came back as date-only")
	}
	if !task.Open() || task.Done() {
		t.Error("an in-process task must count as open")
	}
	if !task.Overdue(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("a task due in April is not overdue in May")
	}
	if task.Overdue(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("a task due in April must not be overdue in March")
	}
}

func TestTodoDoneAndSortOrder(t *testing.T) {
	due := time.Date(2026, 4, 10, 17, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		task   Todo
		done   bool
		bucket int
	}{
		{"completed status", Todo{Status: TaskCompleted, Due: due}, true, 3},
		{"completed date only", Todo{Completed: due}, true, 3},
		{"cancelled", Todo{Status: TaskCancelled}, false, 2},
		{"open with due", Todo{Status: TaskNeedsAction, Due: due}, false, 0},
		{"open without due", Todo{Status: TaskNeedsAction}, false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.task.Done() != tc.done {
				t.Errorf("Done() = %v, want %v", tc.task.Done(), tc.done)
			}
			if bucket, _ := tc.task.SortKey(); bucket != tc.bucket {
				t.Errorf("SortKey bucket = %d, want %d", bucket, tc.bucket)
			}
		})
	}
}

func TestNoteRejectsWrongComponent(t *testing.T) {
	obj, err := NewEvent("event-1")
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if _, err := obj.Note(time.UTC); err == nil {
		t.Error("an event was read as a note")
	}
	if _, err := obj.Todo(time.UTC); err == nil {
		t.Error("an event was read as a task")
	}
	card, err := ParseVCard("/c/1.vcf", "", []byte("BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Ada\r\nUID:1\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatalf("ParseVCard: %v", err)
	}
	if _, err := card.Note(time.UTC); err == nil {
		t.Error("a vCard was read as a note")
	}
}

func TestRelationsParseAndRender(t *testing.T) {
	relations := ParseRelations(" event-1 , , event-2 , event-1 ")
	if len(relations) != 2 {
		t.Fatalf("ParseRelations returned %d relations, want 2: %+v", len(relations), relations)
	}
	if relations[0].UID != "event-1" || relations[1].UID != "event-2" {
		t.Errorf("relations = %+v", relations)
	}
	values := RelationValues(relations)
	if len(values) != 2 || values[0].Text != "event-1" {
		t.Fatalf("RelationValues = %+v", values)
	}
	// PARENT is the default, so writing it out would only add noise a reader
	// has to interpret; anything else is written.
	if got := values[0].Params["RELTYPE"]; len(got) != 0 {
		t.Errorf("RELTYPE = %v, want it left off for the default", got)
	}
	child := RelationValues([]Relation{{UID: "task-3", RelType: RelTypeChild}})
	if got := child[0].Params["RELTYPE"]; len(got) != 1 || got[0] != RelTypeChild {
		t.Errorf("RELTYPE = %v, want CHILD", got)
	}
	if uids := RelationUIDs(relations); strings.Join(uids, ",") != "event-1,event-2" {
		t.Errorf("RelationUIDs = %v", uids)
	}
}
