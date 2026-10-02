//go:build integration

package integration

import (
	"context"
	"encoding/json"
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
	user2 "carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for reviews Р-10 and Р-9: the fields the forms
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

// SoGo and Yandex both give a calendar home and let a calendar be made, so
// the forms run on them whole. Yandex refuses a new address book (SV-19):
// the contact part says so and steps aside rather than failing, because a
// server's limit is not a fault of the screen.
func TestSoGoForms(t *testing.T) {
	endpoint, user, pass := sogoConfig(t)
	runForms(t, endpoint, user, pass)
}

func TestYandexForms(t *testing.T) {
	endpoint, user, pass := yandexConfig(t)
	runForms(t, endpoint, user, pass)
}

// poolCounts prints what the guard did over a live run, so the question of
// a shared pool is settled by numbers on a real network rather than on a
// stand where a connection costs nothing (docs/plan/server.md, «Пул
// соединений»).
func poolCounts(t *testing.T, app *liveApp, what string) {
	t.Helper()
	clients, dialled, requests, writes, fromIdle, resent := app.srv.DAVCounts()
	t.Logf("pool over %s: %d clients built, %d connections dialled, %d requests, %d of them writes, %d of those off an idle connection, %d sent again",
		what, clients, dialled, requests, writes, fromIdle, resent)
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
	bookPath := tryCollection(ctx, t, client, endpoint, user, pass, "MKCOL",
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
	cal := catalogEntry(t, blob, calPath)

	if bookPath == "" {
		t.Logf("the server would not make an address book: the contact part is skipped")
	} else {
		book := catalogEntry(t, blob, bookPath)
		t.Run("contact", func(t *testing.T) { liveContactFields(ctx, t, app, client, conn.ID, book.Path, suffix) })
	}
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
	poolCounts(t, app, "a forms run")
}

// review Р-9: Disconnect leaves the server alone and Carrel with
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
	if lost := liveEditDropping(t, app, rec, form, set); len(lost) > 0 {
		t.Fatalf("save %v: the server did not keep %v", set, lost)
	}
}

// liveEditDropping saves a form and returns what the server did not keep.
// A server that drops a property answers with the form and the report of
// C-9 instead of a redirect, and that is the right answer: Yandex keeps no
// STATUS and no CATEGORIES on an event, and a test that demanded a redirect
// there would be demanding that Carrel lie.
func liveEditDropping[P any](t *testing.T, app *liveApp, rec string, form func(P) view.Form, set map[string]string) []string {
	t.Helper()
	v := liveForm(form(liveDecode[P](t, "edit "+rec, app.get(rec+"/edit"))))
	for k, val := range set {
		v.Set(k, val)
	}
	rr := app.post(rec+"/edit", v)
	if rr.Code == 200 {
		var body struct {
			Loss *view.LossReport `json:"loss"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err == nil && body.Loss != nil {
			var out []string
			for _, p := range body.Loss.Properties {
				out = append(out, string(p))
			}
			t.Logf("save %v: the server kept everything but %v", set, out)
			return out
		}
	}
	if loc := redirected(t, fmt.Sprintf("save %v", set), rr); loc != rec {
		t.Fatalf("save %v went to %q", set, loc)
	}
	return nil
}

func seed(ctx context.Context, t *testing.T, client *dav.Client, file, media, body string) {
	t.Helper()
	if err := putSeed(ctx, client, file, media, body); err != nil {
		t.Fatalf("seed %s: %v", file, err)
	}
}

// putSeed writes a record and reads «it is already there» the way the app
// does: a conditional PUT refused on a request the guard had already sent
// once is that write arriving twice, not a failure (SV-39). The full run
// of 22.09.2026 hit this in the seeding itself. A shared pool keeps
// connections idle where each client used to throw its own away, so the
// dropped-connection race of SV-9 happens oftener now, and the helper
// that seeds has to read a refusal as carefully as the screens do.
func putSeed(ctx context.Context, client *dav.Client, file, media, body string) error {
	_, err := client.PutCreate(ctx, file, dav.Typed(strings.NewReader(strings.ReplaceAll(body, "\n", "\r\n")), media))
	if dav.WasResent(err) && dav.IsStatus(err, http.StatusPreconditionFailed) {
		return nil
	}
	return err
}

// trySeed is seed for a record a server may refuse to hold at all. SoGo
// answers a VJOURNAL PUT with 404: it keeps no notes, and a run that
// demanded one would be testing SoGo's component set, not Carrel's
// timeline. What could not be seeded is said in the log and left out of the
// expectations.
func trySeed(ctx context.Context, t *testing.T, client *dav.Client, file, media, body string) bool {
	t.Helper()
	if err := putSeed(ctx, client, file, media, body); err != nil {
		t.Logf("the server does not hold %s: %v", file, err)
		return false
	}
	return true
}

func liveContactFields(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, book, suffix string) {
	uid := "vera-" + suffix
	file := book + uid + ".vcf"
	seed(ctx, t, client, file, "text/vcard; charset=utf-8", "BEGIN:VCARD\nVERSION:3.0\nUID:"+uid+"\nFN:Vera Lebedeva\nN:Lebedeva;Vera;;;\n"+
		"TEL;TYPE=work:+7 912 000 00 01\nEMAIL:vera@carrel.test\nADR;TYPE=home:PO 7;Flat 2;Lenina 1;Ekaterinburg;;620000;Russia\n"+
		"BDAY:--0412\nCATEGORIES:seed\nX-CUSTOM:keep-me\nEND:VCARD\n")
	// CATEGORIES is in the seed so that what the server keeps of it is seen:
	// without it the check of categories below was skipped on every server,
	// as a property the server «did not take» — found on Davis's first run,
	// 24.09.2026, where the suite ran with -v and the line showed.
	// A server may name the record itself and write a UID of its own over
	// the one it was sent (SV-25). The card is then read back at the
	// address it was written to, and the person meets the identity the
	// server kept.
	seeded := bodyIfThere(ctx, t, client, file)
	if kept := uidOf(seeded); kept != "" && kept != uid {
		t.Logf("the server named the card %q, not %q", kept, uid)
		uid = kept
		// Google answers GET at the address it was sent to and 404 to a
		// DELETE there: the card lives under the name it gave, which is its
		// UID (SV-25). A cleanup at the sent address deleted nothing and said
		// nothing, and by 01.10.2026 eleven «Vera Lebedeva» cards sat in the
		// Google book.
		removeLater(t, client, book+kept)
	}
	keeps := func(line string) bool { return strings.Contains(strings.ToLower(seeded), strings.ToLower(line)) }
	rec := "/app/contacts/" + connID + "/" + url.PathEscape(book) + "/" + url.PathEscape(uid)
	form := func(p view.ContactEditPage) view.Form { return p.Form }

	// asked keeps only what this server took as sent and did not report
	// dropping: what it will not hold is its own answer, said once in the
	// log, and demanding it again would make a server's limit look like a
	// fault of the form.
	asked := func(lost []string, lines ...string) []string {
		var out []string
		for _, line := range lines {
			prop := strings.SplitN(strings.SplitN(line, ";", 2)[0], ":", 2)[0]
			switch {
			case contains(lost, prop):
				t.Logf("the server did not keep %s: it is not asked for again", prop)
			case !keeps(prop):
				t.Logf("the server did not take %s even as sent: it is not asked for again", prop)
			default:
				out = append(out, line)
			}
		}
		return out
	}

	lost := liveEditDropping(t, app, rec, form, map[string]string{"nickname": "Verochka", "categories": "work, family"})
	got, _ := rawBody(ctx, t, client, file)
	if !contains(lost, "BDAY") && keeps("BDAY") {
		yearless(t, "nickname and categories", got, "04", "12")
	}
	mustHave(t, "nickname and categories", got, append([]string{"NICKNAME:Verochka"},
		asked(lost, "CATEGORIES:work,family", "X-CUSTOM:keep-me")...)...)
	// An address is not a string: iCloud moves the box and the flat into
	// the street, joined by newlines, and keeps the same address. What
	// must survive is what was written in it.
	if keeps("ADR") && !contains(lost, "ADR") {
		mustAddress(t, "nickname and categories", got, "PO 7", "Flat 2", "Lenina 1", "Ekaterinburg", "620000", "Russia")
	}

	lost = liveEditDropping(t, app, rec, form, map[string]string{"bday": "13.04", "tel.0.type": "cell", "adr.0.region": "Sverdlovsk"})
	got, _ = rawBody(ctx, t, client, file)
	if !contains(lost, "BDAY") && keeps("BDAY") {
		yearless(t, "birthday", got, "04", "13")
	}
	mustHave(t, "birthday, label, address", got, asked(lost, "X-CUSTOM:keep-me")...)
	mustValue(t, "birthday, label, address", got, "EMAIL", "vera@carrel.test")
	if !contains(lost, "TEL") {
		mustLabel(t, "birthday, label, address", got, "TEL", "cell", "+7 912 000 00 01")
	}
	if keeps("ADR") && !contains(lost, "ADR") {
		mustAddress(t, "birthday, label, address", got, "PO 7", "Flat 2", "Lenina 1", "Ekaterinburg", "Sverdlovsk", "620000", "Russia")
	}
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
	// What of the seed the server kept is read back before anything is
	// edited: a line the server never accepted is not a line Carrel lost,
	// and demanding it would make the server's limit look like a fault of
	// the forms. Yandex takes no X-CUSTOM on an event.
	seeded, _ := rawBody(ctx, t, client, file)
	var exceptions []string
	for _, line := range []string{"EXDATE;TZID=Europe/Moscow:20260119T100000",
		"RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000", "SUMMARY:Duty moved", "X-CUSTOM:keep-me"} {
		if strings.Contains(strings.ToLower(seeded), strings.ToLower(line)) {
			exceptions = append(exceptions, line)
		} else {
			t.Logf("the server did not take %q even as sent: it is not asked for again", line)
		}
	}

	// Yandex keeps no STATUS and no CATEGORIES on an event. Carrel says so
	// and the save stands; what the server dropped is not then demanded of
	// the file.
	dropped := liveEditDropping(t, app, rec, form, map[string]string{"status": "CONFIRMED", "categories": "duty, work"})
	got, _ := rawBody(ctx, t, client, file)
	want := []string{"DTSTART;TZID=Europe/Moscow:20260112T100000"}
	for prop, line := range map[string]string{"STATUS": "STATUS:CONFIRMED", "CATEGORIES": "CATEGORIES:duty,work"} {
		if !contains(dropped, prop) {
			want = append(want, line)
		}
	}
	mustHave(t, "status and categories", got, append(want, exceptions...)...)
	// Yandex rewrites a rule into one of its own liking — COUNT becomes
	// UNTIL, INTERVAL=1 appears, WKST goes — and keeps the series the same
	// series. A server that does that cannot be asked to spell a rule back,
	// so the rule is checked where it is kept as written and named where it
	// is not (docs/servers.md).
	rewrites := !sameRule(got, "FREQ=WEEKLY", "BYDAY=MO", "WKST=MO", "COUNT=6")
	if rewrites {
		t.Logf("the server rewrote the rule: %q", ruleLine(got))
	} else {
		mustRule(t, "status and categories", got, "FREQ=WEEKLY", "BYDAY=MO", "WKST=MO", "COUNT=6")
	}

	// A server that rewrites the rule answers the next save with a loss
	// report naming RRULE, and it is telling the truth: what came back is
	// not what was sent.
	// Until is cleared with the same save that sets Count. A server that
	// turned COUNT into UNTIL leaves the form holding a day, and a series
	// ends after a number of times or on a day, not both — Carrel refuses
	// the pair on the field, which is what a person sees and answers.
	liveEditDropping(t, app, rec, form, map[string]string{
		"repeat.interval": "2", "repeat.count": "8", "repeat.until": "",
	})
	got, _ = rawBody(ctx, t, client, file)
	mustHave(t, "repeat", got, exceptions...)
	if !rewrites {
		mustRule(t, "repeat", got, "FREQ=WEEKLY", "BYDAY=MO", "WKST=MO", "COUNT=8", "INTERVAL=2")
	}

	// Another zone keeps the clock the form shows, 10:00 in Moscow becoming
	// 10:00 in Yekaterinburg: the form is in the event's zone (s-agenda-3,
	// v0.2.135). A server may keep the same instant in a zone of its own choosing —
	// Yandex writes the time back in UTC and drops the VTIMEZONE. Carrel
	// reports the difference, which is the honest answer; the file is then
	// only asked for what the server did keep.
	movedTimes := liveEditDropping(t, app, rec, form, map[string]string{"tz": "Asia/Yekaterinburg"})
	got, _ = rawBody(ctx, t, client, file)
	if len(movedTimes) > 0 {
		t.Logf("the server rewrote the times (%v); it keeps them as:\n%s", movedTimes, got)
		mustHave(t, "time zone", got, exceptions...)
	} else {
		mustHave(t, "time zone", got, append([]string{"DTSTART;TZID=Asia/Yekaterinburg:20260112T100000",
			"DTEND;TZID=Asia/Yekaterinburg:20260112T110000", "TZID:Asia/Yekaterinburg", "BEGIN:VTIMEZONE"}, exceptions...)...)
	}
}

func liveTaskFields(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, cal, suffix string) {
	uid := "slides-" + suffix
	file := cal + uid + ".ics"
	seed(ctx, t, client, file, "text/calendar; charset=utf-8", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\n"+
		"BEGIN:VTODO\nUID:"+uid+"\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nPRIORITY:3\nDESCRIPTION:Keep me\nX-CUSTOM:keep-me\nEND:VTODO\nEND:VCALENDAR\n")
	// A calendar that answered MKCALENDAR with VTODO in its component set
	// may still keep no task: Yandex takes the PUT and holds nothing. There
	// is then no record to edit, and that is the server's word, not a fault
	// of the form.
	if body := bodyIfThere(ctx, t, client, file); !strings.Contains(body, "BEGIN:VTODO") {
		t.Skipf("the calendar kept no task: %q", body)
	}
	list := "/app/tasks/" + connID + "/" + url.PathEscape(cal)
	rec := list + "/" + url.PathEscape(uid)
	form := func(p view.TaskEditPage) view.Form { return p.Form }

	liveEdit(t, app, rec, form, map[string]string{"start.date": "2026-02-01", "allday": "on", "location": "Room 4", "status": "IN-PROCESS"})
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
	d := liveDecode[view.NotePage](t, "note edit", app.get(rec+"/edit")).Doc
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

// checksPasswords asks the server itself whether a wrong password is refused.
// Not every server has credentials to check: Xandikos is standalone and
// leaves that to a proxy in front, so everything it is asked succeeds. What
// depends on a refusal is then untestable there — a fact about the server,
// found by asking it, not by a list of server names in the suite.
func checksPasswords(t *testing.T, endpoint, user, pass string) bool {
	t.Helper()
	req, err := http.NewRequest("PROPFIND", endpoint, nil)
	if err != nil {
		return true
	}
	req.Header.Set("Depth", "0")
	req.SetBasicAuth(user, pass+"-wrong")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return true
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden
}

// review Р-9: the password went wrong — the card says so after
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
	if !checksPasswords(t, endpoint, user, pass) {
		// Xandikos checks no credentials at all: standalone, with the
		// checking left to a proxy in front (testdata/xandikos/README.md).
		// A wrong password cannot be told from a right one there, and
		// saying so on the card would be Carrel inventing a refusal the
		// server never gave. So the half of this check that needs a
		// refusal is skipped here, by asking the server rather than by a
		// list of names — and the Edit half below still runs.
		t.Log("the server accepts any password: the wrong-password half cannot be checked here")
	} else {
		if card.Trace == nil || card.Trace.OK || card.State != view.CollectionUnavailable {
			t.Fatalf("a wrong password tested %+v, state %q", card.Trace, card.State)
		}
		if liveAction(t, card.Actions, "connection.test") == "" {
			t.Fatal("no Test again")
		}
	}
	edit := "/app/settings/connections?" + url.Values{"op": {"edit"}, "account": {connID}}.Encode()
	form := url.Values{"label": {broken.Label}, "server": {endpoint}, "login": {user}, "password": {pass}}
	if pass == "" {
		// An empty field means «leave it as it is» — otherwise editing a label
		// would wipe the password. A server that asks none (Xandikos) is said
		// by the mark, which clears the stored one (s-profile-14).
		form.Set("noPassword", "true")
	}
	page := liveDecode[view.ConnectionsPage](t, "edit", app.post(edit, form))
	for _, n := range page.Shell.Notices {
		if n.Title == "Could not connect" {
			t.Fatalf("edit with the right password failed: %+v", n)
		}
	}
	blob, _ = app.users.Catalog(app.sess.AccountID(), key)
	if len(blob.Connections) != 1 || blob.Connections[0].ID != connID {
		t.Fatalf("after edit: %+v", blob.Connections)
	}
	if got := blob.Connections[0]; got.Password != pass || got.NoPassword != (pass == "") {
		t.Fatalf("after edit the password is %q (no password: %v), want the one just entered", got.Password, got.NoPassword)
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

// runContactFieldsInTheOnlyBook is the contact half of the forms on a server
// that makes no collections: Google and iCloud each give one address book
// and refuse a second (SV-19 in their own shapes), so the run uses what is
// there and leaves the account as it found it.
func runContactFieldsInTheOnlyBook(t *testing.T, endpoint, user, pass, who string) {
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
	suffix := fmt.Sprintf("carrel-%s-%d-%d", who, os.Getpid(), time.Now().Unix()%100000)
	// The book outlives the run, so the card the run seeds is taken away by
	// name. Until 30.09.2026 it was not, and nine «Vera Lebedeva» cards of
	// earlier runs sat in the iCloud book.
	removeLater(t, client, book.Path+"vera-"+suffix+".vcf")
	liveContactFields(ctx, t, app, client, connID, book.Path, suffix)
}
