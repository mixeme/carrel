//go:build integration

package integration

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// S-30.2 on Yandex. The credentials name two addresses — CalDAV and CardDAV,
// each with an app password of its own — but discovery from either walks up to
// the principal and finds both homes, and either password writes (16.09.2026),
// so one address takes everything below.
//
// The shared harness does not fit this server: every runX seeds an address
// book, and Yandex answers MKCOL of a book with 405 from nginx — a book cannot
// be made at all (docs/servers.md, SV-20). What it does allow is tested here
// instead, and that includes the create landing at an address of the server's
// own choosing (SV-19), which is what S-12.1 is about.

func yandexConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_YANDEX_CALDAV_URL")
	user = os.Getenv("CARREL_YANDEX_CALDAV_USER")
	pass = os.Getenv("CARREL_YANDEX_CALDAV_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_YANDEX_CALDAV_URL, CARREL_YANDEX_CALDAV_USER, CARREL_YANDEX_CALDAV_PASS required")
	}
	return endpoint, user, pass
}

func yandexSuffix(what string) string {
	return fmt.Sprintf("carrel-%s-%d-%d", what, os.Getpid(), time.Now().Unix()%100000)
}

// newCollection is the entry the catalog gained, whatever address it is at.
func newCollection(t *testing.T, app *liveApp, before map[string]bool) sealed.Collection {
	t.Helper()
	var fresh []sealed.Collection
	for _, c := range liveCatalog(t, app).Collections {
		if !before[c.Path] {
			fresh = append(fresh, c)
		}
	}
	if len(fresh) != 1 {
		t.Fatalf("the catalog gained %d collections, want one: %+v", len(fresh), fresh)
	}
	return fresh[0]
}

func catalogPaths(t *testing.T, app *liveApp) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, c := range liveCatalog(t, app).Collections {
		out[c.Path] = true
	}
	return out
}

// S-12.1 and S-12.2 on a server that gives a new collection an address of its
// own and says nothing about it (docs/servers.md, SV-19). Nothing here asserts
// the quirk: it asks that the catalog, the redirect and the server agree on
// one address, which is true of every server and is exactly what breaks when
// the address is assumed.
func TestYandexCalendarLifecycle(t *testing.T) {
	endpoint, user, pass := yandexConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)

	displayName := xml.Name{Space: "DAV:", Local: "displayname"}
	suffix := yandexSuffix("s121")
	name := "Carrel S-12.1 " + suffix
	before := catalogPaths(t, app)

	loc := redirected(t, "new calendar", app.post("/app/collections/new?kind=event", url.Values{
		"account": {connID}, "name": {name}, "components": {"VEVENT"},
	}))
	placed := newCollection(t, app, before)
	t.Cleanup(func() { _ = client.Delete(context.Background(), placed.Path, "") })
	t.Logf("asked for a calendar named %q, the server put it at %s", name, placed.Path)

	if want := "/app/calendar/" + connID + "/" + url.PathEscape(placed.Path); loc != want {
		t.Errorf("the create went to %q, want %q: the catalog and the address shown disagree", loc, want)
	}
	if got := liveProps(ctx, t, client, placed.Path, displayName); got["displayname"] != name {
		t.Errorf("the collection the catalog points at = %+v, want the name that was asked for", got)
	}
	if agenda := liveDecode[view.AgendaPage](t, "agenda", app.get(loc)); len(agenda.Notices) > 0 {
		t.Errorf("the page of the new calendar: %+v", agenda.Notices)
	}

	redirected(t, "rename", app.post("/app/collections/rename?"+url.Values{"a": {connID}, "c": {placed.Path}}.Encode(),
		url.Values{"name": {name + " renamed"}, "colour": {"#5c736b"}}))
	if got := liveProps(ctx, t, client, placed.Path, displayName); got["displayname"] != name+" renamed" {
		t.Errorf("after the rename = %+v", got)
	}

	// Deleted by the name the catalog holds, which is the name a person sees.
	entry, ok := liveCatalog(t, app).Collection(connID, placed.Path)
	if !ok {
		t.Fatalf("the catalog lost %s", placed.Path)
	}
	sheet := liveDeleteSheet(t, app, connID, placed.Path)
	if loc := redirected(t, "delete", app.post(liveAction(t, sheet.Actions, "collection.delete.confirm"),
		url.Values{"name": {entry.Name}})); loc != "/app/settings/connections" {
		t.Errorf("delete went to %q", loc)
	}
	if _, err := client.PropFind(ctx, placed.Path, dav.DepthZero, []xml.Name{displayName}); !dav.IsStatus(err, http.StatusNotFound) {
		t.Errorf("after the delete PROPFIND = %v, want 404", err)
	}
	if _, ok := liveCatalog(t, app).Collection(connID, placed.Path); ok {
		t.Error("the catalog still holds it")
	}
}

