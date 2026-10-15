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
	if got := saidRuns(page.Poll.Summary); got != "Polled 1 of 1 source · 2 cards" {
		t.Errorf("the list bar says %q", got)
	}
}

// Typecase Р-55 (the owner's decision of 11.10.2026): a poll still running
// says so in the present. Before anybody has answered it names how many it
// asks (states-3); with some answered — how many of them and what has come
// (states-8: the fixture app-contacts/partial holds that line).
func TestARunningPollSpeaksInThePresent(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"))
	release := f.srv.Stall("REPORT")
	defer release()

	page := decode[view.ContactsListPage](t, f.h.do(get("/app/contacts", f.sess)))
	if page.Poll == nil {
		t.Fatal("the combined book has no poll")
	}
	if got := saidRuns(page.Poll.Summary); got != "Polling 1 source…" {
		t.Errorf("a poll nobody has answered says %q", got)
	}
}

// A calendar counts events, a task list tasks, a notebook notes.
func TestEachKindCountsItsOwn(t *testing.T) {
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup\nDTSTART;VALUE=DATE:20260810\nEND:VEVENT"))
	tickRail(t, f.h, f.sess, "calendar", f.path("calendar"))

	page := combined(t, f.h, f.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	if got := saidRuns(page.Poll.Summary); !strings.HasSuffix(got, "· 1 event") {
		t.Errorf("the agenda bar says %q, want it to count events", got)
	}
}

// A conflict is about the record's own properties, and it names them as the
// record does: «FN», «NOTE», «X-EVOLUTION-FILE-AS» (s-conflict-1). A label
// Carrel invented would be a lie about a property Carrel does not model, and
// a person comparing two versions is looking at the file, not at a form.
func TestAConflictNamesThePropertiesAsTheRecordDoes(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nNOTE:first note\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"

	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	f.srv.Seed(f.col+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Byron\nNOTE:server note\nEND:VCARD\n")
	form.Set("fn", "Ada King")
	f.h.do(postForm(rec+"/edit", f.sess, form))

	page := decode[view.ConflictPage](t, f.h.do(get(rec+"/conflict", f.sess)))
	var named []string
	for _, r := range page.Rows {
		named = append(named, string(r.Property))
	}
	if want := []string{"FN", "N"}; !equalStrings(named, want) {
		t.Errorf("the conflict names %v, want %v", named, want)
	}
}

// The conflict screen always says what happened, in the words of the frame
// (s-conflict-1): a record read and then saved by someone else is not a
// state a person is expected to infer from a table of two columns. It says
// it under its title, not as an alarm over the cards (Рв1; until v0.2.265 a
// notice «Conflict»).
func TestTheConflictScreenSaysWhatHappened(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"

	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	f.srv.Seed(f.col+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Byron\nEND:VCARD\n")
	form.Set("title", "Countess")
	f.h.do(postForm(rec+"/edit", f.sess, form))

	page := decode[view.ConflictPage](t, f.h.do(get(rec+"/conflict", f.sess)))
	if len(page.Notices) != 0 || page.Subtitle != "Another client changed this contact while you were editing it. Nothing has been overwritten." {
		t.Errorf("notices = %+v, subtitle %q", page.Notices, page.Subtitle)
	}
	// Cancel leads back to the record (Рв8), and the page is refreshed by
	// reading it again (Рв7).
	if c := actionByID(page.Actions, "conflict.cancel"); c == nil || c.Method != "GET" || c.Href != rec {
		t.Errorf("Cancel = %+v", c)
	}
	if page.Top.Refresh.Action.Href != rec+"/conflict" {
		t.Errorf("refresh = %+v", page.Top.Refresh)
	}
}

// The rail of a record screen is a way back, not a way out (C-6.3): it
// carries the collections of the account the record lives in, and no other.
// Every frame with «Back to list» shows exactly one account rubric —
// s-contacts-3, s-notes-1, s-conflict-1, the whole of s-io.
func TestTheBackRailStaysInItsAccount(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	page := decode[view.ContactPage](t, f.h.do(get(f.book(f.id, peopleBook)+"/ada-here", f.sess)))
	var accounts []string
	for _, sec := range page.Rail.Sections {
		accounts = append(accounts, string(sec.Account))
	}
	if len(accounts) != 1 {
		t.Fatalf("the back rail shows %v, want only the account of the record", accounts)
	}
}
