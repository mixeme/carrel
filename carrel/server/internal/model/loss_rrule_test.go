package model

import "testing"

// A recurrence rule is a set of parts, not a line of text. SoGo writes back
// FREQ, COUNT, BYDAY, WKST where Carrel sent FREQ, BYDAY, WKST, COUNT — the
// same rule, said in another order. Read as text it looked like a server
// that drops RRULE, and every edit of a repeating event there ended with a
// loss report about a loss that had not happened.
func TestARuleReorderedIsTheSameRule(t *testing.T) {
	sent := NewObject("/c/e.ics", `"1"`, map[string][]RawValue{
		"UID":   {{Text: "e1"}},
		"RRULE": {{Text: "FREQ=WEEKLY;BYDAY=MO;WKST=MO;COUNT=6"}},
	})
	got := NewObject("/c/e.ics", `"2"`, map[string][]RawValue{
		"UID":   {{Text: "e1"}},
		"RRULE": {{Text: "FREQ=WEEKLY;COUNT=6;BYDAY=MO;WKST=MO"}},
	})
	if loss := CompareAfterPut("sogo", sent, got, nil); loss != nil {
		t.Errorf("a reordered rule is reported lost: %+v", loss)
	}
}

// A rule that really changed is still a loss: a part dropped is not an
// order, and a person whose «every second week» became «every week» has to
// be told.
func TestARuleWithAPartMissingIsALoss(t *testing.T) {
	sent := NewObject("/c/e.ics", `"1"`, map[string][]RawValue{
		"UID":   {{Text: "e1"}},
		"RRULE": {{Text: "FREQ=WEEKLY;BYDAY=MO;COUNT=6"}},
	})
	got := NewObject("/c/e.ics", `"2"`, map[string][]RawValue{
		"UID":   {{Text: "e1"}},
		"RRULE": {{Text: "FREQ=WEEKLY;BYDAY=MO"}},
	})
	loss := CompareAfterPut("sogo", sent, got, nil)
	if loss == nil || len(loss.Properties) != 1 || loss.Properties[0] != "RRULE" {
		t.Errorf("a rule that lost its count is not reported: %+v", loss)
	}
}

// And a part whose value changed is a loss too: BYDAY=MO is not BYDAY=TU.
func TestARuleWithAPartChangedIsALoss(t *testing.T) {
	sent := NewObject("/c/e.ics", `"1"`, map[string][]RawValue{
		"UID":   {{Text: "e1"}},
		"RRULE": {{Text: "FREQ=WEEKLY;BYDAY=MO"}},
	})
	got := NewObject("/c/e.ics", `"2"`, map[string][]RawValue{
		"UID":   {{Text: "e1"}},
		"RRULE": {{Text: "FREQ=WEEKLY;BYDAY=TU"}},
	})
	if loss := CompareAfterPut("sogo", sent, got, nil); loss == nil {
		t.Error("a rule moved to another weekday is not reported")
	}
}

// A server that writes its own bookkeeping into a record and drops it again
// on the next write is not losing the person's data. Mail.ru adds
// X-MOZ-LASTACK — when an alarm was last acknowledged — and Carrel, keeping
// every line it does not edit, sends it back and sees it gone. Reported as
// a loss it teaches people to ignore loss reports, which is worse than
// saying nothing.
func TestTheBookkeepingAServerAddsIsNotALoss(t *testing.T) {
	sent := NewObject("/c/e.ics", `"1"`, map[string][]RawValue{
		"UID":           {{Text: "e1"}},
		"SUMMARY":       {{Text: "Duty"}},
		"X-MOZ-LASTACK": {{Text: "20260112T090000Z"}},
	})
	got := NewObject("/c/e.ics", `"2"`, map[string][]RawValue{
		"UID":     {{Text: "e1"}},
		"SUMMARY": {{Text: "Duty"}},
	})
	if loss := CompareAfterPut("mailru", sent, got, nil); loss != nil {
		t.Errorf("the server's own bookkeeping is reported lost: %+v", loss)
	}
}

// A property of the person's that begins with X- is still theirs.
func TestAnUnknownPropertyOfThePersonIsStillALoss(t *testing.T) {
	sent := NewObject("/c/e.ics", `"1"`, map[string][]RawValue{
		"UID":      {{Text: "e1"}},
		"X-CUSTOM": {{Text: "keep-me"}},
	})
	got := NewObject("/c/e.ics", `"2"`, map[string][]RawValue{"UID": {{Text: "e1"}}})
	if loss := CompareAfterPut("mailru", sent, got, nil); loss == nil {
		t.Error("a property the person put there is not reported")
	}
}
