//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

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
