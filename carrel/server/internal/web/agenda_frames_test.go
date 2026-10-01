package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The agenda as s-agenda-1 draws it (the client's scan of the calendar,
// 01.10.2026, groups А and Б): the line under the title, Print, Range among
// the windows, the three ticks of the bar as one GET form, Compact rows, the
// day's rubric and count, the badge of a row, «all day», the poll with the
// source that did not answer named, the refresh, the feeds' section.
func TestAgendaByItsFrame(t *testing.T) {
	app := newMockupApp(t)
	agendaPoll := func(p view.AgendaPage) *view.PollBar { return p.Poll }
	first := combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", agendaPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	page := decode[view.AgendaPage](t, app.h.do(get("/app/calendar?from=2026-08-10&task="+first.Poll.TaskID, app.sess)))

	// Б2: «5 of 7 calendars · 3 accounts · 10 – 16 August 2026 · ‹Europe/Moscow›».
	if n := len(page.Subtitle); n < 2 || page.Subtitle[n-1] != (view.Run{Text: "Europe/Moscow", Mark: view.RunMono}) ||
		!strings.Contains(string(page.Subtitle[0].Text), " calendars · ") ||
		!strings.HasSuffix(string(page.Subtitle[0].Text), " accounts · 10 – 16 August 2026 · ") {
		t.Errorf("subtitle = %+v", page.Subtitle)
	}
	// Б3: Print, as the contacts' (C-5.4).
	if p := actionByID(page.Bar.Actions, "list.print"); p == nil || p.Icon != "i-print" || !strings.Contains(p.Href, "print=1") {
		t.Errorf("Print = %+v", p)
	}
	// Б4: New event writes into a ticked calendar, the last used one.
	newEvent := actionByID(page.Bar.Actions, "record.new")
	if newEvent == nil || !strings.Contains(newEvent.Href, "lichnyj") {
		t.Errorf("New event = %+v, want Личный, the last used", newEvent)
	} else {
		last := ""
		for _, it := range newEvent.Arrow {
			if it.Aside == "last used" {
				last = string(it.Label)
			}
		}
		if last != "Личный" {
			t.Errorf("no «last used» in the arrow: %+v", newEvent.Arrow)
		}
	}
	// Б5: Range beside Day, Week and Month.
	rng := page.Bar.Range
	if r := actionByID(rng.Presets, "range.range"); r == nil || r.Label != "Range" || r.On ||
		!strings.Contains(r.Href, "view=range") || !strings.Contains(r.Href, "to=2026-08-16") {
		t.Errorf("Range = %+v", r)
	}
	// Б7: Events · Tasks due · Notes — one GET form, Events ticked.
	show := page.Bar.Show
	if show == nil || show.Method != "GET" {
		t.Fatalf("the ticks of the bar = %+v", show)
	}
	var with *view.FormRow
	for i, r := range show.Sets[0].Rows {
		if r.Name == "_csrf" {
			t.Error("the GET form of the ticks carries the token")
		}
		if r.Name == "with" && r.Kind == view.InputCheck {
			with = &show.Sets[0].Rows[i]
		}
	}
	if with == nil || len(with.Options) != 3 || !with.Options[0].Selected || with.Options[0].Label != "Events" ||
		with.Options[1].Selected || with.Options[1].Label != "Tasks due" || with.Options[2].Label != "Notes" {
		t.Errorf("the ticks = %+v", with)
	}
	if len(page.Bar.Segments) != 0 {
		t.Errorf("the switches are links still: %+v", page.Bar.Segments)
	}
	// Б8: Compact rows.
	if page.Bar.Compact == nil {
		t.Error("no Compact rows on the agenda")
	}
	// Б9, Б10, Б12, Б13.
	day := page.Days[0]
	if day.Rubric != "Monday · 10 August" || day.Count != len(day.Rows) || day.Count == 0 {
		t.Errorf("the first day = %q, count %d of %d rows", day.Rubric, day.Count, len(day.Rows))
	}
	badges := map[string]view.Badge{}
	times := map[string]view.Text{}
	for _, r := range day.Rows {
		if len(r.Badges) > 0 {
			badges[string(r.Summary)] = r.Badges[0]
		}
		times[string(r.Summary)] = r.Time
	}
	for title, want := range map[string]view.Badge{
		"Синк-ап по релизу":       {Label: "4 attendees", Role: view.BadgeNeutral},
		"Дежурство: первая линия": {Label: "repeats weekly", Role: view.BadgeNeutral},
		"Созвон с подрядчиком":    {Label: "tentative", Role: view.BadgeAlert},
	} {
		if badges[title] != want {
			t.Errorf("%s: badge %+v, want %+v", title, badges[title], want)
		}
	}
	if times["Дежурство: первая линия"] != "all day" {
		t.Errorf("a day's time = %q, want «all day»", times["Дежурство: первая линия"])
	}
	// Б14, Б15: the source that did not answer, by name and in alarm; when
	// the answer was gathered; what is missing.
	if page.Poll == nil {
		t.Fatal("no poll")
	}
	danger := ""
	for _, r := range page.Poll.Summary {
		if r.Mark == view.RunDanger {
			danger = string(r.Text)
		}
	}
	if danger != "baikal · Общий unavailable" || strings.Contains(string(page.Poll.Summary[0].Text), "refused") {
		t.Errorf("poll = %+v", page.Poll.Summary)
	}
	if !strings.HasSuffix(string(page.Poll.Aside), "2 min ago") {
		t.Errorf("the poll's aside = %q", page.Poll.Aside)
	}
	if !strings.HasPrefix(string(page.Poll.Note), "What arrived in time is shown. Records from the unavailable source are missing") {
		t.Errorf("the poll's note = %q", page.Poll.Note)
	}
	// А1: the feeds' section says it is one.
	feeds := false
	for _, sec := range page.Rail.Sections {
		if sec.Account == "Subscriptions" {
			feeds = sec.Feeds
		}
	}
	if !feeds {
		t.Error("the rubric of the subscriptions is not marked a feeds' section")
	}

	// А2: one calendar refreshes too, and says how it is read.
	one := decode[view.AgendaPage](t, app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"?from=2026-08-10", app.sess)))
	if one.Top.Refresh.Action.Href == "" || !one.Top.Refresh.Action.Enabled || one.Top.Refresh.Answered == "" {
		t.Errorf("one calendar's refresh = %+v", one.Top.Refresh)
	}
	if len(one.Subtitle) == 0 || !strings.HasPrefix(string(one.Subtitle[0].Text), "nextcloud · 10 – 16 August 2026") {
		t.Errorf("one calendar's subtitle = %+v", one.Subtitle)
	}
	if one.Bar.Show == nil || one.Bar.Compact == nil || actionByID(one.Bar.Actions, "list.print") == nil {
		t.Errorf("one calendar's bar: show %v, compact %v, actions %+v", one.Bar.Show, one.Bar.Compact, one.Bar.Actions)
	}
}

