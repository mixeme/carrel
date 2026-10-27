package web_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// Б1 of the audit of 11.10.2026 (docs/plan/scans/silent-choices.md): a
// record carried to another server was compared there by the properties of
// its main component alone. A reminder the other server did not keep, an
// occurrence changed by hand it no longer tied to its day — neither was
// seen, and the original, the only whole copy, was deleted. A write is read
// back whole wherever it is the reason to delete something (S-7.1).

const remindedSeries = "BEGIN:VEVENT\nUID:duty\nDTSTAMP:20260101T000000Z\nDTSTART:20260112T100000Z\nDTEND:20260112T110000Z\n" +
	"RRULE:FREQ=WEEKLY;COUNT=4\nSUMMARY:Duty\n" +
	"BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT15M\nDESCRIPTION:Duty soon\nEND:VALARM\nEND:VEVENT\n" +
	"BEGIN:VEVENT\nUID:duty\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID:20260119T100000Z\nDTSTART:20260119T120000Z\n" +
	"DTEND:20260119T130000Z\nSUMMARY:Duty moved\nEND:VEVENT"

func TestAMoveThatLosesPartOfTheRecordKeepsTheOriginal(t *testing.T) {
	for _, tc := range []struct {
		name  string
		drop  string // what the other server does not keep
		names string // what the notice names
	}{
		{"a reminder", "TRIGGER", "VALARM"},
		// The days of a series as days (Р-60, states-36), not by the
		// property the server lost.
		{"the day of an occurrence changed by hand", "RECURRENCE-ID", "one day of the series"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := calendarFixture(t, nil, "duty.ics", ics(remindedSeries))
			other := davtest.New(t)
			other.Login, other.Password = "bob", "other"
			team := davtest.CalendarHome + "team/"
			other.AddCalendar(caldav.Calendar{Path: team, Name: "Team"})
			other.Drop = []string{tc.drop}
			otherID := addAccount(t, f, other)

			record := f.path("calendar") + "/duty"
			q := url.Values{"to": {otherID}, "into": {team}}
			sheet := decode[view.MoveSheet](t, f.h.do(get(record+"/move?"+q.Encode(), f.sess)))
			page := decode[view.EventPage](t, f.h.do(postForm(confirmOf(t, sheet).Href, f.sess, url.Values{})))
			n := noticeTitled(page.Notices, "Copied, and the original kept")
			if n == nil || !strings.Contains(string(n.Body), tc.names) || strings.Contains(string(n.Body), "RECURRENCE-ID") {
				t.Fatalf("notices = %+v", page.Notices)
			}
			if _, _, ok := f.srv.Object(f.col + "duty.ics"); !ok {
				t.Error("the original was deleted")
			}
		})
	}
}

// The same of a whole collection: every record of it is read back whole
// before the original collection goes.
func TestACollectionMoveThatLosesPartOfARecordKeepsBoth(t *testing.T) {
	for _, tc := range []struct{ name, drop, names string }{
		{"a reminder", "TRIGGER", "VALARM"},
		{"the day of an occurrence changed by hand", "RECURRENCE-ID", "one day of the series"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := calendarFixture(t, nil, "duty.ics", ics(remindedSeries))
			other := davtest.New(t)
			other.Login, other.Password = "bob", "other"
			other.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "home/", Name: "Home"})
			other.Drop = []string{tc.drop}
			cf := colMoveFixture{davFixture: f, other: other, otherID: addAccount(t, f, other)}

			page := decode[view.ConnectionsPage](t, f.h.do(confirmCollection(t, cf, false)))
			n := noticeTitled(page.Notices, "Copied, and the original kept")
			if n == nil || n.Details == nil || !strings.Contains(string(n.Details.Lines[0]), tc.names) ||
				strings.Contains(string(n.Details.Lines[0]), "RECURRENCE-ID") {
				t.Fatalf("notices = %+v", page.Notices)
			}
			if _, ok := f.srv.Calendar(f.col); !ok {
				t.Error("the original was deleted")
			}
		})
	}
}

// A server that keeps the record whole in its own spelling has lost nothing:
// the move goes through.
func TestAMoveOfARemindedSeriesGoesThrough(t *testing.T) {
	f := calendarFixture(t, nil, "duty.ics", ics(remindedSeries))
	other := davtest.New(t)
	other.Login, other.Password = "bob", "other"
	team := davtest.CalendarHome + "team/"
	other.AddCalendar(caldav.Calendar{Path: team, Name: "Team"})
	otherID := addAccount(t, f, other)

	record := f.path("calendar") + "/duty"
	q := url.Values{"to": {otherID}, "into": {team}}
	sheet := decode[view.MoveSheet](t, f.h.do(get(record+"/move?"+q.Encode(), f.sess)))
	if resp := f.h.do(postForm(confirmOf(t, sheet).Href, f.sess, url.Values{})); resp.Code != 303 {
		t.Fatalf("move: %d %s", resp.Code, resp.Body.String())
	}
	moved, _, ok := other.Object(team + "duty.ics")
	if !ok {
		t.Fatalf("paths %v", other.Paths(team))
	}
	mustContain(t, moved, "TRIGGER:-PT15M", "RECURRENCE-ID:20260119T100000Z")
	if _, _, ok := f.srv.Object(f.col + "duty.ics"); ok {
		t.Error("the original is still there")
	}
}
