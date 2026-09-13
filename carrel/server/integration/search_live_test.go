//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for S-15: a search through the request layer against a live server —
// CardDAV any-of in one REPORT, CalDAV one REPORT a property, and the match
// done by the server's own collation, which davtest does not imitate.
func TestRadicaleSearch(t *testing.T) {
	runSearch(t, radicaleEndpoint(t), radicaleUser, radicalePass)
}

func TestBaikalSearch(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runSearch(t, endpoint, user, pass)
}

func (a *liveApp) getFragment(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: a.sess.ID()})
	req.Header.Set("Carrel-View", "fragment")
	req.Header.Set("Carrel-Target", "poll")
	rec := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rec, req)
	return rec
}

func runSearch(t *testing.T, endpoint, user, pass string) {
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

	suffix := fmt.Sprintf("carrel-s15-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	calPath := strings.TrimSuffix(res.CalendarHome, "/") + "/" + suffix + "-cal/"
	bookPath := strings.TrimSuffix(res.AddressBookHome, "/") + "/" + suffix + "-book/"
	liveRequest(ctx, t, endpoint, user, pass, "MKCALENDAR", calPath, `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel S-15</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/><C:comp name="VTODO"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)
	t.Cleanup(func() { _ = client.Delete(context.Background(), calPath, "") })
	liveRequest(ctx, t, endpoint, user, pass, "MKCOL", bookPath, `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel S-15</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)
	t.Cleanup(func() { _ = client.Delete(context.Background(), bookPath, "") })

	put := func(path, body, typ string) {
		t.Helper()
		if _, err := client.PutCreate(ctx, path, dav.Typed(strings.NewReader(body), typ)); err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}
	}
	card := func(uid, family, given, extra string) {
		put(bookPath+uid+".vcf", "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:"+uid+"\r\nFN:"+family+" "+given+"\r\nN:"+family+";"+given+";;;\r\n"+
			extra+"END:VCARD\r\n", "text/vcard; charset=utf-8")
	}
	component := func(uid, kind, props string) {
		put(calPath+uid+".ics", "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Carrel test//EN\r\nBEGIN:"+kind+"\r\nUID:"+uid+
			"\r\nDTSTAMP:20260101T000000Z\r\n"+props+"END:"+kind+"\r\nEND:VCALENDAR\r\n", "text/calendar; charset=utf-8")
	}
	card("ada-"+suffix, "Lovelace", "Ada", "EMAIL:ada@carrel.test\r\n")
	card("anna-"+suffix, "Ковалёва", "Анна", "")
	component("launch-"+suffix, "VEVENT", "DTSTART:20261010T100000Z\r\nDTEND:20261010T110000Z\r\nSUMMARY:Launch plan\r\n")
	component("pad-"+suffix, "VEVENT", "DTSTART:20261011T100000Z\r\nDTEND:20261011T110000Z\r\nSUMMARY:Visit\r\nLOCATION:Launch site\r\n")
	component("meet-"+suffix, "VTODO", "SUMMARY:Встреча с Ковалёвой\r\n")

	app := newLiveApp(t)
	redirected(t, "connect", app.post("/app/settings/connections", url.Values{
		"server": {endpoint}, "login": {user}, "password": {pass},
	}))
	// Only the two collections of this run are ticked: the account may hold
	// others whose records would match too.
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	for _, c := range blob.Collections {
		if c.Path == calPath || c.Path == bookPath {
			for _, k := range c.Kinds {
				blob.Selected = append(blob.Selected, sealed.Selection{Kind: k, AccountID: c.AccountID, Path: c.Path})
			}
		}
	}
	if len(blob.Selected) < 3 {
		t.Fatalf("ticks = %+v; catalog %+v", blob.Selected, blob.Collections)
	}
	sealedBlob, err := sealed.Seal(key, blob, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.UserByID(app.sess.AccountID()).SealedBlob = sealedBlob
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}

	find := func(q string) []string {
		t.Helper()
		href := "/app/search?q=" + url.QueryEscape(q)
		page := liveDecode[view.SearchPage](t, "search "+q, app.get(href))
		if page.Poll.TaskID == "" {
			t.Fatalf("search %q started nothing: %+v %+v", q, page.Poll, page.Notices)
		}
		deadline := time.Now().Add(60 * time.Second)
		for {
			bar := liveDecode[struct {
				Body view.PollBar `json:"body"`
			}](t, "poll "+q, app.getFragment("/app/search/progress/"+page.Poll.TaskID+"/poll")).Body
			if !bar.Running {
				for _, sp := range bar.Sources {
					if sp.State != view.SourceDone && sp.State != view.SourceEmpty {
						t.Fatalf("search %q: %s is %s: %s", q, sp.Stamp, sp.State, sp.Message)
					}
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("search %q still running: %+v", q, bar)
			}
			time.Sleep(50 * time.Millisecond)
		}
		var titles []string
		for _, row := range liveDecode[view.SearchPage](t, "result "+q, app.get(href)).Results {
			titles = append(titles, string(row.Title))
		}
		sort.Strings(titles)
		return titles
	}

	if got := strings.Join(find("launch"), "|"); got != "Launch plan|Visit" {
		t.Errorf("launch (summary and location, other case) = %q", got)
	}
	if got := strings.Join(find("LOVELACE"), "|"); got != "Lovelace Ada" {
		t.Errorf("LOVELACE (a contact, other case) = %q", got)
	}
	if got := strings.Join(find("Ковал"), "|"); got != "Встреча с Ковалёвой|Ковалёва Анна" {
		t.Errorf("Ковал (Cyrillic as written) = %q", got)
	}
	t.Logf("%s: ковал (Cyrillic, other case) finds %q", endpoint, find("ковал"))
}
