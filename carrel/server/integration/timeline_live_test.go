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

// SoGo gives both homes and makes both collections, so the timeline runs on
// it whole.
func TestSoGoTimeline(t *testing.T) {
	endpoint, user, pass := sogoConfig(t)
	runTimeline(t, endpoint, user, pass)
}

// The timeline on Yandex, which is the timeline of a server that rewrites
// the very lines it is read from. Yandex will not make an address book
// (SV-20) but keeps one of its own, so the card is no obstacle; what it
// does is drop the `ATTENDEE` of an event written over CalDAV and put its
// own `ORGANIZER` — the mailbox's own address — in its place (SV-38).
//
// A person is looked for in three lines: `ORGANIZER`, `ATTENDEE` and
// `RELATED-TO` (S-15.4). So this run asks both halves of that on one
// event. The card of someone who was only an attendee finds nothing,
// because the server kept nothing to find — and the day it starts keeping
// attendees this goes red, which is the right way round. The card that
// carries the address Yandex wrote into `ORGANIZER` does find the event,
// and says «organiser»: the only live proof of that branch we have.
func TestYandexTimeline(t *testing.T) {
	endpoint, user, pass := yandexConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	// The calendar is made before the connection: the catalog is taken once,
	// when the account is connected, and a collection made after that is
	// not in it.
	res, trace, err := discover.Run(ctx, testGuard(t), endpoint, user, pass, "")
	if err != nil || res.CalendarHome == "" {
		t.Fatalf("discovery: %v %+v %+v", err, res, trace)
	}
	suffix := yandexSuffix("timeline")
	calPath := makeCollection(ctx, t, client, endpoint, user, pass, "MKCALENDAR",
		strings.TrimSuffix(res.CalendarHome, "/")+"/"+suffix+"-cal/", `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel timeline</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/><C:comp name="VTODO"/><C:comp name="VJOURNAL"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)

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

	// The card goes into the book that is there, so it is cleaned up by
	// hand: the book itself must outlive the run.
	mail := "anna-" + suffix + "@carrel.test"
	card := book.Path + "anna-" + suffix + ".vcf"
	seed(ctx, t, client, card, "text/vcard; charset=utf-8",
		"BEGIN:VCARD\nVERSION:3.0\nUID:anna-"+suffix+"\nFN:Anna Kovaleva\nN:Kovaleva;Anna;;;\nEMAIL:"+mail+"\nEND:VCARD\n")
	t.Cleanup(func() { _ = client.Delete(context.Background(), card, "") })
	// Yandex may keep the card under an identity of its own, so the UID the
	// note points at is read back from the book rather than assumed.
	_, uid := liveRecordWhere(ctx, t, client, book.Path, "of Anna", func(b string) bool {
		return strings.Contains(b, mail)
	})
	if uid == "" {
		t.Fatal("the card is in the book without a UID")
	}

	at := time.Now().AddDate(0, 0, -7).UTC().Format("20060102T150405Z")
	seed(ctx, t, client, calPath+"met-"+suffix+".ics", "text/calendar; charset=utf-8",
		"BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\nBEGIN:VEVENT\nUID:met-"+suffix+"\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:"+at+"\nDTEND:"+at+"\nSUMMARY:Review with the team\nATTENDEE;PARTSTAT=ACCEPTED:mailto:"+mail+"\nEND:VEVENT\nEND:VCALENDAR\n")
	// What the server kept is what the timeline can read, so the run takes
	// its expectations from there rather than from what was sent.
	kept := bodyIfThere(ctx, t, client, calPath+"met-"+suffix+".ics")
	if strings.Contains(kept, mail) {
		t.Errorf("the server kept the attendee after all: run the whole timeline here, not this (SV-38)\n%s", kept)
	}
	org := organiserOf(kept)
	if org == "" {
		t.Fatalf("the server kept neither the attendee nor an organiser; it holds:\n%s", kept)
	}
	// The second card carries the address the server itself wrote in. Its
	// timeline is the live proof that the organiser branch works.
	mine := book.Path + "mine-" + suffix + ".vcf"
	seed(ctx, t, client, mine, "text/vcard; charset=utf-8",
		"BEGIN:VCARD\nVERSION:3.0\nUID:mine-"+suffix+"\nFN:Carrel Owner\nN:Owner;Carrel;;;\nEMAIL:"+org+"\nEND:VCARD\n")
	t.Cleanup(func() { _ = client.Delete(context.Background(), mine, "") })
	_, mineUID := liveRecordWhere(ctx, t, client, book.Path, "of the organiser", func(b string) bool {
		return strings.Contains(b, "mine-"+suffix)
	})
	if mineUID == "" {
		t.Fatal("the organiser's card is in the book without a UID")
	}
	trySeed(ctx, t, client, calPath+"note-"+suffix+".ics", "text/calendar; charset=utf-8",
		"BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\nBEGIN:VJOURNAL\nUID:note-"+suffix+"\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:"+at+"\nSUMMARY:Call notes\nRELATED-TO:"+uid+"\nEND:VJOURNAL\nEND:VCALENDAR\n")

	blob := liveCatalog(t, app)
	key, _ := app.sess.DataKey()
	cal := catalogEntry(t, blob, calPath)
	for _, c := range []sealed.Collection{cal, book} {
		for _, k := range c.Kinds {
			blob.Selected = append(blob.Selected, sealed.Selection{Kind: k, AccountID: connID, Path: c.Path})
		}
	}
	for _, k := range []view.Kind{view.KindEvent, view.KindTask, view.KindNote, view.KindContact} {
		if err := app.users.SetSelection(app.sess.AccountID(), key, k, blob.Selected); err != nil {
			t.Fatal(err)
		}
	}

	// The attendee's card: nothing to find, and the screen says so without
	// calling it a failure.
	got := settledTimeline(t, app, connID, book.Path, uid)
	for _, g := range got.Groups {
		for _, row := range g.Rows {
			t.Errorf("the attendee's card found %q — %q, and the server kept no attendee", row.Title, row.Why)
		}
	}
	if len(got.Notices) > 0 {
		t.Errorf("a timeline that found nothing raised a notice: %+v", got.Notices)
	}
	for _, sp := range got.Poll.Sources {
		if sp.State != "empty" {
			t.Errorf("source %s answered %s (%q), not empty: a server that keeps nothing is not a failure",
				sp.Stamp, sp.State, sp.Message)
		}
	}
	if got.Note == "" {
		t.Error("an empty timeline says nothing about why it is empty")
	}

	// The organiser's card: the event is there, and it says why.
	got = settledTimeline(t, app, connID, book.Path, mineUID)
	why := map[string]string{}
	for _, g := range got.Groups {
		for _, row := range g.Rows {
			why[string(row.Title)] = string(row.Why)
		}
	}
	if why["Review with the team"] != "organiser" {
		t.Errorf("the event the server made this card the organiser of: %q (all: %v; sources %s)",
			why["Review with the team"], why, sourceStates(got.Poll))
	}
}

// organiserOf is the address in the first ORGANIZER of a record, without
// its mailto:, or "" when there is none.
func organiserOf(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToUpper(line), "ORGANIZER") {
			continue
		}
		// ORGANIZER;CN=mixeum:mailto:someone@example — the parameters end at
		// the first colon and the address carries a scheme of its own.
		_, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.ToLower(strings.TrimSpace(value))
		return strings.TrimPrefix(value, "mailto:")
	}
	return ""
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
	note := trySeed(ctx, t, client, calPath+"note-"+suffix+".ics", "text/calendar; charset=utf-8",
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

	timelineOf(t, app, conn.ID, book.Path, uid, note)
}

// timelineOf asks the card's timeline and says what it holds. The three
// findings are the whole point of S-15.4: an event that names the card's
// address is there as an attendee, a note that points at the card is there
// as related, and an event that merely spells the person's name in its text
// is not there at all. note says whether the server took the note; a server
// that keeps no notes (SV-37) is not asked for one.
func timelineOf(t *testing.T, app *liveApp, connID, book, uid string, note bool) {
	t.Helper()
	got := settledTimeline(t, app, connID, book, uid)
	why := map[string]string{}
	for _, g := range got.Groups {
		for _, row := range g.Rows {
			why[string(row.Title)] = string(row.Why)
		}
	}
	// A timeline that found nothing at all says which sources it asked and
	// what each answered. Without that the failure reads «nothing matched»,
	// which is the one thing that is never the reason.
	if len(why) == 0 {
		t.Errorf("the timeline is empty; sources: %s; range %+v; note %q; notices %+v",
			sourceStates(got.Poll), got.Range, got.Note, got.Notices)
	}
	if why["Review with the team"] != "attendee · accepted" {
		t.Errorf("the event with the card's address: %q (all: %v)", why["Review with the team"], why)
	}
	if note && why["Call notes"] != "related to this person" {
		t.Errorf("the note related to the card: %q (all: %v)", why["Call notes"], why)
	}
	if _, found := why["Anna Kovaleva said hello"]; found {
		t.Errorf("an event that only names the person in its text is in the timeline: %v", why)
	}
}

// settledTimeline opens the card, asks its timeline and waits for the poll
// to finish. A timeline is a search over the ticked sources (S-15.4), so it
// arrives as a task and the page is asked again until it stops running.
func settledTimeline(t *testing.T, app *liveApp, connID, book, uid string) view.ContactTimelinePage {
	t.Helper()
	rec := "/app/contacts/" + connID + "/" + url.PathEscape(book) + "/" + url.PathEscape(uid)
	if got := app.get(rec); got.Code != 200 {
		t.Fatalf("the card itself: %d %s", got.Code, got.Body.String())
	}
	page := rec + "/timeline"
	got := liveDecode[view.ContactTimelinePage](t, "timeline", app.get(page))
	if got.Poll.TaskID == "" {
		t.Fatalf("no task: %+v", got.Poll)
	}
	deadline := time.Now().Add(90 * time.Second)
	for got.Poll.Running {
		if time.Now().After(deadline) {
			t.Fatalf("the timeline never finished: %+v", got.Poll)
		}
		time.Sleep(200 * time.Millisecond)
		got = liveDecode[view.ContactTimelinePage](t, "timeline", app.get(page))
	}
	return got
}
