//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for S-18: a record moved and copied through the request layer on a
// live server. Within one connection the move is a MOVE the server carries
// out; between connections it is a copy, a read-back and a delete.
func TestRadicaleMove(t *testing.T) {
	runMove(t, radicaleEndpoint(t), radicaleUser, radicalePass)
}

func TestBaikalMove(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runMove(t, endpoint, user, pass)
}

// TestRadicaleBaikalMove needs both servers: Radicale on the Linux machine and
// Baikal's credentials in the environment.
func TestRadicaleBaikalMove(t *testing.T) {
	bEndpoint, bUser, bPass := baikalConfig(t)
	rEndpoint := radicaleEndpoint(t)
	ctx := context.Background()
	suffix := moveSuffix()
	rad := makeMoveCollections(ctx, t, rEndpoint, radicaleUser, radicalePass, suffix+"-r")
	bai := makeMoveCollections(ctx, t, bEndpoint, bUser, bPass, suffix+"-b")

	app := newLiveApp(t)
	radID := connectLive(t, app, rEndpoint, radicaleUser, radicalePass)
	baiID := connectLive(t, app, bEndpoint, bUser, bPass)
	blob := liveCatalog(t, app)

	// A contact from Radicale to Baikal.
	uid := "move-ada-" + suffix
	seedCard(ctx, t, rad.client, rad.bookA, uid)
	from := catalogEntry(t, blob, rad.bookA)
	to := catalogEntry(t, blob, bai.bookA)
	src := "/app/contacts/" + radID + "/" + url.PathEscape(from.Path) + "/" + url.PathEscape(uid)
	sheet := liveDecode[view.MoveSheet](t, "contact sheet", app.get(src+"/move?"+url.Values{"to": {baiID}, "into": {to.Path}}.Encode()))
	if sheet.Warning == "" {
		t.Errorf("a move to another account must say so before it starts: %+v", sheet)
	}
	loc := redirected(t, "contact to Baikal", app.post(liveAction(t, sheet.Actions, "record.move.confirm"), url.Values{}))
	if want := "/app/contacts/" + baiID + "/" + url.PathEscape(to.Path) + "/" + url.PathEscape(uid); loc != want {
		t.Errorf("went to %q, want %q", loc, want)
	}
	got, _ := rawBody(ctx, t, bai.client, bai.bookA+uid+".vcf")
	mustHave(t, "contact on Baikal", got, "UID:"+uid, "FN:Ada Lovelace", "X-CUSTOM:keep-me")
	if _, _, err := rad.client.Get(ctx, rad.bookA+uid+".vcf", nil); !dav.IsStatus(err, http.StatusNotFound) {
		t.Errorf("the original on Radicale: GET = %v, want 404", err)
	}

	// An event with an overridden instance from Baikal to Radicale.
	series := "series-" + suffix
	seedSeries(ctx, t, bai.client, bai.calA, series)
	from = catalogEntry(t, blob, bai.calA)
	to = catalogEntry(t, blob, rad.calA)
	src = "/app/calendar/" + baiID + "/" + url.PathEscape(from.Path) + "/" + url.PathEscape(series)
	sheet = liveDecode[view.MoveSheet](t, "event sheet", app.get(src+"/move?"+url.Values{"to": {radID}, "into": {to.Path}}.Encode()))
	redirected(t, "event to Radicale", app.post(liveAction(t, sheet.Actions, "record.move.confirm"), url.Values{}))
	got, _ = rawBody(ctx, t, rad.client, rad.calA+series+".ics")
	mustHave(t, "event on Radicale", got, "UID:"+series, "RRULE:FREQ=WEEKLY;COUNT=3", "RECURRENCE-ID:20261013T100000Z", "SUMMARY:Standup moved")
	if _, _, err := bai.client.Get(ctx, bai.calA+series+".ics", nil); !dav.IsStatus(err, http.StatusNotFound) {
		t.Errorf("the original on Baikal: GET = %v, want 404", err)
	}
}

