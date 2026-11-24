package web_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// The owner's decision of 24.11.2026: the collections of an account just
// connected are ticked in their sections — before it, Connect left every
// rail empty, the combined lists, the counts and Refresh with nothing to
// ask, and the person had to tick each by hand. An empty selection made by
// the person stays empty (S-14).
func TestANewConnectionIsTicked(t *testing.T) {
	srv := searchServer(t)
	book := davtest.AddressBookHome + "people/"
	cal := davtest.CalendarHome + "work/"
	srv.AddAddressBook(carddav.AddressBook{Path: book, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: cal, Name: "Work", SupportedComponentSet: []string{"VEVENT", "VTODO", "VJOURNAL"}})
	h, sess := davHarness(t)
	if rec := h.do(postForm("/app/settings/connections", sess, url.Values{
		"server": {srv.URL}, "login": {srv.Login}, "password": {srv.Password},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("connect: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := sess.DataKey()
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	f := davFixture{srv: srv, h: h, sess: sess, id: blob.Connections[0].ID}
	for kind, path := range map[view.Kind]string{view.KindContact: book, view.KindEvent: cal, view.KindTask: cal, view.KindNote: cal} {
		got := selected(t, f.h, f.sess, kind)
		if len(got) != 1 || got[0] != f.id+" "+path {
			t.Errorf("%s ticked after Connect: %v, want %s", kind, got, path)
		}
	}
}
