package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
)

type traceFragment struct {
	Body view.DiagnosticTrace `json:"body"`
}

func checkServer(t *testing.T) *davtest.Server {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	srv.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work", SupportedComponentSet: []string{"VEVENT", "VTODO"}})
	srv.AddAddressBook(carddav.AddressBook{Path: davtest.AddressBookHome + "people/", Name: "People"})
	return srv
}

// Test beside the connection form runs discovery and saves nothing; a wrong
// password is a trace that says where it stopped, not a 500.
func TestConnectionTestShowsTheTrace(t *testing.T) {
	srv := checkServer(t)
	h, sess := davHarness(t)
	ok := decode[traceFragment](t, h.do(fragment(postForm("/app/settings/connections/test", sess, url.Values{
		"server": {srv.URL}, "login": {"alice"}, "password": {"davpass"}}), "main"))).Body
	if !ok.OK || len(ok.Steps) < 2 {
		t.Errorf("trace = %+v", ok)
	}
	bad := decode[traceFragment](t, h.do(fragment(postForm("/app/settings/connections/test", sess, url.Values{
		"server": {srv.URL}, "login": {"alice"}, "password": {"wrong"}}), "main"))).Body
	if bad.OK || len(bad.Steps) == 0 {
		t.Errorf("wrong password = %+v", bad)
	}
	key, _ := sess.DataKey()
	if blob, _ := h.users.Catalog(sess.AccountID(), key); len(blob.Connections) != 0 {
		t.Error("Test saved a connection")
	}
}

// U-6.13: the validator's second stage writes only with the box ticked,
// creates, reads back and deletes, leaves nothing, and goes to the journal.
func TestAdminDavValidator(t *testing.T) {
	srv := checkServer(t)
	h, sess := davHarness(t)
	form := url.Values{"server": {srv.URL}, "login": {"alice"}, "password": {"davpass"}}

	page := decode[view.AdminDavPage](t, h.do(postForm("/admin/dav?op=test", sess, form)))
	if page.Trace == nil || !page.Trace.OK || page.Full != nil {
		t.Fatalf("test = %+v", page)
	}
	exercise := actionHref(t, page.Actions, "admin.dav.exercise")

	page = decode[view.AdminDavPage](t, h.do(postForm(exercise, sess, form)))
	if page.Full != nil || profileRow(page.Form, "agree").Error == "" {
		t.Fatalf("ran without consent: %+v", page.Full)
	}
	form.Set("agree", "on")
	page = decode[view.AdminDavPage](t, h.do(postForm(exercise, sess, form)))
	if page.Full == nil || !page.Full.OK {
		t.Fatalf("full = %+v", page.Full)
	}
	done := map[string]int{}
	for _, row := range page.Full.Rows {
		done[string(row.Result)]++
	}
	if done["ok"] != 9 || done["failed"] != 0 {
		t.Errorf("rows = %+v", page.Full.Rows)
	}
	if n := len(srv.Paths(davtest.CalendarHome+"work/")) + len(srv.Paths(davtest.AddressBookHome+"people/")); n != 0 {
		t.Errorf("left on the server: %d", n)
	}
	snap := h.st.Snapshot()
	if e := snap.Audit[len(snap.Audit)-1]; e.What != "dav.exercise" {
		t.Errorf("journal = %+v", e)
	}
}
