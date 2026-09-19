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
	user2 "carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for forms-tail.md and connections-tail.md: the fields the forms
// gained, Mark done and a series' rule, on servers people connect — each
// change writes its line and the rest of the object comes back as it was;
// and a connection whose password went wrong is tested, edited and answers
// again.
func TestRadicaleForms(t *testing.T) {
	runForms(t, radicaleEndpoint(t), radicaleUser, radicalePass)
}

func TestBaikalForms(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runForms(t, endpoint, user, pass)
}

func TestNextcloudForms(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runForms(t, endpoint, user, pass)
}

func runForms(t *testing.T, endpoint, user, pass string) {
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
	suffix := fmt.Sprintf("carrel-forms-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	calPath := makeCollection(ctx, t, client, endpoint, user, pass, "MKCALENDAR",
		strings.TrimSuffix(res.CalendarHome, "/")+"/"+suffix+"-cal/", `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel forms</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/><C:comp name="VTODO"/><C:comp name="VJOURNAL"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
	bookPath := makeCollection(ctx, t, client, endpoint, user, pass, "MKCOL",
		strings.TrimSuffix(res.AddressBookHome, "/")+"/"+suffix+"-book/", `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel forms</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)

	app := newLiveApp(t)
	redirected(t, "connect", app.post("/app/settings/connections", url.Values{
		"label": {"live " + suffix}, "server": {endpoint}, "login": {user}, "password": {pass},
	}))
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	conn := blob.Connections[0]
	cal, book := catalogEntry(t, blob, calPath), catalogEntry(t, blob, bookPath)

	t.Run("contact", func(t *testing.T) { liveContactFields(ctx, t, app, client, conn.ID, book.Path, suffix) })
	t.Run("event", func(t *testing.T) { liveEventFields(ctx, t, app, client, conn.ID, cal.Path, suffix) })
	t.Run("task", func(t *testing.T) { liveTaskFields(ctx, t, app, client, conn.ID, cal.Path, suffix) })
	if cal.Has(view.KindNote) {
		t.Run("note", func(t *testing.T) { liveNoteFields(ctx, t, app, client, conn.ID, cal.Path, suffix) })
	} else {
		t.Logf("the calendar took no VJOURNAL: the note part is skipped")
	}
	t.Run("connection", func(t *testing.T) { liveConnectionEdit(t, app, conn.ID, endpoint, user, pass) })
	// Last: it takes the connection out from under the rest.
	t.Run("disconnect", func(t *testing.T) { liveDisconnect(t, app, conn.ID, endpoint, user, pass, cal.Path) })
}

