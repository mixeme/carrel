package web_test

import (
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// C-10, owner's decision 24.09.2026: two calendars of one connection under
// one name each carry their address name in brackets — in the rail, and in
// every stamp, which is what a combined view's rows and poll show. A book of
// the same name meets neither in a rail and keeps its name as it is; so does
// a calendar whose name is its own.
func TestTwinCollectionsCarryTheirAddressName(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	first := davtest.CalendarHome + "carrel-s-15/"
	second := davtest.CalendarHome + "carrel-s-15-2/"
	srv.AddCalendar(caldav.Calendar{Path: first, Name: "Carrel S-15"})
	srv.AddCalendar(caldav.Calendar{Path: second, Name: "Carrel S-15"})
	srv.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work"})
	srv.AddAddressBook(carddav.AddressBook{Path: davtest.AddressBookHome + "carrel-s-15/", Name: "Carrel S-15"})
	srv.Seed(second+"e.ics", ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART:20260910T100000Z\nSUMMARY:In the second\nEND:VEVENT"))
	f := connectFixture(t, srv, second, nil)

	page := decode[view.AgendaPage](t, f.h.do(get(f.path("calendar")+"?from=2026-09-07", f.sess)))
	railRow(t, page.Rail, "Carrel S-15 (carrel-s-15)")
	railRow(t, page.Rail, "Carrel S-15 (carrel-s-15-2)")
	railRow(t, page.Rail, "Work")
	book := decode[view.ContactsListPage](t, f.h.do(get("/app/contacts/"+f.id+"/"+strings.ReplaceAll(davtest.AddressBookHome+"carrel-s-15/", "/", "%2F"), f.sess)))
	railRow(t, book.Rail, "Carrel S-15")

	tickRail(t, f.h, f.sess, "calendar", f.path("calendar"))
	agenda := combined(t, f.h, f.sess, "/app/calendar?from=2026-09-07", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	var stamps []string
	for _, d := range agenda.Days {
		for _, r := range d.Rows {
			stamps = append(stamps, string(r.Stamp))
		}
	}
	if len(stamps) != 1 || !strings.HasSuffix(stamps[0], " · Carrel S-15 (carrel-s-15-2)") {
		t.Errorf("stamps = %v", stamps)
	}
}
