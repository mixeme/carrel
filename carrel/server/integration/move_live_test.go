//go:build integration

package integration

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	if !strings.Contains(runsText(sheet.Warning), "another account") {
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
		isAda := func(b string) bool { return strings.Contains(b, "FN:Ada Lovelace") }
		intoA := "/app/contacts/" + connID + "/" + url.PathEscape(catalogEntry(t, blob, cols.bookA).Path)
		intoB := "/app/contacts/" + connID + "/" + url.PathEscape(catalogEntry(t, blob, cols.bookB).Path)

		sheet := liveDecode[view.MoveSheet](t, "move sheet", app.get(moveURL("contacts", cols.bookA, uid, cols.bookB, false)))
		// On one account the sheet says only what it always says: a MOVE.
		if !strings.HasPrefix(runsText(sheet.Warning), "Inside one server this is MOVE") {
			t.Errorf("a move on one account warned of something else: %q", sheet.Warning)
		}
		// A server may give the moved record an identity of its own (SoGo does:
		// docs/servers.md, SV-18). Then the move is done and said out loud, and
		// the address the record had is not offered again, because it opens
		// nothing.
		loc := liveLanded(t, "move", app.post(liveAction(t, sheet.Actions, "record.move.confirm"), url.Values{}), intoB)
		kept := loc != ""
		if kept && loc != intoB+"/"+url.PathEscape(uid) {
			t.Errorf("move went to %q", loc)
		}
		if _, _, err := cols.client.Get(ctx, cols.bookA+uid+".vcf", nil); !dav.IsStatus(err, http.StatusNotFound) {
			t.Errorf("the original after MOVE: GET = %v, want 404", err)
		}
		// The name of the resource is the server's to choose, so the card is
		// looked for by what is written in it.
		moved, movedUID := liveRecordWhere(ctx, t, cols.client, cols.bookB, "of Ada", isAda)
		if kept {
			if hasMove(ctx, t, cols.client, cols.bookB) {
				if moved != before {
					t.Errorf("MOVE changed the card:\nbefore %s\nafter  %s", before, moved)
				}
			} else {
				// No MOVE on this server, so the record went over as a copy
				// and a copy is written: it carries a stamp of its own and
				// the order is the writer's (docs/servers.md, SV-44). What
				// must survive is everything the record held.
				mustHave(t, "moved card", moved, "FN:Ada Lovelace", "N:Lovelace;Ada;;;", "X-CUSTOM:keep-me")
				if movedUID != uid {
					t.Errorf("the copy carries UID %s, want %s", movedUID, uid)
				}
			}
		} else {
			// The notice is only true if the identity really did change.
			mustHave(t, "moved card", moved, "FN:Ada Lovelace", "X-CUSTOM:keep-me")
			if movedUID == uid {
				t.Errorf("the page said the identity changed, but the card still carries UID %s", uid)
			}
		}

		sheet = liveDecode[view.MoveSheet](t, "copy sheet", app.get(moveURL("contacts", cols.bookB, movedUID, cols.bookA, true)))
		back := liveLanded(t, "copy back", app.post(liveAction(t, sheet.Actions, "record.copy.confirm"), url.Values{}), intoA)
		if back != "" && back != intoA+"/"+url.PathEscape(movedUID) {
			t.Errorf("copy went to %q", back)
		}
		copied, _ := liveRecordWhere(ctx, t, cols.client, cols.bookA, "of Ada", isAda)
		mustHave(t, "copy", copied, "FN:Ada Lovelace", "X-CUSTOM:keep-me")
		// A copy leaves the original where it was.
		liveRecordWhere(ctx, t, cols.client, cols.bookB, "of Ada", isAda)
	})

	t.Run("event", func(t *testing.T) {
		series := "series-" + suffix
		seedSeries(ctx, t, cols.client, cols.calA, series)
		sheet := liveDecode[view.MoveSheet](t, "move sheet", app.get(moveURL("calendar", cols.calA, series, cols.calB, false)))
		redirected(t, "move", app.post(liveAction(t, sheet.Actions, "record.move.confirm"), url.Values{}))
		if _, _, err := cols.client.Get(ctx, cols.calA+series+".ics", nil); !dav.IsStatus(err, http.StatusNotFound) {
			t.Errorf("the original after MOVE: GET = %v, want 404", err)
		}
		got := liveCardByUID(ctx, t, cols.client, cols.calB, series)
		mustHave(t, "moved event", got, "UID:"+series, "RRULE:FREQ=WEEKLY;COUNT=3", "RECURRENCE-ID:20261013T100000Z", "SUMMARY:Standup moved")
		agenda := liveDecode[view.AgendaPage](t, "agenda at the target",
			app.get("/app/calendar/"+connID+"/"+url.PathEscape(catalogEntry(t, blob, cols.calB).Path)+"?from=2026-10-12"))
		// A week is seven days and the empty ones are shown, so the moved
		// series is looked for across the week rather than in a day of its
		// own (C-6.3). Printing the whole page on failure printed a screen
		// nobody could read; the rows are what the assertion is about.
		var lines []string
		for _, d := range agenda.Days {
			for _, r := range d.Rows {
				lines = append(lines, string(d.Rubric)+" · "+string(r.Summary))
			}
		}
		if len(agenda.Notices) > 0 || len(agenda.Days) != 7 ||
			strings.Join(lines, "|") != "Tuesday, 13 October · Standup moved" {
			t.Errorf("agenda at the target: %d days, rows %v, notices %+v",
				len(agenda.Days), lines, agenda.Notices)
		}
	})
}

