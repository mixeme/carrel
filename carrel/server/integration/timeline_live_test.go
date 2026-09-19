//go:build integration

package integration

import (
	"context"
	"fmt"
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

// S-30.2 for S-15.4: the person's timeline on servers people connect. An
// event with the card's address in ATTENDEE is found as "attendee", a note
// with RELATED-TO the card as "related to this person", and an event that
// only says the name in its text is not found at all.
func TestRadicaleTimeline(t *testing.T) {
	runTimeline(t, radicaleEndpoint(t), radicaleUser, radicalePass)
}

func TestBaikalTimeline(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runTimeline(t, endpoint, user, pass)
}

func TestNextcloudTimeline(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runTimeline(t, endpoint, user, pass)
}

func runTimeline(t *testing.T, endpoint, user, pass string) {
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
	suffix := fmt.Sprintf("carrel-timeline-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	calPath := makeCollection(ctx, t, client, endpoint, user, pass, "MKCALENDAR",
		strings.TrimSuffix(res.CalendarHome, "/")+"/"+suffix+"-cal/", `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel timeline</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/><C:comp name="VTODO"/><C:comp name="VJOURNAL"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
	bookPath := makeCollection(ctx, t, client, endpoint, user, pass, "MKCOL",
		strings.TrimSuffix(res.AddressBookHome, "/")+"/"+suffix+"-book/", `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel timeline</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)

	uid := "anna-" + suffix
	seed(ctx, t, client, bookPath+uid+".vcf", "text/vcard; charset=utf-8",
		"BEGIN:VCARD\nVERSION:3.0\nUID:"+uid+"\nFN:Anna Kovaleva\nEMAIL:anna@carrel.test\nEND:VCARD\n")
	at := time.Now().AddDate(0, 0, -7).UTC().Format("20060102T150405Z")
	seed(ctx, t, client, calPath+"met-"+suffix+".ics", "text/calendar; charset=utf-8",
		"BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\nBEGIN:VEVENT\nUID:met-"+suffix+"\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:"+at+"\nDTEND:"+at+"\nSUMMARY:Review with the team\nATTENDEE;PARTSTAT=ACCEPTED:mailto:anna@carrel.test\nEND:VEVENT\nEND:VCALENDAR\n")
	seed(ctx, t, client, calPath+"note-"+suffix+".ics", "text/calendar; charset=utf-8",
		"BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\nBEGIN:VJOURNAL\nUID:note-"+suffix+"\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:"+at+"\nSUMMARY:Call notes\nRELATED-TO:"+uid+"\nEND:VJOURNAL\nEND:VCALENDAR\n")
	seed(ctx, t, client, calPath+"said-"+suffix+".ics", "text/calendar; charset=utf-8",
		"BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\nBEGIN:VEVENT\nUID:said-"+suffix+"\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:"+at+"\nDTEND:"+at+"\nSUMMARY:Anna Kovaleva said hello\nEND:VEVENT\nEND:VCALENDAR\n")

	app := newLiveApp(t)
	redirected(t, "connect", app.post("/app/settings/connections", url.Values{
		"server": {endpoint}, "login": {user}, "password": {pass},
	}))
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	conn := blob.Connections[0]
	cal, book := catalogEntry(t, blob, calPath), catalogEntry(t, blob, bookPath)
	// The timeline asks the ticked collections (S-15.4).
	for _, c := range []sealed.Collection{cal, book} {
		for _, k := range c.Kinds {
			blob.Selected = append(blob.Selected, sealed.Selection{Kind: k, AccountID: conn.ID, Path: c.Path})
		}
	}
	if err := app.users.SetSelection(app.sess.AccountID(), key, view.KindEvent, blob.Selected); err != nil {
		t.Fatal(err)
	}
	for _, k := range []view.Kind{view.KindTask, view.KindNote, view.KindContact} {
		if err := app.users.SetSelection(app.sess.AccountID(), key, k, blob.Selected); err != nil {
			t.Fatal(err)
		}
	}

	rec := "/app/contacts/" + conn.ID + "/" + url.PathEscape(book.Path) + "/" + url.PathEscape(uid)
	if got := app.get(rec); got.Code != 200 {
		t.Fatalf("the card itself: %d %s", got.Code, got.Body.String())
	}
	page := rec + "/timeline"
	got := liveDecode[view.ContactTimelinePage](t, "timeline", app.get(page))
	if got.Poll.TaskID == "" {
		t.Fatalf("no task: %+v", got.Poll)
	}
	deadline := time.Now().Add(60 * time.Second)
	for got.Poll.Running {
		if time.Now().After(deadline) {
			t.Fatalf("the timeline never finished: %+v", got.Poll)
		}
		time.Sleep(200 * time.Millisecond)
		got = liveDecode[view.ContactTimelinePage](t, "timeline", app.get(page))
	}
	why := map[string]string{}
	for _, g := range got.Groups {
		for _, row := range g.Rows {
			why[string(row.Title)] = string(row.Why)
		}
	}
	if why["Review with the team"] != "attendee · accepted" {
		t.Errorf("the event with the card's address: %q (all: %v)", why["Review with the team"], why)
	}
	if why["Call notes"] != "related to this person" {
		t.Errorf("the note related to the card: %q (all: %v)", why["Call notes"], why)
	}
	if _, found := why["Anna Kovaleva said hello"]; found {
		t.Errorf("an event that only names the person in its text is in the timeline: %v", why)
	}
}
