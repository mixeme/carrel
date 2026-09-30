package web_test

import (
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// A fixture is a promise: the same request gives the same answer tomorrow. The
// only thing in an answer that moves on its own is what the screen calls
// "now" — the stamp under a printed page, «2 min ago», the day an agenda opens
// on. The server takes all of it from one clock, and a test may freeze it.
func TestShownNowComesFromOneClock(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	f.h.srv.SetClock(time.Date(2026, 8, 10, 17, 4, 0, 0, time.UTC))

	page := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	if got := string(page.Print.Taken); got != "10 August 2026, 17:04" {
		t.Errorf("print.taken %q, want the frozen clock", got)
	}

	again := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	if again.Print.Taken != page.Print.Taken {
		t.Errorf("taken moved between two answers: %q then %q", page.Print.Taken, again.Print.Taken)
	}
}

// The rail's last line names the build (C-13). An instance is built with the
// version and the commit; a test says which ones it wants.
func TestTheRailNamesTheBuild(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	f.h.srv.SetBuild("0.0.1", "786949a")

	page := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	if got := string(page.Rail.About.Label) + " " + string(page.Rail.About.Build); got != "Carrel 0.0.1 · 786949a" {
		t.Errorf("rail.about %q, want the name and the build", got)
	}
}

// Without a build — a run from source — the line is the name alone: a version
// nobody set is not shown as one.
func TestWithoutABuildTheRailSaysTheNameAlone(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	f.h.srv.SetBuild("", "")

	page := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	if got := string(page.Rail.About.Label); got != "Carrel" || page.Rail.About.Build != "" {
		t.Errorf("rail.about %q %q, want %q alone", got, page.Rail.About.Build, "Carrel")
	}
}

// The poll of a combined list stamps its own end. That stamp is shown — "the
// list answered at" and the stamp under the printed page — so it comes from
// the same clock as everything else the screen calls "now", and not from the
// wall clock of the machine that ran the poll.
func TestTheEndOfAPollComesFromTheClock(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	f.h.srv.SetClock(time.Date(2026, 8, 10, 17, 4, 0, 0, time.UTC))
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"))

	page := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	if got := string(page.Print.Taken); got != "10 August 2026, 17:04" {
		t.Errorf("print.taken %q, want the frozen clock", got)
	}
	if got := string(page.Poll.AnsweredAt); got != "10 August, 17:04" {
		t.Errorf("poll.answeredAt %q, want the frozen clock", got)
	}
}