func runMove(t *testing.T, endpoint, user, pass string) {
	ctx := context.Background()
	suffix := moveSuffix()
	cols := makeMoveCollections(ctx, t, endpoint, user, pass, suffix)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	blob := liveCatalog(t, app)
	moveURL := func(kind, fromPath, uid, intoPath string, copying bool) string {
		q := url.Values{"to": {connID}, "into": {catalogEntry(t, blob, intoPath).Path}}
		if copying {
			q.Set("copy", "1")
		}
		return "/app/" + kind + "/" + connID + "/" + url.PathEscape(catalogEntry(t, blob, fromPath).Path) + "/" + url.PathEscape(uid) + "/move?" + q.Encode()
	}

	t.Run("contact", func(t *testing.T) {
		uid := "move-ada-" + suffix
		seedCard(ctx, t, cols.client, cols.bookA, uid)
		before, _ := rawBody(ctx, t, cols.client, cols.bookA+uid+".vcf")

		sheet := liveDecode[view.MoveSheet](t, "move sheet", app.get(moveURL("contacts", cols.bookA, uid, cols.bookB, false)))
		if sheet.Warning != "" {
			t.Errorf("a move on one account warned: %q", sheet.Warning)
		}
		redirected(t, "move", app.post(liveAction(t, sheet.Actions, "record.move.confirm"), url.Values{}))
		if _, _, err := cols.client.Get(ctx, cols.bookA+uid+".vcf", nil); !dav.IsStatus(err, http.StatusNotFound) {
			t.Errorf("the original after MOVE: GET = %v, want 404", err)
		}
		moved, _ := rawBody(ctx, t, cols.client, cols.bookB+uid+".vcf")
		if moved != before {
			t.Errorf("MOVE changed the card:\nbefore %s\nafter  %s", before, moved)
		}

		sheet = liveDecode[view.MoveSheet](t, "copy sheet", app.get(moveURL("contacts", cols.bookB, uid, cols.bookA, true)))
		redirected(t, "copy back", app.post(liveAction(t, sheet.Actions, "record.copy.confirm"), url.Values{}))
		copied, _ := rawBody(ctx, t, cols.client, cols.bookA+uid+".vcf")
		mustHave(t, "copy", copied, "UID:"+uid, "FN:Ada Lovelace", "X-CUSTOM:keep-me")
		rawBody(ctx, t, cols.client, cols.bookB+uid+".vcf")
	})

	t.Run("event", func(t *testing.T) {
		series := "series-" + suffix
		seedSeries(ctx, t, cols.client, cols.calA, series)
		sheet := liveDecode[view.MoveSheet](t, "move sheet", app.get(moveURL("calendar", cols.calA, series, cols.calB, false)))
		redirected(t, "move", app.post(liveAction(t, sheet.Actions, "record.move.confirm"), url.Values{}))
		if _, _, err := cols.client.Get(ctx, cols.calA+series+".ics", nil); !dav.IsStatus(err, http.StatusNotFound) {
			t.Errorf("the original after MOVE: GET = %v, want 404", err)
		}
		got, _ := rawBody(ctx, t, cols.client, cols.calB+series+".ics")
		mustHave(t, "moved event", got, "UID:"+series, "RRULE:FREQ=WEEKLY;COUNT=3", "RECURRENCE-ID:20261013T100000Z", "SUMMARY:Standup moved")
		agenda := liveDecode[view.AgendaPage](t, "agenda at the target",
			app.get("/app/calendar/"+connID+"/"+url.PathEscape(catalogEntry(t, blob, cols.calB).Path)+"?from=2026-10-12"))
		if len(agenda.Notices) > 0 || len(agenda.Days) != 1 || agenda.Days[0].Rows[0].Summary != "Standup moved" {
			t.Errorf("agenda at the target = %+v", agenda)
		}
	})
}

