package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// publishFixture: a book with two people and a calendar with an event and a
// note, on one server whose password must never leave the envelope.
func publishFixture(t *testing.T) davFixture {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "dav-secret-pw"
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchBook+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEMAIL:ada@example.com\nEND:VCARD\n")
	srv.Seed(searchBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")
	srv.Seed(searchWork+"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup\nDTSTART:20260810T090000Z\nLOCATION:Room 4\nEND:VEVENT"))
	srv.Seed(searchWork+"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260801T000000Z\nSUMMARY:Reading list\nDTSTART;VALUE=DATE:20260808\nDESCRIPTION:# CRDT\\n\\nKleppmann\\, chapter 5\nEND:VJOURNAL"))
	return connectFixture(t, srv, searchBook, nil)
}

func (f davFixture) publishHref(segment, col, uid string) string {
	q := url.Values{"k": {segment}, "a": {f.id}, "c": {col}}
	if uid != "" {
		q.Set("uid", uid)
	}
	return "/app/publish?" + q.Encode()
}

// publishLink makes a link the way the sheet does and returns its address.
func publishLink(t *testing.T, f davFixture, href string, forever bool) string {
	t.Helper()
	sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
	lives := url.Values{}
	if forever {
		lives.Set("lives", "never")
	}
	got := decode[view.PublishSheet](t, f.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), f.sess, lives)))
	if !strings.HasPrefix(string(got.Link), "/p/") {
		t.Fatalf("no link: %+v", got)
	}
	return string(got.Link)
}

// anon is a request from someone with no session at all.
func anon(path string) *http.Request { return httptest.NewRequest(http.MethodGet, path, nil) }

func publicPage(t *testing.T, f davFixture, link string) view.PublishedPage {
	t.Helper()
	return decode[view.PublishedPage](t, f.h.do(anon(link)))
}

// S-25.1: the sheet names the thing and the lifetime and makes nothing; the
// link is shown once and only its hash is kept; the envelope keeps the
// password sealed; the page is read live, without a session, read-only, and
// leads nowhere into the owner's Carrel.
func TestPublishedBookIsReadLive(t *testing.T) {
	f := publishFixture(t)
	href := f.publishHref("contacts", searchBook, "")
	sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
	// The lifetime is a field — 7 days for contacts unless changed
	// (s-publish-3), 30 for the rest (s-publish-2): the sheet says nothing
	// of it before the link exists.
	if !strings.Contains(string(sheet.Target), "People") || sheet.Expires != "" || sheet.Link != "" {
		t.Fatalf("sheet = %+v", sheet)
	}
	if lives := rowNamed(t, sheet.Form, "lives"); lives.Value != "7" || len(lives.Options) != 4 || lives.Options[3].Value != "never" {
		t.Errorf("link lives for = %+v", lives)
	}
	if len(sheet.Actions) != 2 || sheet.Actions[0].Label != "Create link" {
		t.Errorf("actions = %+v", sheet.Actions)
	}
	if len(journaled(f, "publish.create")) != 0 {
		t.Fatal("opening the sheet published")
	}

	link := publishWith(t, f, href, "download") // the file is asked for (s-publish-3)
	token := strings.TrimPrefix(link, "/p/")
	state, _ := json.Marshal(f.h.st.Snapshot())
	if strings.Contains(string(state), token) || strings.Contains(string(state), "dav-secret-pw") {
		t.Fatal("the state holds the secret or the password in the clear")
	}
	if len(journaled(f, "publish.create")) != 1 {
		t.Error("publishing is not in the journal")
	}

	f.srv.Seed(searchBook+"cy.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:cy\nFN:Cy Twombly\nEND:VCARD\n")
	rec := f.h.do(anon(link))
	page := decode[view.PublishedPage](t, rec)
	if !page.ReadOnly || page.Kind != view.KindContact || len(page.Contacts) != 3 {
		t.Fatalf("page = %+v", page)
	}
	if rec.Header().Get("Referrer-Policy") != "no-referrer" || !strings.Contains(rec.Header().Get("Cache-Control"), "no-store") {
		t.Errorf("headers = %v", rec.Header())
	}
	if body := rec.Body.String(); strings.Contains(body, "/app/") || strings.Contains(body, f.id) || strings.Contains(body, "alice") {
		t.Errorf("the public page leads into the owner's Carrel:\n%s", body)
	}

	dl := f.h.do(anon(link + "/download"))
	if dl.Code != http.StatusOK || !strings.Contains(dl.Body.String(), "FN:Cy Twombly") || !strings.Contains(dl.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("download: %d %v", dl.Code, dl.Header())
	}
}

