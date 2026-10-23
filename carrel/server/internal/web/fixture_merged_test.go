package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// C-14.2: a merge done whole (s-dup-8; from-typecase note 117 p. 12): the
// card of the record that stays, «Merged. The others were deleted.», and the
// records that pointed at the one deleted — a task and a note made from the
// timeline of «Лебедева Вера» in «Контакты» —, each named, now pointing at
// the card of «Архив».
func TestFixtureMergedWithLinks(t *testing.T) {
	app := newMockupApp(t)
	nc := app.servers["nextcloud"]
	nc.Seed(davtest.CalendarHome+"dela/c7d2-5e10-ab43.ics", ics("BEGIN:VTODO\nUID:c7d2-5e10-ab43\nDTSTAMP:20260810T090000Z\n"+
		"SUMMARY:Звонок по каталогу библиотеки\nRELATED-TO:b52e-7d08-c6f1\nEND:VTODO"))
	nc.Seed(davtest.CalendarHome+"zapisi/d8e3-6f21-bc54.ics", ics("BEGIN:VJOURNAL\nUID:d8e3-6f21-bc54\nDTSTAMP:20260812T150000Z\n"+
		"DTSTART;VALUE=DATE:20260812\nSUMMARY:Встреча в библиотеке 12.08\nRELATED-TO:b52e-7d08-c6f1\nEND:VJOURNAL"))
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	page := decode[view.DuplicatesPage](t, app.h.do(get("/app/duplicates?kind=contact", app.sess)))
	var merge string
	for _, g := range page.Groups {
		if g.Decision == view.DecisionNone && strings.Contains(string(g.Rows[0].Title)+string(g.Rows[1].Title), "Лебедева") {
			merge = app.h.do(decideRequest(t, app.sess, g, "duplicates.merge", asItStands(g, nil))).Header().Get("Location")
		}
	}
	if merge == "" {
		t.Fatalf("no pair of Вера Лебедева to merge among %d groups", len(page.Groups))
	}
	plan := decode[view.DuplicatesMergePage](t, app.h.do(get(merge, app.sess)))
	if !strings.HasSuffix(string(plan.Target.Stamp), "Архив") {
		// The record of «Архив» stays (s-dup-2, -8).
		for _, a := range plan.Stays {
			if strings.HasSuffix(string(a.Label), "Архив") {
				plan = decode[view.DuplicatesMergePage](t, app.h.do(get(a.Href, app.sess)))
			}
		}
	}
	var confirm string
	for _, a := range plan.Actions {
		if a.ID == "duplicates.merge.confirm" {
			confirm = a.Href
		}
	}
	// Every ticked collection answers the search for links: the frame draws
	// the merge that pointed every one anew (s-dup-8); «Общий» of the set
	// refuses until asked otherwise.
	app.servers["baikal"].Accept("REPORT")
	app.servers["baikal"].Accept("PROPFIND")
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.h.do(postForm(confirm, app.sess, url.Values{}))
	if rec.Code != http.StatusOK {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body.String())
	}
	app.compare(t, "app-contact/merged.json", rec)
}
