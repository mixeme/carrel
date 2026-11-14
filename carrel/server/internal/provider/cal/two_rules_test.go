package cal

import (
	"bytes"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

// The check of the record's audit, «a second RRULE»: a series another client
// wrote with two rules (RFC 5545 says SHOULD NOT, and some clients do) could
// not be saved at all — go-ical refuses to write a component with more than
// one RRULE, and every edit of the event or the task ended in that error.
// Now the rules are written as they came, whatever else is changed.
func TestARecordWithTwoRulesIsWritten(t *testing.T) {
	for _, kind := range []string{"VEVENT", "VTODO"} {
		t.Run(kind, func(t *testing.T) {
			doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\nBEGIN:" + kind + "\r\nUID:a\r\nDTSTAMP:20260101T000000Z\r\n" +
				"DTSTART:20260112T100000Z\r\nSUMMARY:Sync\r\nRRULE:FREQ=WEEKLY;BYDAY=MO\r\nRRULE:FREQ=MONTHLY;BYMONTHDAY=1\r\n" +
				"END:" + kind + "\r\nEND:VCALENDAR\r\n"
			obj, err := Parse("/c/a.ics", "e", strings.NewReader(doc))
			if err != nil {
				t.Fatal(err)
			}
			patch, err := model.NewPatch(model.SetText{Prop: "SUMMARY", Value: "Sync weekly"})
			if err != nil {
				t.Fatal(err)
			}
			if err := patch.Apply(obj); err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := Encode(&buf, obj); err != nil {
				t.Fatalf("encode: %v", err)
			}
			got := buf.String()
			for _, want := range []string{"SUMMARY:Sync weekly", "RRULE:FREQ=WEEKLY;BYDAY=MO\r\n", "RRULE:FREQ=MONTHLY;BYMONTHDAY=1\r\n"} {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q in\n%s", want, got)
				}
			}
			if strings.Contains(got, "X-CARREL") {
				t.Errorf("a name of Carrel's own was written:\n%s", got)
			}
		})
	}
}
