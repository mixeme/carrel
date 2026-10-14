package web_test

import (
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// Б40 of the audit of 11.10.2026 (the owner's decision of 12.10.2026): an
// edit that gives a connection another server or another login is asked
// what becomes of its published links — they keep opening, now on what the
// connection points at, or they are revoked. Nothing is saved until it is
// answered. Another password, a label, a fingerprint move the links along
// without a word.
func TestConnectionEditAsksAboutItsLinks(t *testing.T) {
	work := davtest.CalendarHome + "work/"
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	f := connectFixture(t, srv, work, nil)
	link := publishLink(t, f, f.publishHref("calendar", work, ""), false)
	if rec := f.h.do(anon(link)); rec.Code != 200 {
		t.Fatalf("the link does not open: %d", rec.Code)
	}
	post := func(v url.Values) view.ConnectionsPage {
		t.Helper()
		q := url.Values{"op": {"edit"}, "account": {f.id}}
		return decode[view.ConnectionsPage](t, f.h.do(postForm("/app/settings/connections?"+q.Encode(), f.sess, v)))
	}
	server := catalogOf(t, f).Connections[0].Server

	// Another password: saved, nothing asked, the link opens.
	srv.Password = "changed"
	page := post(url.Values{"server": {server}, "login": {"alice"}, "password": {"changed"}})
	if formRow(*addForm(t, page), "links") != nil || catalogOf(t, f).Connections[0].Password != "changed" {
		t.Fatalf("a new password asked about the links or was not saved")
	}
	if rec := f.h.do(anon(link)); rec.Code != 200 {
		t.Errorf("after a new password the link: %d", rec.Code)
	}

	// Another login: asked, nothing saved.
	srv.Login = "bob"
	before := catalogOf(t, f).Connections[0]
	page = post(url.Values{"server": {server}, "login": {"bob"}, "password": {""}})
	row := formRow(*addForm(t, page), "links")
	if row == nil || row.Kind != view.InputRadio || len(row.Options) != 2 || row.Error == "" ||
		!strings.Contains(string(row.Hint), "1 published link uses this connection") {
		t.Fatalf("the question = %+v", row)
	}
	if got := catalogOf(t, f).Connections[0]; !reflect.DeepEqual(got, before) {
		t.Error("an unanswered question saved the connection")
	}
	if r := formRow(*addForm(t, page), "login"); r == nil || r.Value != "bob" {
		t.Errorf("what was typed is lost: %+v", r)
	}

	// Kept: saved, the link opens on what the connection points at.
	page = post(url.Values{"server": {server}, "login": {"bob"}, "password": {""}, "links": {"keep"}})
	if catalogOf(t, f).Connections[0].Login != "bob" {
		t.Fatalf("kept: the connection was not saved")
	}
	if rec := f.h.do(anon(link)); rec.Code != 200 {
		t.Errorf("a kept link: %d", rec.Code)
	}

	// Revoked: saved, the link opens no more.
	srv.Login = "alice"
	post(url.Values{"server": {server}, "login": {"alice"}, "password": {""}, "links": {"revoke"}})
	if catalogOf(t, f).Connections[0].Login != "alice" {
		t.Fatalf("revoked: the connection was not saved")
	}
	if rec := f.h.do(anon(link)); rec.Code != 404 {
		t.Errorf("a revoked link: %d", rec.Code)
	}

	// No links left: another login asks nothing.
	srv.Login = "bob"
	page = post(url.Values{"server": {server}, "login": {"bob"}, "password": {""}})
	if formRow(*addForm(t, page), "links") != nil || catalogOf(t, f).Connections[0].Login != "bob" {
		t.Error("a connection without links asked about them")
	}
}

// addForm is the connection form of the page — an empty one when the page
// shows none.
func addForm(t *testing.T, p view.ConnectionsPage) *view.Form {
	t.Helper()
	if p.Add == nil {
		return &view.Form{}
	}
	return p.Add
}