// One record, three kinds: an event with its fields and body, a note as a
// document, a contact with its fields.
func TestPublishedRecords(t *testing.T) {
	f := publishFixture(t)
	ev := publicPage(t, f, publishLink(t, f, f.publishHref("calendar", searchWork, "e1"), false))
	if ev.Title != "Standup" || ev.Kind != view.KindEvent {
		t.Fatalf("event = %+v", ev)
	}
	room := false
	for _, fl := range ev.Fields {
		room = room || fl.Value == "Room 4"
	}
	if !room {
		t.Errorf("event fields = %+v", ev.Fields)
	}
	note := publicPage(t, f, publishLink(t, f, f.publishHref("notes", searchWork, "n1"), true))
	if note.Note == nil || note.Note.Title != "Reading list" || !note.Note.ReadOnly || len(note.Note.Rendered) == 0 {
		t.Errorf("note = %+v", note.Note)
	}
	ada := publicPage(t, f, publishWith(t, f, f.publishHref("contacts", searchBook, "ada"), "emails"))
	if ada.Title != "Ada Lovelace" || len(ada.Fields) == 0 || len(ada.Contacts) != 0 {
		t.Errorf("contact = %+v", ada)
	}
}

// Revoked is never-was: the same 404 as a link that never existed. The list
// shows its owner the link, kept in the blob (owner's decision 19.09.2026);
// Revoke goes by the link's ID, not its secret; someone else cannot revoke it.
func TestRevokedIsNeverWas(t *testing.T) {
	f := publishFixture(t)
	link := publishLink(t, f, f.publishHref("calendar", searchWork, ""), false)
	cal := publicPage(t, f, link)
	if cal.Range == nil || len(cal.Range.Presets) != 3 || !strings.HasPrefix(cal.Range.Presets[0].Href, link+"?from=") {
		t.Errorf("calendar range = %+v", cal.Range)
	}

	list := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess)))
	if len(list.Rows) != 1 || list.Rows[0].Title != "Work" || string(list.Rows[0].Link) != link {
		t.Fatalf("links = %+v", list.Rows)
	}
	revoke := actionHref(t, list.Rows[0].Actions, "publish.revoke")
	if strings.Contains(revoke, strings.TrimPrefix(link, "/p/")) {
		t.Fatalf("Revoke carries the secret: %s", revoke)
	}

	adminID := f.h.st.Snapshot().Users[0].ID
	f.h.invite(adminID, "mallory", "password456")
	if rec := f.h.do(postForm(revoke, f.h.login("mallory", "password456"), url.Values{})); rec.Code != http.StatusNotFound {
		t.Errorf("someone else revoked: %d", rec.Code)
	}
	if rec := f.h.do(postForm(revoke, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("revoke: %d", rec.Code)
	}
	gone := f.h.do(anon(link))
	never := f.h.do(anon("/p/" + strings.Repeat("A", 43)))
	if gone.Code != http.StatusNotFound || never.Code != http.StatusNotFound || gone.Body.String() != never.Body.String() {
		t.Errorf("revoked %d %q, never %d %q", gone.Code, gone.Body.String(), never.Code, never.Body.String())
	}
	if f.h.do(anon(link+"/download")).Code != http.StatusNotFound {
		t.Error("a revoked link still downloads")
	}
	if len(f.h.st.Snapshot().Published) != 0 {
		t.Error("the revoked link's envelope is still in the state")
	}
}