type moveCollections struct {
	client                   *dav.Client
	bookA, bookB, calA, calB string
}

func moveSuffix() string {
	return fmt.Sprintf("carrel-s18-%d-%d", os.Getpid(), time.Now().Unix()%100000)
}

// makeMoveCollections makes two address books and two calendars directly in
// the homes (Radicale's owner_only allows nothing deeper) and removes them
// with the test.
func makeMoveCollections(ctx context.Context, t *testing.T, endpoint, user, pass, suffix string) moveCollections {
	t.Helper()
	g := testGuard(t)
	client, err := dav.NewClient(g, endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	res, trace, err := discover.Run(ctx, g, endpoint, user, pass, "")
	if err != nil || res.CalendarHome == "" || res.AddressBookHome == "" {
		t.Fatalf("discovery: %v %+v %+v", err, res, trace)
	}
	home := func(h string) string { return strings.TrimSuffix(h, "/") + "/" }
	c := moveCollections{
		client: client,
		bookA:  home(res.AddressBookHome) + suffix + "-book-a/",
		bookB:  home(res.AddressBookHome) + suffix + "-book-b/",
		calA:   home(res.CalendarHome) + suffix + "-cal-a/",
		calB:   home(res.CalendarHome) + suffix + "-cal-b/",
	}
	for _, book := range []string{c.bookA, c.bookB} {
		liveRequest(ctx, t, endpoint, user, pass, "MKCOL", book, `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel S-18 `+book[len(book)-2:len(book)-1]+`</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)
		t.Cleanup(func() { _ = client.Delete(context.Background(), book, "") })
	}
	for _, cal := range []string{c.calA, c.calB} {
		liveRequest(ctx, t, endpoint, user, pass, "MKCALENDAR", cal, `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel S-18 `+cal[len(cal)-2:len(cal)-1]+`</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
		t.Cleanup(func() { _ = client.Delete(context.Background(), cal, "") })
	}
	return c
}

func connectLive(t *testing.T, app *liveApp, endpoint, user, pass string) string {
	t.Helper()
	redirected(t, "connect "+endpoint, app.post("/app/settings/connections", url.Values{
		"server": {endpoint}, "login": {user}, "password": {pass},
	}))
	blob := liveCatalog(t, app)
	return blob.Connections[len(blob.Connections)-1].ID
}

func liveCatalog(t *testing.T, app *liveApp) sealed.Blob {
	t.Helper()
	key, _ := app.sess.DataKey()
	blob, ok := app.users.Catalog(app.sess.AccountID(), key)
	if !ok {
		t.Fatal("catalog not readable")
	}
	return blob
}

func seedCard(ctx context.Context, t *testing.T, client *dav.Client, book, uid string) {
	t.Helper()
	card := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:" + uid + "\r\nFN:Ada Lovelace\r\nN:Lovelace;Ada;;;\r\nX-CUSTOM:keep-me\r\nEND:VCARD\r\n"
	if _, err := client.PutCreate(ctx, book+uid+".vcf", dav.Typed(strings.NewReader(card), "text/vcard; charset=utf-8")); err != nil {
		t.Fatalf("seed card: %v", err)
	}
}

func seedSeries(ctx context.Context, t *testing.T, client *dav.Client, cal, uid string) {
	t.Helper()
	series := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20261006T100000Z\r\n" +
		"DTEND:20261006T103000Z\r\nRRULE:FREQ=WEEKLY;COUNT=3\r\nSUMMARY:Standup\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20260101T000000Z\r\nRECURRENCE-ID:20261013T100000Z\r\n" +
		"DTSTART:20261013T120000Z\r\nDTEND:20261013T123000Z\r\nSUMMARY:Standup moved\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	if _, err := client.PutCreate(ctx, cal+uid+".ics", dav.Typed(strings.NewReader(series), "text/calendar; charset=utf-8")); err != nil {
		t.Fatalf("seed series: %v", err)
	}
}
