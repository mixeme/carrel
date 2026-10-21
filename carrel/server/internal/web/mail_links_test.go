package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б37: the events of a range of days that do not fit the letter went as a
// link — and a link is the whole calendar, every day of it. A range goes as
// the file it is or not at all.
func TestALetterDoesNotTurnARangeOfDaysIntoTheWholeCalendar(t *testing.T) {
	f := calendarFixture(t, func(h *harness) {
		h.cfg.MailEnabled = true
		h.cfg.MailMaxAttachBytes = 200
	},
		"in.ics", ics("BEGIN:VEVENT\nUID:in\nDTSTAMP:20260101T000000Z\nDTSTART:20261005T100000Z\nDTEND:20261005T110000Z\nSUMMARY:Inside\nDESCRIPTION:"+strings.Repeat("long ", 80)+"\nEND:VEVENT"),
		"out.ics", ics("BEGIN:VEVENT\nUID:out\nDTSTAMP:20260101T000000Z\nDTSTART:20261020T100000Z\nDTEND:20261020T110000Z\nSUMMARY:Outside\nEND:VEVENT"))
	relay := f.h.relayed()
	attachOne(t, f, url.Values{"what": {"calendar"}, "a": {f.id}, "c": {f.col}, "span": {"dates"}, "from": {"2026-10-01"}, "to": {"2026-10-10"}})
	rec := f.h.do(postForm("/app/mail/compose?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"October"}, "body": {"x"}}))
	if rec.Code == 303 {
		t.Fatalf("the letter went: %+v", relay.Letters())
	}
	sheet := decode[view.MailSheet](t, rec)
	if said := saidRuns(sheet.Limits); !strings.Contains(said, "whole calendar") {
		t.Errorf("the sheet does not say why: %q", said)
	}
	if send := actionByID(sheet.Actions, "mail.send"); send == nil || send.Enabled || !strings.Contains(string(send.Reason), "whole calendar") {
		t.Errorf("Send = %+v, want off: a link would open the whole calendar", send)
	}
	if n := len(f.h.users.PublishedLinks(f.sess.AccountID())); n != 0 {
		t.Errorf("the calendar was published: %d links", n)
	}
}
