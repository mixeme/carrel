package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

func noticeTitled(notices []view.Notice, title string) *view.Notice {
	for i := range notices {
		if string(notices[i].Title) == title {
			return &notices[i]
		}
	}
	return nil
}

func catalogOf(t *testing.T, f davFixture) sealed.Blob {
	t.Helper()
	key, _ := f.sess.DataKey()
	blob, ok := f.h.users.Catalog(f.sess.AccountID(), key)
	if !ok {
		t.Fatal("catalog does not open")
	}
	return blob
}

// tick stores selections the way the combined view will (S-14).
func tick(t *testing.T, f davFixture, sel ...sealed.Selection) {
	t.Helper()
	key, _ := f.sess.DataKey()
	blob := catalogOf(t, f)
	blob.Selected = append(blob.Selected, sel...)
	snap := f.h.st.Snapshot()
	sealedBlob, err := sealed.Seal(key, blob, snap.UserByID(f.sess.AccountID()).Login)
	if err != nil {
		t.Fatal(err)
	}
	err = f.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.UserByID(f.sess.AccountID()).SealedBlob = sealedBlob
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// postHref posts an action the way the client does: its href, query and all.
func postHref(f davFixture, href string) *http.Request {
	u, err := url.Parse(href)
	if err != nil {
		panic(err)
	}
	return postForm(u.EscapedPath(), f.sess, u.Query())
}

func rediscoverPage(t *testing.T, f davFixture) view.ConnectionsPage {
	t.Helper()
	return decode[view.ConnectionsPage](t, f.h.do(postForm("/app/settings/connections", f.sess, url.Values{"op": {"rediscover"}, "account": {f.id}})))
}

func auditCount(h *harness, what string) int {
	n := 0
	for _, e := range h.st.Snapshot().Audit {
		if e.What == what {
			n++
		}
	}
	return n
}

// S-6.1, Typecase's note 103 (the owner, 19.10.2026): Update collection list
// takes what the server holds at once — a new collection comes in unticked,
// marked new, with a line above the cards; a vanished one is marked gone and
// keeps its tick; one that comes back is the same entry again; records are
// not touched.
func TestUpdateCollectionListTakesTheServer(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	work := davtest.CalendarHome + "work/"
	team := davtest.CalendarHome + "team/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	f := connectFixture(t, srv, work, nil)
	tick(t, f, sealed.Selection{Kind: view.KindEvent, AccountID: f.id, Path: work})

	srv.AddCalendar(caldav.Calendar{Path: team, Name: "Team"})
	srv.RemoveCollection(work)

	page := rediscoverPage(t, f)
	var added *view.Notice
	for i := range page.Notices {
		if strings.Contains(string(page.Notices[i].Body), "1 new collection — «Team». Tick it to see it in the rail.") {
			added = &page.Notices[i]
		}
	}
	if added == nil {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if g := noticeTitled(page.Notices, "Gone from the server"); g == nil || !strings.Contains(string(g.Body), "Work") {
		t.Errorf("notices = %+v", page.Notices)
	}
	var fresh *view.CollectionRow
	for i, row := range page.Accounts[0].Collections {
		if string(row.Path) == team {
			fresh = &page.Accounts[0].Collections[i]
		}
		if string(row.Path) == work {
			t.Error("a gone collection is still a row of the card")
		}
	}
	if fresh == nil || !fresh.New || fresh.Tick != view.TickOff {
		t.Fatalf("team's row = %+v", fresh)
	}
	blob := catalogOf(t, f)
	if c, ok := blob.Collection(f.id, team); !ok || c.Gone || c.Name != "Team" {
		t.Errorf("team = %+v (found %v)", c, ok)
	}
	if c, ok := blob.Collection(f.id, work); !ok || !c.Gone {
		t.Errorf("work = %+v (found %v), want kept and gone", c, ok)
	}
	if len(blob.Selected) != 1 || blob.Selected[0].Path != work {
		t.Errorf("selected = %+v, want the tick on Work kept", blob.Selected)
	}
	snap := f.h.st.Snapshot()
	last := snap.Audit[len(snap.Audit)-1]
	if last.What != "connection.rediscover" || last.Detail != f.id+" +1 -1" {
		t.Errorf("audit = %+v", last)
	}
	// New only on the answer of the update.
	again := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	for _, row := range again.Accounts[0].Collections {
		if row.New {
			t.Errorf("%s still new on the next page", row.Label)
		}
	}
	if rec := f.h.do(get("/app/calendar/"+f.id+"/"+url.PathEscape(work), f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("gone collection: %d, want 404", rec.Code)
	}

	// Work comes back: the same entry, the server's name.
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work again"})
	rediscoverPage(t, f)
	entries := 0
	for _, c := range catalogOf(t, f).CollectionsOf(f.id) {
		if c.Path == work {
			entries++
			if c.Gone || c.Name != "Work again" {
				t.Errorf("work = %+v, want back with the server's name", c)
			}
		}
	}
	if entries != 1 {
		t.Errorf("work entries = %d, want the same one", entries)
	}

	page = rediscoverPage(t, f)
	if len(page.Notices) == 0 || !strings.Contains(string(page.Notices[0].Body), "the list is the same as on the server") {
		t.Errorf("notices = %+v", page.Notices)
	}
}

// A server that does not answer has deleted nothing (C-8.2): a failed
// re-discovery changes nothing and says so on the card.
func TestRediscoveryFailureChangesNothing(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	work := davtest.CalendarHome + "work/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	f := connectFixture(t, srv, work, nil)

	srv.RemoveCollection(work)
	srv.Password = "changed"
	page := rediscoverPage(t, f)
	n := noticeTitled(page.Notices, "Could not connect.")
	if n == nil || !strings.Contains(string(n.Body), "Nothing was changed.") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	// The server answered 401, and the notice says what that is: the login
	// and password were refused (the owner's word of 10.10.2026 — said as
	// it is, and nothing guessed beyond it). The steps stay in the trace.
	if string(n.Body) != "The server refused the login and password. Nothing was changed." {
		t.Errorf("the notice of a refused password says %q", n.Body)
	}
	if n.Details == nil || !strings.Contains(fmt.Sprint(n.Details.Lines), "401") {
		t.Errorf("the trace of a refused password: %+v", n.Details)
	}
	if card := page.Accounts[0]; card.State != view.CollectionUnavailable || card.Trace == nil {
		t.Errorf("card state %q, trace %v", card.State, card.Trace)
	}
	if c, _ := catalogOf(t, f).Collection(f.id, work); c.Gone {
		t.Error("a failed re-discovery marked a collection gone")
	}
	if auditCount(f.h, "connection.rediscover") != 0 {
		t.Error("a failed re-discovery was journaled as applied")
	}
	rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{"op": {"rediscover"}, "account": {"a-nobody"}}))
	if rec.Code != http.StatusNotFound {
		t.Errorf("foreign connection: %d, want 404", rec.Code)
	}
}

// S-10.7: the stored password leaves only inside this session's unlock window,
// only on request for that card, and every release is journaled.
func TestConnectionPasswordOnlyInsideUnlockWindow(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	people := davtest.AddressBookHome + "people/"
	srv.AddAddressBook(carddav.AddressBook{Path: people, Name: "People"})
	f := connectFixture(t, srv, people, nil)
	reveal := "/app/settings/connections?reveal=" + f.id

	rec := f.h.do(get(reveal, f.sess))
	if strings.Contains(rec.Body.String(), "davpass") {
		t.Fatal("password shown without the unlock sheet")
	}
	page := decode[view.ConnectionsPage](t, rec)
	// The sheet names the connection (Нс18, v0.2.292; until then a bare
	// /app/settings/unlock).
	if pw := page.Accounts[0].Password; pw.Shown || pw.Reveal.Href != "/app/settings/unlock?for="+f.id {
		t.Errorf("password = %+v, want the sheet", pw)
	}

	if rec := f.h.do(fragment(postForm("/app/settings/unlock", f.sess, url.Values{"password": {"password123"}}), "sheet")); rec.Code != http.StatusOK {
		t.Fatalf("unlock: %d", rec.Code)
	}
	rec = f.h.do(get("/app/settings/connections", f.sess))
	if strings.Contains(rec.Body.String(), "davpass") {
		t.Fatal("an open window alone put the password on the page")
	}
	page = decode[view.ConnectionsPage](t, rec)
	if href := page.Accounts[0].Password.Reveal.Href; href != reveal {
		t.Errorf("reveal href = %q, want %q", href, reveal)
	}
	if auditCount(f.h, "connection.password.reveal") != 0 {
		t.Fatal("journaled a release that did not happen")
	}

	rec = f.h.do(get(reveal, f.sess))
	page = decode[view.ConnectionsPage](t, rec)
	if pw := page.Accounts[0].Password; !pw.Shown || pw.Value != "davpass" || pw.Reveal.Label != "Hide password" {
		// s-profile-4: the shown password's button is «Hide» (Нс29).
		t.Errorf("password = %+v", pw)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	snap := f.h.st.Snapshot()
	if last := snap.Audit[len(snap.Audit)-1]; last.What != "connection.password.reveal" || last.Detail != f.id {
		t.Errorf("audit = %+v", last)
	}

	other := f.h.login("owner", "password123")
	if strings.Contains(f.h.do(get(reveal, other)).Body.String(), "davpass") {
		t.Error("another session of the same person inherited the window")
	}
	if strings.Contains(f.h.do(get("/app/settings/connections?reveal=a-nobody", f.sess)).Body.String(), "davpass") {
		t.Error("a foreign reveal showed a password")
	}
	if n := auditCount(f.h, "connection.password.reveal"); n != 1 {
		t.Errorf("reveal lines = %d, want 1", n)
	}
}
