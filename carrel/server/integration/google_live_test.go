//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// S-30.2 on Google. Two connections, one per protocol: contacts answer at
// googleapis.com and calendars at www.google.com/calendar/dav/, and each host
// 404s the other's home (docs/servers.md, SV-24).
//
// The shared harness does not fit: neither a calendar nor a second address
// book can be made here - MKCALENDAR answers 404 and MKCOL 400 - so what the
// server does allow is tested instead, in the collections that are already
// there.

func googleCardConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_GOOGLE_CARDDAV_URL")
	user = os.Getenv("CARREL_GOOGLE_CARDDAV_USER")
	pass = os.Getenv("CARREL_GOOGLE_CARDDAV_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_GOOGLE_CARDDAV_* required")
	}
	return endpoint, user, pass
}

// A contact made through the pages of the only book this account has. Google
// names the resource itself and writes a UID of its own over the one it was
// sent (docs/servers.md, SV-25), so the card is read back at the address it
// was written to and the person lands on the identity the server kept. Google
// also builds FN out of N, the way Yandex does (SV-21), and the editor writes
// N, so the name arrives whole.
func TestGoogleContactInTheOnlyBook(t *testing.T) {
	endpoint, user, pass := googleCardConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)

	var book sealed.Collection
	for _, c := range liveCatalog(t, app).Collections {
		if c.Has(view.KindContact) {
			book = c
			break
		}
	}
	if book.Path == "" {
		t.Skip("no address book in the catalog")
	}
	list := "/app/contacts/" + connID + "/" + url.PathEscape(book.Path)
	family := "Probe-" + yandexSuffix("google")

	form := liveForm(liveDecode[view.ContactEditPage](t, "new contact form", app.get(list+"/new")).Form)
	form.Set("n.given", "Carrel")
	form.Set("n.family", family)
	rec := app.post(list+"/new", form)
	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating a contact answered %d, want a redirect to the card: %s", rec.Code, rec.Body.String())
	}

	// Whatever it was called, the card must be there and hold what was typed.
	stored, storedUID := liveRecordWhere(ctx, t, client, book.Path, "of "+family, func(b string) bool {
		return strings.Contains(b, family)
	})
	removeLater(t, client, book.Path+storedUID)
	t.Logf("the server kept it under UID %q: %s", storedUID, strings.TrimSpace(stored))

	if loc != list+"/"+storedUID {
		t.Errorf("the page leads to %q, want the card under the UID the server kept (%s)", loc, storedUID)
	}
	card := liveDecode[view.ContactPage](t, "the new card", app.get(loc))
	if !strings.Contains(string(card.Name), family) {
		t.Errorf("the card at %s shows %q, want the name that was typed", loc, card.Name)
	}
}

// The fields the forms gained, on the only book Google gives: the same run
// as on the servers where a book can be made, against the book that is
// already there (owner's decision 20.09.2026). Nothing is created and
// nothing is deleted but the card this test writes.
func TestGoogleContactFields(t *testing.T) {
	endpoint, user, pass := googleCardConfig(t)
	runContactFieldsInTheOnlyBook(t, endpoint, user, pass, "google")
}

func googleCalConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_GOOGLE_CALDAV_URL")
	user = os.Getenv("CARREL_GOOGLE_CALDAV_USER")
	pass = os.Getenv("CARREL_GOOGLE_CALDAV_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_GOOGLE_CALDAV_* required")
	}
	return endpoint, user, pass
}

// An event's whole life through the pages — made, read back, edited,
// deleted — in the calendar this account already has. A calendar cannot be
// made here (MKCALENDAR answers 404), so the shared run, which makes its
// own, does not fit; this is the same kind of run as for Google's contacts
// (owner's decision 20.09.2026). Until 24.09.2026 no suite wrote a single
// calendar record on Google at all: the coverage table in docs/servers.md
// said so, and this closes it.
//
// Nothing in it may reach anyone: no attendee is ever added, or Google would
// send an invitation. The event stands in 2031 so that it meets nothing of
// the owner's own, and it is deleted by the test — and once more on cleanup,
// should the test stop half way.
func TestGoogleEventInTheExistingCalendar(t *testing.T) {
	endpoint, user, pass := googleCalConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)

	var cal sealed.Collection
	for _, c := range liveCatalog(t, app).Collections {
		if c.Has(view.KindEvent) && !c.ReadOnly {
			cal = c
			break
		}
	}
	if cal.Path == "" {
		t.Skip("no calendar this account can write in the catalog")
	}
	events := "/app/calendar/" + connID + "/" + url.PathEscape(cal.Path)
	summary := "Carrel probe " + yandexSuffix("google")

	form := liveForm(liveDecode[view.EventEditPage](t, "new event form", app.get(events+"/new")).Form)
	form.Set("summary", summary)
	form.Set("start.date", "2031-03-11")
	form.Set("start.time", "09:30")
	// A save that answers with the form may have written the event all the
	// same — a loss report does (S-7.1) — and on 01.10.2026 one such run left
	// its event in the owner's calendar: the cleanup below is set up only
	// after the redirect. The form names the record it holds.
	made := app.post(events+"/new", form)
	if made.Code != http.StatusSeeOther {
		var page view.EventEditPage
		if json.Unmarshal(made.Body.Bytes(), &page) == nil && page.UID != "" {
			removeLater(t, client, cal.Path+page.UID+".ics")
		}
	}
	loc := redirected(t, "new event", made)
	uid, _ := url.PathUnescape(loc[strings.LastIndex(loc, "/")+1:])
	// Google keeps both the UID and the resource name of an event (SV-25
	// notes that only a card is renamed).
	file := cal.Path + uid + ".ics"
	removeLater(t, client, file)
	got, _ := rawBody(ctx, t, client, file)
	mustHave(t, "new event", got, "UID:"+uid, "SUMMARY:"+summary, "20310311")
	// Only an attendee of the event itself would be invited. Google writes
	// two reminders of its own into every event (SV-25), and the e-mail one
	// carries an ATTENDEE — the calendar's owner, whom the reminder is for —
	// so the reminders are left out before looking.
	if strings.Contains(withoutAlarms(got), "ATTENDEE") {
		t.Fatalf("the event names an attendee — Google would have sent an invitation:\n%s", got)
	}

	if event := liveDecode[view.EventPage](t, "the event", app.get(loc)); string(event.Summary) != summary {
		t.Errorf("the event page shows %q, want %q", event.Summary, summary)
	}

	form = liveForm(liveDecode[view.EventEditPage](t, "edit form", app.get(loc+"/edit")).Form)
	form.Set("summary", summary+" moved")
	redirected(t, "edit", app.post(loc+"/edit", form))
	got, _ = rawBody(ctx, t, client, file)
	mustHave(t, "edited event", got, "SUMMARY:"+summary+" moved")

	event := liveDecode[view.EventPage](t, "event before delete", app.get(loc))
	redirected(t, "delete", app.post(liveAction(t, event.Actions, "record.delete"), url.Values{}))
	if _, _, err := client.Get(ctx, file, nil); !dav.IsStatus(err, http.StatusNotFound) {
		t.Errorf("after delete GET %s = %v, want 404", file, err)
	}
}

// withoutAlarms is a calendar text with its VALARM blocks taken out.
func withoutAlarms(text string) string {
	var out []string
	in := false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		switch strings.TrimSpace(line) {
		case "BEGIN:VALARM":
			in = true
			continue
		case "END:VALARM":
			in = false
			continue
		}
		if !in {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
