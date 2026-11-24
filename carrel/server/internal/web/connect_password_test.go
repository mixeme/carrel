package web_test

import (
	"net/url"
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/view"
)

// The owner, 24.11.2026, with a screenshot: Connect after Test only said
// «Could not connect. The server refused the login and password.» — Test
// only does not give the password back, the field was empty, and Carrel
// asked the server with no password. An empty password without «The server
// asks no password» is said at its field, and no server is asked; after
// Test only the field has the caret and says to type it again.
func TestConnectAsksForTheMissingPassword(t *testing.T) {
	const missing = "Type the password, or tick «The server asks no password»."
	const again = "Type it again to connect: Carrel does not send a password back."
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	f := connectFixture(t, srv, searchBook, nil)
	other := searchServer(t)
	other.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "Office"})

	row := func(p view.ConnectionsPage) view.FormRow {
		t.Helper()
		if p.Add == nil {
			t.Fatal("no connection form")
		}
		for _, r := range p.Add.Sets[0].Rows {
			if r.Name == "password" {
				return r
			}
		}
		t.Fatal("no password field")
		return view.FormRow{}
	}
	form := url.Values{"label": {"Baikal"}, "server": {other.URL}, "login": {other.Login}, "password": {""}}
	for _, path := range []string{"/app/settings/connections/new", "/app/settings/connections/new/test"} {
		before := len(other.Requests())
		p := decode[view.ConnectionsPage](t, f.h.do(postForm(path, f.sess, form)))
		if r := row(p); string(r.Error) != missing || !r.Focus {
			t.Errorf("%s without a password: password field %+v, want %q with the caret", path, r, missing)
		}
		if len(p.Shell.Notices) != 0 {
			t.Errorf("%s without a password: notices %+v", path, p.Shell.Notices)
		}
		if n := len(other.Requests()) - before; n != 0 {
			t.Errorf("%s without a password asked the server %d times", path, n)
		}
		for _, r := range p.Add.Sets[0].Rows {
			if r.Name == "label" && string(r.Value) != "Baikal" {
				t.Errorf("%s: the label came back %q", path, r.Value)
			}
		}
	}
	form.Set("password", other.Password)
	p := decode[view.ConnectionsPage](t, f.h.do(postForm("/app/settings/connections/new/test", f.sess, form)))
	if r := row(p); string(r.Hint) != again || !r.Focus || r.Value != "" {
		t.Errorf("after Test only: password field %+v, want empty with %q and the caret", r, again)
	}
}
