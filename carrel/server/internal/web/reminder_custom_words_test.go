package web

import "testing"

// The reasons of «Custom…» (the owner's words of 03.11.2026): the field by
// its label, then what it takes.
func TestCustomReminderReasons(t *testing.T) {
	for _, c := range []struct {
		count, unit, at string
		allDay          bool
		want            string
	}{
		{"1.5", "hours", "", false, "How long: enter a whole number, 0 or more."},
		{"-2", "hours", "", false, "How long: enter a whole number, 0 or more."},
		{"2", "", "", false, "Unit: choose minutes, hours, days or weeks before."},
		{"1", "days", "9 o'clock", true, "At: enter a time as HH:MM, for example 09:00."},
	} {
		if _, got := customTrigger(c.count, c.unit, c.at, c.allDay); got != c.want {
			t.Errorf("customTrigger(%q, %q, %q) says %q, want %q", c.count, c.unit, c.at, got, c.want)
		}
	}
}
