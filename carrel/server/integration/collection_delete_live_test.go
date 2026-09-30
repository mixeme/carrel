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
	"carrel.dev/carrel/server/view"
)

// S-30.2 for S-12.4: a calendar and an address book deleted on a live server
// through the sheet. The sheet counts what the server holds; its file, taken
// first, holds the series, its instance and a task the calendar of events
// keeps; a name that is not the one shown sends nothing; the name shown takes
// the collection off the server and out of the catalog.
func TestRadicaleCollectionDelete(t *testing.T) {
	runCollectionDelete(t, radicaleEndpoint(t), radicaleUser, radicalePass)
}

func TestBaikalCollectionDelete(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runCollectionDelete(t, endpoint, user, pass)
}

func runCollectionDelete(t *testing.T, endpoint, user, pass string) {
	ctx := context.Background()
	suffix := fmt.Sprintf("carrel-s124-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	client := liveClient(t, endpoint, user, pass)
	calHome, bookHome := liveHomes(ctx, t, endpoint, user, pass)

	calPath := calHome + suffix + "-cal/"
	bookPath := bookHome + suffix + "-book/"
	calPath = makeCollection(ctx, t, client, endpoint, user, pass, "MKCALENDAR", calPath, `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel S-12.4</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/><C:comp name="VTODO"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
	bookPath = makeCollection(ctx, t, client, endpoint, user, pass, "MKCOL", bookPath, `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel S-12.4 book</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)

	series := "series-" + suffix
	seedSeries(ctx, t, client, calPath, series)
	task := "task-" + suffix
	todo := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\nBEGIN:VTODO\r\nUID:" + task +
		"\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Write report\r\nX-CUSTOM:keep-me\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	if _, err := client.PutCreate(ctx, calPath+task+".ics", dav.Typed(strings.NewReader(todo), "text/calendar; charset=utf-8")); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	ada := "ada-" + suffix
	seedCard(ctx, t, client, bookPath, ada)

	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	blob := liveCatalog(t, app)
	cal, book := catalogEntry(t, blob, calPath), catalogEntry(t, blob, bookPath)

	t.Run("calendar", func(t *testing.T) {
		sheet := liveDeleteSheet(t, app, connID, cal.Path)
		if w := string(sheet.Warning); !strings.Contains(w, "all 2 records") {
			t.Errorf("warning = %q", w)
		}
		if sheet.Export == nil || string(sheet.Export.Label) != "Download .ics first" {
			t.Fatalf("export = %+v", sheet.Export)
		}
		body := liveDownload(t, app, sheet.Export.Href, ".ics")
		if strings.Count(body, "BEGIN:VCALENDAR") != 1 || strings.Count(body, "BEGIN:VTODO") != 1 {
			t.Errorf("the file before deleting is not one envelope with the task once:\n%s", body)
		}
		mustHave(t, "file before deleting", body, "UID:"+series, "RRULE:FREQ=WEEKLY;COUNT=3", "RECURRENCE-ID:20261013T100000Z", "UID:"+task, "X-CUSTOM:keep-me")

		// The name is the server's to keep: SoGo names a collection after its
		// address and ignores the displayname of the create (SV-17). The sheet
		// asks for what the catalog holds, which is what the person sees.
		liveDeleteByName(ctx, t, app, client, connID, cal.Path, sheet, cal.Name)
	})

	t.Run("contacts", func(t *testing.T) {
		sheet := liveDeleteSheet(t, app, connID, book.Path)
		if w := string(sheet.Warning); !strings.Contains(w, "address book and all 1 contact in it") {
			t.Errorf("warning = %q", w)
		}
		if sheet.Export == nil || string(sheet.Export.Label) != "Download .vcf first" {
			t.Fatalf("export = %+v", sheet.Export)
		}
		body := liveDownload(t, app, sheet.Export.Href, ".vcf")
		mustHave(t, "file before deleting", body, "UID:"+ada, "FN:Ada Lovelace", "X-CUSTOM:keep-me")

		liveDeleteByName(ctx, t, app, client, connID, book.Path, sheet, book.Name)
	})
}

func liveDeleteSheet(t *testing.T, app *liveApp, connID, path string) view.CollectionDeleteSheet {
	t.Helper()
	return liveDecode[view.CollectionDeleteSheet](t, "delete sheet", app.get("/app/collections/delete?"+url.Values{
		"a": {connID}, "c": {path},
	}.Encode()))
}

// liveDeleteByName types a wrong name, then the one the sheet shows, and
// looks at the server and the catalog after each.
func liveDeleteByName(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, path string, sheet view.CollectionDeleteSheet, name string) {
	t.Helper()
	var hint view.Text
	for _, set := range sheet.Form.Sets {
		for _, row := range set.Rows {
			// The name is the field's sample since 28.09.2026
			// (s-profile-9, s-profile-19); before that it was the hint.
			if row.Name == "name" {
				hint = row.Placeholder
				if hint == "" {
					hint = row.Hint
				}
			}
		}
	}
	if string(hint) != name {
		t.Fatalf("the sheet asks for %q, want %q", hint, name)
	}
	confirm := liveAction(t, sheet.Actions, "collection.delete.confirm")

	// A wrong name has to be wrong on every server: lower case is not, because
	// SoGo names a collection after its address and that is lower case already
	// (docs/servers.md, SV-17).
	again := liveDecode[view.CollectionDeleteSheet](t, "wrong name", app.post(confirm, url.Values{"name": {name + " (not this one)"}}))
	if again.Subject != sheet.Subject {
		t.Errorf("a wrong name answered %+v", again)
	}
	if _, err := client.PropFind(ctx, path, dav.DepthZero, []xml.Name{displayNameProp}); err != nil {
		t.Fatalf("a wrong name reached the server: PROPFIND = %v", err)
	}

	if loc := redirected(t, "delete", app.post(confirm, url.Values{"name": {" " + name + " "}})); loc != "/app/settings/connections" {
		t.Errorf("delete went to %q", loc)
	}
	// The collection is gone from the server: either the address answers 404,
	// or the server says it is deleted. Nextcloud keeps a deleted calendar at
	// its address, among the children of the home, and calls it a calendar to
	// any request that does not ask about the deletion — so the question is
	// asked here the way discovery asks it, with nc:deleted-at beside the
	// resource type (docs/servers.md, SV-30). An address book on the same
	// server is deleted outright.
	ms, err := client.PropFind(ctx, path, dav.DepthZero, []xml.Name{resourceTypeProp, deletedAtProp})
	switch {
	case dav.IsStatus(err, http.StatusNotFound):
	case err != nil:
		t.Errorf("after delete PROPFIND = %v, want 404 or a collection the server calls deleted", err)
	default:
		var kind, when string
		for _, resp := range ms.Responses {
			kind += string(resp.Props[resourceTypeProp])
			when += strings.TrimSpace(string(resp.Props[deletedAtProp]))
		}
		stillOfItsKind := strings.Contains(kind, ":calendar/") || strings.Contains(kind, ":addressbook/") ||
			strings.Contains(kind, "<calendar ") || strings.Contains(kind, "<addressbook ")
		if when == "" && stillOfItsKind {
			t.Errorf("after delete the address is still a collection of its kind and says nothing about a deletion: %s", kind)
		}
		t.Logf("the address still answers: resourcetype %s, deleted at %q", kind, when)
	}
	if _, ok := liveCatalog(t, app).Collection(connID, path); ok {
		t.Error("the catalog still holds it")
	}
}
