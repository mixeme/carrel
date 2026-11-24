package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Нс23 of the scan of 15.10.2026 (s-profile-1, the card of baikal): a
// connection whose server refused the session's last read says so on its
// card — an alarm, «last answer refused» instead of a time, Test again, and
// under it the line of what the refusal means. A read that answers after it
// puts the card back.
func TestARefusedConnectionSaysSo(t *testing.T) {
	frame := mockupFrame(t, "s-profile-1")
	card := frame[strings.Index(frame, `<div class="tc-card tc-card--alert">`):]
	m := regexp.MustCompile(`(?s)<div class="tc-card__alert-note">(.*?)</div>`).FindStringSubmatch(card)
	if m == nil || !strings.Contains(card[:strings.Index(card, m[0])], "last answer refused") {
		t.Fatal("s-profile-1 draws no refused card with its line: read the frame again")
	}
	note := frameText(m[1])

	f := calendarFixture(t, nil)
	tickRail(t, f.h, f.sess, "calendar", f.path("calendar"))
	card0 := func() view.ConnectionCard {
		t.Helper()
		return decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))).Accounts[0]
	}

	f.srv.Refuse("REPORT", 401, "")
	combined(t, f.h, f.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	c := card0()
	if c.State != view.CollectionUnavailable || !c.Refused || c.Answered != "" || string(c.Note) != note {
		t.Errorf("refused card = state %q, refused %v, answered %q, note %q; the frame draws %q", c.State, c.Refused, c.Answered, c.Note, note)
	}
	if c.Actions[0].ID != "connection.test" || c.Actions[0].Label != "Test again" {
		t.Errorf("the first action = %+v, the frame draws Test again", c.Actions[0])
	}
	// Nothing to discover on a server that refuses the password: the
	// refused card draws Test again, Edit, Disconnect and no Update
	// collection list (s-profile-1, -27; acceptance round 7, 24.11.2026 —
	// the frame comparison let a longer row of buttons pass).
	for _, a := range c.Actions {
		if a.ID == "connection.rediscover" {
			t.Errorf("the refused card offers %s", a.Label)
		}
	}
	if !strings.Contains(card[:strings.Index(card, m[0])], "Test again") || strings.Contains(card[:strings.Index(card, m[0])], "Update collection list") {
		t.Error("s-profile-1's refused card: read the frame again")
	}

	f.srv.Accept("REPORT")
	combined(t, f.h, f.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	c = card0()
	if c.State != view.CollectionOK || c.Refused || c.Answered == "" || c.Note != "" || c.Actions[0].Label != "Test" {
		t.Errorf("answered again = state %q, refused %v, answered %q, note %q, %q", c.State, c.Refused, c.Answered, c.Note, c.Actions[0].Label)
	}
}
