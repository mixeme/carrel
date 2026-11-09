package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The filter narrows what the page shows, not the whole calendar (the owner,
// 07.11; Typecase's note 185, s-agenda-31): with «синк» the week of 10 August
// stays the week, «Синк-ап по релизу» is left of its three events, the line
// says «1 of 3 events», ← Today → page with the word, and a week where
// nothing holds it is the week's empty state «Nothing with «синк» this week.».
func TestTheFilterNarrowsTheWeek(t *testing.T) {
	frame := mockupFrame(t, "s-agenda-31")
	for _, want := range []string{"1 of 3 events", "?from=2026-08-03&amp;filter=синк", "Nothing with «синк» this week."} {
		if !strings.Contains(frame, want) {
			t.Fatalf("s-agenda-31 does not draw %q: read the frame again", want)
		}
	}
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "calendar", "lichnyj")
	week := decode[view.AgendaPage](t, app.h.do(get(list+"?from=2026-08-10&filter="+url.QueryEscape("синк"), app.sess)))
	var rows []string
	for _, d := range week.Days {
		for _, r := range d.Rows {
			rows = append(rows, string(d.Rubric)+": "+string(r.Summary))
		}
		if d.Count != len(d.Rows) {
			t.Errorf("%s counts %d of %d rows", d.Rubric, d.Count, len(d.Rows))
		}
	}
	if got := strings.Join(rows, "; "); got != "Monday · 10 August: Синк-ап по релизу" {
		t.Errorf("the narrowed week = %q", got)
	}
	if got := runsText(week.Subtitle); got != "nextcloud · 10 – 16 August 2026 · Europe/Moscow · 1 of 3 events" {
		t.Errorf("subtitle = %q", got)
	}
	if week.Bar.Range == nil {
		t.Fatal("the narrowed week has no ← Today →")
	}
	paged := 0
	for _, a := range week.Bar.Range.Presets {
		if a.ID == "range.previous" || a.ID == "range.current" || a.ID == "range.next" {
			paged++
			if q, err := url.ParseQuery(a.Href[strings.Index(a.Href, "?")+1:]); err != nil || q.Get("filter") != "синк" {
				t.Errorf("%s leads to %q without the word", a.ID, a.Href)
			}
		}
	}
	if paged != 3 {
		t.Errorf("%d of ← Today →", paged)
	}

	empty := decode[view.AgendaPage](t, app.h.do(get(list+"?from=2027-01-04&filter="+url.QueryEscape("синк"), app.sess)))
	if len(empty.Days) != 0 || empty.Empty == nil || empty.Empty.Title != "Nothing with «синк» this week." || empty.Empty.Body != "" {
		t.Errorf("an empty narrowed week: %d days, %+v", len(empty.Days), empty.Empty)
	}
}
