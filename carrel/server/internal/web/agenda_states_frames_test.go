package web_test

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// frameEmpty is the empty state a frame draws: its bold title and its words.
func frameEmpty(t *testing.T, id string) (title, body string) {
	t.Helper()
	m := regexp.MustCompile(`(?s)<div class="tc-empty[^"]*"><b>(.*?)</b>(.*?)(?:<button|</div>)`).FindStringSubmatch(mockupFrame(t, id))
	if m == nil {
		t.Fatalf("%s draws no empty state: read the frame again", id)
	}
	return frameText(m[1]), frameText(m[2])
}

// frameSubtitle is the line under a frame's title.
func frameSubtitle(t *testing.T, id string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<div class="tc-head__sub">(.*?)</div>`).FindStringSubmatch(mockupFrame(t, id))
	if m == nil {
		t.Fatalf("%s has no subtitle: read the frame again", id)
	}
	return frameText(m[1])
}

// shapeOf turns a frame's line into a pattern its data fits: numbers, the
// window of days and a calendar's name stand for any of their kind.
func shapeOf(line string) *regexp.Regexp {
	p := regexp.QuoteMeta(line)
	p = regexp.MustCompile(`\d+`).ReplaceAllString(p, `\d+`)
	p = regexp.MustCompile(`January|February|March|April|May|June|July|August|September|October|November|December`).ReplaceAllString(p, `[A-Z][a-z]+`)
	p = strings.ReplaceAll(p, "Личный", `\S+`)
	return regexp.MustCompile(`^` + p + `$`)
}

func runsText(r view.Runs) string {
	var b strings.Builder
	for _, run := range r {
		b.WriteString(string(run.Text))
	}
	return b.String()
}

// A day without events is not printed (the owner's decision of 02.10.2026,
// task 77): a week with events has no empty rubric (s-agenda-1 lost its
// empty Wednesday), and a week of one calendar without any is one line —
// states-15: «No events this week.» and where and when nothing is, the
// subtitle «nextcloud · 2 – 8 November 2026 · 0 events». The row prints no
// zone (s-agenda-1: the zone is in the card, at When).
func TestAgendaPrintsNoEmptyDay(t *testing.T) {
	if strings.Contains(mockupFrame(t, "s-agenda-1"), "Nothing this day.") {
		t.Fatal("s-agenda-1 draws an empty day: read the frame again")
	}
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "calendar", "lichnyj")

	rec := app.h.do(get(list+"?from=2026-08-10", app.sess))
	week := decode[view.AgendaPage](t, rec)
	if len(week.Days) == 0 || week.Empty != nil {
		t.Fatalf("the week of the frame: %d days, empty %+v", len(week.Days), week.Empty)
	}
	for _, d := range week.Days {
		if len(d.Rows) == 0 || d.Count != len(d.Rows) {
			t.Errorf("%s: %d rows, count %d — a day without events is not printed", d.Rubric, len(d.Rows), d.Count)
		}
	}
	var raw struct {
		Days []struct {
			Rows []map[string]any `json:"rows"`
		} `json:"days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, d := range raw.Days {
		for _, r := range d.Rows {
			if _, ok := r["zone"]; ok {
				t.Errorf("a row carries a zone: %v", r)
			}
		}
	}

	empty := decode[view.AgendaPage](t, app.h.do(get(list+"?from=2027-01-04", app.sess)))
	if len(empty.Days) != 0 {
		t.Errorf("the empty week prints %d days", len(empty.Days))
	}
	title, body := frameEmpty(t, "states-15")
	if empty.Empty == nil || string(empty.Empty.Title)+"." != title || !shapeOf(body).MatchString(string(empty.Empty.Body)) || len(empty.Empty.Actions) != 0 {
		t.Errorf("the empty week says %+v, want (states-15) %q %q", empty.Empty, title, body)
	}
	if want := "Nothing in Личный from 4 to 10 January."; empty.Empty != nil && empty.Empty.Body != view.Text(want) {
		t.Errorf("body %q, want %q", empty.Empty.Body, want)
	}
	if sub := frameSubtitle(t, "states-15"); !shapeOf(sub).MatchString(runsText(empty.Subtitle)) {
		t.Errorf("subtitle %q, want the shape of %q (states-15)", runsText(empty.Subtitle), sub)
	}
	if got := runsText(week.Subtitle); got != "nextcloud · 10 – 16 August 2026 · 2 events" {
		t.Errorf("subtitle of the week = %q", got)
	}
}

