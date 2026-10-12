package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б8 of the audit of 11.10.2026: an edit refused because the record changed
// elsewhere, then written from the conflict screen with «Use only my edit»,
// is the edit the person made — also the attendee typed into the line, the
// attendee unticked, and a task's «All day».
func TestAnEditWrittenFromTheConflictScreenIsTheEdit(t *testing.T) {
	refuse := func(t *testing.T, f davFixture, kind, file, changed string, set url.Values) string {
		t.Helper()
		rec := f.path(kind) + "/e"
		form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		f.srv.Seed(f.col+file, changed)
		for k, v := range set {
			if len(v) == 1 && v[0] == "" {
				delete(form, k)
				continue
			}
			form[k] = v
		}
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/conflict" {
			t.Fatalf("save: %d %s, want the conflict screen", resp.Code, resp.Header().Get("Location"))
		}
		page := decode[view.ConflictPage](t, f.h.do(get(rec+"/conflict", f.sess)))
		if resp := f.h.do(postForm(actionHref(t, page.Actions, "conflict.mine"), f.sess, url.Values{})); resp.Code != http.StatusSeeOther {
			t.Fatalf("use my edit: %d %s", resp.Code, resp.Body.String())
		}
		return unfolded(body(t, f, file))
	}
	t.Run("attendees", func(t *testing.T) {
		event := func(place string) string {
			return ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART:20260112T100000Z\nDTEND:20260112T110000Z\nSUMMARY:Sync\n" + place +
				"ATTENDEE;CN=Bob:mailto:bob@example.com\nATTENDEE;CN=Dan:mailto:dan@example.com\nEND:VEVENT")
		}
		f := calendarFixture(t, nil, "e.ics", event(""))
		after := refuse(t, f, "calendar", "e.ics", event("LOCATION:Written elsewhere\n"),
			url.Values{"attendee": {"mailto:dan@example.com"}, "attendee.add": {"carol@example.com"}})
		mustContain(t, after, "mailto:dan@example.com", "mailto:carol@example.com", "LOCATION:Written elsewhere")
		if strings.Contains(after, "bob@example.com") {
			t.Errorf("the attendee taken away is still there:\n%s", after)
		}
	})
	t.Run("a task's whole day", func(t *testing.T) {
		task := func(place string) string {
			return ics("BEGIN:VTODO\nUID:e\nDTSTAMP:20260101T000000Z\nSUMMARY:Ship\n" + place + "DUE:20260112T180000Z\nEND:VTODO")
		}
		f := calendarFixture(t, nil, "e.ics", task(""))
		after := refuse(t, f, "tasks", "e.ics", task("LOCATION:Written elsewhere\n"), url.Values{"allday": {"on"}})
		mustContain(t, after, "DUE;VALUE=DATE:20260112", "LOCATION:Written elsewhere")
	})
}
