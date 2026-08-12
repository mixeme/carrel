// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import "testing"

func TestDiffReportsPropertyChanges(t *testing.T) {
	local, err := ParseVCard("local.vcf", `"1"`, []byte(
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:ada\r\nFN:Ada Lovelace\r\nTEL:+1\r\nX-CUSTOM:mine\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	remote, err := ParseVCard("remote.vcf", `"2"`, []byte(
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:ada\r\nFN:Ada Byron\r\nEMAIL:ada@example.org\r\nX-CUSTOM:theirs\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatal(err)
	}

	lines, err := Diff(local, remote)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]DiffLine{}
	for _, line := range lines {
		byName[line.Name] = line
	}
	if line, ok := byName["FN"]; !ok || line.Kind != DiffChanged {
		t.Fatalf("FN = %+v", line)
	}
	if line, ok := byName["TEL"]; !ok || line.Kind != DiffOnlyLocal {
		t.Fatalf("TEL = %+v", line)
	}
	if line, ok := byName["EMAIL"]; !ok || line.Kind != DiffOnlyRemote {
		t.Fatalf("EMAIL = %+v", line)
	}
	if line, ok := byName["X-CUSTOM"]; !ok || line.Kind != DiffChanged {
		t.Fatalf("X-CUSTOM = %+v", line)
	}
}

func TestDiffIgnoresVolatileProperties(t *testing.T) {
	local, err := ParseVCard("a.vcf", `"1"`, []byte(
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:x\r\nFN:A\r\nREV:20260101T000000Z\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	remote, err := ParseVCard("b.vcf", `"2"`, []byte(
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:x\r\nFN:A\r\nREV:20260202T000000Z\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	lines, err := Diff(local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatalf("diff = %+v, want empty", lines)
	}
}
