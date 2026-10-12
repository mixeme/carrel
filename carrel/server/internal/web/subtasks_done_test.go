package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// А9 of the audit of 11.10.2026 (the owner's decision of 12.10.2026: a task
// marked done closes its open subtasks with it, without a question). Every
// level under it is closed; a subtask that repeats goes on to its next time
// and stays open, as a repeating task marked done does (А2); a task that
// only names it otherwise, and a task of its own, are left alone. Taking the
// mark off the parent opens nothing again.
func TestATaskMarkedDoneClosesItsSubtasks(t *testing.T) {
	task := func(uid, title string, more ...string) string {
		return ics("BEGIN:VTODO\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nSUMMARY:" + title + strings.Join(append([]string{""}, more...), "\n") + "\nEND:VTODO")
	}
	f := calendarFixture(t, nil,
		"parent.ics", task("parent", "Prepare the fair"),
		"stand.ics", task("stand", "Order the stand", "RELATED-TO:parent", "STATUS:COMPLETED", "COMPLETED:20260102T000000Z"),
		"leaflets.ics", task("leaflets", "Print the leaflets", "RELATED-TO;RELTYPE=PARENT:parent"),
		"proof.ics", task("proof", "Read the proof", "RELATED-TO:leaflets"),
		"weekly.ics", task("weekly", "Call the printer", "RELATED-TO:parent", "DTSTART:20260105T090000Z", "DUE:20260105T100000Z", "RRULE:FREQ=WEEKLY"),
		"sibling.ics", task("sibling", "Another fair", "RELATED-TO;RELTYPE=SIBLING:parent"),
		"other.ics", task("other", "Buy coal"),
	)
	mark := func(uid, done string) {
		t.Helper()
		_, etag, ok := f.srv.Object(f.col + uid + ".ics")
		if !ok {
			t.Fatalf("%s is not on the server", uid)
		}
		href := f.path("tasks") + "/" + uid + "/edit?" + url.Values{"op": {"done"}, "etag": {etag}, "done": {done}}.Encode()
		if rec := f.h.do(postForm(href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
			t.Fatalf("mark %s %s: %d %s", uid, done, rec.Code, rec.Body.String())
		}
	}
	done := func(uid string) bool {
		t.Helper()
		body, _, _ := f.srv.Object(f.col + uid + ".ics")
		return strings.Contains(unfolded(body), "STATUS:COMPLETED")
	}

	mark("parent", "on")
	for _, uid := range []string{"parent", "stand", "leaflets", "proof"} {
		if !done(uid) {
			t.Errorf("%s is still open", uid)
		}
	}
	for _, uid := range []string{"weekly", "sibling", "other"} {
		if done(uid) {
			t.Errorf("%s was closed", uid)
		}
	}
	if body, _, _ := f.srv.Object(f.col + "weekly.ics"); !strings.Contains(unfolded(body), "DTSTART:20260112T090000Z") {
		t.Errorf("the repeating subtask did not go on to its next time:\n%s", body)
	}
	if body, _, _ := f.srv.Object(f.col + "leaflets.ics"); !strings.Contains(unfolded(body), "COMPLETED:") || !strings.Contains(unfolded(body), "RELATED-TO;RELTYPE=PARENT:parent") {
		t.Errorf("the subtask closed is not whole:\n%s", body)
	}

	mark("parent", "off")
	if done("parent") || !done("leaflets") || !done("proof") {
		t.Errorf("after the mark is taken off: parent done %v, leaflets %v, proof %v — want the parent open and the subtasks as they were", done("parent"), done("leaflets"), done("proof"))
	}
}
