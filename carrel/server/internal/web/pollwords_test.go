package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The bar over a list says it the way the frames say it: «Polled 5 of 6
// sources · 14 events», and the noun is the kind's — a contact is a card,
// not a «record». The server writes the sentence (C-3.1), so the words are
// the product's and not a client's guess.
func TestThePollOverAListSpeaksTheWordsOfTheFrame(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"))

	page := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	if got := string(page.Poll.Summary); got != "Polled 1 of 1 source · 2 cards" {
		t.Errorf("the list bar says %q", got)
	}
}

// A calendar counts events, a task list tasks, a notebook notes.
func TestEachKindCountsItsOwn(t *testing.T) {
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup\nDTSTART;VALUE=DATE:20260810\nEND:VEVENT"))
	tickRail(t, f.h, f.sess, "calendar", f.path("calendar"))

	page := combined(t, f.h, f.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	if got := string(page.Poll.Summary); !strings.HasSuffix(got, "· 1 event") {
		t.Errorf("the agenda bar says %q, want it to count events", got)
	}
}
