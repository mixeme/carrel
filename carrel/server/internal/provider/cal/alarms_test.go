package cal

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

type modelObject = model.Object

func eventFixture(t *testing.T) *model.Object {
	t.Helper()
	data, err := os.ReadFile("testdata/event.ics")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := Parse("/c/e.ics", "e1", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

func encoded(t *testing.T, obj *model.Object) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// A kept alarm is written back as the other client wrote it; a new one is
// added; the rest of the document stays.
func TestSetAlarmsKeepsExistingAndAdds(t *testing.T) {
	obj := eventFixture(t)
	alarms, err := Alarms(obj)
	if err != nil {
		t.Fatal(err)
	}
	if len(alarms) != 1 || alarms[0].Trigger != "-PT10M" || alarms[0].Description != "Ping" {
		t.Fatalf("alarms = %+v", alarms)
	}
	next, err := SetAlarms(obj, []Alarm{alarms[0], NewAlarm("-PT1H")})
	if err != nil {
		t.Fatal(err)
	}
	if next.ETag != "e1" || next.Path != "/c/e.ics" {
		t.Errorf("identity lost: %q %q", next.Path, next.ETag)
	}
	got := encoded(t, next)
	for _, want := range []string{"TRIGGER:-PT10M", "DESCRIPTION:Ping", "TRIGGER:-PT1H", "BEGIN:VTIMEZONE", "SUMMARY:Team sync"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if n := strings.Count(got, "BEGIN:VALARM"); n != 2 {
		t.Errorf("VALARM count = %d", n)
	}
	again, _ := Alarms(next)
	if len(again) != 2 {
		t.Errorf("re-read alarms = %+v", again)
	}
}

// A patch applied before SetAlarms survives it: the set is written onto the
// patched record, not onto the one that arrived.
func TestSetAlarmsCarriesPatchedProps(t *testing.T) {
	obj := eventFixture(t)
	patch, _ := model.NewPatch(model.SetText{Prop: "SUMMARY", Value: "Renamed"})
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	next, err := SetAlarms(obj, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := encoded(t, next)
	if !strings.Contains(got, "SUMMARY:Renamed") || strings.Contains(got, "VALARM") {
		t.Errorf("encoded:\n%s", got)
	}
}

// Overrides are not the main component; their alarms are not the main set.
func TestSetAlarmsLeavesOverridesAlone(t *testing.T) {
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//T//EN\r\nBEGIN:VEVENT\r\nUID:s\r\nDTSTAMP:20260101T000000Z\r\n" +
		"DTSTART:20260115T100000Z\r\nRRULE:FREQ=WEEKLY;COUNT=3\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:s\r\nDTSTAMP:20260101T000000Z\r\nRECURRENCE-ID:20260122T100000Z\r\nDTSTART:20260122T120000Z\r\n" +
		"BEGIN:VALARM\r\nACTION:AUDIO\r\nTRIGGER:-PT5M\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	obj, err := Parse("/c/s.ics", "e", strings.NewReader(ics))
	if err != nil {
		t.Fatal(err)
	}
	next, err := SetAlarms(obj, []Alarm{NewAlarm("-PT15M")})
	if err != nil {
		t.Fatal(err)
	}
	got := encoded(t, next)
	if !strings.Contains(got, "ACTION:AUDIO") || !strings.Contains(got, "TRIGGER:-PT15M") || strings.Count(got, "BEGIN:VALARM") != 2 {
		t.Errorf("encoded:\n%s", got)
	}
}

// A reminder given another time counts from where it counted: one before a
// task's due date (RELATED=END) stays before it.
func TestAlarmAtKeepsWhatItCountsFrom(t *testing.T) {
	doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\nBEGIN:VTODO\r\nUID:a\r\nDTSTAMP:20260101T000000Z\r\n" +
		"DUE:20260112T180000Z\r\nSUMMARY:Duty\r\nBEGIN:VALARM\r\nACTION:EMAIL\r\nTRIGGER;RELATED=END:-PT1H\r\nDESCRIPTION:Soon\r\nEND:VALARM\r\n" +
		"END:VTODO\r\nEND:VCALENDAR\r\n"
	obj, err := Parse("/c/a.ics", "e1", strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	alarms, err := Alarms(obj)
	if err != nil || len(alarms) != 1 {
		t.Fatalf("alarms = %+v, %v", alarms, err)
	}
	moved := alarms[0].At("-P1D")
	if !moved.RelatedEnd {
		t.Error("the moved reminder counts from the start")
	}
	next, err := SetAlarms(obj, []Alarm{moved})
	if err != nil {
		t.Fatal(err)
	}
	if got := encoded(t, next); !strings.Contains(got, "TRIGGER;RELATED=END:-P1D") || !strings.Contains(got, "ACTION:EMAIL") {
		t.Errorf("encoded:\n%s", got)
	}
}

// A reminder is known by what it does and when, not by how a server spells
// it or what it adds of its own (Б1 of the audit of 11.10.2026).
func TestMissingAlarms(t *testing.T) {
	record := func(alarms string) *model.Object {
		t.Helper()
		doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\nBEGIN:VEVENT\r\nUID:a\r\nDTSTAMP:20260101T000000Z\r\n" +
			"DTSTART:20260112T100000Z\r\nSUMMARY:Duty\r\n" + strings.ReplaceAll(alarms, "\n", "\r\n") + "END:VEVENT\r\nEND:VCALENDAR\r\n"
		obj, err := Parse("/c/a.ics", "e1", strings.NewReader(doc))
		if err != nil {
			t.Fatal(err)
		}
		return obj
	}
	quarter := "BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT15M\nDESCRIPTION:Soon\nEND:VALARM\n"
	hour := "BEGIN:VALARM\nACTION:EMAIL\nTRIGGER;RELATED=END:-PT1H\nDESCRIPTION:Soon\nEND:VALARM\n"
	sent := record(quarter + hour)
	for _, tc := range []struct {
		name    string
		got     string
		missing int
	}{
		{"kept as sent", quarter + hour, 0},
		{"kept in another spelling, with the server's own marks",
			"BEGIN:VALARM\nUID:x-1\nACTION:display\nTRIGGER;VALUE=DURATION:-PT900S\nX-MOZ-LASTACK:20260101T000000Z\nEND:VALARM\n" + hour, 0},
		{"one gone", hour, 1},
		{"both gone", "", 2},
		{"the trigger gone", "BEGIN:VALARM\nACTION:DISPLAY\nDESCRIPTION:Soon\nEND:VALARM\n" + hour, 1},
		{"counted from the start now", quarter + "BEGIN:VALARM\nACTION:EMAIL\nTRIGGER:-PT1H\nDESCRIPTION:Soon\nEND:VALARM\n", 1},
		{"another action", "BEGIN:VALARM\nACTION:AUDIO\nTRIGGER:-PT15M\nEND:VALARM\n" + hour, 1},
	} {
		if got := MissingAlarms(sent, record(tc.got)); got != tc.missing {
			t.Errorf("%s: %d missing, want %d", tc.name, got, tc.missing)
		}
	}
	if lost := AlarmLoss([]string{"X-CUSTOM"}, sent, record(hour)); len(lost) != 2 || lost[1] != "VALARM" {
		t.Errorf("AlarmLoss = %v", lost)
	}
	if lost := AlarmLoss(nil, sent, record(quarter+hour)); lost != nil {
		t.Errorf("AlarmLoss of nothing lost = %v", lost)
	}
}
