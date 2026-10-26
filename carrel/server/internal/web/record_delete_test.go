package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// Note 69 of Typecase (task 166; s-agenda-26, s-tasks-18; the owner's
// decisions А8, А3, А12): Delete of a record that has something to lose
// answers with a sheet — what is pointing at it (its published link, the
// records related to it among the loaded ones), whether the server may tell
// its attendees it is cancelled, its attachments — written into it, going
// with it; a link to the attachment folder, staying, with a tick to delete
// the file too —, and its copies. The sheet's Delete deletes what it says.
// A record with nothing to lose is deleted at once, as before.
func TestDeleteAsksWhatIsLost(t *testing.T) {
	srv := davtest.New(t)
	srv.ScheduleOutbox = davtest.Principal + "outbox/"
	srv.UserAddresses = []string{"mailto:mix@example.org", davtest.Principal}
	f := attachFixtureOn(t, srv, 0, []string{"carrel"},
		"plain.ics", attachEvent("plain", "Plain", ""),
		"e2.ics", attachEvent("e2", "Review", "ORGANIZER:mailto:mix@example.org\nATTENDEE;PARTSTAT=ACCEPTED:mailto:ann@example.org\nATTENDEE:mailto:bob@example.org\n"),
		"e3.ics", attachEvent("e3", "Follow-up", "RELATED-TO;RELTYPE=SIBLING:e2\n"))
	f.srv.SeedFile(attachFiles+"carrel/agenda.pdf", "%PDF agenda")
	at := davtest.CalendarHome + "work/e2.ics"
	body, _, _ := f.srv.Object(at)
	body = strings.Replace(body, "END:VEVENT", "ATTACH;VALUE=BINARY;ENCODING=BASE64;FILENAME=checklist.md:aGVsbG8=\r\n"+
		"ATTACH;FMTTYPE=application/pdf:"+f.srv.URL+attachFiles+"carrel/agenda.pdf\r\nEND:VEVENT", 1)
	f.srv.Seed(at, body)
	// The list was read: the related record is among the loaded ones.
	tickRail(t, f.h, f.sess, "calendar", f.path("calendar"))
	combined(t, f.h, f.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	link := publishLink(t, f, f.publishHref("calendar", davtest.CalendarHome+"work/", "e2"), false)

	del := func(uid string) string {
		t.Helper()
		return actionHref(t, eventAt(t, f, uid).Actions, "record.delete")
	}

	// Nothing to lose: deleted at once.
	if rec := f.h.do(postForm(del("plain"), f.sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("plain: %d %s", rec.Code, rec.Body)
	}
	if _, _, ok := f.srv.Object(davtest.CalendarHome + "work/plain.ics"); ok {
		t.Fatal("plain: not deleted")
	}

	// Something to lose: the sheet, and nothing deleted yet.
	sheet := decode[view.RecordDeleteSheet](t, f.h.do(postForm(del("e2"), f.sess, url.Values{})))
	if _, _, ok := f.srv.Object(at); !ok {
		t.Fatal("the sheet deleted the event")
	}
	// The words of s-agenda-26 (note 135), its server's name aside.
	msgs := frameMsgs(t, "s-agenda-26")
	if sheet.Title != "Delete event" || sheet.Subject != "Review" || !msgDrawn(msgs, string(sheet.Warning)) {
		t.Errorf("head = %q %q %q, the frame %q", sheet.Title, sheet.Subject, sheet.Warning, msgs)
	}
	cancels := ""
	for _, m := range msgs {
		if strings.HasSuffix(m, "a cancellation.") {
			_, cancels, _ = strings.Cut(m, " ")
		}
	}
	refs := map[string]string{}
	for _, r := range sheet.References {
		refs[string(r.What)] = string(r.Then)
	}
	if refs["1 published link"] != "answers “not found” from that moment" ||
		refs["1 record related to it"] != "among the loaded ones; their link is left pointing at nothing" {
		t.Errorf("references = %v", refs)
	}
	if cancels == "" || !strings.HasSuffix(string(sheet.Cancels), " "+cancels) {
		t.Errorf("cancels = %q, the frame «… %s»", sheet.Cancels, cancels)
	}
	atts := map[string]string{}
	for _, a := range sheet.Attachments {
		atts[string(a.What)] = string(a.Then)
	}
	if atts["checklist.md"] != "written into the event, goes with it" || !strings.HasSuffix(atts["agenda.pdf"], ", the file stays") {
		t.Errorf("attachments = %v", atts)
	}
	tick := formRow(sheet.Form, "attachment")
	if tick == nil || tick.Kind != view.InputCheck || len(tick.Options) != 1 || tick.Options[0].Label != "Delete agenda.pdf too" || tick.Options[0].Selected {
		t.Fatalf("the tick of the file = %+v", tick)
	}
	d := actionByID(sheet.Actions, "record.delete")
	if d == nil || !d.Danger || !d.Submits || actionByID(sheet.Actions, "record.delete.cancel") == nil {
		t.Fatalf("actions = %+v", sheet.Actions)
	}

	// Delete with the file ticked: the event, the file and the link go.
	form := formFields(sheet.Form)
	form.Set("attachment", tick.Options[0].Value)
	if rec := f.h.do(postForm(sheet.Form.Action, f.sess, form)); rec.Code != 303 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if _, _, ok := f.srv.Object(at); ok {
		t.Error("the event stays")
	}
	if _, ok := f.srv.File(attachFiles + "carrel/agenda.pdf"); ok {
		t.Error("the file stays though ticked")
	}
	if noticeTitled(publicPage(t, f, link).Notices, "No longer there.") == nil {
		t.Error("the link still opens the event")
	}
}
