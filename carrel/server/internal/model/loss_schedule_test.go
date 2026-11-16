package model

import "testing"

// docs/servers.md, SV-60: a server that schedules (RFC 6638 — Baikal,
// Nextcloud: sabre) writes on an attendee it sent an invitation to how the
// sending went, SCHEDULE-STATUS=1.1. The parameter is the server's (RFC 6638
// §7.3: it is set by the server, and a client's value is replaced); the
// attendee is the one sent, and calling him lost told a person of a loss
// that had not happened — found by the live matrix on Baikal, 16.11.2026.
// Everything else on the line is still compared.
func TestAScheduleStatusTheServerWritesIsNotALoss(t *testing.T) {
	eve := func(p map[string]string) map[string][]RawValue {
		return map[string][]RawValue{"ATTENDEE": {{Text: "mailto:eve@example.com", Params: p}}}
	}
	for _, c := range []struct {
		name      string
		sent, got map[string]string
		wantLost  bool
	}{
		{name: "written by the server", sent: map[string]string{"PARTSTAT": "NEEDS-ACTION", "RSVP": "TRUE"},
			got: map[string]string{"PARTSTAT": "NEEDS-ACTION", "RSVP": "TRUE", "SCHEDULE-STATUS": "1.1"}},
		{name: "changed by the server", sent: map[string]string{"CN": "Eve", "SCHEDULE-STATUS": "1.1"},
			got: map[string]string{"CN": "Eve", "SCHEDULE-STATUS": "3.7"}},
		{name: "taken away by the server", sent: map[string]string{"CN": "Eve", "SCHEDULE-STATUS": "1.1"},
			got: map[string]string{"CN": "Eve"}},
		{name: "another parameter lost", sent: map[string]string{"CN": "Eve", "ROLE": "CHAIR"},
			got: map[string]string{"CN": "Eve", "SCHEDULE-STATUS": "1.1"}, wantLost: true},
	} {
		sent := NewObject("/c/e.ics", `"1"`, eve(c.sent))
		got := NewObject("/c/e.ics", `"2"`, eve(c.got))
		if loss := CompareAfterPut("baikal", sent, got, nil); (loss != nil) != c.wantLost {
			t.Errorf("%s: loss %+v, want lost: %v", c.name, loss, c.wantLost)
		}
	}
}
