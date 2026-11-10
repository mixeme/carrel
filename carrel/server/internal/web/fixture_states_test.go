package web_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// C-14.3: the states of the tasks and the notes that their frames draw
// (Typecase task 97): a list the stand refuses in the combined view
// (states-20, -27), a list with nothing in it (states-21, -28), a list this
// account may only read (states-23, -30), and a note whose save the server
// kept only in part (states-31). The polls still running are taken with the
// calendar's (TestFixtureLossAndPartialOfTheOtherKinds).
func TestFixtureStatesOfTasksAndNotes(t *testing.T) {
	for _, one := range []struct{ dir, segment, slug, stamp string }{
		{"app-tasks-all", "tasks", "rabota", "nextcloud · Работа"},
		{"app-notes-all", "notes", "dnevnik", "nextcloud · Дневник"},
	} {
		// unavailable: one list of nextcloud refuses its read, the other
		// answers, and the poll names the one missing (states-20, -27:
		// «REPORT …/rabota/ → 503 Service Unavailable»).
		t.Run(one.dir+"-unavailable", func(t *testing.T) {
			app := newMockupApp(t)
			// The section was read an hour ago, as for the poll still
			// running: the rail keeps the tags of the list that refuses now
			// (states-20 «релиз», -27 «книги»), and what was read then is
			// too old to answer from.
			app.h.srv.SetClock(mockupNow(t).Add(-time.Hour))
			openList(t, app, "/app/"+one.segment)
			// The list changed on its server since: its change tag moved,
			// so the read is asked again, and it is the read that refuses.
			nc := app.servers["nextcloud"]
			paths := nc.Paths(davtest.CalendarHome + one.slug + "/")
			if len(paths) == 0 {
				t.Fatalf("%s is empty: read the set again", one.slug)
			}
			body, _, _ := nc.Object(paths[0])
			nc.Seed(paths[0], body)
			nc.RefuseAt("REPORT", app.realPath(t, one.slug), 503, "the server is busy")
			defer nc.Accept("REPORT")
			app.h.srv.SetClock(mockupNow(t))
			first := app.h.do(get("/app/"+one.segment, app.sess))
			task := taskOf(t, first)
			if task == "" {
				t.Fatal("the combined view polls nothing")
			}
			waitPoll(t, app.h, app.sess, task, func(b view.PollBar) bool {
				refused := false
				for _, sp := range b.Sources {
					switch {
					case sp.State == view.SourceWaiting || sp.State == view.SourceQuerying:
						return false
					case sp.State == view.SourceDone && sp.Count == nil:
						return false
					case strings.HasPrefix(string(sp.Stamp), one.stamp) && sp.State != view.SourceDone:
						refused = true
					}
				}
				return refused
			})
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			if one.segment == "tasks" {
				// «Продлить страховку» just marked done from its row, as
				// in s-tasks-1: it stays under Open, crossed (states-19,
				// -20).
				tickMockupTask(t, app, decode[view.TasksPage](t, app.h.do(get("/app/tasks?task="+task, app.sess))), "Продлить страховку")
			}
			app.compare(t, one.dir+"/unavailable.json", app.h.do(get("/app/"+one.segment+"?task="+task, app.sess)))
		})
	}
	for _, one := range []struct{ dir, segment, slug, comp string }{
		// empty: «Работа» holds no task — its events stay, the list of
		// tasks is empty (states-21); «Дневник» holds no note (states-28).
		{"app-tasks", "tasks", "rabota", "BEGIN:VTODO"},
		{"app-notes", "notes", "dnevnik", "BEGIN:VJOURNAL"},
	} {
		t.Run(one.dir+"-empty", func(t *testing.T) {
			app := newMockupApp(t)
			nc := app.servers["nextcloud"]
			gone := 0
			for _, p := range nc.Paths(davtest.CalendarHome + one.slug + "/") {
				if body, _, ok := nc.Object(p); ok && strings.Contains(body, one.comp) {
					nc.Unseed(p)
					gone++
				}
			}
			if gone == 0 {
				t.Fatalf("%s holds no %s: read the set again", one.slug, one.comp)
			}
			// The section was opened first, as a person comes to the list:
			// the rail carries its counts and tags.
			openList(t, app, "/app/"+one.segment)
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			app.compare(t, one.dir+"/empty.json", app.h.do(get(app.listPath(t, one.segment, one.slug), app.sess)))
		})
	}
	// readonly: the feed «Праздники РФ» in the tasks — a feed can only be
	// read (states-23) —, and «Общие с семьёй», whose server granted no
	// write (states-30).
	t.Run("app-tasks-readonly", func(t *testing.T) {
		app := newMockupApp(t)
		openList(t, app, "/app/tasks")
		feed := ""
		key, _ := app.sess.DataKey()
		blob, _ := app.h.users.Catalog(app.sess.AccountID(), key)
		for _, s := range blob.Subscriptions {
			if s.Name == "Праздники РФ" {
				feed = "/app/tasks/" + s.ID + "/" + url.PathEscape(s.FeedCollection().Path)
			}
		}
		if feed == "" {
			t.Fatal("the set has no «Праздники РФ»")
		}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-tasks/readonly.json", app.h.do(get(feed, app.sess)))
	})
	t.Run("app-notes-readonly", func(t *testing.T) {
		app := newMockupApp(t)
		openList(t, app, "/app/notes")
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-notes/readonly.json", app.h.do(get(app.listPath(t, "notes", "zapiski-obshchie"), app.sess)))
	})
	// loss: the note of s-notes-20 saved, and the server did not keep two
	// properties of other clients it carried (states-31). The change is one
	// the frame cannot show: the title was «Заметка со встречи» on the
	// stand, and the person wrote the date after it.
	t.Run("app-note-edit-loss", func(t *testing.T) {
		app := newMockupApp(t)
		attachMockupRecording(t, app)
		openList(t, app, "/app/calendar?from=2026-08-10")
		openList(t, app, "/app/tasks")
		openList(t, app, "/app/notes")
		nc := app.servers["nextcloud"]
		p := davtest.CalendarHome + "zapisi/5c81-77aa-0d64.ics"
		body, _, ok := nc.Object(p)
		if !ok || !strings.Contains(body, "SUMMARY:Заметка со встречи 10.08\r\n") {
			t.Fatal("the note is not the set's: read the set again")
		}
		body = strings.Replace(body, "SUMMARY:Заметка со встречи 10.08\r\n", "SUMMARY:Заметка со встречи\r\n", 1)
		at := strings.Index(body, "END:VJOURNAL")
		nc.Seed(p, body[:at]+"X-EVOLUTION-MEMO-COLOR:#4A6B52\r\nX-CARREL-UNKNOWN-FLAG:1\r\n"+body[at:])
		app.h.do(get(app.listPath(t, "notes", "zapisi"), app.sess))
		rec := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64"
		doc := decode[view.NotePage](t, app.h.do(wideNotes(get(rec+"/edit", app.sess)))).Doc
		save, err := url.Parse(actionHref(t, doc.Bar, "record.save"))
		if err != nil {
			t.Fatal(err)
		}
		// Everything the form holds goes back, as a client sends it.
		form := url.Values{}
		for _, row := range doc.Fields {
			switch {
			case row.Name == "":
			case row.Multiple:
				for _, o := range row.Options {
					if o.Selected {
						form.Add(row.Name, o.Value)
					}
				}
			default:
				form.Set(row.Name, string(row.Value))
			}
		}
		for k, v := range save.Query() {
			form[k] = v
		}
		form.Set("summary", "Заметка со встречи 10.08")
		nc.Drop = []string{"X-EVOLUTION-MEMO-COLOR", "X-CARREL-UNKNOWN-FLAG"}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-note-edit/loss.json", app.h.do(wideNotes(postForm(save.EscapedPath(), app.sess, form))))
	})
}
