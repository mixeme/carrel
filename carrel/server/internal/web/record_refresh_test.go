package web_test

import (
	"regexp"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The client's stop C17: the top bar of a card and of its edit draws the
// refresh with «2 min ago» (s-contacts-6, s-contacts-3, s-agenda-2,
// s-tasks-2, s-notes-2), as on a list: the circle reads the record again,
// the words say when its collection was read.
func TestRecordPagesRefresh(t *testing.T) {
	frameRefresh := regexp.MustCompile(`<button class="tc-btn[^"]*"[^>]*><svg[^>]*><use href="#(i-refresh)"/></svg>([^<]*)</button>`)
	for _, id := range []string{"s-contacts-6", "s-contacts-3"} {
		if frameRefresh.FindStringSubmatch(mockupFrame(t, id)) == nil {
			t.Fatalf("%s draws no refresh: read the frame again", id)
		}
	}
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	card := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	check := func(what, here string, r view.Refresh) {
		t.Helper()
		if r.Action.Href != here || r.Action.Method != "GET" || r.Action.Icon != "i-refresh" || !r.Action.Enabled || r.Answered == "" || r.Running {
			t.Errorf("%s: refresh = %+v, want a GET to %s with when it was read", what, r, here)
		}
	}
	check("card", card, decode[view.ContactPage](t, app.h.do(get(card, app.sess))).Top.Refresh)
	check("edit", card+"/edit", decode[view.ContactEditPage](t, app.h.do(get(card+"/edit", app.sess))).Top.Refresh)
}
