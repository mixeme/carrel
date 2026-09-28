package cal

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// A series whose Thursday was overridden: the override carries its own
// ORGANIZER and ATTENDEE, and the person stands in both components — as
// attendee in the series, as the one who delegated in the override, and as
// the sender of the organiser's line there.
const rewriteSeries = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//T//EN
BEGIN:VEVENT
UID:series
DTSTAMP:20260101T000000Z
DTSTART:20260105T100000Z
RRULE:FREQ=WEEKLY;COUNT=10
SUMMARY:Planning
ORGANIZER;CN=Boss:mailto:boss@example.org
ATTENDEE;CN=Анна Ковалёва;PARTSTAT=ACCEPTED;ROLE=REQ-PARTICIPANT;RSVP=TRUE:MAILTO:A.Kovaleva@Example.org
ATTENDEE;CN=Bob;PARTSTAT=NEEDS-ACTION:mailto:bob@example.org
END:VEVENT
BEGIN:VEVENT
UID:series
DTSTAMP:20260101T000000Z
RECURRENCE-ID:20260108T100000Z
DTSTART:20260108T110000Z
SUMMARY:Planning, later
ORGANIZER;CN=Boss;SENT-BY="mailto:a.kovaleva@example.org":mailto:boss@example.org
ATTENDEE;CN=Carol;DELEGATED-FROM="mailto:bob@example.org","mailto:a.kovaleva@example.org":mailto:carol@example.org
ATTENDEE;CN=Bob;DELEGATED-TO="mailto:carol@example.org":mailto:bob@example.org
END:VEVENT
END:VCALENDAR
`

// S-15.4: a replaced address is rewritten in every component of the record
// — the main one and every overridden occurrence — in the value and in
// SENT-BY, DELEGATED-FROM and DELEGATED-TO. Everything else of the line stays:
// CN, PARTSTAT, ROLE, RSVP are the person's answer and the organiser's
// question, and the scheme is kept as it was written.
func TestRewriteAddressesReachesEveryComponent(t *testing.T) {
	obj := parseICS(t, rewriteSeries)
	out, n, err := RewriteAddresses(obj, map[string]string{"a.kovaleva@example.org": "a.kovaleva@slovo.example"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("rewritten = %d, want 3: the attendee, SENT-BY and one of DELEGATED-FROM", n)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, out); err != nil {
		t.Fatal(err)
	}
	doc := unfold(buf.String())
	if strings.Contains(strings.ToLower(doc), "a.kovaleva@example.org") {
		t.Errorf("the old address is still in the record:\n%s", doc)
	}
	for _, want := range []string{"MAILTO:a.kovaleva@slovo.example", "mailto:a.kovaleva@slovo.example"} {
		if !strings.Contains(doc, want) {
			t.Errorf("missing %q:\n%s", want, doc)
		}
	}
	// The main component, read the way everything else reads it.
	var anna Attendee
	for _, a := range Attendees(out) {
		if a.Name == "Анна Ковалёва" {
			anna = a
		}
	}
	if anna.Email != "a.kovaleva@slovo.example" || anna.Status != "ACCEPTED" {
		t.Errorf("the attendee = %+v", anna)
	}
	for _, p := range []string{"ROLE=REQ-PARTICIPANT", "RSVP=TRUE", "PARTSTAT=ACCEPTED"} {
		if !strings.Contains(doc, p) {
			t.Errorf("the parameter %s was lost:\n%s", p, doc)
		}
	}
	// The override: SENT-BY and the delegated list, the other names kept.
	var sentBy string
	var from []string
	for _, p := range People(out) {
		if p.Organiser && p.SentBy != "" {
			sentBy = p.SentBy
		}
		if p.Name == "Carol" {
			from = p.DelegatedFrom
		}
	}
	if sentBy != "a.kovaleva@slovo.example" {
		t.Errorf("SENT-BY of the overridden organiser = %q", sentBy)
	}
	if len(from) != 2 || from[0] != "bob@example.org" || from[1] != "a.kovaleva@slovo.example" {
		t.Errorf("DELEGATED-FROM = %q, want bob kept and the address replaced in place", from)
	}
	if !strings.Contains(doc, "RECURRENCE-ID:20260108T100000Z") || !strings.Contains(doc, "SUMMARY:Planning, later") {
		t.Errorf("the override is not whole:\n%s", doc)
	}
	// The record read before is not the record written.
	if a := Attendees(obj); a[0].Email != "A.Kovaleva@Example.org" {
		t.Errorf("the object read was changed in place: %+v", a)
	}
}

// A record that does not name the address is left as it is, and says so.
func TestRewriteAddressesLeavesOthersAlone(t *testing.T) {
	obj := parseICS(t, rewriteSeries)
	_, n, err := RewriteAddresses(obj, map[string]string{"nobody@example.org": "x@example.org"})
	if err != nil || n != 0 {
		t.Errorf("n = %d, err = %v", n, err)
	}
}

// S-15.4: a meeting all of whose occurrences are behind is history — it
// will show the new address as if the person had been invited at it.
func TestEndedIsEveryOccurrenceBehind(t *testing.T) {
	obj := parseICS(t, rewriteSeries)
	if Ended(obj, time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)) {
		t.Error("a series with occurrences ahead has not ended")
	}
	if !Ended(obj, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("ten weeks from January are over by June")
	}
	endless := parseICS(t, strings.Replace(rewriteSeries, "COUNT=10", "BYDAY=MO", 1))
	if Ended(endless, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("a series without an end never ends")
	}
}

func unfold(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n ", ""), "\r\n\t", "")
}
