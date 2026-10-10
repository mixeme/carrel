package web_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// davHarness is a harness whose guard lets DAV requests reach httptest on
// the loopback address.
func davHarness(t *testing.T) (*harness, *session.Session) {
	t.Helper()
	h := newHarness(t)
	h.cfg.DAVSSRFAllowlist = "127.0.0.1"
	h.rebuild()
	h.admin("owner", "password123")
	return h, h.login("owner", "password123")
}

// Step 1 of S-11…S-26: POST /app/settings/connections runs discovery and
// writes the catalog — homes, colour, max-resource-size, rights — and the
// catalog is what {col} is checked against afterwards.
func TestAddConnectionWritesCatalog(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	work := davtest.CalendarHome + "work/"
	people := davtest.AddressBookHome + "people/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work", MaxResourceSize: 2048})
	srv.SetColour(work, "#3366CCFF")
	srv.AddAddressBook(carddav.AddressBook{Path: people, Name: "People", ReadOnly: true})

	h, sess := davHarness(t)
	rec := h.do(postForm("/app/settings/connections", sess, url.Values{
		"server": {srv.URL}, "login": {"alice"}, "password": {"davpass"},
	}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST: %d %s", rec.Code, rec.Body.String())
	}

	key, _ := sess.DataKey()
	blob, ok := h.users.Catalog(sess.AccountID(), key)
	if !ok || len(blob.Connections) != 1 {
		t.Fatalf("catalog = %+v", blob)
	}
	conn := blob.Connections[0]
	if !strings.HasPrefix(conn.ID, "a-") || conn.Login != "alice" || conn.Password != "davpass" ||
		conn.CalendarHome != davtest.CalendarHome || conn.AddressBookHome != davtest.AddressBookHome || conn.FileRoot != "" {
		t.Errorf("connection = %+v", conn)
	}
	w, ok := blob.Collection(conn.ID, work)
	if !ok || w.Colour != "#3366CCFF" || w.MaxResourceSize != 2048 || w.Name != "Work" || w.ChangeTag == "" {
		t.Errorf("work = %+v (found %v)", w, ok)
	}
	if p, ok := blob.Collection(conn.ID, people); !ok || !p.ReadOnly || !p.Has(view.KindContact) {
		t.Errorf("people = %+v (found %v)", p, ok)
	}
	if len(blob.Selected) != 0 {
		t.Errorf("selected = %+v; a new tick is not set by itself", blob.Selected)
	}

	snap := h.st.Snapshot()
	if n := snap.UserByLogin("owner").ConnCount; n != 1 {
		t.Errorf("ConnCount = %d", n)
	}
	last := snap.Audit[len(snap.Audit)-1]
	if last.What != "connection.add" || last.Detail != conn.ID {
		t.Errorf("audit = %+v", last)
	}
	host := strings.TrimPrefix(srv.URL, "http://")
	for _, e := range snap.Audit {
		if strings.Contains(e.Detail, host) || strings.Contains(e.Whom, host) {
			t.Errorf("the plain journal names the server: %+v", e)
		}
	}

	rec = h.do(get("/app/settings/connections", sess))
	if strings.Contains(rec.Body.String(), "davpass") {
		t.Fatal("the connections page shows the password without the unlock sheet")
	}
	var page view.ConnectionsPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Accounts) != 1 || len(page.Accounts[0].Collections) != 2 {
		t.Fatalf("accounts = %+v", page.Accounts)
	}
	for _, row := range page.Accounts[0].Collections {
		switch string(row.Path) {
		case work:
			if row.Colour != "#3366cc" {
				t.Errorf("work colour = %q, want the server's without alpha", row.Colour)
			}
		case people:
			if row.Colour == "" || !row.ReadOnly {
				t.Errorf("people row = %+v, want a derived colour and read-only", row)
			}
		}
	}

	if rec := h.do(get("/app/calendar/"+conn.ID+"/"+url.PathEscape(work), sess)); rec.Code != http.StatusOK {
		t.Errorf("catalog collection: %d", rec.Code)
	}
	if rec := h.do(get("/app/calendar/"+conn.ID+"/"+url.PathEscape(davtest.CalendarHome+"other/"), sess)); rec.Code != http.StatusNotFound {
		t.Errorf("collection outside the catalog: %d, want 404", rec.Code)
	}
}

// C-9.1: a failed connection says at which step it stopped, carries every
// step, and saves nothing. A DAV refusal is a screen, not a 500.
func TestAddConnectionFailureSavesNothing(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "right"
	h, sess := davHarness(t)

	rec := h.do(postForm("/app/settings/connections", sess, url.Values{
		"server": {srv.URL}, "login": {"alice"}, "password": {"wrong"},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST: %d, want the page again", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"wrong"`) {
		t.Error("the entered password is echoed back")
	}
	var page view.ConnectionsPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	var failure *view.Notice
	for i := range page.Notices {
		if page.Notices[i].Title == "Could not connect" {
			failure = &page.Notices[i]
		}
	}
	if failure == nil || failure.Details == nil || len(failure.Details.Lines) < 3 {
		t.Fatalf("notices = %+v", page.Notices)
	}
	// A wrong password is said as what the server said — it refused the
	// login and password (the owner's word of 10.10.2026) —, and the step
	// that failed stays in the trace.
	if string(failure.Body) != "The server refused the login and password. Nothing was saved." {
		t.Errorf("body = %q, want the refusal said as it is", failure.Body)
	}
	if trace := fmt.Sprint(failure.Details.Lines); !strings.Contains(trace, "principal") || !strings.Contains(trace, "401") {
		t.Errorf("trace = %q, want the step that failed and the answer", trace)
	}
	if got := rowNamed(t, *page.Add, "server").Value; got != text(srv.URL) {
		t.Errorf("server field = %q, want what was entered", got)
	}

	key, _ := sess.DataKey()
	if blob, _ := h.users.Catalog(sess.AccountID(), key); len(blob.Connections) != 0 || len(blob.Collections) != 0 {
		t.Errorf("a failed discovery saved %+v", blob)
	}
}

func TestAddConnectionRejectsAddressWithoutScheme(t *testing.T) {
	h, sess := davHarness(t)
	rec := h.do(postForm("/app/settings/connections", sess, url.Values{
		"server": {"dav.example.com"}, "login": {"alice"}, "password": {"x"},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST: %d", rec.Code)
	}
	var page view.ConnectionsPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if row := rowNamed(t, *page.Add, "server"); row.Error == "" {
		t.Errorf("server row = %+v, want an error", row)
	}
}

func text(s string) view.Text { return view.Text(s) }
