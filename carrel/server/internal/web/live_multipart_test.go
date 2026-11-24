package web_test

import (
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// The live layer sends a form as the browser's FormData — multipart — with
// its token in X-CSRF-Token. Found on the phone's rail panel, 24.11.2026:
// unticking one book unticked every book («0 of 4 books»). With the token in
// the header nothing read the body, a handler's ParseForm skips a multipart
// body, and the tick was an empty selection. A form so sent is read as one
// sent plainly.
func TestALiveTickReadsItsMultipartForm(t *testing.T) {
	srv := searchServer(t)
	people := davtest.AddressBookHome + "people/"
	office := davtest.AddressBookHome + "office/"
	srv.AddAddressBook(carddav.AddressBook{Path: people, Name: "People"})
	srv.AddAddressBook(carddav.AddressBook{Path: office, Name: "Office"})
	f := connectFixture(t, srv, people, nil)
	f.col = people
	both := f.path("contacts")
	f.col = office
	tickRail(t, f.h, f.sess, "contacts", both, f.path("contacts"))
	if got := selected(t, f.h, f.sess, view.KindContact); len(got) != 2 {
		t.Fatalf("ticked = %v", got)
	}
	req := multipartPost("/app/sources/contacts", f.sess, map[string]string{"toggle": f.path("contacts"), "back": "/app/contacts"})
	req.Header.Set("X-CSRF-Token", f.sess.CSRFToken())
	req.Header.Set("Carrel-View", "fragment")
	req.Header.Set("Carrel-Target", "rail")
	if rec := f.h.do(req); rec.Code != 200 {
		t.Fatalf("live tick: %d %s", rec.Code, rec.Body.String())
	}
	if got := selected(t, f.h, f.sess, view.KindContact); len(got) != 1 {
		t.Errorf("after unticking Office the ticked are %v, want People alone", got)
	}
}
