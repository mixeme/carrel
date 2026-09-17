//go:build integration

package integration

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for S-12.3: a whole collection moved and copied between two live
// servers through the request layer. Radicale holds the originals, Baikal
// takes them: the calendar is made there with its name, colour and component
// set, every record travels, and the original goes only after the copies are
// read back.
func TestRadicaleBaikalCollectionMove(t *testing.T) {
	bEndpoint, bUser, bPass := baikalConfig(t)
	rEndpoint := radicaleEndpoint(t)
	ctx := context.Background()
	suffix := moveSuffix()

	rClient := liveClient(t, rEndpoint, radicaleUser, radicalePass)
	bClient := liveClient(t, bEndpoint, bUser, bPass)
	rCalHome, rBookHome := liveHomes(ctx, t, rEndpoint, radicaleUser, radicalePass)
	bCalHome, bBookHome := liveHomes(ctx, t, bEndpoint, bUser, bPass)

	calPath := rCalHome + suffix + "-cal/"
	bookPath := rBookHome + suffix + "-book/"
	calPath = makeCollection(ctx, t, rClient, rEndpoint, radicaleUser, radicalePass, "MKCALENDAR", calPath, `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:I="http://apple.com/ns/ical/">
  <D:set><D:prop>
    <D:displayname>Carrel S-12.3</D:displayname>
    <I:calendar-color>#6b5c73</I:calendar-color>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/><C:comp name="VTODO"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
	bookPath = makeCollection(ctx, t, rClient, rEndpoint, radicaleUser, radicalePass, "MKCOL", bookPath, `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel S-12.3 book</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)

	series := "series-" + suffix
	seedSeries(ctx, t, rClient, calPath, series)
	task := "task-" + suffix
	todo := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\nBEGIN:VTODO\r\nUID:" + task +
		"\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Write report\r\nX-CUSTOM:keep-me\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	if _, err := rClient.PutCreate(ctx, calPath+task+".ics", dav.Typed(strings.NewReader(todo), "text/calendar; charset=utf-8")); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	uid := "ada-" + suffix
	seedCard(ctx, t, rClient, bookPath, uid)

	app := newLiveApp(t)
	radID := connectLive(t, app, rEndpoint, radicaleUser, radicalePass)
	baiID := connectLive(t, app, bEndpoint, bUser, bPass)
	blob := liveCatalog(t, app)
	srcCal := catalogEntry(t, blob, calPath)
	srcBook := catalogEntry(t, blob, bookPath)

	// The calendar moves: Baikal holds it, Radicale no longer does.
	sheet := liveDecode[view.CollectionMoveSheet](t, "calendar sheet", app.get("/app/collections/move?"+url.Values{
		"a": {radID}, "c": {srcCal.Path}, "to": {baiID},
	}.Encode()))
	if sheet.Warning == "" || !strings.Contains(string(sheet.Subject), "Carrel S-12.3") {
		t.Fatalf("sheet = %+v", sheet)
	}
	loc := redirected(t, "move calendar", app.post(liveAction(t, sheet.Actions, "collection.move.confirm"), url.Values{}))
	newCal := livePathOf(t, loc, "/app/calendar/"+baiID+"/")
	t.Cleanup(func() { _ = bClient.Delete(context.Background(), newCal, "") })
	if !strings.HasPrefix(newCal, bCalHome) {
		t.Errorf("the calendar went to %q, outside %q", newCal, bCalHome)
	}
	got, _ := rawBody(ctx, t, bClient, newCal+series+".ics")
	mustHave(t, "moved series", got, "UID:"+series, "RRULE:FREQ=WEEKLY;COUNT=3", "RECURRENCE-ID:20261013T100000Z", "SUMMARY:Standup moved")
	got, _ = rawBody(ctx, t, bClient, newCal+task+".ics")
	mustHave(t, "moved task", got, "UID:"+task, "X-CUSTOM:keep-me")
	props := liveProps(ctx, t, bClient, newCal, displayNameProp, calendarColourProp)
	if props["displayname"] != "Carrel S-12.3" || !strings.Contains(strings.ToLower(props["calendar-color"]), "6b5c73") {
		t.Errorf("the calendar on Baikal = %+v", props)
	}
	if _, err := rClient.PropFind(ctx, calPath, dav.DepthZero, []xml.Name{displayNameProp}); !dav.IsStatus(err, http.StatusNotFound) {
		t.Errorf("the original calendar on Radicale: PROPFIND = %v, want 404", err)
	}
	blob = liveCatalog(t, app)
	if _, ok := blob.Collection(radID, srcCal.Path); ok {
		t.Error("the catalog still holds the original calendar")
	}
	if entry, ok := blob.Collection(baiID, newCal); !ok || entry.Name != "Carrel S-12.3" {
		t.Errorf("new catalog entry = %+v (found %v)", entry, ok)
	}

	// The address book is copied: both stay.
	sheet = liveDecode[view.CollectionMoveSheet](t, "book sheet", app.get("/app/collections/move?"+url.Values{
		"a": {radID}, "c": {srcBook.Path}, "to": {baiID}, "copy": {"1"},
	}.Encode()))
	loc = redirected(t, "copy book", app.post(liveAction(t, sheet.Actions, "collection.copy.confirm"), url.Values{}))
	newBook := livePathOf(t, loc, "/app/contacts/"+baiID+"/")
	t.Cleanup(func() { _ = bClient.Delete(context.Background(), newBook, "") })
	if !strings.HasPrefix(newBook, bBookHome) {
		t.Errorf("the book went to %q, outside %q", newBook, bBookHome)
	}
	got, _ = rawBody(ctx, t, bClient, newBook+uid+".vcf")
	mustHave(t, "copied card", got, "UID:"+uid, "FN:Ada Lovelace", "X-CUSTOM:keep-me")
	if _, _, err := rClient.Get(ctx, bookPath+uid+".vcf", nil); err != nil {
		t.Errorf("a copy took the original card away: %v", err)
	}
	blob = liveCatalog(t, app)
	if _, ok := blob.Collection(radID, srcBook.Path); !ok {
		t.Error("a copy took the original book out of the catalog")
	}
}

