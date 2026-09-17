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

// S-30.2 on iCloud. Contacts answer at contacts.icloud.com and the calendar
// host refuses these credentials outright (docs/servers.md, SV-28), so the set
// is contacts only. The shared harness does not fit either: the account has one
// address book and MKCOL of a second answers 403, so what the server allows is
// tested in the book that is there.

func icloudCardConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_ICLOUD_CARDDAV_URL")
	user = os.Getenv("CARREL_ICLOUD_CARDDAV_USER")
	pass = os.Getenv("CARREL_ICLOUD_CARDDAV_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_ICLOUD_CARDDAV_* required")
	}
	return endpoint, user, pass
}

// A contact made through the pages of the only book this account has, then
// read, edited and deleted. iCloud keeps a card as it was sent — version, UID,
// name parts and a foreign property — so this is the whole of S-7 on a server
// that argues with none of it, and the only thing it does not send is a name
// for the book itself (SV-29).
func TestICloudContactLifecycle(t *testing.T) {
	endpoint, user, pass := icloudCardConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)

	var book sealed.Collection
	for _, c := range liveCatalog(t, app).Collections {
		if c.Has(view.KindContact) && !c.ReadOnly {
			book = c
			break
		}
	}
	if book.Path == "" {
		t.Skip("no address book in the catalog")
	}
	list := "/app/contacts/" + connID + "/" + url.PathEscape(book.Path)
	family := "Probe-" + yandexSuffix("icloud")

	form := liveForm(liveDecode[view.ContactEditPage](t, "new contact form", app.get(list+"/new")).Form)
	form.Set("n.given", "Carrel")
	form.Set("n.family", family)
	form.Set("note", "written by the live set")
	rec := app.post(list+"/new", form)
	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating a contact answered %d, want a redirect to the card: %s", rec.Code, rec.Body.String())
	}

	at, stored := liveHasRecord(ctx, t, client, book.Path, func(b string) bool {
		return strings.Contains(b, family)
	})
	if at == "" {
		t.Fatalf("no card holding %q in %s after a 303 to %q", family, book.Path, loc)
	}
	removeLater(t, client, at)
	storedUID := uidOf(stored)
	if loc != list+"/"+storedUID {
		t.Errorf("the page leads to %q, want the card under UID %s (at %s)", loc, storedUID, at)
	}
	mustHave(t, "the card iCloud kept", stored, "N:"+family+";Carrel;;;", "NOTE:written by the live set")

	// The card opens, and an edit of one field leaves the others alone.
	card := liveDecode[view.ContactPage](t, "the new card", app.get(loc))
	if !strings.Contains(string(card.Name), family) {
		t.Fatalf("the card at %s shows %q", loc, card.Name)
	}
	edit := liveForm(liveDecode[view.ContactEditPage](t, "edit", app.get(loc+"/edit")).Form)
	edit.Set("org", "Carrel Live")
	redirected(t, "edit", app.post(loc+"/edit", edit))
	_, after := liveHasRecord(ctx, t, client, book.Path, func(b string) bool {
		return strings.Contains(b, family)
	})
	mustHave(t, "after the edit", after, "ORG:Carrel Live", "NOTE:written by the live set", "N:"+family+";Carrel;;;")

	// And it deletes, which is what says the address the page holds is real.
	// The version travels with the form the edit page carries (S-8).
	del := liveForm(liveDecode[view.ContactEditPage](t, "edit again", app.get(loc+"/edit")).Form)
	redirected(t, "delete", app.post(loc+"/delete", del))
	if gone, _ := liveHasRecord(ctx, t, client, book.Path, func(b string) bool {
		return strings.Contains(b, family)
	}); gone != "" {
		t.Errorf("the card is still in the book after a delete: %s", gone)
	}
}
