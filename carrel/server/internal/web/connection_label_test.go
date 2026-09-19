package web_test

import (
	"net/url"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// connections-tail.md, Label: the account is a label, not an identifier
// (C-4), and until now it was always the host. The label names the card and
// every "account · collection" stamp; Edit changes it; empty, the host.
func TestConnectionLabel(t *testing.T) {
	work := davtest.CalendarHome + "work/"
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	h, sess := davHarness(t)
	if rec := h.do(postForm("/app/settings/connections", sess, url.Values{
		"label": {"  nextcloud   home "}, "server": {srv.URL}, "login": {"alice"}, "password": {"davpass"},
	})); rec.Code != 303 {
		t.Fatalf("connect: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := sess.DataKey()
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	f := davFixture{srv: srv, h: h, sess: sess, id: blob.Connections[0].ID, col: work}
	if blob.Connections[0].Label != "nextcloud home" {
		t.Errorf("label = %q", blob.Connections[0].Label)
	}

	page := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections", sess)))
	card := cardOf(t, page, f.id)
	if card.Title != "nextcloud home" {
		t.Errorf("card title = %q", card.Title)
	}
	if r := formRow(page.Add, "label"); r == nil || r.Value != "" {
		t.Errorf("add form label row = %+v", r)
	}

	edit := actionByID(card.Actions, "connection.edit")
	u, _ := url.Parse(edit.Href)
	form := decode[view.ConnectionsPage](t, h.do(get(u.RequestURI(), sess))).Add
	if r := formRow(form, "label"); r == nil || r.Value != "nextcloud home" {
		t.Errorf("edit form label = %+v", r)
	}
	save, _ := url.Parse(actionByID(form.Actions, "connection.edit.save").Href)
	v := save.Query()
	v.Set("label", "")
	v.Set("server", blob.Connections[0].Server)
	v.Set("login", "alice")
	decode[view.ConnectionsPage](t, h.do(postForm(save.EscapedPath(), sess, v)))
	blob, _ = h.users.Catalog(sess.AccountID(), key)
	if blob.Connections[0].Label != "" {
		t.Errorf("label after emptying = %q", blob.Connections[0].Label)
	}
	card = cardOf(t, decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections", sess))), f.id)
	if want := u.Host; card.Title == "nextcloud home" || card.Title == "" {
		t.Errorf("card title = %q, want the host again (%s)", card.Title, want)
	}
}