type moveCollections struct {
	client                   *dav.Client
	bookA, bookB, calA, calB string
}

// liveCardByUID is the record of a collection that carries this UID, whatever
// the server called the resource. SoGo names a moved or copied resource itself
// (docs/servers.md, SV-18), and Carrel finds a record by its UID for the same
// reason; a test that looked it up by name would be testing an assumption no
// standard makes.
// hasMove asks the server whether it has MOVE at all. Xandikos answers 405 to
// every one of them, across collections and for a rename alike, and offers no
// COPY either (docs/servers.md, SV-44); a record still moves there, but as a
// copy, and a copy is written and cannot be byte-for-byte what it was. Which
// standard to hold the move to is therefore the server's answer to give, not
// a list of server names in this suite.
func hasMove(ctx context.Context, t *testing.T, client *dav.Client, collection string) bool {
	t.Helper()
	from := collection + fmt.Sprintf("carrel-move-probe-%d.vcf", os.Getpid())
	to := collection + fmt.Sprintf("carrel-move-probe-%d-2.vcf", os.Getpid())
	card := strings.Join([]string{"BEGIN:VCARD", "VERSION:3.0", "UID:carrel-move-probe", "FN:Probe", "END:VCARD", ""}, "\r\n")
	if _, err := client.PutCreate(ctx, from, dav.Typed(strings.NewReader(card), "text/vcard; charset=utf-8")); err != nil {
		t.Logf("the MOVE probe could not be seeded (%v); assuming the server has MOVE", err)
		return true
	}
	err := client.Move(ctx, from, to, false)
	if err == nil {
		_ = client.Delete(ctx, to, "")
		return true
	}
	_ = client.Delete(ctx, from, "")
	if dav.IsStatus(err, http.StatusMethodNotAllowed) || dav.IsStatus(err, http.StatusNotImplemented) {
		t.Logf("this server has no MOVE (%v): the move is held to «nothing lost», not «nothing changed»", err)
		return false
	}
	return true
}

func liveCardByUID(ctx context.Context, t *testing.T, client *dav.Client, collection, uid string) string {
	t.Helper()
	body, _ := liveRecordWhere(ctx, t, client, collection, "with UID "+uid, func(b string) bool {
		return strings.Contains(b, "UID:"+uid)
	})
	return body
}

