package web_test

import "testing"

// states-1: the calendar of an instance with nothing connected — the
// section's name, «No server connected yet», the explanation with Connect
// account, «Nothing connected yet» under «Sources» in the rail. The instance
// is the frames' (their day, build and davloom), with no connection and no
// subscription made. Every section alike (kinds are equal; the owner from a
// phone, 24.11.2026: the text stood at another height in Contacts and
// Tasks, and Files was another page with a button not green): the frame is
// the calendar's, its make-up every section's. The files' root waits for
// its frame (Typecase's task 279: New folder in the rail's foot is off
// there), TestAnInstanceWithNothingConnectedSaysSo holds its model.
func TestFixtureNothingConnected(t *testing.T) {
	h := newHarness(t)
	h.cfg.Timezone = mockupZone
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	h.srv.SetClock(mockupNow(t))
	h.users.SetClock(mockupDayAt(t, mockupCreatedDay))
	h.srv.SetBuild(mockupVersion, mockupCommit)
	h.admin("mix", "password123")
	h.users.SetClock(mockupNow(t))
	app := &mockupApp{h: h, sess: h.login("mix", "password123"), byAccount: map[string]string{}, byPath: map[string]string{}}
	app.compare(t, "app-agenda-all/nothing.json", h.do(get("/app/calendar?from=2026-08-10", app.sess)))
	app.compare(t, "app-contacts/nothing.json", h.do(get("/app/contacts", app.sess)))
	app.compare(t, "app-tasks-all/nothing.json", h.do(get("/app/tasks", app.sess)))
	app.compare(t, "app-notes-all/nothing.json", h.do(get("/app/notes", app.sess)))
}
