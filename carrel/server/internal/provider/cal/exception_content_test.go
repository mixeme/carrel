package cal

import (
	"strings"
	"testing"
)

// A RECURRENCE-ID left in place does not prove the moved occurrence was
// kept: its own fields and reminders must survive the write too.
func TestExceptionLossIncludesTheContents(t *testing.T) {
	const alarm = "BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT10M\nDESCRIPTION:Bring keys\nEND:VALARM\n"
	body := strings.Replace(retimeSeries, "SUMMARY:Duty moved\n", "SUMMARY:Duty moved\nLOCATION:Office\n"+alarm, 1)
	sent := parseICS(t, body)
	for _, tc := range []struct{ name, from, to string }{
		{"title", "SUMMARY:Duty moved", "SUMMARY:Changed"},
		{"location", "LOCATION:Office\n", ""},
		{"start", "20260126T120000", "20260126T140000"},
		{"end", "20260126T130000", "20260126T150000"},
		{"reminder", alarm, ""},
		{"reminder contents", "DESCRIPTION:Bring keys", "DESCRIPTION:Changed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := parseICS(t, strings.Replace(body, tc.from, tc.to, 1))
			if days := LostDays(sent, got); len(days) != 1 || !days[0].Moved || days[0].To.IsZero() {
				t.Errorf("lost days = %+v, want the occurrence with its original moved start", days)
			}
			if loss := ExceptionLoss(nil, sent, got); len(loss) != 1 || loss[0] != "RECURRENCE-ID" {
				t.Errorf("loss = %v", loss)
			}
		})
	}
	// Equal instants, a new UID and server timestamps are not data loss.
	kept := strings.NewReplacer(
		"UID:duty", "UID:server-id",
		"DTSTAMP:20260101T000000Z", "DTSTAMP:20260102T000000Z",
		"RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000", "RECURRENCE-ID:20260126T070000Z",
		"DTSTART;TZID=Europe/Moscow:20260126T120000", "DTSTART:20260126T090000Z",
		"DTEND;TZID=Europe/Moscow:20260126T130000", "DTEND:20260126T100000Z",
	).Replace(body)
	if days := LostDays(sent, parseICS(t, kept)); len(days) != 0 {
		t.Errorf("the same occurrence spelled in UTC: %+v", days)
	}
	floating := strings.Replace(body, "DTSTART;TZID=Europe/Moscow:20260126T120000", "DTSTART:20260126T090000", 1)
	if days := LostDays(sent, parseICS(t, floating)); len(days) != 1 {
		t.Errorf("a floating time is not the UTC instant: %+v", days)
	}
}