// A server that does not answer is a notice to the reader — not a 500, and
// not the server's name.
func TestPublishedSourceDown(t *testing.T) {
	f := publishFixture(t)
	link := publishLink(t, f, f.publishHref("contacts", searchBook, ""), false)
	f.srv.Refuse("REPORT", http.StatusServiceUnavailable, "down")
	rec := f.h.do(anon(link))
	page := decode[view.PublishedPage](t, rec)
	if !hasNotice(page.Notices, "The source did not answer") || len(page.Actions) != 0 {
		t.Errorf("page = %+v", page)
	}
	if strings.Contains(rec.Body.String(), "127.0.0.1") {
		t.Errorf("the reader is told the server:\n%s", rec.Body.String())
	}
}

// Publishing is reading: a record page offers it, a read-only collection
// too. A collection is published from its menu in Connections only (owner's
// decision, to-typecase task 12): the head of its list draws New, Import,
// Export and Print (s-contacts-1, s-agenda-1) — Print is the client's.
func TestPublishIsOffered(t *testing.T) {
	f := publishFixture(t)
	card := decode[view.ContactPage](t, f.h.do(get(f.path("contacts")+"/ada", f.sess)))
	if actionHref(t, menuActions(card.Menu), "publish") != f.publishHref("contacts", searchBook, "ada") {
		t.Errorf("record menu = %+v", card.Menu)
	}
	// Print of a list is the list with print=1 (the client's plan, decision
	// 4): the contacts carry it, the agenda not yet.
	head := func(frame string) []string {
		acts := mockupFrame(t, frame)
		acts = acts[strings.Index(acts, `class="tc-head"`):]
		acts = acts[strings.Index(acts, `class="tc-acts"`):]
		acts = acts[:strings.Index(acts, `</div>`)]
		out := []string{}
		for _, m := range frameButton.FindAllStringSubmatch(acts, -1) {
			if label := frameText(m[1]); label != "Print" || frame == "s-contacts-1" || frame == "s-agenda-1" {
				out = append(out, label)
			}
		}
		return out
	}
	list := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	if got, want := labels(list.Bar.Actions), head("s-contacts-1"); !equalStrings(got, want) {
		t.Errorf("contacts head = %q, want (s-contacts-1) %q", got, want)
	}
	agenda := decode[view.AgendaPage](t, f.h.do(get("/app/calendar/"+f.id+"/"+url.PathEscape(searchWork), f.sess)))
	if got, want := labels(agenda.Bar.Actions), head("s-agenda-1"); !equalStrings(got, want) {
		t.Errorf("agenda head = %q, want (s-agenda-1) %q", got, want)
	}
	if rec := f.h.do(get("/app/publish?k=contacts&a=nope&c=x", f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("a foreign target: %d", rec.Code)
	}
}

// «Link lives for» (s-publish-1): the lifetime chosen is the lifetime made —
// 7, 14 or 30 days, or none.
func TestPublishLinkLivesForWhatWasChosen(t *testing.T) {
	f := publishFixture(t)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	// Both clocks: the list of links drops an expired one by the service's
	// own, and on the wall clock a 7-day link made on 24.09 at noon was gone
	// by 01.10 at noon — the test went red on its own that afternoon.
	f.h.srv.SetClock(now)
	f.h.users.SetClock(now)
	href := f.publishHref("contacts", searchBook, "ada")
	for _, c := range []struct {
		lives string
		days  int // 0: no expiry
	}{{"7", 7}, {"14", 14}, {"30", 30}, {"never", 0}, {"", 30}} {
		sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
		made := decode[view.PublishSheet](t, f.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), f.sess, url.Values{"lives": {c.lives}})))
		if made.Link == "" {
			t.Fatalf("lives %q: no link", c.lives)
		}
		got := f.h.users.PublishedLinks(f.sess.AccountID())[0].ExpiresAt
		switch {
		case c.days == 0 && got != nil:
			t.Errorf("lives %q: expires %v, want never", c.lives, got)
		case c.days != 0 && (got == nil || got.Sub(now) != time.Duration(c.days)*24*time.Hour):
			t.Errorf("lives %q: expires %v, want in %d days", c.lives, got, c.days)
		}
	}
}
