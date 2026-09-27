//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/internal/web"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for S-11: the record routes against a live server, through the real
// request layer — connect, list, card, edit, conflict, create, delete. The
// httptest suite runs go-webdav's own server; this one runs what people
// connect.
func TestRadicaleRecords(t *testing.T) {
	runRecords(t, radicaleEndpoint(t), radicaleUser, radicalePass)
}

func TestBaikalRecords(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runRecords(t, endpoint, user, pass)
}

type liveApp struct {
	t     *testing.T
	srv   *web.Server
	st    *store.Store
	users *user.Service
	sess  *session.Session
}

func newLiveApp(t *testing.T) *liveApp {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	key, err := crypt.CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = dir
	cfg.Timezone = "UTC"
	cfg.DAVSSRFAllowlist = "127.0.0.1,localhost"
	cfg.DAVRequestTimeout = 30 * time.Second
	mgr := session.NewManager(cfg)
	users := user.New(st, mgr, cfg)
	if err := users.SetupFirstAdmin(ctx, "owner", "password123"); err != nil {
		t.Fatal(err)
	}
	id, dataKey, _, ok := users.Authenticate(ctx, "owner", "password123")
	if !ok {
		t.Fatal("login")
	}
	sess, err := mgr.Create(id, "owner", dataKey)
	if err != nil {
		t.Fatal(err)
	}
	return &liveApp{t: t, srv: web.New(cfg, st, mgr, users, web.JSONRenderer{}, "live", "live"), st: st, users: users, sess: sess}
}

func (a *liveApp) get(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: a.sess.ID()})
	rec := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rec, req)
	return rec
}

func (a *liveApp) post(path string, form url.Values) *httptest.ResponseRecorder {
	form.Set("_csrf", a.sess.CSRFToken())
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: a.sess.ID()})
	rec := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rec, req)
	return rec
}

// redirected fails with the page when a write answered anything but a
// redirect: a notice, a form error, or a loss report.
func redirected(t *testing.T, step string, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("%s: status %d, want 303:\n%s", step, rec.Code, rec.Body.String())
	}
	return rec.Header().Get("Location")
}

func liveDecode[T any](t *testing.T, step string, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if rec.Code != http.StatusOK {
		// Where a redirect went is the whole difference between a write that
		// was carried out and a session that lapsed, and the body of a
		// redirect is empty, so the header is the only evidence there is.
		if loc := rec.Header().Get("Location"); loc != "" {
			t.Fatalf("%s: status %d to %s:\n%s", step, rec.Code, loc, rec.Body.String())
		}
		t.Fatalf("%s: status %d:\n%s", step, rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("%s: decode %T: %v", step, v, err)
	}
	return v
}

func liveForm(f view.Form) url.Values {
	out := url.Values{}
	for _, set := range f.Sets {
		for _, row := range set.Rows {
			switch {
			case row.Name == "_csrf":
			case row.Kind == view.InputCheck && len(row.Options) > 0:
				// Several ticks under one name: each one ticked is a value.
				out.Del(row.Name)
				for _, o := range row.Options {
					if o.Selected {
						out.Add(row.Name, o.Value)
					}
				}
			default:
				out.Set(row.Name, string(row.Value))
			}
		}
	}
	return out
}

func liveAction(t *testing.T, actions []view.Action, id string) string {
	t.Helper()
	for _, a := range actions {
		if a.ID == id {
			return a.Href
		}
	}
	t.Fatalf("no action %q", id)
	return ""
}

// liveRequest sends a raw DAV request to an absolute path on the endpoint's
// host, with the target's own credentials.
func liveRequest(ctx context.Context, t *testing.T, endpoint, user, pass, method, path, body string) {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = path
	req, err := http.NewRequestWithContext(ctx, method, u.String(), strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	req.SetBasicAuth(user, pass)
	resp, err := testGuard(t).HTTPClient().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("%s %s: %s %s", method, path, resp.Status, msg)
	}
}

func rawBody(ctx context.Context, t *testing.T, c *dav.Client, path string) (string, string) {
	t.Helper()
	rc, etag, err := c.Get(ctx, path, nil)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	// Unfold so a folded line still matches.
	return strings.ReplaceAll(strings.ReplaceAll(string(b), "\r\n ", ""), "\r\n", "\n"), etag
}

func mustHave(t *testing.T, step, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(strings.ToLower(got), strings.ToLower(w)) {
			t.Errorf("%s: missing %q in\n%s", step, w, got)
		}
	}
}

