//go:build integration

package integration

import (
	"context"
	"encoding/xml"
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

// S-30.2 on iCloud's calendar host, caldav.icloud.com. It answers these
// credentials since 30.09.2026 (docs/servers.md, SV-28). See the head of
// icloud_live_test.go for why the shared runs are called by halves here.

func icloudCalConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_ICLOUD_CALDAV_URL")
	user = os.Getenv("CARREL_ICLOUD_CALDAV_USER")
	pass = os.Getenv("CARREL_ICLOUD_CALDAV_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_ICLOUD_CALDAV_* required")
	}
	return endpoint, user, pass
}

// icloudCalendars makes a calendar for events and one for tasks, and removes
// both with the test. iCloud keeps one component per calendar: asked for
// VEVENT and VTODO it keeps the first and answers 201 all the same
// (docs/servers.md, SV-52), so the two kinds cannot share one here.
func icloudCalendars(ctx context.Context, t *testing.T, client *dav.Client, endpoint, user, pass, suffix, label string) (events, tasks string) {
	t.Helper()
	res, trace, err := discover.Run(ctx, testGuard(t), endpoint, user, pass, "")
	if err != nil || res.CalendarHome == "" {
		t.Fatalf("discovery: %v %+v %+v", err, res, trace)
	}
	home := strings.TrimSuffix(res.CalendarHome, "/") + "/"
	events = makeCollection(ctx, t, client, endpoint, user, pass, "MKCALENDAR", home+suffix+"-cal/", `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:I="http://apple.com/ns/ical/">
  <D:set><D:prop>
    <D:displayname>`+label+`</D:displayname>
    <I:calendar-color>#3366CCFF</I:calendar-color>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
	tasks = makeCollection(ctx, t, client, endpoint, user, pass, "MKCALENDAR", home+suffix+"-tasks/", `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>`+label+` tasks</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VTODO"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
	return events, tasks
}

// What discovery finds on the calendar host: the home, calendars of both
// kinds with their colours, and scheduling with the person's own addresses
// (S-6, S-15.4). The account's calendars are Apple's own — «Напоминания»
// holds VTODO alone, the other two VEVENT alone — so the kinds come from the
// component set of each, never from a guess.
func TestICloudCalendarDiscovery(t *testing.T) {
	endpoint, user, pass := icloudCalConfig(t)
	res, trace, err := discover.Run(context.Background(), testGuard(t), endpoint, user, pass, "")
	if err != nil {
		t.Fatalf("discovery: %v %+v", err, trace)
	}
	if res.CalendarHome == "" {
		t.Fatalf("no calendar home: %+v", res)
	}
	if res.Scheduling != discover.SchedulingYes {
		t.Errorf("scheduling = %q, want %q: iCloud announces a schedule-outbox-URL", res.Scheduling, discover.SchedulingYes)
	}
	if len(res.Addresses) == 0 {
		t.Errorf("iCloud schedules but no own address was read: %+v", res)
	}
	for _, a := range res.Addresses {
		if strings.HasPrefix(a, "mailto:") || a != strings.ToLower(a) {
			t.Errorf("own address %q: want it lower case, without mailto:", a)
		}
	}
	var events, tasks int
	for _, c := range res.Collections {
		switch {
		case len(c.Components) != 1:
			t.Errorf("%s holds %v: every iCloud calendar holds one component (SV-52)", c.Path, c.Components)
		case c.Components[0] == "VEVENT":
			events++
		case c.Components[0] == "VTODO":
			tasks++
		}
		if c.Colour == "" {
			t.Errorf("%s (%q) came without a colour", c.Path, c.DisplayName)
		}
		if c.DisplayName == "" {
			t.Errorf("%s came without a name", c.Path)
		}
	}
	if events == 0 || tasks == 0 {
		t.Errorf("found %d event and %d task calendars, want both kinds: %+v", events, tasks, res.Collections)
	}
}

// S-11 on iCloud: the calendar half of runRecords — an event made through
// the pages with a reminder, the agenda, a series with an override written by
// another client, and a task made and marked done — with the events and the
// task in calendars of their own (SV-52).
func TestICloudRecords(t *testing.T) {
	endpoint, user, pass := icloudCalConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	suffix := yandexSuffix("s11")
	cal, taskCal := icloudCalendars(ctx, t, client, endpoint, user, pass, suffix, "Carrel S-11")
	seedColour(ctx, t, client, cal, "#3366CCFF")

	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	blob := liveCatalog(t, app)
	ev, tk := catalogEntry(t, blob, cal), catalogEntry(t, blob, taskCal)
	if ev.ReadOnly || tk.ReadOnly || ev.ChangeTag == "" || tk.ChangeTag == "" {
		t.Errorf("catalog rights or change tag: events %+v, tasks %+v", ev, tk)
	}
	if !ev.Has(view.KindEvent) || !tk.Has(view.KindTask) {
		t.Errorf("catalog kinds: events %v, tasks %v", ev.Kinds, tk.Kinds)
	}
	if ev.Colour == "" {
		t.Errorf("calendar-color set at creation did not reach the catalog: %+v", ev)
	}
	// The new event is given no end: iCloud answers 404 to an event without
	// one, and Carrel writes the end out (SV-53).
	liveCalendarIn(ctx, t, app, client, connID, ev.Path, tk.Path, suffix, nil)
}

// docs/servers.md, SV-53: iCloud answers PUT of a VEVENT that has DTSTART and
// neither DTEND nor DURATION with 404 Not Found and an empty body — timed and
// all-day alike — although RFC 5545 §3.6.1 allows it. Until v0.2.80 Carrel's
// new-event form wrote no end when none was typed, and the first event a
// person made on iCloud without one was refused with «The server refused the
// change … 404». Carrel now writes the end out — DTEND equal to DTSTART for a
// time, the next day for a date — the same event by RFC 5545 §3.6.1, and
// iCloud takes it with 201.
func TestICloudEventWithoutAnEnd(t *testing.T) {
	endpoint, user, pass := icloudCalConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	cals := icloudEventCalendars(t, app)
	if len(cals) == 0 {
		t.Skip("no event calendar")
	}
	events := "/app/calendar/" + connID + "/" + url.PathEscape(cals[0].Path)
	for _, allDay := range []bool{false, true} {
		form := liveForm(liveDecode[view.EventEditPage](t, "new event form", app.get(events+"/new")).Form)
		form.Set("summary", "Carrel probe "+yandexSuffix("sv53"))
		form.Set("start.date", "2031-03-13")
		if allDay {
			form.Set("start.allday", "on")
		} else {
			form.Set("start.time", "09:30")
		}
		loc := redirected(t, "new event without an end", app.post(events+"/new", form))
		uid, _ := url.PathUnescape(loc[strings.LastIndex(loc, "/")+1:])
		removeLater(t, client, cals[0].Path+uid+".ics")
		got, _ := rawBody(ctx, t, client, cals[0].Path+uid+".ics")
		want := []string{"UID:" + uid, "DTSTART:20310313T093000Z", "DTEND:20310313T093000Z"}
		if allDay {
			want = []string{"UID:" + uid, "DTSTART;VALUE=DATE:20310313", "DTEND;VALUE=DATE:20310314"}
		}
		mustHave(t, "new event without an end", got, want...)
	}
}

// Review Р-9 and Р-10 on iCloud: the event and task halves of runForms, the
// password gone wrong and edited, and Disconnect. No note: iCloud makes no
// calendar for VJOURNAL — MKCALENDAR with it alone answers 500 (SV-52).
func TestICloudForms(t *testing.T) {
	endpoint, user, pass := icloudCalConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	suffix := yandexSuffix("forms")
	cal, taskCal := icloudCalendars(ctx, t, client, endpoint, user, pass, suffix, "Carrel forms")

	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	blob := liveCatalog(t, app)
	ev, tk := catalogEntry(t, blob, cal), catalogEntry(t, blob, taskCal)

	t.Run("event", func(t *testing.T) {
		liveEventFields(ctx, t, app, client, connID, ev.Path, suffix)
	})
	t.Run("task", func(t *testing.T) {
		liveTaskFields(ctx, t, app, client, connID, tk.Path, suffix)
	})
	t.Run("connection", func(t *testing.T) { liveConnectionEdit(t, app, connID, endpoint, user, pass) })
	// Last: it takes the connection out from under the rest.
	t.Run("disconnect", func(t *testing.T) { liveDisconnect(t, app, connID, endpoint, user, pass, ev.Path) })
	poolCounts(t, app, "an iCloud forms run")
}

// S-12.1, S-12.2 and S-12.4 on iCloud through the pages: a calendar asked
// for with events and tasks, renamed with a colour, and deleted by its name.
// iCloud keeps only the first component of the set (SV-52); what is held
// here is S-12.2 — the catalog holds the server's answer, not the form's.
func TestICloudCollections(t *testing.T) {
	endpoint, user, pass := icloudCalConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)

	displayName := xml.Name{Space: "DAV:", Local: "displayname"}
	colour := xml.Name{Space: "http://apple.com/ns/ical/", Local: "calendar-color"}
	comps := xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "supported-calendar-component-set"}

	suffix := yandexSuffix("s12")
	name := "Поездки & планы " + suffix
	loc := redirected(t, "new calendar", app.post("/app/collections/new?kind=event", url.Values{
		"account": {connID}, "name": {name}, "colour": {"#6b4e7a"}, "components": {"VEVENT", "VTODO"},
	}))
	calPath := livePathOf(t, loc, "/app/calendar/"+connID+"/")
	removeLater(t, client, calPath)

	got := liveProps(ctx, t, client, calPath, displayName, colour, comps)
	t.Logf("asked for VEVENT and VTODO; the server holds %q", got["supported-calendar-component-set"])
	if got["displayname"] != name || !strings.Contains(strings.ToLower(got["calendar-color"]), "6b4e7a") {
		t.Errorf("calendar on the server = %+v", got)
	}
	entry := catalogEntry(t, liveCatalog(t, app), calPath)
	held := discover.CalendarKinds(componentsOf(got["supported-calendar-component-set"]))
	for _, k := range held {
		if !entry.Has(k) {
			t.Errorf("the server holds %s and the catalog does not: server %q, catalog %v", k, got["supported-calendar-component-set"], entry.Kinds)
		}
	}
	if len(entry.Kinds) != len(held) {
		t.Errorf("the catalog holds %v, the server %v: a kind the server does not hold is a promise nobody keeps", entry.Kinds, held)
	}
	// What the server did not take is said on the calendar (SV-52): iCloud
	// keeps the first component, so the tasks are named, the events not.
	agenda := liveDecode[view.AgendaPage](t, "agenda", app.get(loc))
	if len(agenda.Notices) != 1 || !strings.Contains(string(agenda.Notices[0].Title), "tasks") ||
		strings.Contains(string(agenda.Notices[0].Title), "events") || !strings.Contains(string(agenda.Notices[0].Body), "another calendar") {
		t.Errorf("agenda of the new calendar: %s; want the one notice that the server did not take tasks", noticeText(agenda.Notices))
	}

	redirected(t, "rename", app.post("/app/collections/rename?"+url.Values{"a": {connID}, "c": {calPath}}.Encode(),
		url.Values{"name": {name + " renamed"}, "colour": {"#3f6e8c"}}))
	got = liveProps(ctx, t, client, calPath, displayName, colour)
	if got["displayname"] != name+" renamed" || !strings.Contains(strings.ToLower(got["calendar-color"]), "3f6e8c") {
		t.Errorf("after rename = %+v", got)
	}

	entry = catalogEntry(t, liveCatalog(t, app), calPath)
	liveDeleteByName(ctx, t, app, client, connID, entry.Path, liveDeleteSheet(t, app, connID, entry.Path), entry.Name)
}

