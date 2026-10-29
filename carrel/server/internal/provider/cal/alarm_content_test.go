package cal

import (
	"strings"
	"testing"
)

// A reminder kept at the same time can still lose its message, recipient,
// sound or repetitions. A move must not delete the intact original then.
func TestAlarmLossIncludesTheContents(t *testing.T) {
	const body = "ACTION:EMAIL\nTRIGGER:-PT15M\nSUMMARY:Duty\nDESCRIPTION:Bring the keys\nATTENDEE:mailto:ada@example.org\nATTACH:https://example.org/sound\nREPEAT:2\nDURATION:PT5M\nX-KEEP:mine\n"
	record := func(body string) *modelObject {
		return parseICS(t, "BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:duty\nDTSTART:20260112T100000Z\nBEGIN:VALARM\n"+body+"END:VALARM\nEND:VEVENT\nEND:VCALENDAR\n")
	}
	sent := record(body)
	for _, line := range strings.Split(strings.TrimSpace(body), "\n")[2:] {
		t.Run(strings.SplitN(line, ":", 2)[0], func(t *testing.T) {
			got := record(strings.Replace(body, line+"\n", "", 1))
			if loss := AlarmLoss(nil, sent, got); len(loss) != 1 || loss[0] != "VALARM" {
				t.Fatalf("dropped %s: loss %v, want VALARM", line, loss)
			}
		})
	}
	got := record(strings.Replace(body, "Bring the keys", "Bring nothing", 1))
	if MissingAlarms(sent, got) != 1 {
		t.Error("a changed message was called intact")
	}
	kept := record(strings.Replace(body, "TRIGGER:-PT15M", "TRIGGER;VALUE=DURATION:-PT900S", 1) + "UID:server-alarm\nACKNOWLEDGED:20260101T000000Z\nX-MOZ-LASTACK:20260101T000000Z\n")
	if loss := AlarmLoss(nil, sent, kept); len(loss) != 0 {
		t.Errorf("the same reminder with server marks: %v", loss)
	}
	// Equal triggers are not interchangeable when their contents differ;
	// each returned alarm can keep only one of the sent alarms.
	join := func(a, b string) string { return a + "END:VALARM\nBEGIN:VALARM\n" + b }
	if n := MissingAlarms(record(join(body, body)), record(body)); n != 1 {
		t.Errorf("two identical reminders returned once: %d missing", n)
	}
	short := strings.Replace(body, "X-KEEP:mine\n", "", 1)
	if n := MissingAlarms(record(join(short, body)), record(join(body, short))); n != 0 {
		t.Errorf("the same reminders in reverse order: %d missing", n)
	}
}
