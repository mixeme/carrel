package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// An event of several days stands in each day of the week it takes (Typecase
// note 182, task 257): «Поездка в Казань», all day 12–14 August, is a row on
// Wednesday, Thursday and Friday with «day 1 of 3» and on, and the week
// counts it once — «Личный 3 events», the combined calendar «7 events»
// (s-agenda-1, -17).
func TestAnEventOfSeveralDaysStandsInEachOfThem(t *testing.T) {
	for _, badge := range []string{"day 1 of 3", "day 2 of 3", "day 3 of 3"} {
		if !strings.Contains(mockupFrame(t, "s-agenda-1"), badge) {
			t.Fatalf("s-agenda-1 does not draw %q: read the frame again", badge)
		}
	}
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	week := decode[view.AgendaPage](t, app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"?from=2026-08-10", app.sess)))
	want := map[string]string{
		"Wednesday · 12 August": "day 1 of 3",
		"Thursday · 13 August":  "day 2 of 3",
		"Friday · 14 August":    "day 3 of 3",
	}
	for _, d := range week.Days {
		for _, r := range d.Rows {
			if r.Summary != "Поездка в Казань" {
				continue
			}
			badge, ok := want[string(d.Rubric)]
			if !ok {
				t.Errorf("the trip stands on %s", d.Rubric)
				continue
			}
			delete(want, string(d.Rubric))
			if r.Time != "all day" || len(r.Badges) == 0 || string(r.Badges[len(r.Badges)-1].Label) != badge {
				t.Errorf("%s: %q %+v, want all day and %q", d.Rubric, r.Time, r.Badges, badge)
			}
			if first := badge == "day 1 of 3"; r.Continues == first {
				t.Errorf("%s: continues %v", d.Rubric, r.Continues)
			}
		}
	}
	if len(want) != 0 {
		t.Errorf("the trip is missing on %v", want)
	}
	if got := runsText(week.Subtitle); got != "nextcloud · 10 – 16 August 2026 · Europe/Moscow · 3 events" {
		t.Errorf("subtitle of the week = %q", got)
	}
	if week.Bar.Loaded != 3 {
		t.Errorf("the bar counts %d events, want 3", week.Bar.Loaded)
	}
}
