//go:build integration

package integration

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for S-12.1–12.2: create and rename through the request layer on a
// live server — the slug, one MKCALENDAR or MKCOL with name, colour and
// components, a PROPPATCH for the label, and a catalog path that re-discovery
// recognises as the one the server reports.
func TestRadicaleCollections(t *testing.T) {
	runCollections(t, radicaleEndpoint(t), radicaleUser, radicalePass)
}

func TestBaikalCollections(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runCollections(t, endpoint, user, pass)
}

func liveProps(ctx context.Context, t *testing.T, c *dav.Client, path string, names ...xml.Name) map[string]string {
	t.Helper()
	ms, err := c.PropFind(ctx, path, dav.DepthZero, names)
	if err != nil {
		t.Fatalf("PROPFIND %s: %v", path, err)
	}
	out := map[string]string{}
	for _, r := range ms.Responses {
		for n, v := range r.Props {
			inner := strings.TrimSpace(string(v))
			// A text property arrives as XML content: "&" is "&amp;".
			var txt struct {
				Text string `xml:",chardata"`
			}
			if !strings.Contains(inner, "<") && xml.Unmarshal([]byte("<v>"+inner+"</v>"), &txt) == nil {
				inner = strings.TrimSpace(txt.Text)
			}
			out[n.Local] = inner
		}
	}
	return out
}

// componentsOf reads the comp names out of a supported-calendar-component-set
// as liveProps flattened it: the inner XML, each name in a name= attribute.
// XML quotes an attribute either way, and iCloud writes name='VEVENT': read
// by double quotes alone, its set came out empty, and an empty set means
// every kind.
func componentsOf(raw string) []string {
	var out []string
	for _, q := range []string{`"`, `'`} {
		for _, part := range strings.Split(raw, `name=`+q)[1:] {
			if i := strings.Index(part, q); i > 0 {
				out = append(out, part[:i])
			}
		}
	}
	return out
}

// escapesNamesTwice is a server that escapes a collection's displayname once
// more on the way out: sent «A &amp; B», it answers «A &amp;amp; B», so every
// client reads «A &amp; B» (docs/servers.md, SV-50: DAViCal). What the server
// shows is the name, and the catalog must hold that (S-12.2); Carrel's own
// escaping is held to the letter by every other server in the lineup.
type collectionsQuirk int

const escapesNamesTwice collectionsQuirk = 1