// Previous · Today · Next lead the agenda's bar (s-agenda-1, task 77): the
// step back and forward are icons, back and next, named by the step —
// «Previous week», «Next week» — and a day or a month steps by itself.
func TestAgendaPagingIsTheFrames(t *testing.T) {
	frame := mockupFrame(t, "s-agenda-1")
	link := regexp.MustCompile(`<a class="tc-btn" href="[^"]*" title="([^"]*)"><svg viewBox="0 0 24 24"><use href="#(i-[\w-]+)"/></svg></a>`)
	drawn := link.FindAllStringSubmatch(frame, -1)
	if len(drawn) != 2 {
		t.Fatalf("s-agenda-1 draws %d steps: read the frame again", len(drawn))
	}
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "calendar", "lichnyj")
	for _, scale := range []struct{ query, step string }{{"", "week"}, {"&view=day", "day"}, {"&view=month", "month"}} {
		page := decode[view.AgendaPage](t, app.h.do(get(list+"?from=2026-08-10"+scale.query, app.sess)))
		if page.Bar.Range == nil || len(page.Bar.Range.Presets) < 3 {
			t.Fatalf("%s: presets %+v", scale.step, page.Bar.Range)
		}
		p := page.Bar.Range.Presets
		for i, want := range [][2]string{
			{strings.Replace(drawn[0][1], "week", scale.step, 1), drawn[0][2]},
			{"Today", ""},
			{strings.Replace(drawn[1][1], "week", scale.step, 1), drawn[1][2]},
		} {
			if string(p[i].Label) != want[0] || p[i].Icon != want[1] {
				t.Errorf("%s: preset %d = %q %q, want %q %q", scale.step, i, p[i].Label, p[i].Icon, want[0], want[1])
			}
		}
	}
}

// Nothing ticked (states-14): «0 of 7 calendars · 0 accounts» — no window
// and no zone, nothing is shown —, and «No calendars are ticked.» with
// «Tick every calendar».
func TestNothingTickedAgendaIsTheFrame(t *testing.T) {
	app := newMockupApp(t)
	if rec := app.h.do(postForm("/app/sources/calendar", app.sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("untick everything: %d", rec.Code)
	}
	page := decode[view.AgendaPage](t, app.h.do(get("/app/calendar", app.sess)))
	if sub := frameSubtitle(t, "states-14"); !shapeOf(sub).MatchString(runsText(page.Subtitle)) {
		t.Errorf("subtitle %q, want the shape of %q (states-14)", runsText(page.Subtitle), sub)
	}
	title, _ := frameEmpty(t, "states-14")
	if page.Empty == nil || string(page.Empty.Title)+"." != title || len(page.Days) != 0 {
		t.Errorf("empty %+v, %d days; want %q and no day", page.Empty, len(page.Days), title)
	}
}

// While the poll runs, the calendar's bar says how long it has run, at its
// right in mono — «0.8 s» (states-13; «0.4 s», states-3).
func TestAgendaPollSaysHowLongItRuns(t *testing.T) {
	m := regexp.MustCompile(`<div class="tc-runbar">.*?<span class="tc-mono tc-push">([^<]*)</span></div>`).FindStringSubmatch(mockupFrame(t, "states-13"))
	if m == nil {
		t.Fatal("states-13 draws no time at the bar's right: read the frame again")
	}
	app := newMockupApp(t)
	release := app.servers["radicale · vps"].Stall("REPORT")
	defer release()
	page := decode[view.AgendaPage](t, app.h.do(get("/app/calendar", app.sess)))
	if page.Poll == nil || !page.Poll.Running {
		t.Fatalf("poll = %+v, want it running", page.Poll)
	}
	if !regexp.MustCompile(`^\d+\.\d s$`).MatchString(string(page.Poll.Aside)) {
		t.Errorf("aside = %q, want a time like %q (states-13)", page.Poll.Aside, m[1])
	}
}