// liveRecordWhere is the record of a collection the test recognises, with the
// UID the server ended up giving it. A server may re-identify a record it was
// handed (SV-18), so what is written in the record is the only thing a test
// can look for; neither the address nor the identifier is Carrel's to assume.
// liveHasRecord walks a collection and answers with the address and body of
// the first resource that matches, or two empty strings. Unlike
// liveRecordWhere it does not fail: a caller may be proving absence, and
// absence is not a broken test.
func liveHasRecord(ctx context.Context, t *testing.T, client *dav.Client, collection string, match func(string) bool) (href, body string) {
	t.Helper()
	ms, err := client.PropFind(ctx, collection, dav.DepthOne, []xml.Name{{Space: "DAV:", Local: "getetag"}})
	if err != nil {
		t.Fatalf("PROPFIND %s: %v", collection, err)
	}
	for _, resp := range ms.Responses {
		at := resp.Href
		if u, err := url.Parse(at); err == nil {
			at = u.Path
		}
		if strings.HasSuffix(at, "/") {
			continue
		}
		if b, _ := rawBody(ctx, t, client, at); match(b) {
			return at, b
		}
	}
	return "", ""
}

func liveRecordWhere(ctx context.Context, t *testing.T, client *dav.Client, collection, what string, match func(string) bool) (string, string) {
	t.Helper()
	ms, err := client.PropFind(ctx, collection, dav.DepthOne, []xml.Name{{Space: "DAV:", Local: "getetag"}})
	if err != nil {
		t.Fatalf("PROPFIND %s: %v", collection, err)
	}
	var walked []string
	for _, resp := range ms.Responses {
		href := resp.Href
		if u, err := url.Parse(href); err == nil {
			href = u.Path
		}
		if strings.HasSuffix(href, "/") {
			continue
		}
		body, _ := rawBody(ctx, t, client, href)
		if match(body) {
			return body, uidOf(body)
		}
		found := "no UID line"
		if u := uidOf(body); u != "" {
			found = "UID:" + u
		}
		walked = append(walked, href+" ("+found+")")
	}
	// What the walk saw is the evidence: a resource under another name or
	// another identifier is one thing (SV-18), an empty listing quite another.
	t.Fatalf("no record %s in %s; %d resource(s) walked: %s",
		what, collection, len(walked), strings.Join(walked, ", "))
	return "", ""
}

// uidOf is the identifier a record carries, or "" when it carries none.
func uidOf(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "UID:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "UID:"))
		}
	}
	return ""
}

// liveLanded is where a write into another collection ended up: the address it
// redirected to when the server kept the identity of the record, and "" when
// the server gave it one of its own (SV-18) and Carrel said so instead of
// redirecting to an address that opens nothing. wantOpen is the collection the
// notice leads into: since v0.2.30 the notice opens the record there under the
// identity the server gave it («Open it there», record.open), not the address
// it had.
func liveLanded(t *testing.T, step string, rec *httptest.ResponseRecorder, wantOpen string) string {
	t.Helper()
	if rec.Code == http.StatusSeeOther {
		return rec.Header().Get("Location")
	}
	page := liveDecode[view.ContactPage](t, step, rec)
	if !hasLiveNotice(page.Notices, "another identity") {
		t.Fatalf("%s: a page with nothing said about the identity: %+v", step, page.Notices)
	}
	for _, n := range page.Notices {
		for _, a := range n.Actions {
			if a.ID == "record.open" {
				if !strings.HasPrefix(a.Href, wantOpen+"/") {
					t.Errorf("%s: the notice leads to %q, want a record in %q", step, a.Href, wantOpen)
				}
				return ""
			}
		}
	}
	t.Fatalf("%s: the notice offers no way to the moved record: %+v", step, page.Notices)
	return ""
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
	for _, bp := range []*string{&c.bookA, &c.bookB} {
		want := *bp
		letter := want[len(want)-2 : len(want)-1]
		*bp = makeCollection(ctx, t, client, endpoint, user, pass, "MKCOL", want, `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel S-18 `+letter+`</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)
	}
	for _, cp := range []*string{&c.calA, &c.calB} {
		want := *cp
		letter := want[len(want)-2 : len(want)-1]
		*cp = makeCollection(ctx, t, client, endpoint, user, pass, "MKCALENDAR", want, `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel S-18 `+letter+`</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
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
