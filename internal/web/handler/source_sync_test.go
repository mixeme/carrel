// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/session"
)

func TestFormatRelativePast(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	if got := formatRelativePast(now.Add(-30*time.Second), now); got != "just now" {
		t.Fatalf("recent = %q", got)
	}
	if got := formatRelativePast(now.Add(-2*time.Minute), now); got != "2 min ago" {
		t.Fatalf("minutes = %q", got)
	}
	if got := formatRelativePast(now.Add(-3*24*time.Hour), now); got != "3 days ago" {
		t.Fatalf("days = %q", got)
	}
}

func TestParseCTagTime(t *testing.T) {
	if _, ok := parseCTagTime("1754000000"); !ok {
		t.Fatal("unix ctag should parse")
	}
	if _, ok := parseCTagTime("not-a-date"); ok {
		t.Fatal("opaque ctag should not parse as time")
	}
}

func TestCollectionMetaLabel(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	meta := session.CollectionMeta{
		FetchedAt:   now.Add(-2 * time.Minute),
		CTag:        "1754000000",
		ObjectCount: 2,
	}
	label := collectionMetaLabel(meta, now)
	if label == "" {
		t.Fatal("expected connections meta label")
	}
	if got := formatChangedLabel(meta.CTag, now); got == "" {
		t.Fatal("expected changed label from unix ctag")
	}
}
