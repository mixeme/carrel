package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The client's scan of the calendar, Г3: «Синк-ап по релизу» is weekly on
// Monday at 12:00 Asia/Yekaterinburg with UNTIL=20261228T235959Z — the last
// Monday is 28 December. The UNTIL instant is 29 December in Moscow (02:59)
// and in Yekaterinburg (04:59), and a day taken from the instant alone said
// «until 29 December» on the card and 2026-12-29 in Until of the form. The
// day shown is the last day an occurrence can start on: an instant before
// the series' start time of its day ends the series the day before.
func TestUntilIsTheLastDayTheSeriesCanStart(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "calendar", "lichnyj") + "/8f1c-4a02-b7d3"

	page := decode[view.EventPage](t, app.h.do(get(rec, app.sess)))
	if s := string(page.Recurrence.Summary); !strings.Contains(s, "until 28 December 2026") {
		t.Errorf("the card's rule says %q, want until 28 December 2026", s)
	}
	edit := decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess)))
	if got := formFields(edit.Form).Get("repeat.until"); got != "2026-12-28" {
		t.Errorf("Until of the form = %q, want 2026-12-28", got)
	}
}
