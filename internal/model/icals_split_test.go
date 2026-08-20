// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import "testing"

// What every client exports is one VCALENDAR holding the whole calendar. Read
// as a single object it kept the first component and lost the rest, so a 123
// event file imported one event.
func TestParseICalsSplitsAWholeCalendar(t *testing.T) {
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		"BEGIN:VTIMEZONE\r\nTZID:Europe/Moscow\r\nBEGIN:STANDARD\r\n" +
		"DTSTART:19700101T000000\r\nTZOFFSETFROM:+0300\r\nTZOFFSETTO:+0300\r\n" +
		"END:STANDARD\r\nEND:VTIMEZONE\r\n" +
		"BEGIN:VTIMEZONE\r\nTZID:Europe/Berlin\r\nBEGIN:STANDARD\r\n" +
		"DTSTART:19700101T000000\r\nTZOFFSETFROM:+0100\r\nTZOFFSETTO:+0100\r\n" +
		"END:STANDARD\r\nEND:VTIMEZONE\r\n" +
		"BEGIN:VEVENT\r\nUID:one\r\nDTSTAMP:20260101T000000Z\r\nDTSTART;TZID=Europe/Moscow:20260210T120000\r\nSUMMARY:First\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:two\r\nDTSTAMP:20260101T000000Z\r\nDTSTART;TZID=Europe/Moscow:20260211T120000\r\nSUMMARY:Second\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:two\r\nDTSTAMP:20260101T000000Z\r\nRECURRENCE-ID;TZID=Europe/Moscow:20260212T120000\r\n" +
		"DTSTART;TZID=Europe/Moscow:20260212T140000\r\nSUMMARY:Second, moved\r\nEND:VEVENT\r\n" +
		"BEGIN:VTODO\r\nUID:three\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:A task\r\nEND:VTODO\r\n" +
		"END:VCALENDAR\r\n"

	parsed := ParseICals([]byte(body))
	if len(parsed) != 3 {
		t.Fatalf("got %d objects, want 3 (two events and a task; the override belongs to its master)", len(parsed))
	}
	for i, p := range parsed {
		if p.Error != "" {
			t.Fatalf("object %d: %s", i, p.Error)
		}
	}

	uids := map[string]bool{}
	for _, p := range parsed {
		uids[p.Object.UID()] = true
	}
	for _, want := range []string{"one", "two", "three"} {
		if !uids[want] {
			t.Errorf("UID %q did not survive the split", want)
		}
	}

	// The override has to travel with its master, or one series becomes two.
	for _, p := range parsed {
		if p.Object.UID() != "two" {
			continue
		}
		raw, err := p.Object.Marshal()
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if n := countSub(string(raw), "BEGIN:VEVENT"); n != 2 {
			t.Errorf("the master and its override are in %d resources, want 1", n)
		}
		if !contains(string(raw), "TZID:Europe/Moscow") {
			t.Error("the time zone the entry names did not travel with it")
		}
		if contains(string(raw), "TZID:Europe/Berlin") {
			t.Error("an unrelated time zone was copied into the entry")
		}
	}

	if p := parsed[2]; p.Object.Component() != "VTODO" {
		t.Errorf("the task came out as %s; a calendar export carries tasks too", p.Object.Component())
	}
}

// One entry in one VCALENDAR is the other normal case — a single .ics saved
// from an invitation — and it must not be disturbed.
func TestParseICalsLeavesASingleEntryAlone(t *testing.T) {
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:only\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260210T120000Z\r\nSUMMARY:Alone\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	parsed := ParseICals([]byte(body))
	if len(parsed) != 1 {
		t.Fatalf("got %d objects, want 1", len(parsed))
	}
	if parsed[0].Object.UID() != "only" {
		t.Errorf("UID = %q", parsed[0].Object.UID())
	}
}

func countSub(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

func contains(s, sub string) bool { return countSub(s, sub) > 0 }