// connections-tail.md: Disconnect leaves the server alone and Carrel with
// nothing of the connection; connecting again finds the collections anew,
// without the ticks, and the links made before are dead.
func liveDisconnect(t *testing.T, app *liveApp, connID, endpoint, user, pass, calPath string) {
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	conn, _ := blob.Connection(connID)
	if err := app.users.SetSelection(app.sess.AccountID(), key, view.KindEvent,
		[]sealed.Selection{{Kind: view.KindEvent, AccountID: connID, Path: calPath}}); err != nil {
		t.Fatal(err)
	}
	link, err := app.users.Publish(app.sess.AccountID(), key, conn,
		user2.PublishTarget{AccountID: connID, Kind: "calendar", Col: calPath, Title: "Carrel forms"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sheet := liveDecode[view.ConnectionRemoveSheet](t, "disconnect sheet",
		app.get("/app/settings/connections/disconnect?account="+connID))
	if len(sheet.References) == 0 {
		t.Errorf("the sheet names nothing that goes: %+v", sheet)
	}
	confirm, _ := url.Parse(liveAction(t, sheet.Actions, "connection.disconnect.confirm"))
	redirected(t, "disconnect", app.post(confirm.EscapedPath()+"?"+confirm.RawQuery, url.Values{}))

	blob, _ = app.users.Catalog(app.sess.AccountID(), key)
	if len(blob.Connections) != 0 || len(blob.Collections) != 0 || len(blob.Selected) != 0 {
		t.Fatalf("after Disconnect the catalog holds %+v", blob)
	}
	if _, ok := app.users.OpenPublished(link); ok {
		t.Error("a link of the connection still opens")
	}
	// The server kept everything, and connecting again finds it — unticked.
	redirected(t, "connect again", app.post("/app/settings/connections", url.Values{
		"server": {endpoint}, "login": {user}, "password": {pass},
	}))
	blob, _ = app.users.Catalog(app.sess.AccountID(), key)
	if len(blob.Connections) != 1 || blob.Connections[0].ID == connID {
		t.Fatalf("connected again: %+v", blob.Connections)
	}
	if len(blob.Selected) != 0 {
		t.Errorf("the ticks came back: %+v", blob.Selected)
	}
	if catalogEntry(t, blob, calPath).Path == "" {
		t.Error("the calendar is gone from the server")
	}
}

// liveEdit opens a record's form, sets fields and saves; the answer must be
// the record, not a notice, an error or a loss report.
func liveEdit[P any](t *testing.T, app *liveApp, rec string, form func(P) view.Form, set map[string]string) {
	t.Helper()
	v := liveForm(form(liveDecode[P](t, "edit "+rec, app.get(rec+"/edit"))))
	for k, val := range set {
		v.Set(k, val)
	}
	if loc := redirected(t, fmt.Sprintf("save %v", set), app.post(rec+"/edit", v)); loc != rec {
		t.Fatalf("save %v went to %q", set, loc)
	}
}

func seed(ctx context.Context, t *testing.T, client *dav.Client, file, media, body string) {
	t.Helper()
	if _, err := client.PutCreate(ctx, file, dav.Typed(strings.NewReader(strings.ReplaceAll(body, "\n", "\r\n")), media)); err != nil {
		t.Fatalf("seed %s: %v", file, err)
	}
}

func liveContactFields(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, book, suffix string) {
	uid := "vera-" + suffix
	file := book + uid + ".vcf"
	seed(ctx, t, client, file, "text/vcard; charset=utf-8", "BEGIN:VCARD\nVERSION:3.0\nUID:"+uid+"\nFN:Vera Lebedeva\nN:Lebedeva;Vera;;;\n"+
		"TEL;TYPE=work:+7 912 000 00 01\nEMAIL:vera@carrel.test\nADR;TYPE=home:PO 7;Flat 2;Lenina 1;Ekaterinburg;;620000;Russia\n"+
		"BDAY:--0412\nX-CUSTOM:keep-me\nEND:VCARD\n")
	rec := "/app/contacts/" + connID + "/" + url.PathEscape(book) + "/" + url.PathEscape(uid)
	form := func(p view.ContactEditPage) view.Form { return p.Form }

	liveEdit(t, app, rec, form, map[string]string{"nickname": "Verochka", "categories": "work, family"})
	got, _ := rawBody(ctx, t, client, file)
	yearless(t, "nickname and categories", got, "04", "12")
	mustHave(t, "nickname and categories", got, "NICKNAME:Verochka", "CATEGORIES:work,family", "X-CUSTOM:keep-me",
		"ADR;TYPE=home:PO 7;Flat 2;Lenina 1;Ekaterinburg;;620000;Russia")

	liveEdit(t, app, rec, form, map[string]string{"bday": "--04-13", "tel.0.type": "cell", "adr.0.region": "Sverdlovsk"})
	got, _ = rawBody(ctx, t, client, file)
	yearless(t, "birthday", got, "04", "13")
	mustHave(t, "birthday, label, address", got, "TEL;TYPE=cell:+7 912 000 00 01",
		"PO 7;Flat 2;Lenina 1;Ekaterinburg;Sverdlovsk;620000;Russia", "EMAIL:vera@carrel.test", "X-CUSTOM:keep-me")
}

// yearless checks a birthday without a year in either form a server keeps
// it: --MMDD (Radicale), or Apple's placeholder year with its marker (sabre:
// Baikal, Nextcloud).
func yearless(t *testing.T, step, got, month, day string) {
	t.Helper()
	for _, l := range strings.Split(got, "\n") {
		if !strings.HasPrefix(l, "BDAY") {
			continue
		}
		v := l[strings.LastIndex(l, ":")+1:]
		digits := strings.ReplaceAll(strings.TrimPrefix(v, "--"), "-", "")
		if strings.HasPrefix(v, "--") && digits == month+day {
			return
		}
		if i := strings.Index(l, "X-APPLE-OMIT-YEAR="); i > 0 && len(digits) == 8 && digits[4:] == month+day &&
			strings.HasPrefix(l[i+len("X-APPLE-OMIT-YEAR="):], digits[:4]) {
			return
		}
	}
	t.Errorf("%s: no birthday --%s-%s without a year in\n%s", step, month, day, got)
}

func liveEventFields(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, cal, suffix string) {
	uid := "duty-" + suffix
	file := cal + uid + ".ics"
	seed(ctx, t, client, file, "text/calendar; charset=utf-8", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\n"+
		"BEGIN:VEVENT\nUID:"+uid+"\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260112T100000\n"+
		"DTEND;TZID=Europe/Moscow:20260112T110000\nSUMMARY:Duty\nRRULE:FREQ=WEEKLY;BYDAY=MO;WKST=MO;COUNT=6\n"+
		"EXDATE;TZID=Europe/Moscow:20260119T100000\nX-CUSTOM:keep-me\nEND:VEVENT\n"+
		"BEGIN:VEVENT\nUID:"+uid+"\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID;TZID=Europe/Moscow:20260126T100000\n"+
		"DTSTART;TZID=Europe/Moscow:20260126T120000\nDTEND;TZID=Europe/Moscow:20260126T130000\nSUMMARY:Duty moved\nEND:VEVENT\n"+
		"END:VCALENDAR\n")
	rec := "/app/calendar/" + connID + "/" + url.PathEscape(cal) + "/" + url.PathEscape(uid)
	form := func(p view.EventEditPage) view.Form { return p.Form }
	exceptions := []string{"EXDATE;TZID=Europe/Moscow:20260119T100000", "RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000", "SUMMARY:Duty moved", "X-CUSTOM:keep-me"}

	liveEdit(t, app, rec, form, map[string]string{"status": "CONFIRMED", "categories": "duty, work"})
	got, _ := rawBody(ctx, t, client, file)
	mustHave(t, "status and categories", got, append([]string{"STATUS:CONFIRMED", "CATEGORIES:duty,work",
		"DTSTART;TZID=Europe/Moscow:20260112T100000", "RRULE:FREQ=WEEKLY;BYDAY=MO;WKST=MO;COUNT=6"}, exceptions...)...)

	liveEdit(t, app, rec, form, map[string]string{"repeat.interval": "2", "repeat.count": "8"})
	got, _ = rawBody(ctx, t, client, file)
	mustHave(t, "repeat", got, append([]string{"RRULE:FREQ=WEEKLY;BYDAY=MO;WKST=MO;COUNT=8;INTERVAL=2"}, exceptions...)...)

	liveEdit(t, app, rec, form, map[string]string{"tz": "Asia/Yekaterinburg"})
	got, _ = rawBody(ctx, t, client, file)
	mustHave(t, "time zone", got, append([]string{"DTSTART;TZID=Asia/Yekaterinburg:20260112T120000",
		"DTEND;TZID=Asia/Yekaterinburg:20260112T130000", "TZID:Asia/Yekaterinburg", "BEGIN:VTIMEZONE"}, exceptions...)...)
}

func liveTaskFields(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, cal, suffix string) {
	uid := "slides-" + suffix
	file := cal + uid + ".ics"
	seed(ctx, t, client, file, "text/calendar; charset=utf-8", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\n"+
		"BEGIN:VTODO\nUID:"+uid+"\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nPRIORITY:3\nDESCRIPTION:Keep me\nX-CUSTOM:keep-me\nEND:VTODO\nEND:VCALENDAR\n")
	list := "/app/tasks/" + connID + "/" + url.PathEscape(cal)
	rec := list + "/" + url.PathEscape(uid)
	form := func(p view.TaskEditPage) view.Form { return p.Form }

	liveEdit(t, app, rec, form, map[string]string{"start.date": "2026-02-01", "start.allday": "on", "location": "Room 4", "status": "IN-PROCESS"})
	got, _ := rawBody(ctx, t, client, file)
	mustHave(t, "task fields", got, "DTSTART;VALUE=DATE:20260201", "LOCATION:Room 4", "STATUS:IN-PROCESS", "PRIORITY:3", "X-CUSTOM:keep-me")

	page := liveDecode[view.TaskPage](t, "card", app.get(rec))
	done := liveAction(t, page.Actions, "task.done")
	u, _ := url.Parse(done)
	if loc := redirected(t, "mark done", app.post(u.EscapedPath()+"?"+u.RawQuery, url.Values{})); loc != rec {
		t.Fatalf("mark done went to %q", loc)
	}
	got, _ = rawBody(ctx, t, client, file)
	mustHave(t, "mark done", got, "STATUS:COMPLETED", "COMPLETED:", "SUMMARY:Slides", "DESCRIPTION:Keep me", "LOCATION:Room 4", "X-CUSTOM:keep-me")
}

func liveNoteFields(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, cal, suffix string) {
	uid := "plan-" + suffix
	file := cal + uid + ".ics"
	seed(ctx, t, client, file, "text/calendar; charset=utf-8", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\n"+
		"BEGIN:VJOURNAL\nUID:"+uid+"\nDTSTAMP:20260101T000000Z\nDTSTART;VALUE=DATE:20260110\nSUMMARY:Plan\nDESCRIPTION:Body\nX-CUSTOM:keep-me\nEND:VJOURNAL\nEND:VCALENDAR\n")
	rec := "/app/notes/" + connID + "/" + url.PathEscape(cal) + "/" + url.PathEscape(uid)
	d := liveDecode[view.NoteDoc](t, "note edit", app.get(rec+"/edit"))
	v := url.Values{"summary": {string(d.Title)}, "description": {string(d.Body)}}
	for _, r := range d.Fields {
		v.Set(r.Name, string(r.Value))
	}
	v.Set("categories", "plans, work")
	v.Set("start.date", "2026-01-11")
	save, _ := url.Parse(liveAction(t, d.Bar, "record.save"))
	for k, vals := range save.Query() {
		v[k] = vals
	}
	redirected(t, "note save", app.post(save.EscapedPath(), v))
	got, _ := rawBody(ctx, t, client, file)
	mustHave(t, "note fields", got, "DTSTART;VALUE=DATE:20260111", "CATEGORIES:plans,work", "DESCRIPTION:Body", "X-CUSTOM:keep-me")
}

// connections-tail.md: the password went wrong — the card says so after
// Test, Edit with the right one saves under the same ID, and Test passes.
func liveConnectionEdit(t *testing.T, app *liveApp, connID, endpoint, user, pass string) {
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	broken := blob.Connections[0]
	broken.Password = pass + "-wrong"
	if err := app.users.UpdateConnection(app.sess.AccountID(), key, broken); err != nil {
		t.Fatal(err)
	}
	test := "/app/settings/connections?" + url.Values{"op": {"test"}, "account": {connID}}.Encode()
	card := liveCard(t, liveDecode[view.ConnectionsPage](t, "test broken", app.post(test, url.Values{})), connID)
	if card.Trace == nil || card.Trace.OK || card.State != view.CollectionUnavailable {
		t.Fatalf("a wrong password tested %+v, state %q", card.Trace, card.State)
	}
	if liveAction(t, card.Actions, "connection.test") == "" {
		t.Fatal("no Test again")
	}
	edit := "/app/settings/connections?" + url.Values{"op": {"edit"}, "account": {connID}}.Encode()
	page := liveDecode[view.ConnectionsPage](t, "edit", app.post(edit, url.Values{
		"label": {broken.Label}, "server": {endpoint}, "login": {user}, "password": {pass},
	}))
	for _, n := range page.Shell.Notices {
		if n.Title == "Could not connect" {
			t.Fatalf("edit with the right password failed: %+v", n)
		}
	}
	blob, _ = app.users.Catalog(app.sess.AccountID(), key)
	if len(blob.Connections) != 1 || blob.Connections[0].ID != connID || blob.Connections[0].Password != pass {
		t.Fatalf("after edit: %+v", blob.Connections)
	}
	card = liveCard(t, liveDecode[view.ConnectionsPage](t, "test again", app.post(test, url.Values{})), connID)
	if card.Trace == nil || !card.Trace.OK || card.State != view.CollectionOK {
		t.Errorf("after edit the connection tested %+v, state %q", card.Trace, card.State)
	}
}

func liveCard(t *testing.T, page view.ConnectionsPage, id string) view.ConnectionCard {
	t.Helper()
	for _, c := range page.Accounts {
		if c.AccountID == id {
			return c
		}
	}
	t.Fatalf("no card %s", id)
	return view.ConnectionCard{}
}
