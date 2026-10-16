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

// Note 70 of Typecase (task 164; s-profile-22, -23): the question waits in
// the dialog of a connection that has links — hidden, named by the fields
// whose change asks it (FormRow.Changed: Server URL and Username). With a
// script it shows while one of them differs from what came, without one it
// comes after the first Save, as before.
func TestTheLinksQuestionWaitsHidden(t *testing.T) {
	work := davtest.CalendarHome + "work/"
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	f := connectFixture(t, srv, work, nil)
	q := url.Values{"op": {"edit"}, "account": {f.id}}
	if row := formRow(*addForm(t, decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections?"+q.Encode(), f.sess)))), "links"); row != nil {
		t.Fatalf("a connection without links has the question: %+v", row)
	}
	publishLink(t, f, f.publishHref("calendar", work, ""), false)
	row := formRow(*addForm(t, decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections?"+q.Encode(), f.sess)))), "links")
	if row == nil || !row.Hidden || !reflect.DeepEqual(row.Changed, []string{"server", "login"}) || row.Error != "" || len(row.Options) != 2 {
		t.Fatalf("the question in waiting = %+v", row)
	}
	// Asked after Save, it shows; a field that came shown stays so.
	page := decode[view.ConnectionsPage](t, f.h.do(postForm("/app/settings/connections?"+q.Encode(), f.sess,
		url.Values{"server": {catalogOf(t, f).Connections[0].Server}, "login": {"bob"}, "password": {""}})))
	if row := formRow(*addForm(t, page), "links"); row == nil || row.Hidden || row.Error == "" {
		t.Errorf("the question asked = %+v", row)
	}
}
