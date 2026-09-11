package model

import (
	"testing"
)

func TestCalendarPatchMainOnly(t *testing.T) {
	obj := NewCalendarObject("/c/e.ics", "e1", "VEVENT", map[string][]RawValue{
		"SUMMARY": {{Text: "A"}},
		"UID":     {{Text: "u1"}},
	}, []byte("BEGIN:VCALENDAR\nEND:VCALENDAR\n"))
	patch, err := NewPatch(SetText{Prop: "SUMMARY", Value: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	if obj.PropText("SUMMARY")[0] != "B" {
		t.Fatal("patch not applied")
	}
	if !obj.IsCalendar() {
		t.Fatal("not calendar")
	}
}