func runRecords(t *testing.T, endpoint, user, pass string) {
	ctx := context.Background()
	g := testGuard(t)
	client, err := dav.NewClient(g, endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	// Only the homes are needed here. A fresh server has no collection yet,
	// and discovery rightly calls that a bad check; the collections below are
	// what the connection step then finds.
	res, trace, err := discover.Run(ctx, g, endpoint, user, pass, "")
	if err != nil || res.CalendarHome == "" || res.AddressBookHome == "" {
		t.Fatalf("discovery: %v %+v %+v", err, res, trace)
	}

	suffix := fmt.Sprintf("carrel-s11-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	// Radicale has one home for both kinds: the two names must differ.
	calPath := strings.TrimSuffix(res.CalendarHome, "/") + "/" + suffix + "-cal/"
	bookPath := strings.TrimSuffix(res.AddressBookHome, "/") + "/" + suffix + "-book/"
	calPath = makeCollection(ctx, t, client, endpoint, user, pass, "MKCALENDAR", calPath, `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:I="http://apple.com/ns/ical/">
  <D:set><D:prop>
    <D:displayname>Carrel S-11</D:displayname>
    <I:calendar-color>#3366CCFF</I:calendar-color>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/><C:comp name="VTODO"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
	bookPath = makeCollection(ctx, t, client, endpoint, user, pass, "MKCOL", bookPath, `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel S-11</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)
	seedColour(ctx, t, client, calPath, "#3366CCFF")

	app := newLiveApp(t)
	redirected(t, "connect", app.post("/app/settings/connections", url.Values{
		"server": {endpoint}, "login": {user}, "password": {pass},
	}))
	key, _ := app.sess.DataKey()
	blob, ok := app.users.Catalog(app.sess.AccountID(), key)
	if !ok || len(blob.Connections) != 1 {
		t.Fatalf("catalog = %+v", blob)
	}
	conn := blob.Connections[0]
	cal, book := catalogEntry(t, blob, calPath), catalogEntry(t, blob, bookPath)
	if cal.ReadOnly || book.ReadOnly || cal.ChangeTag == "" || book.ChangeTag == "" {
		t.Errorf("catalog rights or change tag: calendar %+v, book %+v", cal, book)
	}
	if !cal.Has(view.KindEvent) || !cal.Has(view.KindTask) || !book.Has(view.KindContact) {
		t.Errorf("catalog kinds: calendar %v, book %v", cal.Kinds, book.Kinds)
	}
	if cal.Colour == "" {
		t.Errorf("calendar-color set at creation did not reach the catalog: %+v", cal)
	}

	t.Run("contacts", func(t *testing.T) { liveContacts(ctx, t, app, client, conn.ID, book.Path, suffix) })
	t.Run("apple card", func(t *testing.T) { liveAppleCard(ctx, t, app, client, conn.ID, book.Path, suffix) })
	t.Run("calendar", func(t *testing.T) { liveCalendar(ctx, t, app, client, conn.ID, cal.Path, suffix) })
}

// seedColour makes sure the calendar carries the colour this suite goes on to
// look for. Not every server takes one at creation: Xandikos answers
// MKCALENDAR 201 and names calendar-color in a 404 propstat of the
// mkcalendar-response, while a PROPPATCH a moment later sets it and keeps it
// (docs/servers.md, SV-41). What is checked below is that Carrel's catalog
// shows the colour the server holds; whether the server took it at creation
// is the stand's business, not the check's, and Carrel's own create settles
// it the same way (settleColour).
func seedColour(ctx context.Context, t *testing.T, client *dav.Client, path, colour string) {
	t.Helper()
	name := xml.Name{Space: "http://apple.com/ns/ical/", Local: "calendar-color"}
	shown := func() string {
		ms, err := client.PropFind(ctx, path, dav.DepthZero, []xml.Name{name})
		if err != nil {
			return ""
		}
		for _, r := range ms.Responses {
			if v := strings.TrimSpace(string(r.Props[name])); v != "" {
				return v
			}
		}
		return ""
	}
	if shown() != "" {
		return
	}
	if err := client.PropPatch(ctx, path, []dav.Prop{{Name: name, Value: dav.TextValue(colour)}}, nil); err != nil {
		t.Logf("the create dropped calendar-color and PROPPATCH would not set it either: %v", err)
		return
	}
	t.Logf("calendar-color was dropped at creation and set by PROPPATCH (SV-41); it now shows %q", shown())
}

// catalogEntry finds a collection by path;// catalogEntry finds a collection by path; the server may spell the path it
// reports differently (a trailing slash, percent-encoding).
func catalogEntry(t *testing.T, blob sealed.Blob, path string) sealed.Collection {
	t.Helper()
	norm := func(p string) string {
		if u, err := url.PathUnescape(p); err == nil {
			p = u
		}
		return strings.TrimSuffix(p, "/")
	}
	for _, c := range blob.Collections {
		if norm(c.Path) == norm(path) {
			return c
		}
	}
	t.Fatalf("%s not in the catalog: %+v", path, blob.Collections)
	return sealed.Collection{}
}

func liveContacts(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, book, suffix string) {
	uid := "ada-" + suffix
	file := book + uid + ".vcf"
	card := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:" + uid + "\r\nFN:Ada Lovelace\r\nN:Lovelace;Ada;;;\r\n" +
		"EMAIL;TYPE=work:ada@old.carrel.test\r\nX-CUSTOM:keep-me\r\nEND:VCARD\r\n"
	if _, err := client.PutCreate(ctx, file, dav.Typed(strings.NewReader(card), "text/vcard; charset=utf-8")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	list := "/app/contacts/" + connID + "/" + url.PathEscape(book)
	rec := list + "/" + url.PathEscape(uid)

	page := liveDecode[view.ContactsListPage](t, "list", app.get(list))
	if len(page.Notices) > 0 || len(page.Groups) != 1 || len(page.Groups[0].Rows) != 1 || page.Groups[0].Rows[0].UID != uid {
		t.Fatalf("list = %+v", page)
	}
	cardPage := liveDecode[view.ContactPage](t, "card", app.get(rec))
	var foreign bool
	for _, f := range cardPage.Foreign {
		foreign = foreign || (f.Label == "X-CUSTOM" && f.Value == "keep-me")
	}
	if cardPage.Name != "Ada Lovelace" || !foreign {
		t.Errorf("card = %+v", cardPage)
	}

	form := liveForm(liveDecode[view.ContactEditPage](t, "edit", app.get(rec+"/edit")).Form)
	form.Set("fn", "Ada King")
	form.Set("email.0", "ada@new.carrel.test")
	redirected(t, "edit", app.post(rec+"/edit", form))
	got, etag := rawBody(ctx, t, client, file)
	mustHave(t, "edit", got, "FN:Ada King", "ada@new.carrel.test", "type=work", "X-CUSTOM:keep-me", "UID:"+uid)

	// Another client writes between reading the form and saving it.
	form = liveForm(liveDecode[view.ContactEditPage](t, "edit again", app.get(rec+"/edit")).Form)
	other := strings.Replace(strings.ReplaceAll(got, "\n", "\r\n"), "END:VCARD", "NOTE:server note\r\nEND:VCARD", 1)
	if _, err := client.Put(ctx, file, dav.Typed(strings.NewReader(other), "text/vcard; charset=utf-8"), `"`+etag+`"`); err != nil {
		t.Fatalf("other client: %v", err)
	}
	form.Set("title", "Countess")
	if loc := redirected(t, "stale edit", app.post(rec+"/edit", form)); loc != rec+"/conflict" {
		t.Fatalf("stale edit went to %q, want the conflict screen", loc)
	}
	conflict := liveDecode[view.ConflictPage](t, "conflict", app.get(rec+"/conflict"))
	if len(conflict.Rows) != 1 || conflict.Rows[0].Property != "TITLE" || conflict.Rows[0].Mine != "Countess" {
		t.Fatalf("conflict rows = %+v", conflict.Rows)
	}
	redirected(t, "combine", app.post(liveAction(t, conflict.Actions, "conflict.combine"), url.Values{}))
	got, _ = rawBody(ctx, t, client, file)
	mustHave(t, "combine", got, "TITLE:Countess", "NOTE:server note", "FN:Ada King", "X-CUSTOM:keep-me")

	cardPage = liveDecode[view.ContactPage](t, "card before delete", app.get(rec))
	if loc := redirected(t, "delete", app.post(liveAction(t, cardPage.Actions, "record.delete"), url.Values{})); loc != list {
		t.Errorf("delete went to %q", loc)
	}
	if _, _, err := client.Get(ctx, file, nil); !dav.IsStatus(err, http.StatusNotFound) {
		t.Errorf("after delete GET = %v, want 404", err)
	}
}

// liveAppleCard edits one field of a card as Apple Contacts writes it and
// reads the server's copy: the groups that tie a label to its value, every
// TYPE value and the list of categories are still there (SV-32). Through
// go-vcard alone one save dropped all three.
func liveAppleCard(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, book, suffix string) {
	uid := "apple-" + suffix
	file := book + uid + ".vcf"
	card := strings.Join([]string{"BEGIN:VCARD", "VERSION:3.0", "UID:" + uid, "FN:Grace Hopper", "N:Hopper;Grace;;;",
		"item1.EMAIL;type=INTERNET;type=HOME;type=pref:grace@home.carrel.test", "item1.X-ABLabel:_$!<Home>!$_",
		"item2.TEL;type=CELL;type=VOICE:+1 555 0100", "item2.X-ABLabel:Navy", "CATEGORIES:work,friends", "END:VCARD", ""}, "\r\n")
	if _, err := client.PutCreate(ctx, file, dav.Typed(strings.NewReader(card), "text/vcard; charset=utf-8")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := "/app/contacts/" + connID + "/" + url.PathEscape(book) + "/" + url.PathEscape(uid)
	form := liveForm(liveDecode[view.ContactEditPage](t, "edit", app.get(rec+"/edit")).Form)
	form.Set("title", "Rear Admiral")
	redirected(t, "edit", app.post(rec+"/edit", form))
	got, _ := rawBody(ctx, t, client, file)
	mustHave(t, "edit", got, "TITLE:Rear Admiral", "item1.EMAIL", "item1.X-ABLabel", "item2.TEL", "item2.X-ABLabel:Navy", "home", "voice", "grace@home.carrel.test")
	if strings.Contains(got, `work\,friends`) || !strings.Contains(strings.ToLower(got), "categories:work,friends") {
		t.Errorf("categories are no longer a list:\n%s", got)
	}
}

func liveCalendar(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, cal, suffix string) {
	events := "/app/calendar/" + connID + "/" + url.PathEscape(cal)
	tasks := "/app/tasks/" + connID + "/" + url.PathEscape(cal)

	form := liveForm(liveDecode[view.EventEditPage](t, "new event form", app.get(events+"/new")).Form)
	form.Set("summary", "Live dentist, "+suffix)
	form.Set("start.date", "2026-10-05")
	form.Set("start.time", "09:30")
	form.Set("reminder.new", "-PT10M")
	loc := redirected(t, "new event", app.post(events+"/new", form))
	m := regexp.MustCompile(`/([0-9a-f]{32})$`).FindStringSubmatch(loc)
	if m == nil {
		t.Fatalf("new event went to %q", loc)
	}
	eventFile := cal + m[1] + ".ics"
	got, _ := rawBody(ctx, t, client, eventFile)
	mustHave(t, "new event", got, "UID:"+m[1], "DTSTART:20261005T093000Z", "TRIGGER:-PT10M", `SUMMARY:Live dentist\, `+suffix)

	agenda := liveDecode[view.AgendaPage](t, "agenda", app.get(events+"?from=2026-10-05"))
	// A week is seven days, and the days nothing falls on are shown empty
	// (review Р-12): a gap in an agenda is a day, not a missing row.
	if len(agenda.Notices) > 0 || len(agenda.Days) != 7 || len(agenda.Days[0].Rows) != 1 ||
		agenda.Days[0].Rows[0].Time != "09:30" {
		t.Fatalf("agenda = %+v", agenda)
	}
	event := liveDecode[view.EventPage](t, "event", app.get(loc))
	if len(event.Reminders) != 1 || event.Reminders[0].Label != "10 minutes before" {
		t.Errorf("reminders = %+v", event.Reminders)
	}

	series := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:series-" + suffix + "\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20261006T100000Z\r\n" +
		"DTEND:20261006T103000Z\r\nRRULE:FREQ=WEEKLY;COUNT=3\r\nSUMMARY:Standup\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:series-" + suffix + "\r\nDTSTAMP:20260101T000000Z\r\nRECURRENCE-ID:20261013T100000Z\r\n" +
		"DTSTART:20261013T120000Z\r\nDTEND:20261013T123000Z\r\nSUMMARY:Standup moved\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	if _, err := client.PutCreate(ctx, cal+"series-"+suffix+".ics", dav.Typed(strings.NewReader(series), "text/calendar; charset=utf-8")); err != nil {
		t.Fatalf("seed series: %v", err)
	}
	agenda = liveDecode[view.AgendaPage](t, "agenda with override", app.get(events+"?from=2026-10-12"))
	var lines []string
	for _, d := range agenda.Days {
		for _, r := range d.Rows {
			lines = append(lines, string(r.Time)+" "+string(r.Summary))
		}
	}
	if strings.Join(lines, "|") != "12:00 Standup moved" {
		t.Errorf("week of 12 October = %v, want the override alone", lines)
	}

	form = liveForm(liveDecode[view.TaskEditPage](t, "new task form", app.get(tasks+"/new")).Form)
	form.Set("summary", "Live task")
	form.Set("due.date", "2026-10-07")
	form.Set("due.allday", "on")
	taskLoc := redirected(t, "new task", app.post(tasks+"/new", form))
	form = liveForm(liveDecode[view.TaskEditPage](t, "task edit", app.get(taskLoc+"/edit")).Form)
	form.Set("done", "on")
	redirected(t, "task done", app.post(taskLoc+"/edit", form))
	taskFile := cal + taskLoc[strings.LastIndex(taskLoc, "/")+1:] + ".ics"
	got, _ = rawBody(ctx, t, client, taskFile)
	mustHave(t, "task done", got, "STATUS:COMPLETED", "COMPLETED:", "DUE;VALUE=DATE:20261007")
	list := liveDecode[view.TasksPage](t, "done tasks", app.get(tasks+"?show=done"))
	if len(list.Groups) != 1 || len(list.Groups[0].Rows) != 1 || !list.Groups[0].Rows[0].Done {
		t.Errorf("done tasks = %+v", list.Groups)
	}

	event = liveDecode[view.EventPage](t, "event before delete", app.get(loc))
	redirected(t, "delete event", app.post(liveAction(t, event.Actions, "record.delete"), url.Values{}))
	if _, _, err := client.Get(ctx, eventFile, nil); !dav.IsStatus(err, http.StatusNotFound) {
		t.Errorf("after delete GET = %v, want 404", err)
	}
}

// mustRule checks a recurrence rule by its parts. A server may keep the
// parts in another order — SoGo writes FREQ, COUNT, BYDAY, WKST where
// Carrel sent FREQ, BYDAY, WKST, COUNT — and the rule is the same rule.
// Comparing the line as text would call a normalising server a fault.
func mustRule(t *testing.T, step, got string, parts ...string) {
	t.Helper()
	var line string
	for _, l := range strings.Split(strings.ReplaceAll(got, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.ToUpper(l), "RRULE:") {
			line = strings.TrimSpace(l[len("RRULE:"):])
		}
	}
	if line == "" {
		t.Errorf("%s: no RRULE in\n%s", step, got)
		return
	}
	have := map[string]bool{}
	for _, p := range strings.Split(line, ";") {
		have[strings.ToUpper(strings.TrimSpace(p))] = true
	}
	for _, p := range parts {
		if !have[strings.ToUpper(p)] {
			t.Errorf("%s: the rule has no %q, only %q", step, p, line)
		}
	}
	if len(have) != len(parts) {
		t.Errorf("%s: the rule is %q, want exactly %v", step, line, parts)
	}
}

// contains says whether a list names something, ignoring case: a server
// that answers "status" and one that answers "STATUS" mean the same.
func contains(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

// ruleLine is the recurrence rule of an object as the server keeps it.
func ruleLine(got string) string {
	var line string
	for _, l := range strings.Split(strings.ReplaceAll(got, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.ToUpper(l), "RRULE:") {
			line = strings.TrimSpace(l[len("RRULE:"):])
		}
	}
	return line
}

// sameRule says whether the kept rule is exactly these parts.
func sameRule(got string, parts ...string) bool {
	have := map[string]bool{}
	for _, p := range strings.Split(ruleLine(got), ";") {
		if p = strings.ToUpper(strings.TrimSpace(p)); p != "" {
			have[p] = true
		}
	}
	if len(have) != len(parts) {
		return false
	}
	for _, p := range parts {
		if !have[strings.ToUpper(p)] {
			return false
		}
	}
	return true
}

// bodyIfThere is rawBody for a place the server may simply not have: it
// answers with the empty string instead of ending the test. A PUT the
// server accepted and then kept nothing of is a fact about the server.
func bodyIfThere(ctx context.Context, t *testing.T, c *dav.Client, path string) string {
	t.Helper()
	rc, _, err := c.Get(ctx, path, nil)
	if err != nil {
		t.Logf("GET %s: %v", path, err)
		return ""
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Logf("read %s: %v", path, err)
		return ""
	}
	return string(b)
}

// mustValue checks that a property carries a value, whatever parameters the
// server hung on it. Google writes back ADR;TYPE=HOME,PREF and
// EMAIL;TYPE=PREF where Carrel sent ADR;TYPE=home and a bare EMAIL: the
// parameters are the server's business, the value is the person's.
func mustValue(t *testing.T, step, got, prop, value string) {
	t.Helper()
	prop = strings.ToUpper(prop)
	for _, l := range strings.Split(strings.ReplaceAll(got, "\r\n", "\n"), "\n") {
		name, val, ok := strings.Cut(strings.TrimSpace(l), ":")
		if !ok {
			continue
		}
		name = strings.ToUpper(strings.SplitN(name, ";", 2)[0])
		if name == prop && strings.EqualFold(strings.TrimSpace(val), value) {
			return
		}
	}
	t.Errorf("%s: no %s with %q in\n%s", step, prop, value, got)
}

// mustAddress checks that every part written into an address is still in
// it. A server may move the parts between components — iCloud folds the box
// and the flat into the street with newlines — and the address is the same
// address; a person loses nothing and Carrel reports nothing.
func mustAddress(t *testing.T, step, got string, parts ...string) {
	t.Helper()
	var line string
	for _, l := range strings.Split(strings.ReplaceAll(got, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(l)), "ADR") {
			line = l
		}
	}
	if line == "" {
		t.Errorf("%s: no address in\n%s", step, got)
		return
	}
	for _, p := range parts {
		if !strings.Contains(line, p) {
			t.Errorf("%s: the address has no %q, only %q", step, p, strings.TrimSpace(line))
		}
	}
}

// noticeText renders notices with what is inside their diagnostic, which
// is where a far server's own words are. Printing a page with %+v shows
// Details as a pointer address — the run of 22.09.2026 against Xandikos
// said «the server did not answer as expected» four times and never once
// said what the server had actually answered.
func noticeText(notices []view.Notice) string {
	if len(notices) == 0 {
		return "(no notices)"
	}
	var out []string
	for _, n := range notices {
		line := string(n.Role) + ": " + string(n.Title) + " — " + string(n.Body)
		if n.Details != nil {
			line += " | " + string(n.Details.Summary)
			for _, l := range n.Details.Lines {
				line += " | " + string(l)
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n    ")
}
