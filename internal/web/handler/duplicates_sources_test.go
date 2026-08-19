// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
)

// §15 scores loaded records, and a plain WebDAV folder has none. Offering one
// as a source got it the calendar-query every other source gets, which the
// server answers with 400 — so the duplicates screen showed a permanent
// "Unavailable" it could do nothing about. Search is the mode that does want
// file collections, and it keeps them.
func TestDuplicatesDoNotPollFileCollections(t *testing.T) {
	h := startDAVHost(t)
	a := filesApp(t, h)
	sess := a.session()

	dups, err := a.findSources(sess, findRequest{Mode: modeDuplicates})
	if err != nil {
		t.Fatalf("duplicate sources: %v", err)
	}
	if len(dups) == 0 {
		t.Fatal("duplicates has no sources at all")
	}
	for _, row := range dups {
		if row.Kind == discovery.KindFiles {
			t.Errorf("duplicates offers the file collection %q as a source", row.Label())
		}
	}

	found, err := a.findSources(sess, findRequest{Mode: modeSearch})
	if err != nil {
		t.Fatalf("search sources: %v", err)
	}
	var files int
	for _, row := range found {
		if row.Kind == discovery.KindFiles {
			files++
		}
	}
	if files == 0 {
		t.Error("search no longer spans file collections; it searches filenames and should")
	}
}
