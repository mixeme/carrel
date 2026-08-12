// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMarshalFoldsLongLines(t *testing.T) {
	photo := strings.Repeat("QUJDRA", 400)
	obj := mustParse(t, "BEGIN:VCARD\r\n"+
		"VERSION:3.0\r\n"+
		"UID:x\r\n"+
		"PHOTO;ENCODING=b;TYPE=JPEG:"+photo+"\r\n"+
		"END:VCARD\r\n")

	out, err := obj.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for i, line := range strings.Split(strings.TrimSuffix(string(out), "\r\n"), "\r\n") {
		if len(line) > foldLimit {
			t.Fatalf("line %d is %d octets, limit is %d: %q", i, len(line), foldLimit, line)
		}
	}
	if !strings.Contains(string(out), "\r\n ") {
		t.Fatal("nothing was folded")
	}
	if got := unfold(string(out)); !strings.Contains(got, "PHOTO;ENCODING=b;TYPE=JPEG:"+photo+"\r\n") {
		t.Error("unfolding does not restore the original line")
	}
}

// TestFoldKeepsRunesWhole checks that folding never cuts a multi-byte character
// in half, which would turn a name into replacement characters on every other
// client that reads the card.
func TestFoldKeepsRunesWhole(t *testing.T) {
	note := strings.Repeat("Привет мир ", 20)
	obj := mustParse(t, "BEGIN:VCARD\r\n"+
		"VERSION:3.0\r\n"+
		"UID:x\r\n"+
		"NOTE:"+note+"\r\n"+
		"END:VCARD\r\n")

	out, err := obj.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !utf8.Valid(out) {
		t.Fatal("folded card is not valid UTF-8")
	}
	for _, line := range strings.Split(string(out), "\r\n") {
		if !utf8.ValidString(line) {
			t.Fatalf("folded line is not valid UTF-8: %q", line)
		}
		if len(line) > foldLimit {
			t.Fatalf("line is %d octets: %q", len(line), line)
		}
	}
	reparsed, err := ParseVCard("/ab/x.vcf", "", out)
	if err != nil {
		t.Fatalf("ParseVCard: %v", err)
	}
	if got := reparsed.Property("NOTE")[0].Text; got != note {
		t.Errorf("NOTE did not survive folding:\n got %q\nwant %q", got, note)
	}
}

func TestShortLinesAreNotFolded(t *testing.T) {
	obj := mustParse(t, "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:x\r\nFN:Ada\r\nEND:VCARD\r\n")
	out, err := obj.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(out), "\r\n ") {
		t.Errorf("a short card was folded:\n%s", out)
	}
}
