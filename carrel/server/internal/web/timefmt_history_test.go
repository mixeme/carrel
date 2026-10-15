package web

import (
	"testing"
	"time"
)

// Пк20 (s-find-2): a search of the history says when it was made — today
// relatively, then «yesterday», then the day of the week, then the day.
func TestHistoryWhen(t *testing.T) {
	tf := timeFmt{tz: time.UTC}
	now := time.Date(2026, 8, 13, 15, 0, 0, 0, time.UTC) // a Thursday
	for at, want := range map[time.Time]string{
		now.Add(-2 * time.Hour):                       "2 hours ago",
		time.Date(2026, 8, 12, 23, 0, 0, 0, time.UTC): "yesterday",
		time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC):  "Monday",
		time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC):   "1 August",
		time.Date(2025, 12, 1, 9, 0, 0, 0, time.UTC):  "1 December 2025",
	} {
		if got := string(tf.historyWhen(at, now)); got != want {
			t.Errorf("%s: %q, want %q", at, got, want)
		}
	}
}