// docs/servers.md, SV-20: an address book cannot be made on this server at all
// — nginx answers MKCOL with 405. What is asked of Carrel is that the refusal
// arrives as the content of the sheet, with the method and the code in it, and
// that nothing is left in the catalog.
func TestYandexAddressBookRefused(t *testing.T) {
	endpoint, user, pass := yandexConfig(t)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	before := len(liveCatalog(t, app).Collections)

	sheet := liveDecode[view.CollectionNewSheet](t, "new address book", app.post("/app/collections/new?kind=contact", url.Values{
		"account": {connID}, "name": {"Carrel S-12.2 " + yandexSuffix("s122")},
	}))
	if sheet.Failure == nil {
		t.Fatalf("a create this server refuses answered without a failure: %+v", sheet)
	}
	if sheet.Failure.Details == nil || !strings.Contains(string(sheet.Failure.Details.Summary), "MKCOL") {
		t.Fatalf("the refusal does not name the request: %+v", sheet.Failure)
	}
	var said405 bool
	for _, l := range sheet.Failure.Details.Lines {
		said405 = said405 || strings.Contains(string(l), "405")
	}
	if !said405 {
		t.Errorf("the refusal does not say what the server answered: %+v", sheet.Failure.Details.Lines)
	}
	if n := len(liveCatalog(t, app).Collections); n != before {
		t.Errorf("the catalog went from %d collections to %d on a refused create", before, n)
	}
}

// The one address book this server has is writable, even though none can be
// made (SV-20). What it does to the card is the finding: this server builds FN
// from N and throws away the FN it was given (docs/servers.md, SV-21), and the
// contact form has one name field — FN — and no N at all (`web.editFields`).
// So a contact made here arrives without a name, and the record page cannot
// show one. The test holds the server's behaviour as it is: it goes red the
// day Yandex keeps what it is sent, which is the day the finding is stale.
func TestYandexContactInTheOnlyBook(t *testing.T) {
	endpoint, user, pass := yandexConfig(t)
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
	fn := "Carrel Probe " + yandexSuffix("s11")

	form := liveForm(liveDecode[view.ContactEditPage](t, "new contact form", app.get(list+"/new")).Form)
	form.Set("fn", fn)
	loc := redirected(t, "new contact", app.post(list+"/new", form))
	uid, err := url.PathUnescape(loc[strings.LastIndex(loc, "/")+1:])
	if err != nil {
		t.Fatalf("the new contact went to %q", loc)
	}
	t.Cleanup(func() { _ = client.Delete(context.Background(), book.Path+uid+".vcf", "") })

	// What the server actually kept, found by what is written in the card
	// rather than by the address or the identifier Carrel chose.
	stored, storedUID := liveRecordWhere(ctx, t, client, book.Path, "with UID "+uid, func(b string) bool {
		return strings.Contains(b, "UID:"+uid)
	})
	t.Logf("the page went to %q (uid %q); the book holds it under UID %q", loc, uid, storedUID)
	t.Logf("what the server kept:\n%s", stored)
	if storedUID != uid {
		t.Errorf("the identifier changed: wrote %q, the server holds %q", uid, storedUID)
	}
	// The name arrives whole. This server builds FN from N and throws away the
	// FN it was sent (docs/servers.md, SV-21), so the card is nameless unless
	// N is written — and the editor writes it now, cutting a display name into
	// Given and Family when the two parts are empty (U-6.1). Before that, this
	// same write left FN: and N:;;;; on the server.
	if !strings.Contains(stored, "FN:"+fn) {
		t.Errorf("the server kept no FN:%s\n%s", fn, stored)
	}
	parts := strings.SplitN(fn, " ", 2)
	if wantN := "N:" + parts[1] + ";" + parts[0] + ";"; !strings.Contains(stored, wantN) {
		t.Errorf("the server kept no %q, so the cut did not reach it:\n%s", wantN, stored)
	}

	// The page is logged and not asserted: reading this book answers
	// "Personal is unavailable" in most runs, which is its own open question.
	page := liveDecode[view.ContactPage](t, "card", app.get(loc))
	t.Logf("the record page shows name %q, %d notice(s), %d action(s): %+v",
		page.Name, len(page.Notices), len(page.Actions), page.Notices)

	// Taken away directly: a record the page cannot read offers no delete.
	if err := client.Delete(ctx, book.Path+uid+".vcf", ""); err != nil {
		t.Errorf("delete %s: %v", book.Path+uid+".vcf", err)
	}
}
