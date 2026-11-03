package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
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

// A subtask the server would not take is said in the owner's words of
// 03.11.2026, as a delete in Files is: how many of how many, the answer in
// the line when it is one for all, what was marked and what stays open;
// each subtask with its answer under «Still open».
func TestASubtaskNotClosedIsSaid(t *testing.T) {
	task := func(uid, title string, more ...string) string {
		return ics("BEGIN:VTODO\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nSUMMARY:" + title + strings.Join(append([]string{""}, more...), "\n") + "\nEND:VTODO")
	}
	f := calendarFixture(t, nil,
		"parent.ics", task("parent", "Release 2.0"),
		"notes.ics", task("notes", "Write the notes", "RELATED-TO:parent"),
		"letter.ics", task("letter", "Write the letter", "RELATED-TO:parent"),
		"tag.ics", task("tag", "Tag the build", "RELATED-TO:parent"),
	)
	f.srv.RefuseAt("PUT", f.col+"letter.ics", http.StatusForbidden, "")
	_, etag, _ := f.srv.Object(f.col + "parent.ics")
	href := f.path("tasks") + "/parent/edit?" + url.Values{"op": {"done"}, "etag": {etag}, "done": {"on"}}.Encode()
	rec := f.h.do(postForm(href, f.sess, url.Values{}))
	if rec.Code != http.StatusOK {
		t.Fatalf("mark: %d, want the card with the notice", rec.Code)
	}
	page := decode[view.TaskPage](t, rec)
	n := mustNotice(t, page.Notices, "Task done. Mark done failed for 1 of 3 subtasks: 403 Forbidden: this account has no write access here.")
	if string(n.Body) != "2 marked done, 1 still open." || n.Details == nil ||
		detailText(n.Details) != "Still open | Write the letter · 403 Forbidden: this account has no write access here." {
		t.Errorf("notice = %q / %+v", n.Body, n.Details)
	}
}
