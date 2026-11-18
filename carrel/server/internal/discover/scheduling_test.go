package discover

import (
	"slices"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// S-15.4, S-6: a rewritten attendee can set off letters, and only a server
// that schedules sends them (RFC 6638: its principal has a
// schedule-outbox-URL). Discovery asks the principal once, and with it for
// the addresses that are the person's own (calendar-user-address-set): a
// meeting organised at one of them is the person's, not somebody else's.
func TestDiscoveryAsksWhetherThePrincipalSchedules(t *testing.T) {
	srv := davtest.New(t)
	srv.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work"})
	srv.ScheduleOutbox = davtest.Principal + "outbox/"
	srv.UserAddresses = []string{"MAILTO:Mix@Example.org", "/alice/", "mailto:mix@work.example"}

	res, trace, err := Run(t.Context(), loopbackGuard(), srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Scheduling != SchedulingYes {
		t.Errorf("scheduling = %q, want %q", res.Scheduling, SchedulingYes)
	}
	if want := []string{"mix@example.org", "mix@work.example"}; !slices.Equal(res.Addresses, want) {
		t.Errorf("addresses = %q, want %q: mailto addresses only, lower-cased and without the scheme", res.Addresses, want)
	}
	if !traceHas(trace, stepScheduling, view.CheckOK) {
		t.Errorf("the question belongs on the trace: %+v", trace.Steps)
	}

	// A principal that answers without an outbox does not schedule.
	plain := davtest.New(t)
	plain.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work"})
	res, _, err = Run(t.Context(), loopbackGuard(), plain.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Scheduling != SchedulingNo || len(res.Addresses) != 0 {
		t.Errorf("without an outbox: scheduling %q, addresses %q", res.Scheduling, res.Addresses)
	}
}

// A server with no principal cannot say (Mail.ru): the answer is unknown, not
// «no» — the sheet then says the server did not say (s-contacts-15).
func TestNoPrincipalLeavesSchedulingUnknown(t *testing.T) {
	res, _, err := runAgainst(t, fakeDAV{dirs: []string{"/", "/docs/"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Scheduling != SchedulingUnknown {
		t.Errorf("scheduling = %q, want unknown", res.Scheduling)
	}
}

// DAViCal keeps the scheduling boxes in the calendar home and calls them
// calendars (.in/ and .out/), resourcetype calendar and nothing of RFC 6638
// (acceptance on our test servers, 18.11.2026: «Carrel Inbox» and «Carrel
// Outbox» stood in every rail). The principal names them; discovery leaves
// them out and says so on the trace.
func TestTheSchedulingBoxesAreNotCollections(t *testing.T) {
	srv := davtest.New(t)
	in, out := davtest.CalendarHome+"inbox/", davtest.CalendarHome+"outbox/"
	srv.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: in, Name: "Carrel Inbox"})
	srv.AddCalendar(caldav.Calendar{Path: out, Name: "Carrel Outbox"})
	srv.ScheduleInbox, srv.ScheduleOutbox = in, out

	res, trace, err := Run(t.Context(), loopbackGuard(), srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range res.Collections {
		names = append(names, c.DisplayName)
	}
	if !slices.Equal(names, []string{"Work"}) {
		t.Errorf("collections = %q, want Work alone", names)
	}
	said := false
	for _, s := range trace.Steps {
		said = said || s.Detail == "2 scheduling boxes (inbox, outbox) left out"
	}
	if !said || res.Scheduling != SchedulingYes {
		t.Errorf("trace = %+v, scheduling %q", trace.Steps, res.Scheduling)
	}
}
