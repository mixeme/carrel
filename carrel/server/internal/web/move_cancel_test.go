package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// А12 of the audit of 11.10.2026 (the owner's decision of 12.10.2026: say
// that the people may be told of a cancellation, and take on nothing more).
// An event with attendees that a move takes off a server which schedules
// (RFC 6638, the principal's schedule-outbox-URL) may be announced to them
// as cancelled: the sheet says so, in «may», when the move goes to another
// account. A copy, an event without attendees, a server that does not
// schedule, and a move inside the account say nothing — and nothing is said
// of invitations anywhere.
func TestAMoveSaysTheAttendeesMayHearOfACancellation(t *testing.T) {
	events := map[string]string{
		"meet.ics": ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T090000Z\nSUMMARY:Planning\n" +
			"ORGANIZER:mailto:alice@example.com\nATTENDEE:mailto:anna@example.com\nEND:VEVENT"),
		"solo.ics": ics("BEGIN:VEVENT\nUID:solo\nDTSTAMP:20260101T000000Z\nDTSTART:20260811T090000Z\nSUMMARY:Alone\nEND:VEVENT"),
	}
	build := func(schedules bool) (davFixture, string, string) {
		srv := searchServer(t)
		if schedules {
			srv.ScheduleOutbox = davtest.Principal + "outbox/"
		}
		side := davtest.CalendarHome + "side/"
		srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
		srv.AddCalendar(caldav.Calendar{Path: side, Name: "Side"})
		for name, body := range events {
			srv.Seed(searchWork+name, body)
		}
		f := connectFixture(t, srv, searchWork, nil)
		other := davtest.New(t)
		other.Login, other.Password = "alice", "secret"
		away := davtest.CalendarHome + "away/"
		other.AddCalendar(caldav.Calendar{Path: away, Name: "Away"})
		if rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{
			"server": {other.URL}, "login": {other.Login}, "password": {other.Password},
		})); rec.Code != http.StatusSeeOther {
			t.Fatalf("connect the other server: %d %s", rec.Code, rec.Body.String())
		}
		key, _ := f.sess.DataKey()
		blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
		otherID := ""
		for _, conn := range blob.Connections {
			if conn.ID != f.id {
				otherID = conn.ID
			}
		}
		return f, otherID + "|" + away, f.id + "|" + side
	}
	sheet := func(f davFixture, uid, target string, copying bool) view.MoveSheet {
		t.Helper()
		to, into, _ := strings.Cut(target, "|")
		q := url.Values{"to": {to}, "into": {into}}
		if copying {
			q.Set("copy", "1")
		}
		return decode[view.MoveSheet](t, f.h.do(get("/app/calendar/"+f.id+"/"+url.PathEscape(searchWork)+"/"+uid+"/move?"+q.Encode(), f.sess)))
	}

	f, away, side := build(true)
	said := sheet(f, "meet", away, false).Cancels
	if !strings.Contains(string(said), "may") || !strings.Contains(string(said), "cancel") || strings.Contains(strings.ToLower(string(said)), "invit") {
		t.Errorf("a move of a meeting off a server that schedules says %q, want that its attendees may hear of a cancellation, and nothing of invitations", said)
	}
	for name, s := range map[string]view.MoveSheet{
		"a copy":                     sheet(f, "meet", away, true),
		"an event without attendees": sheet(f, "solo", away, false),
		"a move inside the account":  sheet(f, "meet", side, false),
	} {
		if s.Cancels != "" {
			t.Errorf("%s says %q, want nothing", name, s.Cancels)
		}
	}
	// A calendar's move says the same of its meetings, the last sentence of
	// what it does (note 112 p. 5); its copy says nothing.
	cancelOf := func(w view.Runs) view.Text {
		said := saidRuns(w)
		i := strings.Index(said, " may tell the attendees")
		if i < 0 {
			return ""
		}
		return view.Text(said[strings.LastIndex(said[:i], ". ")+2:])
	}
	calendar := func(f davFixture, copying bool) view.CollectionMoveSheet {
		t.Helper()
		q := url.Values{"a": {f.id}, "c": {searchWork}}
		if copying {
			q.Set("copy", "1")
		}
		return decode[view.CollectionMoveSheet](t, f.h.do(get("/app/collections/move?"+q.Encode(), f.sess)))
	}
	if said := cancelOf(calendar(f, false).Warning); !strings.Contains(string(said), "may") || !strings.Contains(string(said), "cancel") {
		t.Errorf("a calendar's move off a server that schedules says %q", said)
	}
	if said := cancelOf(calendar(f, true).Warning); said != "" {
		t.Errorf("a calendar's copy says %q, want nothing", said)
	}
	// Deleting the calendar takes its meetings off the server as moving it
	// does (Кл18).
	deleting := func(f davFixture) view.CollectionDeleteSheet {
		t.Helper()
		return decode[view.CollectionDeleteSheet](t, f.h.do(get("/app/collections/delete?"+url.Values{"a": {f.id}, "c": {searchWork}}.Encode(), f.sess)))
	}
	if said := cancelOf(deleting(f).Warning); said != cancelOf(calendar(f, false).Warning) {
		t.Errorf("a calendar's delete off a server that schedules says %q, its move %q", said, cancelOf(calendar(f, false).Warning))
	}

	plain, away, _ := build(false)
	if s := sheet(plain, "meet", away, false); s.Cancels != "" {
		t.Errorf("a server that does not schedule says %q, want nothing", s.Cancels)
	}
	if said := cancelOf(calendar(plain, false).Warning); said != "" {
		t.Errorf("a calendar's move off a server that does not schedule says %q, want nothing", said)
	}
	if said := cancelOf(deleting(plain).Warning); said != "" {
		t.Errorf("a calendar's delete off a server that does not schedule says %q, want nothing", said)
	}
}
