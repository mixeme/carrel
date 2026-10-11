package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б36 of the audit of 11.10.2026: a link put into a letter and revoked
// before the letter went was published again by Send — for thirty days,
// under a new address, without what the person had set on it. What a person
// took back is not given out again: the letter does not go, and says why.
func TestALetterDoesNotPublishARevokedLinkAgain(t *testing.T) {
	f, relay := letterFixture(t)
	decode[view.PublishSheet](t, f.h.do(postForm("/app/publish?"+url.Values{"k": {"contacts"}, "a": {f.id}, "c": {searchBook}, "uid": {"ada"}}.Encode(), f.sess, url.Values{})))
	page := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess)))
	if len(page.Rows) != 1 {
		t.Fatalf("rows = %+v", page.Rows)
	}
	if rec := f.h.do(postForm(actionHref(t, page.Rows[0].Actions, "publish.mail"), f.sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("send this link: %d", rec.Code)
	}
	// Revoked while the letter is being written.
	for _, l := range f.h.users.PublishedLinks(f.sess.AccountID()) {
		if err := f.h.users.RevokePublished(f.sess.AccountID(), l.ID); err != nil {
			t.Fatal(err)
		}
	}
	rec := f.h.do(postForm("/app/mail/compose?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"Ada"}, "body": {"See"}}))
	if rec.Code == 303 {
		t.Fatalf("the letter went")
	}
	sheet := decode[view.MailSheet](t, rec)
	if said := saidRuns(sheet.Limits); !strings.Contains(said, "revoked") {
		t.Errorf("the sheet does not say why: %q", said)
	}
	if n := len(relay.Letters()); n != 0 {
		t.Errorf("letters = %d", n)
	}
	if n := len(f.h.users.PublishedLinks(f.sess.AccountID())); n != 0 {
		t.Errorf("published again: %d links", n)
	}
}

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
	if n := len(f.h.users.PublishedLinks(f.sess.AccountID())); n != 0 {
		t.Errorf("the calendar was published: %d links", n)
	}
}
