package web_test

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/view"
)

// S-22: the state of a collection comes from what the session read — when a
// list was read, and the newest change the server reported among its records —
// on the record's source block and on the connection card, without a request
// of its own. An agenda reads a week: it tells when, not what changed; a whole
// read of another kind in the same calendar does.
func TestCollectionStateFromWhatWasRead(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchWork+"call.ics", ics("BEGIN:VEVENT\nUID:call\nDTSTAMP:20260101T000000Z\nDTSTART:20260910T150000Z\nSUMMARY:Call\nEND:VEVENT"))
	srv.Seed(searchWork+"todo.ics", ics("BEGIN:VTODO\nUID:todo\nDTSTAMP:20260101T000000Z\nSUMMARY:Write\nEND:VTODO"))
	f := connectFixture(t, srv, searchBook, nil)

	contacts := "/app/contacts/" + f.id + "/" + url.PathEscape(searchBook)
	calendar := "/app/calendar/" + f.id + "/" + url.PathEscape(searchWork)
	tasks := "/app/tasks/" + f.id + "/" + url.PathEscape(searchWork)
	year := strconv.Itoa(time.Now().Year())
	rows := func() map[string]view.CollectionRow {
		t.Helper()
		out := map[string]view.CollectionRow{}
		page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
		for _, r := range page.Accounts[0].Collections {
			out[string(r.Path)] = r
		}
		return out
	}

	if src := decode[view.ContactPage](t, f.h.do(get(contacts+"/ada", f.sess))).Source; src.LastRead != "" || src.Changed != "" {
		t.Errorf("before a list was read: %+v", src)
	}
	if r := rows()[searchBook]; r.LastRead != "" || r.Changed != "" {
		t.Errorf("row before a list was read: %+v", r)
	}

	f.h.do(get(contacts, f.sess))
	src := decode[view.ContactPage](t, f.h.do(get(contacts+"/ada", f.sess))).Source
	if src.LastRead != "just now" || !strings.Contains(string(src.Changed), year) {
		t.Errorf("after the list: %+v", src)
	}
	asked := len(srv.Requests())
	if r := rows()[searchBook]; r.LastRead != src.LastRead || r.Changed != src.Changed {
		t.Errorf("row = %+v, source block %+v", r, src)
	}
	if len(srv.Requests()) != asked {
		t.Error("the connection card asked the server")
	}

	f.h.do(get(calendar, f.sess))
	if ev := decode[view.EventPage](t, f.h.do(get(calendar+"/call", f.sess))).Source; ev.LastRead != "just now" || ev.Changed != "" {
		t.Errorf("after the agenda alone: %+v", ev)
	}
	f.h.do(get(tasks, f.sess))
	if ev := decode[view.EventPage](t, f.h.do(get(calendar+"/call", f.sess))).Source; ev.LastRead != "just now" || !strings.Contains(string(ev.Changed), year) {
		t.Errorf("after the tasks of the same calendar: %+v", ev)
	}
}
