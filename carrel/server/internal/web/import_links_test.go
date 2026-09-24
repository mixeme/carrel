package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// runImportOf takes a file into the list of segment and runs it.
func runImportOf(t *testing.T, f davFixture, segment, name, body string) view.ImportPage {
	t.Helper()
	page := decode[view.ImportPage](t, f.h.do(get(f.path(segment)+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, name, []byte(body))))
	page = decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{})))
	if page.Report == nil {
		t.Fatalf("no report: %+v", page)
	}
	return page
}

// storedWith is the body of the one record in collection whose text holds
// needle.
func storedWith(t *testing.T, f davFixture, collection, needle string) string {
	t.Helper()
	for _, p := range f.srv.Paths(collection) {
		if body, _, ok := f.srv.Object(p); ok && strings.Contains(body, needle) {
			return body
		}
	}
	t.Fatalf("nothing in %s holds %q", collection, needle)
	return ""
}

var uidLineOf = regexp.MustCompile(`(?m)^UID:(\S+)`)

// S-19, owner's decision 24.09.2026: links inside an imported file travel
// with it. The parent task's identity is taken in the calendar, so it is
// written under a fresh one — and its subtask, from the same file, points at
// the fresh one. A link to an identity the file does not hold goes as it
// came, and a record already on the server that named the taken identity is
// not touched: where that identity came from is not known.
func TestImportKeepsLinksInsideTheFile(t *testing.T) {
	onServer := ics("BEGIN:VTODO\nUID:parent\nDTSTAMP:20260101T000000Z\nSUMMARY:Already here\nEND:VTODO")
	pointing := ics("BEGIN:VTODO\nUID:old-child\nDTSTAMP:20260101T000000Z\nSUMMARY:Points at parent\nRELATED-TO:parent\nEND:VTODO")
	f := calendarFixture(t, nil, "parent.ics", onServer, "old-child.ics", pointing)

	file := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n" +
		"BEGIN:VTODO\r\nUID:parent\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Imported parent\r\nEND:VTODO\r\n" +
		"BEGIN:VTODO\r\nUID:child\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Imported child\r\nRELATED-TO;RELTYPE=PARENT:parent\r\nEND:VTODO\r\n" +
		"BEGIN:VTODO\r\nUID:loner\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Imported loner\r\nRELATED-TO:elsewhere\r\nEND:VTODO\r\n" +
		"END:VCALENDAR\r\n"
	page := runImportOf(t, f, "tasks", "tasks.ics", file)
	if page.Report.Created != 3 || page.Report.Renamed != 1 {
		t.Fatalf("report = %+v", page.Report)
	}

	parent := storedWith(t, f, f.col, "Imported parent")
	fresh := uidLineOf.FindStringSubmatch(parent)[1]
	if fresh == "parent" {
		t.Fatalf("the taken identity was written again: %q", parent)
	}
	child := storedWith(t, f, f.col, "Imported child")
	if !strings.Contains(child, "RELATED-TO;RELTYPE=PARENT:"+fresh) {
		t.Errorf("the subtask does not follow its parent to %s:\n%s", fresh, child)
	}
	if row := tableRow(t, page.Report.Rows, "Created", "Imported child"); !strings.Contains(string(row[2]), "follow their new identities") {
		t.Errorf("child row = %q", row[2])
	}
	if loner := storedWith(t, f, f.col, "Imported loner"); !strings.Contains(loner, "RELATED-TO:elsewhere") {
		t.Errorf("a link outside the file changed:\n%s", loner)
	}
	if old := storedWith(t, f, f.col, "Points at parent"); !strings.Contains(old, "RELATED-TO:parent") {
		t.Errorf("a record already on the server was relinked:\n%s", old)
	}
}

// The same when the server, not Carrel, gives the identities (Google,
// SV-25): the links are known to be stale only after the write, so the
// records that hold them are written once more, with If-Match. A group's
// MEMBER and a card's RELATED name the members by urn:uuid:; the form and
// the parameters stay.
func TestImportLinksFollowTheServersIdentities(t *testing.T) {
	f := importFixture(t)
	f.srv.CreateRewritesUID = true
	file := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:grace\r\nFN:Grace Hopper\r\nEND:VCARD\r\n" +
		"BEGIN:VCARD\r\nVERSION:4.0\r\nUID:team\r\nKIND:group\r\nFN:Team\r\nMEMBER:urn:uuid:grace\r\nEND:VCARD\r\n" +
		"BEGIN:VCARD\r\nVERSION:4.0\r\nUID:bob\r\nFN:Bob Builder\r\nRELATED;TYPE=friend:urn:uuid:grace\r\nEND:VCARD\r\n"
	page := runImportOf(t, f, "contacts", "cards.vcf", file)
	if page.Report.Created != 3 {
		t.Fatalf("report = %+v", page.Report)
	}

	grace := storedWith(t, f, peopleBook, "Grace Hopper")
	theirs := uidLineOf.FindStringSubmatch(grace)[1]
	if theirs == "grace" {
		t.Fatalf("the stand did not rewrite the UID: %q", grace)
	}
	if team := storedWith(t, f, peopleBook, "FN:Team"); !strings.Contains(team, "MEMBER:urn:uuid:"+theirs) {
		t.Errorf("the group does not name its member as the server does (%s):\n%s", theirs, team)
	}
	bob := storedWith(t, f, peopleBook, "Bob Builder")
	if !strings.Contains(bob, "urn:uuid:"+theirs) || !strings.Contains(strings.ToLower(bob), "type=friend") {
		t.Errorf("the friend link lost its target or its type (%s):\n%s", theirs, bob)
	}
	if row := tableRow(t, page.Report.Rows, "Created", "Bob"); !strings.Contains(string(row[2]), "the server gave them") {
		t.Errorf("bob row = %q", row[2])
	}
}

// Two records of a file under one identity: the first keeps it, the second
// is written under a fresh one, and a link to that identity leads to the
// first — the order of the file decides, and nothing is moved.
func TestImportLinksToADuplicateLeadToTheFirst(t *testing.T) {
	f := calendarFixture(t, nil)
	file := strings.Join([]string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Other//Other//EN",
		"BEGIN:VTODO", "UID:dup", "DTSTAMP:20260101T000000Z", "SUMMARY:First dup", "END:VTODO",
		"BEGIN:VTODO", "UID:dup", "DTSTAMP:20260101T000000Z", "SUMMARY:Second dup", "END:VTODO",
		"BEGIN:VTODO", "UID:kid", "DTSTAMP:20260101T000000Z", "SUMMARY:Kid", "RELATED-TO:dup", "END:VTODO",
		"END:VCALENDAR", ""}, "\r\n")
	page := runImportOf(t, f, "tasks", "tasks.ics", file)
	if page.Report.Created != 3 || page.Report.Renamed != 1 {
		t.Fatalf("report = %+v", page.Report)
	}
	if first := storedWith(t, f, f.col, "First dup"); uidLineOf.FindStringSubmatch(first)[1] != "dup" {
		t.Errorf("the first lost its identity:\n%s", first)
	}
	if kid := storedWith(t, f, f.col, "SUMMARY:Kid"); !strings.Contains(kid, "RELATED-TO:dup") {
		t.Errorf("the link left the first record:\n%s", kid)
	}
}
