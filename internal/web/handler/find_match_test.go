// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/model"
)

func TestEventMatchLabelPrefersAttendee(t *testing.T) {
	terms := matchTerms{emails: []string{"ada@example.org"}}
	ev := model.Event{
		Summary: "Budget meeting",
		Attendees: []model.Attendee{
			{URI: "mailto:ada@example.org", PartStat: "ACCEPTED"},
		},
	}
	if got := eventMatchLabel(ev, terms); got != "attendee · accepted" {
		t.Fatalf("label = %q, want attendee match", got)
	}
}

func TestEventMatchLabelFallsBackToText(t *testing.T) {
	terms := matchTerms{name: "Ada Lovelace"}
	ev := model.Event{Summary: "Call with Ada Lovelace"}
	if got := eventMatchLabel(ev, terms); got != "mentioned in the title" {
		t.Fatalf("label = %q, want text match", got)
	}
}

func TestTodoMatchLabelCombinesOverdue(t *testing.T) {
	terms := matchTerms{name: "invoice"}
	past := time.Now().Add(-48 * time.Hour)
	todo := model.Todo{Summary: "Chase the invoice", Due: past}
	if got := todoMatchLabel(todo, terms); got != "mentioned in the title · overdue" {
		t.Fatalf("label = %q", got)
	}
}

func TestAppendAttachmentRowsBuildsFilesTab(t *testing.T) {
	rows := []resultRow{{
		Kind: "event", Title: "Sync", Sort: "20260810",
		Files: []string{"cover.pdf"},
	}}
	out := appendAttachmentRows(rows)
	if len(out) != 2 || out[1].Kind != "file" || out[1].Title != "cover.pdf" {
		t.Fatalf("files tab rows = %+v", out)
	}
	if out[1].MatchLabel != "attached to «Sync»" {
		t.Fatalf("match label = %q", out[1].MatchLabel)
	}
}
