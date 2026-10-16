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

// Note 71 of Typecase (task 175): a new event's and a new task's form has
// its Calendar and its List too (s-agenda-15, s-tasks-14), the one it was
// opened in chosen; another chosen, the record is made there.
func TestANewRecordIsMadeWhereItsFormChose(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	work := davtest.CalendarHome + "work/"
	home := davtest.CalendarHome + "home/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: home, Name: "Home"})
	f := connectFixture(t, srv, work, nil)

	for _, c := range []struct{ list, label, title string }{
		{"calendar", "Calendar", "Planning"},
		{"tasks", "List", "Buy stamps"},
	} {
		page := decode[struct {
			Form view.Form `json:"form"`
		}](t, f.h.do(get(f.path(c.list)+"/new", f.sess)))
		row := formRowOf(page.Form, "notebook")
		if row == nil || string(row.Label) != c.label || string(row.Value) != f.path(c.list) {
			t.Fatalf("%s: the collection = %+v, want %s on %s", c.list, row, c.label, f.path(c.list))
		}
		var there string
		for _, o := range row.Options {
			if !o.Selected {
				there = o.Value
			}
		}
		form := formFields(page.Form)
		form.Set("summary", c.title)
		form.Set("notebook", there)
		resp := f.h.do(postForm(f.path(c.list)+"/new", f.sess, form))
		at := resp.Header().Get("Location")
		if resp.Code != 303 || !strings.HasPrefix(at, there+"/") {
			t.Fatalf("%s: %d %s %s", c.list, resp.Code, at, resp.Body.String())
		}
		uid := strings.TrimPrefix(at, there+"/")
		if i := strings.IndexAny(uid, "/?"); i >= 0 {
			uid = uid[:i]
		}
		made, _, ok := srv.Object(home + uid + ".ics")
		if !ok || !strings.Contains(unfolded(made), "SUMMARY:"+c.title) {
			t.Errorf("%s: not made in Home: %q", c.list, made)
		}
		if _, _, in := srv.Object(work + uid + ".ics"); in {
			t.Errorf("%s: made in Work as well", c.list)
		}
	}
}
