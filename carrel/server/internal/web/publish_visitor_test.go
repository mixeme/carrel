package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б39 of the audit of 11.10.2026 (the owner's decision of 12.10.2026): a
// published calendar, event, task list and task give the visitor the time
// always and of the rest only what was ticked — the title by default. The
// page and the file are made of a record built of what is allowed: a
// reminder, a property Carrel does not know, what was not ticked is not in
// it.
func TestPublishedEventShowsWhatIsTicked(t *testing.T) {
	f := publishFixture(t)
	f.srv.Seed(searchWork+"e2.ics", ics("BEGIN:VEVENT\nUID:e2\nDTSTAMP:20260801T000000Z\nSUMMARY:Board review\n"+
		"DTSTART:20260811T090000Z\nDTEND:20260811T100000Z\nLOCATION:Room 9\nDESCRIPTION:Salaries\nCATEGORIES:hr\n"+
		"ORGANIZER:mailto:boss@example.org\nATTENDEE:mailto:ada@example.org\nATTACH:https://example.org/plan.pdf\n"+
		"X-PRIVATE:yes\nCLASS:CONFIDENTIAL\nBEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT10M\nDESCRIPTION:Bring numbers\nEND:VALARM\nEND:VEVENT"))
	href := f.publishHref("calendar", searchWork, "e2")
	sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
	if r := rowNamed(t, sheet.Form, "time"); r.Value != "on" || !r.ReadOnly || rowNamed(t, sheet.Form, "title").Value != "on" ||
		rowNamed(t, sheet.Form, "location").Value != "" || rowNamed(t, sheet.Form, "attendees").Value != "" {
		t.Errorf("the sheet does not start with the time and the title: %+v", sheet.Form.Sets[0])
	}
	file := func(link string) string {
		t.Helper()
		rec := f.h.do(anon(link + "/download"))
		if rec.Code != 200 {
			t.Fatalf("download: %d", rec.Code)
		}
		return rec.Body.String()
	}
	never := []string{"X-PRIVATE", "CLASS", "VALARM", "Bring numbers"}

	// The sheet as it opens: the time and the title.
	link := publishWith(t, f, href, "title", "download")
	got := file(link)
	mustContain(t, got, "SUMMARY:Board review", "DTSTART:20260811T090000Z")
	for _, gone := range append(never, "Room 9", "Salaries", "CATEGORIES", "ORGANIZER", "ATTENDEE", "ATTACH") {
		if strings.Contains(got, gone) {
			t.Errorf("the title alone gives %q:\n%s", gone, got)
		}
	}
	if page := publicPage(t, f, link); page.Title != "Board review" || strings.Contains(saidRuns(page.Lede), "Room 9") || page.Body != "" {
		t.Errorf("page = %q / %q / %q", page.Title, saidRuns(page.Lede), page.Body)
	}

	// The title taken off: the time is still given, the title is not.
	got = file(publishWith(t, f, href, "download"))
	if strings.Contains(got, "Board review") || !strings.Contains(got, "SUMMARY:Busy") {
		t.Errorf("without the title:\n%s", got)
	}

	// The place, the people and the text ticked: those, and still nothing
	// of what has no tick.
	got = file(publishWith(t, f, href, "title", "location", "attendees", "description", "download"))
	mustContain(t, got, "LOCATION:Room 9", "DESCRIPTION:Salaries", "ATTENDEE:mailto:ada@example.org", "ORGANIZER:mailto:boss@example.org")
	for _, gone := range append(never, "CATEGORIES", "ATTACH") {
		if strings.Contains(got, gone) {
			t.Errorf("ticked fields give %q too:\n%s", gone, got)
		}
	}

	// A plain POST — no sheet — is the sheet as it opens.
	plain := decode[view.PublishSheet](t, f.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), f.sess, url.Values{})))
	got = file(string(plain.Link))
	if !strings.Contains(got, "SUMMARY:Board review") || strings.Contains(got, "Room 9") {
		t.Errorf("a plain publish:\n%s", got)
	}
}

// The same for a task list: the page's rows and the file.
func TestPublishedTaskListShowsWhatIsTicked(t *testing.T) {
	f := publishFixture(t)
	f.srv.Seed(searchWork+"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260801T000000Z\nSUMMARY:Pay the rent\n"+
		"DUE;VALUE=DATE:20260815\nDESCRIPTION:Account 4401\nSTATUS:NEEDS-ACTION\nX-PRIVATE:yes\nEND:VTODO"))
	link := publishWith(t, f, f.publishHref("tasks", searchWork, ""), "title", "download")
	page := publicPage(t, f, link)
	if len(page.Tasks) != 1 || page.Tasks[0].Summary != "Pay the rent" {
		t.Fatalf("tasks = %+v", page.Tasks)
	}
	rec := f.h.do(anon(link + "/download"))
	if body := rec.Body.String(); rec.Code != 200 || !strings.Contains(body, "DUE;VALUE=DATE:20260815") ||
		strings.Contains(body, "Account 4401") || strings.Contains(body, "X-PRIVATE") {
		t.Errorf("download %d:\n%s", rec.Code, body)
	}
}
