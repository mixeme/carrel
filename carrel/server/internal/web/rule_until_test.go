package web

import (
	"testing"
	"time"
)

// ruleUntil is the last day a series can start on (the client's scan Г3), not
// the day of the UNTIL instant: the day before when the instant falls before
// the series' time of day, the instant's own day otherwise, and what the form
// writes back reads as the day typed.
func TestRuleUntil(t *testing.T) {
	msk, _ := time.LoadLocation("Europe/Moscow")
	yekt, _ := time.LoadLocation("Asia/Yekaterinburg")
	for _, c := range []struct {
		name, raw string
		start     time.Time
		want      string
	}{
		{"Mondays at 12:00 Yekaterinburg, UNTIL the end of the 28th in UTC",
			"FREQ=WEEKLY;BYDAY=MO;UNTIL=20261228T235959Z", time.Date(2026, 8, 10, 12, 0, 0, 0, yekt), "2026-12-28"},
		{"every day at 01:00 Moscow: 29 December 01:00 is before UNTIL (02:59)",
			"FREQ=DAILY;UNTIL=20261228T235959Z", time.Date(2026, 8, 10, 1, 0, 0, 0, msk), "2026-12-29"},
		{"a date UNTIL is that day", "FREQ=WEEKLY;UNTIL=20261228", time.Date(2026, 8, 10, 0, 0, 0, 0, msk), "2026-12-28"},
		{"no start: the instant's day", "FREQ=WEEKLY;UNTIL=20261228T235959Z", time.Time{}, "2026-12-29"},
	} {
		day, ok := ruleUntil(c.raw, c.start, msk)
		if !ok || day.Format("2006-01-02") != c.want {
			t.Errorf("%s: %v %v, want %s", c.name, day, ok, c.want)
		}
	}
	if _, ok := ruleUntil("FREQ=WEEKLY;COUNT=3", time.Time{}, msk); ok {
		t.Error("a rule without UNTIL has an until day")
	}

	// The form writes the end of the day typed in the instance zone; reading
	// it back gives that day, whatever the series' time of day.
	start := time.Date(2026, 8, 10, 12, 0, 0, 0, yekt)
	have := repeatValues("FREQ=WEEKLY;BYDAY=MO;COUNT=5", msk, start)
	want := append([]string(nil), have...)
	want[repCount], want[repUntil] = "", "2026-12-28"
	rule, err := repeatRule(want, have, false, msk)
	if err != nil {
		t.Fatal(err)
	}
	if got := repeatValues(rule, msk, start)[repUntil]; got != "2026-12-28" {
		t.Errorf("written %s, read back %s", rule, got)
	}
}