// icloudEventCalendars are the event calendars of the account that were there
// before any test: the owner's own, which a test writes in and leaves as it
// found them.
func icloudEventCalendars(t *testing.T, app *liveApp) []sealed.Collection {
	t.Helper()
	var out []sealed.Collection
	for _, c := range liveCatalog(t, app).Collections {
		if c.Has(view.KindEvent) && !c.ReadOnly && !strings.Contains(c.Path, "carrel-") {
			out = append(out, c)
		}
	}
	return out
}

// S-18 on iCloud, between the two event calendars the account came with: a
// series with an override moves from one to the other by MOVE, then is copied
// back — the copy waits for SV-55: iCloud keeps one resource per UID in the
// whole account. It stands in October 2031 so that it meets nothing of the owner's,
// and everything it leaves is taken away — both addresses, whatever happened.
func TestICloudMove(t *testing.T) {
	endpoint, user, pass := icloudCalConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	cals := icloudEventCalendars(t, app)
	if len(cals) < 2 {
		t.Skipf("the account has %d event calendars, want two to move between", len(cals))
	}
	from, into := cals[0], cals[1]
	suffix := moveSuffix()
	series := "series-" + suffix
	removeLater(t, client, from.Path+series+".ics", into.Path+series+".ics")
	seed(ctx, t, client, from.Path+series+".ics", "text/calendar; charset=utf-8", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\n"+
		"BEGIN:VEVENT\nUID:"+series+"\nDTSTAMP:20260101T000000Z\nDTSTART:20311007T100000Z\n"+
		"DTEND:20311007T103000Z\nRRULE:FREQ=WEEKLY;COUNT=3\nSUMMARY:Standup\nEND:VEVENT\n"+
		"BEGIN:VEVENT\nUID:"+series+"\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID:20311014T100000Z\n"+
		"DTSTART:20311014T120000Z\nDTEND:20311014T123000Z\nSUMMARY:Standup moved\nEND:VEVENT\nEND:VCALENDAR\n")
	moveURL := func(fromPath, intoPath string, copying bool) string {
		q := url.Values{"to": {connID}, "into": {intoPath}}
		if copying {
			q.Set("copy", "1")
		}
		return "/app/calendar/" + connID + "/" + url.PathEscape(fromPath) + "/" + url.PathEscape(series) + "/move?" + q.Encode()
	}

	sheet := liveDecode[view.MoveSheet](t, "move sheet", app.get(moveURL(from.Path, into.Path, false)))
	if !strings.HasPrefix(string(sheet.Warning), "Inside one server this is MOVE") {
		t.Errorf("a move on one account warned of something else: %q", sheet.Warning)
	}
	redirected(t, "move", app.post(liveAction(t, sheet.Actions, "record.move.confirm"), url.Values{}))
	if _, _, err := client.Get(ctx, from.Path+series+".ics", nil); !dav.IsStatus(err, http.StatusNotFound) {
		t.Errorf("the original after MOVE: GET = %v, want 404", err)
	}
	got := liveCardByUID(ctx, t, client, into.Path, series)
	mustHave(t, "moved event", got, "UID:"+series, "RRULE:FREQ=WEEKLY;COUNT=3", "RECURRENCE-ID:20311014T100000Z", "SUMMARY:Standup moved")
	agenda := liveDecode[view.AgendaPage](t, "agenda at the target",
		app.get("/app/calendar/"+connID+"/"+url.PathEscape(into.Path)+"?from=2031-10-13"))
	var lines []string
	for _, d := range agenda.Days {
		for _, r := range d.Rows {
			lines = append(lines, string(d.Rubric)+" · "+string(r.Summary))
		}
	}
	if all := strings.Join(lines, "|"); len(agenda.Notices) > 0 || !strings.Contains(all, "14 October") || !strings.Contains(all, "Standup moved") {
		t.Errorf("agenda at the target: rows %v, notices %s", lines, noticeText(agenda.Notices))
	}

	t.Run("copy back", func(t *testing.T) {
		if os.Getenv("CARREL_ICLOUD_SV55") == "" {
			t.Skip("docs/servers.md, SV-55: iCloud holds one resource per UID in the whole account, and a copy keeps " +
				"its UID, so PUT of the copy answers 412 — CARREL_ICLOUD_SV55=1 runs this anyway")
		}
		sheet := liveDecode[view.MoveSheet](t, "copy sheet", app.get(moveURL(into.Path, from.Path, true)))
		redirected(t, "copy back", app.post(liveAction(t, sheet.Actions, "record.copy.confirm"), url.Values{}))
		copied := liveCardByUID(ctx, t, client, from.Path, series)
		mustHave(t, "copy", copied, "RRULE:FREQ=WEEKLY;COUNT=3", "SUMMARY:Standup moved")
		// A copy leaves the original where it was.
		liveCardByUID(ctx, t, client, into.Path, series)
	})
}

