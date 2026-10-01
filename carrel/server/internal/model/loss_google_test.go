package model

import "testing"

// docs/servers.md, SV-56: Google keeps an event whose DTEND is its DTSTART
// without the DTEND. RFC 5545 §3.6.1 gives an event without an end the end
// it would have had — none for a time, the next day for a date — so the end
// that was sent is the end the server holds, and calling it lost told the
// person of a loss that had not happened.
func TestAnEndTheStartImpliesIsNotLostWhenTheServerLeavesItOut(t *testing.T) {
	for _, c := range []struct {
		name       string
		start, end RawValue
		got        map[string][]RawValue
		wantLost   bool
	}{
		{name: "time", start: RawValue{Text: "20310311T093000Z"}, end: RawValue{Text: "20310311T093000Z"}},
		{name: "time in a zone",
			start: RawValue{Text: "20310311T093000", Params: map[string]string{"TZID": "Europe/Moscow"}},
			end:   RawValue{Text: "20310311T093000", Params: map[string]string{"TZID": "Europe/Moscow"}}},
		{name: "date",
			start: RawValue{Text: "20310311", Params: map[string]string{"VALUE": "DATE"}},
			end:   RawValue{Text: "20310312", Params: map[string]string{"VALUE": "DATE"}}},
		{name: "date at the end of a month",
			start: RawValue{Text: "20310228", Params: map[string]string{"VALUE": "DATE"}},
			end:   RawValue{Text: "20310301", Params: map[string]string{"VALUE": "DATE"}}},
		// An end the start does not imply is the person's, and a server
		// that drops it has lost it.
		{name: "an hour later", start: RawValue{Text: "20310311T093000Z"}, end: RawValue{Text: "20310311T103000Z"}, wantLost: true},
		{name: "two days",
			start:    RawValue{Text: "20310311", Params: map[string]string{"VALUE": "DATE"}},
			end:      RawValue{Text: "20310313", Params: map[string]string{"VALUE": "DATE"}},
			wantLost: true},
		{name: "another zone",
			start:    RawValue{Text: "20310311T093000", Params: map[string]string{"TZID": "Europe/Moscow"}},
			end:      RawValue{Text: "20310311T093000", Params: map[string]string{"TZID": "Europe/Berlin"}},
			wantLost: true},
		// A length the server kept another way is read by the ordinary
		// rule: a DURATION in place of the DTEND is not what was sent.
		{name: "a duration instead", start: RawValue{Text: "20310311T093000Z"}, end: RawValue{Text: "20310311T093000Z"},
			got: map[string][]RawValue{"DURATION": {{Text: "PT0S"}}}, wantLost: true},
	} {
		sent := NewObject("/c/e.ics", `"1"`, map[string][]RawValue{
			"SUMMARY": {{Text: "Call"}},
			"DTSTART": {c.start},
			"DTEND":   {c.end},
		})
		props := map[string][]RawValue{
			"SUMMARY": {{Text: "Call"}},
			"DTSTART": {c.start},
		}
		for k, v := range c.got {
			props[k] = v
		}
		got := NewObject("/c/e.ics", `"2"`, props)
		loss := CompareAfterPut("google", sent, got, nil)
		lost := loss != nil && len(loss.Properties) == 1 && loss.Properties[0] == "DTEND"
		if lost != c.wantLost || loss != nil && !lost {
			t.Errorf("%s: loss %+v, want DTEND lost: %v", c.name, loss, c.wantLost)
		}
		if created := CompareAfterCreate("google", sent, got, nil); (created != nil) != c.wantLost {
			t.Errorf("%s, after create: loss %+v, want DTEND lost: %v", c.name, created, c.wantLost)
		}
	}

	// The start moved as well: that is a loss of its own, and the end the
	// old start implied is not taken for granted against the new one.
	sent := NewObject("/c/e.ics", `"1"`, map[string][]RawValue{
		"DTSTART": {{Text: "20310311T093000Z"}},
		"DTEND":   {{Text: "20310311T093000Z"}},
	})
	got := NewObject("/c/e.ics", `"2"`, map[string][]RawValue{"DTSTART": {{Text: "20310311T100000Z"}}})
	if loss := CompareAfterPut("google", sent, got, nil); loss == nil || len(loss.Properties) != 2 {
		t.Errorf("start moved and end gone: loss %+v, want DTEND and DTSTART", loss)
	}
}

// docs/servers.md, SV-56: Google marks the first phone, address and e-mail
// of a card PREF. The mark is the server's — the labels the person chose are
// all there — and calling the value lost told them of a loss that had not
// happened. The other way round is a loss: a PREF the person had and the
// server took away.
func TestAPreferredMarkTheServerAddsIsNotALoss(t *testing.T) {
	for _, c := range []struct {
		name      string
		sent, got map[string]string
		wantLost  bool
	}{
		{name: "a label", sent: map[string]string{"TYPE": "cell"}, got: map[string]string{"TYPE": "CELL,PREF"}},
		{name: "no label", sent: nil, got: map[string]string{"TYPE": "PREF"}},
		{name: "PREF as it was", sent: map[string]string{"TYPE": "work,pref"}, got: map[string]string{"TYPE": "WORK,PREF"}},
		{name: "PREF taken away", sent: map[string]string{"TYPE": "work,pref"}, got: map[string]string{"TYPE": "WORK"}, wantLost: true},
		{name: "another label", sent: map[string]string{"TYPE": "cell"}, got: map[string]string{"TYPE": "WORK,PREF"}, wantLost: true},
		{name: "another parameter", sent: map[string]string{"TYPE": "cell"}, got: map[string]string{"TYPE": "CELL,PREF", "X-A": "1"}, wantLost: true},
	} {
		sent := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{"TEL": {{Text: "+1 555 0100", Params: c.sent}}})
		got := NewObject("/c/u.vcf", `"2"`, map[string][]RawValue{"TEL": {{Text: "+1 555 0100", Params: c.got}}})
		if loss := CompareAfterPut("google", sent, got, nil); (loss != nil) != c.wantLost {
			t.Errorf("%s: loss %+v, want lost: %v", c.name, loss, c.wantLost)
		}
	}
}
