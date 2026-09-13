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