func liveClient(t *testing.T, endpoint, user, pass string) *dav.Client {
	t.Helper()
	client, err := dav.NewClient(testGuard(t), endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func liveHomes(ctx context.Context, t *testing.T, endpoint, user, pass string) (cal, book string) {
	t.Helper()
	res, trace, err := discover.Run(ctx, testGuard(t), endpoint, user, pass, "")
	if err != nil || res.CalendarHome == "" || res.AddressBookHome == "" {
		t.Fatalf("discovery: %v %+v %+v", err, res, trace)
	}
	return strings.TrimSuffix(res.CalendarHome, "/") + "/", strings.TrimSuffix(res.AddressBookHome, "/") + "/"
}

// livePathOf is the collection address inside a list page's address.
func livePathOf(t *testing.T, loc, prefix string) string {
	t.Helper()
	rest, ok := strings.CutPrefix(loc, prefix)
	if !ok {
		t.Fatalf("went to %q, want it under %q", loc, prefix)
	}
	p, err := url.PathUnescape(rest)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

var (
	displayNameProp    = xml.Name{Space: "DAV:", Local: "displayname"}
	calendarColourProp = xml.Name{Space: "http://apple.com/ns/ical/", Local: "calendar-color"}
	resourceTypeProp   = xml.Name{Space: "DAV:", Local: "resourcetype"}
	// A server with a trash bin tells the truth about a deleted collection
	// only when asked about it (docs/servers.md, SV-30).
	deletedAtProp = xml.Name{Space: "http://nextcloud.com/ns", Local: "deleted-at"}
)
