package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Нс25, Нс26 of the scan of 15.10.2026 (s-profile-1): the card of a
// subscription says whether a password is saved for the link and shows it
// behind the same unlock as a connection's; its table «Lands in» is a row a
// kind — the kind, its tick, how many it holds — and one «Changed on the
// server» for all; a feed whose last fetch was refused is an alarm with the
// line of what the refusal means and Check again, and keeps the time it was
// last read.
func TestASubscriptionCardIsTheFrames(t *testing.T) {
	frame := mockupFrame(t, "s-profile-1")
	feeds := frame[strings.Index(frame, "Subscriptions · external"):]
	lands := regexp.MustCompile(`(?s)<span class="tc-table__filename">([^<]*)</span>`).FindAllStringSubmatch(feeds, -1)
	if len(lands) < 2 || lands[0][1] != "Calendar" || lands[1][1] != "Tasks" {
		t.Fatalf("s-profile-1 draws no rows of Lands in: read the frame again (%v)", lands)
	}
	alarm := feeds[strings.Index(feeds, `<div class="tc-card tc-card--alert">`):]
	m := regexp.MustCompile(`(?s)<div class="tc-card__alert-note">(.*?)</div>`).FindStringSubmatch(alarm)
	if m == nil || !strings.Contains(alarm, "password saved") || !strings.Contains(alarm, "Check again") {
		t.Fatal("s-profile-1 draws no refused feed: read the frame again")
	}
	note := frameText(m[1])

	f := feedHarness(t)
	feed := newFeedServer(t, holidays, "mix")
	sub := subscribe(t, f, url.Values{"url": {feed.URL + "/ru.ics"}, "login": {"mix"}, "password": {"pw"}})
	card := func() view.SubscriptionCard {
		t.Helper()
		p := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
		if len(p.Subscriptions) != 1 {
			t.Fatalf("subscriptions = %+v", p.Subscriptions)
		}
		return p.Subscriptions[0]
	}

	c := card()
	if c.Access != "password saved" || c.Password == nil || c.Password.Reveal.Label != "Show password" ||
		c.Password.Reveal.Href != "/app/settings/unlock?for="+sub.ID {
		t.Errorf("a feed behind a password = access %q, password %+v", c.Access, c.Password)
	}
	// Inside the unlock window the link's password is shown, as a
	// connection's is, and the journal says so.
	sheet := decode[view.UnlockSheet](t, f.h.do(get(c.Password.Reveal.Href, f.sess)))
	if sheet.Subject != c.Title {
		t.Errorf("the sheet asks for %q, the card is %q", sheet.Subject, c.Title)
	}
	if rec := f.h.do(fragment(postForm("/app/settings/unlock", f.sess, url.Values{"password": {"password123"}}), "sheet")); rec.Code != 200 {
		t.Fatalf("unlock: %d", rec.Code)
	}
	shown := decode[view.ConnectionsPage](t, f.h.do(get(card().Password.Reveal.Href, f.sess))).Subscriptions[0]
	if pw := shown.Password; pw == nil || !pw.Shown || pw.Value != "pw" || pw.Reveal.Label != "Hide password" || auditCount(f.h, "connection.password.reveal") != 1 {
		t.Errorf("shown = %+v, journal %d", pw, auditCount(f.h, "connection.password.reveal"))
	}
	if len(c.Lands) != 2 || c.Lands[0].Label != "Calendar" || c.Lands[1].Label != "Tasks" {
		t.Fatalf("lands in = %+v", c.Lands)
	}
	for _, l := range c.Lands {
		if l.Toggle.ID != "feed.tick" || l.Toggle.Method != "POST" || l.Objects != nil {
			t.Errorf("row %s before a read = %+v", l.Label, l)
		}
	}

	// Ticked and read: the count of each kind, the one Changed.
	for i, l := range card().Lands {
		if l.Tick == view.TickOn {
			continue
		}
		if rec := f.h.do(postForm(l.Toggle.Href, f.sess, url.Values{"pick": {l.Pick}})); rec.Code != 303 || rec.Header().Get("Location") != "/app/settings/connections" {
			t.Fatalf("tick %s: %d %q", l.Label, rec.Code, rec.Header().Get("Location"))
		}
		if card().Lands[i].Tick != view.TickOn {
			t.Fatalf("%s not ticked by its tick", l.Label)
		}
	}
	combined(t, f.h, f.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	combined(t, f.h, f.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	c = card()
	for _, l := range c.Lands {
		if l.Tick != view.TickOn || l.Objects == nil || *l.Objects != 1 {
			t.Errorf("row %s read = tick %q, objects %v", l.Label, l.Tick, l.Objects)
		}
	}
	read := c.Checked
	if read == "" || c.State != view.CollectionOK || c.Note != "" {
		t.Errorf("read card = checked %q, state %q, note %q", c.Checked, c.State, c.Note)
	}

	// The password stops fitting: the alarm, the line, Check again.
	feed.mu.Lock()
	feed.login = "somebody-else"
	feed.mu.Unlock()
	combined(t, f.h, f.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	c = card()
	if c.State != view.CollectionUnavailable || string(c.Note) != note || c.Checked != read || c.Actions[0].Label != "Check again" {
		t.Errorf("refused card = state %q, note %q, checked %q, %q; the frame draws %q", c.State, c.Note, c.Checked, c.Actions[0].Label, note)
	}

	// Untick Tasks from the card: off there, Calendar stays.
	if rec := f.h.do(postForm(c.Lands[1].Toggle.Href, f.sess, url.Values{"pick": {c.Lands[1].Pick}})); rec.Code != 303 {
		t.Fatalf("untick Tasks: %d", rec.Code)
	}
	if c = card(); c.Lands[0].Tick != view.TickOn || c.Lands[1].Tick != view.TickOff {
		t.Errorf("after untick = %q, %q", c.Lands[0].Tick, c.Lands[1].Tick)
	}
}

// A feed without a password says so, and has nothing to show.
func TestAFeedWithoutAPasswordSaysSo(t *testing.T) {
	f := feedHarness(t)
	feed := newFeedServer(t, holidays, "")
	subscribe(t, f, url.Values{"url": {feed.URL + "/ru.ics"}})
	p := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	if c := p.Subscriptions[0]; c.Access != "no password" || c.Password != nil {
		t.Errorf("a feed without a password = access %q, password %+v", c.Access, c.Password)
	}
}