// icloudBook is the only address book of the contacts connection.
func icloudBook(t *testing.T, app *liveApp, cardConn string) sealed.Collection {
	t.Helper()
	for _, c := range liveCatalog(t, app).Collections {
		if c.AccountID == cardConn && c.Has(view.KindContact) && !c.ReadOnly {
			return c
		}
	}
	t.Fatal("no address book on the contacts host")
	return sealed.Collection{}
}

// icloudTick ticks exactly these collections for every kind named.
func icloudTick(t *testing.T, app *liveApp, cols []sealed.Collection, kinds ...view.Kind) {
	t.Helper()
	var picks []sealed.Selection
	for _, c := range cols {
		for _, k := range c.Kinds {
			picks = append(picks, sealed.Selection{Kind: k, AccountID: c.AccountID, Path: c.Path})
		}
	}
	key, _ := app.sess.DataKey()
	for _, k := range kinds {
		if err := app.users.SetSelection(app.sess.AccountID(), key, k, picks); err != nil {
			t.Fatal(err)
		}
	}
}

// S-15 on iCloud across its two hosts: events and a task in calendars made
// for the run, two cards in the only book, and one search over all three
// sources of two connections. The match is the server's own:
// i;unicode-casemap by name, which CalDAV asks for.
func TestICloudSearch(t *testing.T) {
	endpoint, user, pass := icloudCalConfig(t)
	cardEndpoint, cardUser, cardPass := icloudCardConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	cards := liveClient(t, cardEndpoint, cardUser, cardPass)
	suffix := yandexSuffix("s15")
	cal, taskCal := icloudCalendars(ctx, t, client, endpoint, user, pass, suffix, "Carrel S-15")

	component := func(in, uid, kind, props string) {
		t.Helper()
		seed(ctx, t, client, in+uid+".ics", "text/calendar; charset=utf-8", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Carrel test//EN\nBEGIN:"+kind+
			"\nUID:"+uid+"\nDTSTAMP:20260101T000000Z\n"+props+"END:"+kind+"\nEND:VCALENDAR\n")
	}
	component(cal, "launch-"+suffix, "VEVENT", "DTSTART:20311010T100000Z\nDTEND:20311010T110000Z\nSUMMARY:Launch plan\n")
	component(cal, "pad-"+suffix, "VEVENT", "DTSTART:20311011T100000Z\nDTEND:20311011T110000Z\nSUMMARY:Visit\nLOCATION:Launch site\n")
	component(taskCal, "meet-"+suffix, "VTODO", "SUMMARY:Встреча с Ковалёвой\n")

	app := newLiveApp(t)
	connectLive(t, app, endpoint, user, pass)
	cardConn := connectLive(t, app, cardEndpoint, cardUser, cardPass)
	book := icloudBook(t, app, cardConn)
	card := func(uid, family, given, extra string) {
		t.Helper()
		removeLater(t, cards, book.Path+uid+".vcf")
		seed(ctx, t, cards, book.Path+uid+".vcf", "text/vcard; charset=utf-8", "BEGIN:VCARD\nVERSION:3.0\nUID:"+uid+"\nFN:"+family+" "+given+
			"\nN:"+family+";"+given+";;;\n"+extra+"END:VCARD\n")
	}
	card("ada-"+suffix, "Lovelace", "Ada", "EMAIL:ada@carrel.test\n")
	card("anna-"+suffix, "Ковалёва", "Анна", "")

	// Only the calendars of this run are ticked, and the book: the owner's
	// own calendars may hold anything.
	blob := liveCatalog(t, app)
	icloudTick(t, app, []sealed.Collection{catalogEntry(t, blob, cal), catalogEntry(t, blob, taskCal), book},
		view.KindEvent, view.KindTask, view.KindContact)

	if got := strings.Join(liveSearch(t, app, "launch"), "|"); got != "Launch plan|Visit" {
		t.Errorf("launch (summary and location, other case) = %q", got)
	}
	if got := strings.Join(liveSearch(t, app, "LOVELACE"), "|"); got != "Lovelace Ada" {
		t.Errorf("LOVELACE (a contact, other case) = %q", got)
	}
	if got := strings.Join(liveSearch(t, app, "Ковал"), "|"); got != "Встреча с Ковалёвой|Ковалёва Анна" {
		t.Errorf("Ковал (Cyrillic as written) = %q", got)
	}
	if got := strings.Join(liveSearch(t, app, "ковал"), "|"); got != "Встреча с Ковалёвой|Ковалёва Анна" {
		t.Errorf("ковал (Cyrillic, other case) = %q", got)
	}
}

