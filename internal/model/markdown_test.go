// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRenderMarkdownFrontMatter(t *testing.T) {
	note := Note{
		UID: "note-1", Summary: "Ship: the plan", Description: "First line.\n\nSecond line.\n",
		Categories: []string{"work", "work", "planning"},
		Date:       time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC), DateOnly: true,
		Related: []Relation{{UID: "event-9", RelType: RelTypeParent}},
		Other:   []Property{{Name: "X-JTX-COLOR", Values: []Value{Text("blue")}}},
	}
	out := string(RenderMarkdown(note, MarkdownSource{Account: "Work", Collection: "Journal"}))
	for _, want := range []string{
		"---\n",
		`title: "Ship: the plan"`, // a colon would otherwise change the meaning
		"date: 2026-02-12",
		"uid: note-1",
		"tags:\n  - work\n  - planning\n", // deduplicated, order kept
		"related:\n  - event-9\n",
		"account: Work",
		"collection: Journal",
		"carrel_properties:\n",
		"X-JTX-COLOR: blue",
		"First line.\n\nSecond line.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered note lacks %q:\n%s", want, out)
		}
	}
}

func TestMarkdownRoundTrip(t *testing.T) {
	note := Note{
		UID: "note-2", Summary: "Standup", Description: "Ada takes the migration.",
		Categories: []string{"work"},
		Date:       time.Date(2026, 2, 12, 9, 30, 0, 0, time.UTC),
		Related:    []Relation{{UID: "event-9", RelType: RelTypeParent}},
		Other:      []Property{{Name: "X-JTX-COLOR", Values: []Value{Text("blue")}}},
	}
	body := RenderMarkdown(note, MarkdownSource{Account: "Work", Collection: "Journal"})
	parsed, err := ParseMarkdown("2026-02-12-standup.md", body, time.Time{})
	if err != nil {
		t.Fatalf("ParseMarkdown: %v", err)
	}
	if parsed.Title != "Standup" || parsed.UID != "note-2" {
		t.Errorf("parsed identity = %q/%q", parsed.Title, parsed.UID)
	}
	if parsed.Body != "Ada takes the migration." {
		t.Errorf("parsed body = %q", parsed.Body)
	}
	if !parsed.HasDate || !parsed.Date.Equal(note.Date) {
		t.Errorf("parsed date = %v (has = %v), want %v", parsed.Date, parsed.HasDate, note.Date)
	}
	if len(parsed.Tags) != 1 || parsed.Tags[0] != "work" {
		t.Errorf("parsed tags = %v", parsed.Tags)
	}
	if len(parsed.Related) != 1 || parsed.Related[0].UID != "event-9" {
		t.Errorf("parsed relations = %+v", parsed.Related)
	}
	if len(parsed.Extra) != 1 || parsed.Extra[0].Name != "X-JTX-COLOR" {
		t.Fatalf("parsed extras = %+v", parsed.Extra)
	}

	// The whole point of the extras is that a note exported and imported again
	// still carries what this build does not render (§8, §23.9).
	obj, err := NewJournal("note-2")
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	if err := obj.Apply(parsed.Patch(time.UTC)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	back, err := obj.Note(time.UTC)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if back.Summary != "Standup" || back.Description != "Ada takes the migration." {
		t.Errorf("re-imported note = %+v", back)
	}
	if len(back.Related) != 1 || back.Related[0].UID != "event-9" {
		t.Errorf("re-imported relations = %+v", back.Related)
	}
	found := false
	for _, prop := range back.Other {
		if prop.Name == "X-JTX-COLOR" && len(prop.Values) == 1 && prop.Values[0].Text == "blue" {
			found = true
		}
	}
	if !found {
		t.Errorf("the extra property did not survive the round trip: %+v", back.Other)
	}
}

func TestParseMarkdownWithoutFrontMatter(t *testing.T) {
	mtime := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	note, err := ParseMarkdown("notes/2026-01-05 Kitchen.md", []byte("# Kitchen\n\nMeasure the alcove.\n"), mtime)
	if err != nil {
		t.Fatalf("ParseMarkdown: %v", err)
	}
	if note.Title != "Kitchen" {
		t.Errorf("Title = %q, want the heading", note.Title)
	}
	if note.Body != "Measure the alcove." {
		t.Errorf("Body = %q, want the heading removed", note.Body)
	}
	// A date in the file name is better evidence than the file's own mtime,
	// which is only when the copy happened to be written.
	if !note.HasDate || note.Date.Format("2006-01-02") != "2026-01-05" || !note.DateOnly {
		t.Errorf("Date = %v (dateOnly = %v)", note.Date, note.DateOnly)
	}

	plain, err := ParseMarkdown("scratch.txt", []byte("Buy milk.\n"), mtime)
	if err != nil {
		t.Fatalf("ParseMarkdown: %v", err)
	}
	if plain.Title != "scratch" {
		t.Errorf("Title = %q, want the file name", plain.Title)
	}
	if !plain.Date.Equal(mtime) {
		t.Errorf("Date = %v, want the mtime %v", plain.Date, mtime)
	}
	if _, err := ParseMarkdown("empty.md", []byte("   \n"), mtime); err == nil {
		t.Error("an empty file was accepted")
	}
}

func TestParseMarkdownUnclosedFrontMatterIsBody(t *testing.T) {
	note, err := ParseMarkdown("odd.md", []byte("---\ntitle: Half\n\nbody text\n"), time.Time{})
	if err != nil {
		t.Fatalf("ParseMarkdown: %v", err)
	}
	// Without a closing fence there is no front matter, so nothing may be
	// silently eaten: the text stays the note.
	if !strings.Contains(note.Body, "title: Half") {
		t.Errorf("Body = %q, want the whole file", note.Body)
	}
}

func TestMarkdownFilenameIsUniqueAndTransliterated(t *testing.T) {
	taken := map[string]bool{}
	date := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)
	first := MarkdownFilename(Note{UID: "a", Summary: "Договор", Date: date}, taken)
	if first != "2026-02-12-dogovor.md" {
		t.Errorf("filename = %q, want the transliterated form", first)
	}
	second := MarkdownFilename(Note{UID: "b", Summary: "Договор", Date: date}, taken)
	if second == first {
		t.Fatalf("two notes were given the same file name %q", first)
	}
	if !strings.HasPrefix(second, "2026-02-12-dogovor-") {
		t.Errorf("second filename = %q", second)
	}
	third := MarkdownFilename(Note{Summary: "?!", Date: time.Time{}}, taken)
	if third != "note.md" {
		t.Errorf("unnameable note = %q, want note.md", third)
	}
}

func TestReadMarkdownImportPayloadZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range []struct{ name, body string }{
		{"notes/one.md", "---\ntitle: One\nuid: u1\n---\n\nFirst.\n"},
		{"notes/two.md", "Second.\n"},
		{"notes/skip.png", "not a note"},
		{"notes/empty.md", "  "},
	} {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatalf("Create %s: %v", f.name, err)
		}
		if _, err := w.Write([]byte(f.body)); err != nil {
			t.Fatalf("Write %s: %v", f.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	parsed, err := ReadMarkdownImportPayload("notes.zip", buf.Bytes(), 100)
	if err != nil {
		t.Fatalf("ReadMarkdownImportPayload: %v", err)
	}
	if len(parsed) != 3 {
		t.Fatalf("read %d entries, want 3 (the image skipped): %+v", len(parsed), parsed)
	}
	var errors int
	for _, item := range parsed {
		if item.Error != "" {
			errors++
		}
	}
	// The unreadable file is reported rather than failing the batch.
	if errors != 1 {
		t.Errorf("got %d reported errors, want 1", errors)
	}

	truncated, err := ReadMarkdownImportPayload("notes.zip", buf.Bytes(), 1)
	if err == nil {
		t.Error("a batch over the ceiling was accepted silently")
	}
	if len(truncated) != 1 {
		t.Errorf("truncated to %d notes, want 1", len(truncated))
	}
}

func TestReadMarkdownImportPayloadSingleFile(t *testing.T) {
	parsed, err := ReadMarkdownImportPayload("note.md", []byte("Just a line.\n"), 10)
	if err != nil {
		t.Fatalf("ReadMarkdownImportPayload: %v", err)
	}
	if len(parsed) != 1 || parsed[0].Error != "" || parsed[0].Note.Body != "Just a line." {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestMarkdownPatchCannotRewriteIdentity(t *testing.T) {
	// A front matter block is a file from anywhere. Even if an extra property
	// named UID reached a patch, applying it must fail rather than turn one
	// note into another (§8).
	note := MarkdownNote{
		Title: "Trying it on", HasDate: true, DateOnly: true,
		Date:  time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		Extra: []Property{{Name: "UID", Values: []Value{Text("hijacked")}}},
	}
	obj, err := NewJournal("real-uid")
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	if err := obj.Apply(note.Patch(time.UTC)); err == nil {
		t.Fatal("a patch carrying UID was applied")
	}
	if obj.UID() != "real-uid" {
		t.Errorf("UID = %q, want the object's own", obj.UID())
	}
}

func TestParseMarkdownIgnoresProtectedExtras(t *testing.T) {
	body := "---\ntitle: T\ncarrel_properties:\n  - \"UID: hijacked\"\n  - \"X-KEEP: yes\"\n---\n\nBody.\n"
	note, err := ParseMarkdown("t.md", []byte(body), time.Time{})
	if err != nil {
		t.Fatalf("ParseMarkdown: %v", err)
	}
	for _, prop := range note.Extra {
		if prop.Name == "UID" {
			t.Errorf("UID was accepted as an extra property: %+v", note.Extra)
		}
	}
	if len(note.Extra) != 1 || note.Extra[0].Name != "X-KEEP" {
		t.Errorf("Extra = %+v, want only X-KEEP", note.Extra)
	}
}
