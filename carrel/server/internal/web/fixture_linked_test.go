package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// C-14.2: the card of a linked record, /app/{kind}/linked/{uid}, of every
// kind (s-contacts-33, s-agenda-25, s-tasks-17, s-notes-32; from-typecase
// note 117 p. 5): what the person chose, each choice marked with the
// collection it came from, Edit to «Which copy to edit?», «⋯» of the linked
// record, where it is kept and what each copy holds otherwise.
func TestFixtureLinkedRecords(t *testing.T) {
	app := newMockupApp(t)
	linkMockupEvent(t, app)
	linkMockupTask(t, app)
	// «Поездка в Казань», which the task is related to (s-tasks-17: «·
	// event · 12 Aug», as its card s-tasks-2), read with the calendar.
	// The recording in the attachment folder, the note's second file
	// (s-notes-32, as s-notes-2).
	attachMockupRecording(t, app)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	openList(t, app, "/app/tasks")
	openList(t, app, "/app/notes")
	// Анна's timeline gathered, as on her card (s-contacts-6): the linked
	// record shows it too (Р-78, note 123 p. 10).
	rec := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	tl := decode[view.ContactTimelinePage](t, app.h.do(get(rec+"/timeline", app.sess)))
	waitPoll(t, app.h, app.sess, tl.Poll.TaskID, func(b view.PollBar) bool { return !b.Running })
	// The copy of «Синк-ап по релизу» in «Общий» answers: the frame draws
	// what it holds otherwise (s-agenda-25).
	app.servers["baikal"].Accept("REPORT")
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	for _, c := range []struct{ fixture, address string }{
		{"app-contact-linked/normal.json", "/app/contacts/linked/4f2b-9c31-a0e7"},
		{"app-event-linked/normal.json", "/app/calendar/linked/8f1c-4a02-b7d3"},
		{"app-task-linked/normal.json", "/app/tasks/linked/aa11-2d09-5f84"},
		{"app-note-linked/normal.json", "/app/notes/linked/5c81-77aa-0d64"},
	} {
		t.Run(c.fixture, func(t *testing.T) {
			// The note frames draw the full width chosen (s-notes-32, as
			// s-notes-2).
			app.compare(t, c.fixture, app.h.do(wideNotes(get(c.address, app.sess))))
		})
	}
}