// S-15.4 on iCloud across its two hosts: the card lives in the only book of
// the contacts connection, the events in a calendar of the other. The event
// that names the card's address is found as an attendee; the one that only
// says the name is not. No note: iCloud keeps no VJOURNAL (SV-52).
//
// Nothing may reach anyone: iCloud schedules (RFC 6638), so the attendee
// carries SCHEDULE-AGENT=CLIENT — the server is told not to send the
// invitation — and the address is under .test, which no mail reaches.
func TestICloudTimeline(t *testing.T) {
	endpoint, user, pass := icloudCalConfig(t)
	cardEndpoint, cardUser, cardPass := icloudCardConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	cards := liveClient(t, cardEndpoint, cardUser, cardPass)
	suffix := yandexSuffix("timeline")
	cal, _ := icloudCalendars(ctx, t, client, endpoint, user, pass, suffix, "Carrel timeline")
	mail := "anna-" + suffix + "@carrel.test"

	at := time.Now().AddDate(0, 0, -7).UTC().Format("20060102T150405Z")
	seed(ctx, t, client, cal+"met-"+suffix+".ics", "text/calendar; charset=utf-8",
		"BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\nBEGIN:VEVENT\nUID:met-"+suffix+"\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:"+at+"\nDTEND:"+at+"\nSUMMARY:Review with the team\n"+
			"ATTENDEE;PARTSTAT=ACCEPTED;SCHEDULE-AGENT=CLIENT:mailto:"+mail+"\nEND:VEVENT\nEND:VCALENDAR\n")
	seed(ctx, t, client, cal+"said-"+suffix+".ics", "text/calendar; charset=utf-8",
		"BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\nBEGIN:VEVENT\nUID:said-"+suffix+"\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:"+at+"\nDTEND:"+at+"\nSUMMARY:Anna Kovaleva said hello\nEND:VEVENT\nEND:VCALENDAR\n")
	kept, _ := rawBody(ctx, t, client, cal+"met-"+suffix+".ics")
	t.Logf("what iCloud kept of the event:\n%s", kept)
	if !strings.Contains(strings.ToLower(kept), mail) {
		t.Fatalf("iCloud did not keep the attendee, so there is nothing for the timeline to find:\n%s", kept)
	}

	app := newLiveApp(t)
	connectLive(t, app, endpoint, user, pass)
	cardConn := connectLive(t, app, cardEndpoint, cardUser, cardPass)
	book := icloudBook(t, app, cardConn)
	uid := "anna-" + suffix
	removeLater(t, cards, book.Path+uid+".vcf")
	seed(ctx, t, cards, book.Path+uid+".vcf", "text/vcard; charset=utf-8",
		"BEGIN:VCARD\nVERSION:3.0\nUID:"+uid+"\nFN:Anna Kovaleva\nN:Kovaleva;Anna;;;\nEMAIL:"+mail+"\nEND:VCARD\n")

	icloudTick(t, app, []sealed.Collection{catalogEntry(t, liveCatalog(t, app), cal), book},
		view.KindEvent, view.KindTask, view.KindNote, view.KindContact)
	timelineOf(t, app, cardConn, book.Path, uid, false)
}