// Б4: New event never writes into a calendar that is not ticked. The last
// used one, once unticked, is not where the button leads; a ticked one is.
func TestCombinedNewEventIsATickedCalendar(t *testing.T) {
	app := newMockupApp(t)
	key, _ := app.sess.DataKey()
	blob, _ := app.h.users.Catalog(app.sess.AccountID(), key)
	for _, c := range blob.Collections {
		if collectionSlugOf(c.Path) == "dni-rozhdeniya" {
			if err := app.h.users.SetLastUsed(app.sess.AccountID(), key, sealed.Selection{Kind: view.KindEvent, AccountID: c.AccountID, Path: c.Path}); err != nil {
				t.Fatal(err)
			}
		}
	}
	page := decode[view.AgendaPage](t, app.h.do(get("/app/calendar", app.sess)))
	a := actionByID(page.Bar.Actions, "record.new")
	if a == nil || strings.Contains(a.Href, "dni-rozhdeniya") {
		t.Fatalf("New event = %+v: Дни рождения is not ticked", a)
	}
	ticked := false
	for _, it := range a.Arrow {
		ticked = ticked || it.Href == strings.Replace(a.Href, "?pick=1", "", 1) || it.Href == a.Href
	}
	if !ticked {
		t.Errorf("New event leads to %s, none of the arrow's calendars %+v", a.Href, a.Arrow)
	}
}

// Б15, А2: nothing ticked — no poll, what to do instead, and a refresh that
// says why it has nothing to ask.
func TestAgendaNothingTicked(t *testing.T) {
	app := newMockupApp(t)
	if rec := app.h.do(postForm("/app/sources/calendar", app.sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("untick everything: %d", rec.Code)
	}
	page := decode[view.AgendaPage](t, app.h.do(get("/app/calendar", app.sess)))
	if page.Poll != nil {
		t.Errorf("nothing ticked and a poll: %+v", page.Poll)
	}
	if page.Empty == nil || len(page.Empty.Actions) != 1 || page.Empty.Actions[0].ID != "rail.tick.all" ||
		page.Empty.Actions[0].Label != "Tick every calendar" {
		t.Errorf("empty = %+v", page.Empty)
	}
	if r := page.Top.Refresh.Action; r.ID == "" || r.Enabled || r.Reason == "" {
		t.Errorf("refresh = %+v, want shown, off, with its reason", r)
	}
}
