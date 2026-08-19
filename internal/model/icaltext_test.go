// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"testing"
	"time"
)

// RFC 5545 §3.3.11 escapes have to come off on the way in. They did not, so a
// note written in jtx Board showed its line breaks as two characters and, on
// the next save, had them escaped a second time.
func TestUnescapeICalText(t *testing.T) {
	bs := string(rune(92))
	for _, tc := range []struct{ name, in, want string }{
		{"plain", "just text", "just text"},
		{"newline", "one" + bs + "ntwo", "one\ntwo"},
		{"capital newline", "one" + bs + "Ntwo", "one\ntwo"},
		{"comma", "Lovelace" + bs + ", Ada", "Lovelace, Ada"},
		{"semicolon", "a" + bs + ";b", "a;b"},
		{"backslash", "a" + bs + bs + "b", "a" + bs + "b"},
		{"bare comma survives", "one, two, three", "one, two, three"},
		{"unknown escape kept", "a" + bs + "qb", "a" + bs + "qb"},
		{"trailing backslash kept", "a" + bs, "a" + bs},
	} {
		if got := unescapeICalText(tc.in); got != tc.want {
			t.Errorf("%s: unescapeICalText(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// A description with an unescaped comma in it is common and legal enough in the
// wild that losing its tail would be worse than the escaping bug. Prop.Text
// would; this must not.
func TestNoteKeepsTextAfterABareComma(t *testing.T) {
	bs := string(rune(92))
	raw := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		"BEGIN:VJOURNAL\r\nUID:n1\r\nDTSTART;VALUE=DATE:20260818\r\n" +
		"SUMMARY:Shopping\r\nDESCRIPTION:milk, bread" + bs + "nand cheese\r\n" +
		"END:VJOURNAL\r\nEND:VCALENDAR\r\n"
	obj, err := ParseICal("/c/n1.ics", "", []byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	note, err := obj.Note(time.UTC)
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	if want := "milk, bread\nand cheese"; note.Description != want {
		t.Errorf("Description = %q, want %q", note.Description, want)
	}
}
