package web_test

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// Test only says what it found over its trace (Typecase's note 232,
// s-profile-39, -40): passed — what was found and that nothing is kept
// until Connect; failed — where discovery stopped, what to do, that nothing
// was saved. Asked into the dialog's zone (the live layer), the answer is a
// fragment of that zone, and an empty password is said there too: the field
// is not drawn again.
func TestTestOnlySaysWhatItFound(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	f := connectFixture(t, srv, searchBook, nil)
	other := searchServer(t)
	other.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "Office"})
	const test = "/app/settings/connections/new/test"

	outcome := func(p view.ConnectionsPage) view.Notice {
		t.Helper()
		if len(p.Shell.Notices) == 0 {
			t.Fatalf("no outcome: %+v", p)
		}
		return p.Shell.Notices[0]
	}

	form := url.Values{"label": {"Baikal"}, "server": {other.URL}, "login": {other.Login}, "password": {other.Password}}
	p := decode[view.ConnectionsPage](t, f.h.do(postForm(test, f.sess, form)))
	n := outcome(p)
	if n.Title != "Test passed:" || n.Role != view.NoticeInfo ||
		!strings.HasPrefix(string(n.Body), "Carrel can sign in and found 1 collection. Nothing is saved until you press Connect.") || p.Trace == nil {
		t.Errorf("passed: %+v, trace %+v", n, p.Trace)
	}

	form.Set("password", "wrong")
	p = decode[view.ConnectionsPage](t, f.h.do(postForm(test, f.sess, form)))
	n = outcome(p)
	if n.Title != "Test failed:" || n.Role != view.NoticeAlert ||
		!strings.HasSuffix(string(n.Body), ": the server refused the login and password. Check them and test again. Nothing was saved.") || p.Trace == nil {
		t.Errorf("failed: %+v, trace %+v", n, p.Trace)
	}

	// Into the zone: the fragment of connect-answer.
	form.Set("password", "")
	req := postForm(test, f.sess, form)
	req.Header.Set(render.HeaderView, render.ViewFragment)
	req.Header.Set(render.HeaderTarget, "connect-answer")
	rec := f.h.do(req)
	var frag struct {
		Target string               `json:"target"`
		Body   view.ConnectionsPage `json:"body"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &frag); err != nil {
		t.Fatalf("%d %s: %v", rec.Code, rec.Body.String(), err)
	}
	if frag.Target != "connect-answer" {
		t.Errorf("the answer's target = %q", frag.Target)
	}
	if n := outcome(frag.Body); n.Title != "Test not run:" || n.Role != view.NoticeAlert {
		t.Errorf("an empty password, into the zone: %+v", n)
	}
}