func runCollections(t *testing.T, endpoint, user, pass string, quirks ...collectionsQuirk) {
	shown := func(name string) string {
		for _, q := range quirks {
			if q == escapesNamesTwice {
				return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(name)
			}
		}
		return name
	}
	ctx := context.Background()
	g := testGuard(t)
	client, err := dav.NewClient(g, endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	res, trace, err := discover.Run(ctx, g, endpoint, user, pass, "")
	if err != nil || res.CalendarHome == "" || res.AddressBookHome == "" {
		t.Fatalf("discovery: %v %+v %+v", err, res, trace)
	}
	suffix := fmt.Sprintf("carrel-s12-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	home := func(h string) string { return strings.TrimSuffix(h, "/") + "/" }

	// Connecting needs one collection on the server; this one is made by hand.
	// With a name, as Carrel's own form always sends one: Davis refuses an
	// address book without it, 500 (docs/servers.md, SV-49).
	seed := home(res.AddressBookHome) + suffix + "-seed/"
	seed = makeCollection(ctx, t, client, endpoint, user, pass, "MKCOL", seed, `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop><D:resourcetype><D:collection/><C:addressbook/></D:resourcetype><D:displayname>Seed</D:displayname></D:prop></D:set>
</D:mkcol>`)

	app := newLiveApp(t)
	redirected(t, "connect", app.post("/app/settings/connections", url.Values{
		"server": {endpoint}, "login": {user}, "password": {pass},
	}))
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	conn := blob.Connections[0]

	displayName := xml.Name{Space: "DAV:", Local: "displayname"}
	colour := xml.Name{Space: "http://apple.com/ns/ical/", Local: "calendar-color"}
	comps := xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "supported-calendar-component-set"}
	resourceType := xml.Name{Space: "DAV:", Local: "resourcetype"}

	name := "Поездки & планы " + suffix
	calPath := home(res.CalendarHome) + "poezdki-plany-" + suffix + "/"
	loc := redirected(t, "new calendar", app.post("/app/collections/new?kind=event", url.Values{
		"account": {conn.ID}, "name": {name}, "colour": {"#6b4e7a"}, "components": {"VEVENT", "VTODO"},
	}))
	t.Cleanup(func() { _ = client.Delete(context.Background(), calPath, "") })
	// The sheet is opened from Connections and goes back there (Р-75,
	// Typecase's note 112); the calendar is where it was asked to be.
	if loc != "/app/settings/connections" {
		t.Fatalf("new calendar went to %q, want back to Connections", loc)
	}
	loc = "/app/calendar/" + conn.ID + "/" + url.PathEscape(calPath)
	got := liveProps(ctx, t, client, calPath, displayName, colour, comps)
	if got["displayname"] != shown(name) || !strings.Contains(strings.ToLower(got["calendar-color"]), "6b4e7a") ||
		!strings.Contains(got["supported-calendar-component-set"], "VEVENT") ||
		!strings.Contains(got["supported-calendar-component-set"], "VTODO") {
		t.Errorf("calendar on the server = %+v", got)
	}
	// Whether the server narrowed the set to what was asked is the server's
	// business — Xandikos holds every component in every calendar and cannot
	// be told otherwise (docs/servers.md, SV-43). What is checked here is the
	// rule of S-12.2: the catalog holds the server's answer, not the form's.
	// A catalog narrower than the server hides records that are really there.
	blob, _ = app.users.Catalog(app.sess.AccountID(), key)
	entry := catalogEntry(t, blob, calPath)
	if entry.Name != got["displayname"] {
		t.Errorf("the catalog names the calendar %q, the server %q", entry.Name, got["displayname"])
	}
	for _, k := range discover.CalendarKinds(componentsOf(got["supported-calendar-component-set"])) {
		if !entry.Has(k) {
			t.Errorf("the server holds %s and the catalog does not: server %q, catalog %v",
				k, got["supported-calendar-component-set"], entry.Kinds)
		}
	}
	if agenda := liveDecode[view.AgendaPage](t, "agenda", app.get(loc)); len(agenda.Notices) > 0 {
		t.Errorf("agenda of the new calendar: %+v", agenda.Notices)
	}

	redirected(t, "rename", app.post("/app/collections/rename?"+url.Values{"a": {conn.ID}, "c": {calPath}}.Encode(),
		url.Values{"name": {name + " renamed"}, "colour": {"#3f6e8c"}}))
	got = liveProps(ctx, t, client, calPath, displayName, colour)
	if got["displayname"] != shown(name+" renamed") || !strings.Contains(strings.ToLower(got["calendar-color"]), "3f6e8c") {
		t.Errorf("after rename = %+v", got)
	}

	bookPath := home(res.AddressBookHome) + suffix + "-team/"
	redirected(t, "new address book", app.post("/app/collections/new?kind=contact", url.Values{
		"account": {conn.ID}, "name": {"Team " + suffix}, "address": {suffix + "-team"},
	}))
	t.Cleanup(func() { _ = client.Delete(context.Background(), bookPath, "") })
	got = liveProps(ctx, t, client, bookPath, displayName, resourceType)
	if got["displayname"] != "Team "+suffix || !strings.Contains(got["resourcetype"], "addressbook") {
		t.Errorf("address book on the server = %+v", got)
	}

	// Radicale keeps both kinds in one home: the book's address is taken for a
	// calendar too, and the PROPFIND of the home is what says so.
	if home(res.CalendarHome) == home(res.AddressBookHome) {
		sheet := liveDecode[view.CollectionNewSheet](t, "taken address", app.post("/app/collections/new?kind=event", url.Values{
			"account": {conn.ID}, "name": {"Taken"}, "components": {"VEVENT"}, "address": {suffix + "-team"},
		}))
		var addrErr view.Text
		for _, r := range sheet.Form.Sets[0].Rows {
			if r.Name == "address" {
				addrErr = r.Error
			}
		}
		if addrErr == "" {
			t.Errorf("a taken address in a shared home was not refused: %+v", sheet)
		}
	}

	page := liveDecode[view.ConnectionsPage](t, "rediscover", app.post("/app/settings/connections", url.Values{"op": {"rediscover"}, "account": {conn.ID}}))
	for _, n := range page.Notices {
		if n.Title == "Collections changed on the server" && strings.Contains(string(n.Body), suffix) {
			t.Errorf("re-discovery offers what Carrel created as new: %s", n.Body)
		}
	}
}