// An event and a task through the pages in the calendars the account came
// with — Apple's own, not ones a test made: made, read, edited, deleted. The
// shared runs above write in calendars they make; this is what a person
// connecting iCloud actually writes in. Both stand in 2031, carry no
// attendee, and are removed on cleanup should the test stop half way.
func TestICloudInTheExistingCalendars(t *testing.T) {
	endpoint, user, pass := icloudCalConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	cals := icloudEventCalendars(t, app)
	var taskCal sealed.Collection
	for _, c := range liveCatalog(t, app).Collections {
		if c.Has(view.KindTask) && !c.ReadOnly && !strings.Contains(c.Path, "carrel-") {
			taskCal = c
			break
		}
	}
	if len(cals) == 0 || taskCal.Path == "" {
		t.Skipf("the account has %d event calendars and task list %q", len(cals), taskCal.Path)
	}
	summary := "Carrel probe " + yandexSuffix("icloud")

	t.Run("event", func(t *testing.T) {
		cal := cals[0]
		events := "/app/calendar/" + connID + "/" + url.PathEscape(cal.Path)
		form := liveForm(liveDecode[view.EventEditPage](t, "new event form", app.get(events+"/new")).Form)
		form.Set("summary", summary)
		form.Set("start.date", "2031-03-11")
		form.Set("start.time", "09:30")
		// An end of its own; one without is TestICloudEventWithoutAnEnd (SV-53).
		form.Set("end.date", "2031-03-11")
		form.Set("end.time", "10:00")
		loc := redirected(t, "new event", app.post(events+"/new", form))
		uid, _ := url.PathUnescape(loc[strings.LastIndex(loc, "/")+1:])
		file := cal.Path + uid + ".ics"
		removeLater(t, client, file)
		got, _ := rawBody(ctx, t, client, file)
		mustHave(t, "new event", got, "UID:"+uid, "SUMMARY:"+summary, "20310311")
		if event := liveDecode[view.EventPage](t, "the event", app.get(loc)); string(event.Summary) != summary {
			t.Errorf("the event page shows %q, want %q", event.Summary, summary)
		}
		form = liveForm(liveDecode[view.EventEditPage](t, "edit form", app.get(loc+"/edit")).Form)
		form.Set("summary", summary+" moved")
		redirected(t, "edit", app.post(loc+"/edit", form))
		got, _ = rawBody(ctx, t, client, file)
		mustHave(t, "edited event", got, "SUMMARY:"+summary+" moved")
		event := liveDecode[view.EventPage](t, "event before delete", app.get(loc))
		redirected(t, "delete", app.post(liveAction(t, event.Actions, "record.delete"), url.Values{}))
		if _, _, err := client.Get(ctx, file, nil); !dav.IsStatus(err, http.StatusNotFound) {
			t.Errorf("after delete GET %s = %v, want 404", file, err)
		}
	})

	t.Run("task", func(t *testing.T) {
		tasks := "/app/tasks/" + connID + "/" + url.PathEscape(taskCal.Path)
		form := liveForm(liveDecode[view.TaskEditPage](t, "new task form", app.get(tasks+"/new")).Form)
		form.Set("summary", summary)
		form.Set("due.date", "2031-03-12")
		form.Set("allday", "on")
		loc := redirected(t, "new task", app.post(tasks+"/new", form))
		uid, _ := url.PathUnescape(loc[strings.LastIndex(loc, "/")+1:])
		file := taskCal.Path + uid + ".ics"
		removeLater(t, client, file)
		got, _ := rawBody(ctx, t, client, file)
		mustHave(t, "new task", got, "UID:"+uid, "SUMMARY:"+summary, "DUE;VALUE=DATE:20310312")
		if task := liveDecode[view.TaskPage](t, "the task", app.get(loc)); string(task.Summary) != summary {
			t.Errorf("the task page shows %q, want %q", task.Summary, summary)
		}
		liveEdit(t, app, loc, func(p view.TaskEditPage) view.Form { return p.Form }, map[string]string{"summary": summary + " moved"})
		got, _ = rawBody(ctx, t, client, file)
		mustHave(t, "edited task", got, "SUMMARY:"+summary+" moved", "DUE;VALUE=DATE:20310312")
		task := liveDecode[view.TaskPage](t, "task before delete", app.get(loc))
		redirected(t, "delete", app.post(liveAction(t, task.Actions, "record.delete"), url.Values{}))
		if _, _, err := client.Get(ctx, file, nil); !dav.IsStatus(err, http.StatusNotFound) {
			t.Errorf("after delete GET %s = %v, want 404", file, err)
		}
	})
}
