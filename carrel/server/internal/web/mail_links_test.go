package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б37, С19 (the owner's decision of 21.10.2026): the events of a range of
// days that do not fit the letter go as a link to these days — as a
// published range is —, never as the whole calendar.
func TestALetterSendsARangeOfDaysAsALinkToTheseDays(t *testing.T) {
	f := calendarFixture(t, func(h *harness) {
		h.cfg.MailEnabled = true
		h.cfg.MailMaxAttachBytes = 200
	},
		"in.ics", ics("BEGIN:VEVENT\nUID:in\nDTSTAMP:20260101T000000Z\nDTSTART:20261005T100000Z\nDTEND:20261005T110000Z\nSUMMARY:Inside\nDESCRIPTION:"+strings.Repeat("long ", 80)+"\nEND:VEVENT"),
		"out.ics", ics("BEGIN:VEVENT\nUID:out\nDTSTAMP:20260101T000000Z\nDTSTART:20261020T100000Z\nDTEND:20261020T110000Z\nSUMMARY:Outside\nEND:VEVENT"))
	relay := f.h.relayed()
	attachOne(t, f, url.Values{"what": {"calendar"}, "a": {f.id}, "c": {f.col}, "span": {"dates"}, "from": {"2026-10-01"}, "to": {"2026-10-10"}})
	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	if len(sheet.Attach) != 1 || !sheet.Attach[0].AsLink {
		t.Fatalf("attached = %+v, want the days as a link", sheet.Attach)
	}
	if send := actionByID(sheet.Form.Actions, "mail.send"); send == nil || send.Reason != "Add a recipient." {
		t.Errorf("Send = %+v, want off for want of a recipient alone", send)
	}
	rec := f.h.do(postForm("/app/mail/send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"October"}, "text": {"x"}}))
	if rec.Code != 303 || len(relay.Letters()) != 1 {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	links := f.h.users.PublishedLinks(f.sess.AccountID())
	if len(links) != 1 || links[0].Target.From != "2026-10-01" || links[0].Target.To != "2026-10-10" {
		t.Errorf("links = %+v, want one of the days chosen", links)
	}
}
