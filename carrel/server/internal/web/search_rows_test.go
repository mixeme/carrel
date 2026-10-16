package web_test

import (
	"regexp"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Пк12 of the scan of 15.10.2026 (s-find-1): a found contact is a row of a
// face — initials, the phone, the address, the organisation —, a found file
// a row of a file — its icon, its size, when it changed in numbers, whether
// it can be written. (An event's and a task's rows wait for the frames'
// answers, В1 and В2.)
func TestSearchRowsByKind(t *testing.T) {
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	search := func(q string) []view.FindRow {
		t.Helper()
		first := decode[view.SearchPage](t, app.h.do(get("/app/search?q="+q, app.sess)))
		waitPoll(t, app.h, app.sess, first.Poll.TaskID, nil)
		return decode[view.SearchPage](t, app.h.do(get("/app/search?q="+q, app.sess))).Results
	}
	var contact, found *view.FindRow
	for _, r := range search("Ковалёва") {
		if r.Kind == view.KindContact && contact == nil {
			contact = &r
		}
	}
	for _, r := range search("смета") {
		if r.Kind == view.KindFile && found == nil {
			found = &r
		}
	}
	if contact == nil || found == nil {
		t.Fatalf("contact %v, file %v", contact, found)
	}
	if contact.Avatar == nil || contact.Avatar.Initials == "" || contact.Subtitle == "" || contact.Detail == "" || contact.Caption == "" {
		t.Errorf("contact row = %+v", *contact)
	}
	if found.Icon == "" || found.Size == "" || !regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d$`).MatchString(string(found.Changed)) || found.Access != "rw" {
		t.Errorf("file row = %+v", *found)
	}
}
