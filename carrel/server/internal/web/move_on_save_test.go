package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/caldav"
)

// Р19 of scans/kinds-parity.md: a note's edit chooses its notebook, and
// saving into another moves it there (s-notes-20, S-18); an event's and a
// task's could only be moved by the sheet of Move. Now their forms have
// «Calendar» and «List» in the head, the collection they lie in chosen, a
// read-only one offered off with its reason; a save into another moves the
// record there with what was changed.
func TestEventsAndTasksMoveOnSave(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	work := davtest.CalendarHome + "work/"
	home := davtest.CalendarHome + "home/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: home, Name: "Home"})
	srv.Seed(work+"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\nEND:VEVENT"))
	srv.Seed(work+"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nEND:VTODO"))
	f := connectFixture(t, srv, work, nil)

	for _, c := range []struct{ list, uid, label, title string }{
		{"calendar", "e1", "Calendar", "Review again"},
		{"tasks", "t1", "List", "Slides again"},
	} {
		rec := f.path(c.list) + "/" + c.uid
		page := decode[struct {
			Form view.Form `json:"form"`
		}](t, f.h.do(get(rec+"/edit", f.sess)))
		row := formRowOf(page.Form, "notebook")
		if row == nil || string(row.Label) != c.label || len(row.Options) < 2 {
			t.Fatalf("%s: the collection = %+v, want %s with both", c.uid, row, c.label)
		}
		var there string
		for _, o := range row.Options {
			if o.Selected && o.Value != string(row.Value) {
				t.Errorf("%s: %q selected, the value %q", c.uid, o.Value, row.Value)
			}
			if !o.Selected {
				there = o.Value
			}
		}
		form := formFields(page.Form)
		form.Set("summary", c.title)
		form.Set("notebook", there)
		resp := f.h.do(postForm(rec+"/edit", f.sess, form))
		if resp.Code != 303 || !strings.HasPrefix(resp.Header().Get("Location"), there+"/"+c.uid) {
			t.Fatalf("%s: %d %s %s", c.uid, resp.Code, resp.Header().Get("Location"), resp.Body.String())
		}
		moved, _, ok := srv.Object(home + c.uid + ".ics")
		if !ok || !strings.Contains(unfolded(moved), "SUMMARY:"+c.title) {
			t.Errorf("%s: not moved with its change: %q", c.uid, moved)
		}
		if _, _, still := srv.Object(work + c.uid + ".ics"); still {
			t.Errorf("%s: left behind in Work", c.uid)
		}
	}
}
