package web_test

import (
	"net/url"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// review Р-9, Label: the account is a label, not an identifier
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
	page = decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections?op=add", sess)))
	if r := formRow(*page.Add, "label"); r == nil || r.Value != "" {
		t.Errorf("add form label row = %+v", r)
	}

	edit := actionByID(card.Actions, "connection.edit")
	u, _ := url.Parse(edit.Href)
	form := *decode[view.ConnectionsPage](t, h.do(get(u.RequestURI(), sess))).Add
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

// Two connections never share a label (owner's decision 24.09.2026,
// s-profile-11): the label is how sources are told apart. What is compared
// is the label a connection acts under — without one, the server's host —
// so a second connection to the same server without a label is refused too.
// Letter case does not tell two stamps apart, and does not here. Edit is
// held to the same.
func TestATakenLabelIsRefused(t *testing.T) {
	work := davtest.CalendarHome + "work/"
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	h, sess := davHarness(t)
	connect := func(label string) view.ConnectionsPage {
		rec := h.do(postForm("/app/settings/connections", sess, url.Values{
			"label": {label}, "server": {srv.URL}, "login": {"alice"}, "password": {"davpass"},
		}))
		if rec.Code == 303 {
			return view.ConnectionsPage{}
		}
		return decode[view.ConnectionsPage](t, rec)
	}
	key, _ := sess.DataKey()
	count := func() int {
		blob, _ := h.users.Catalog(sess.AccountID(), key)
		return len(blob.Connections)
	}

	if page := connect("nextcloud"); page.Add != nil || count() != 1 {
		t.Fatalf("the first connection was not made: %+v", page.Add)
	}
	page := connect("Nextcloud")
	sameCaret(t, "s-profile-11", "a taken label", *page.Add)
	if r := formRow(*page.Add, "label"); r == nil || r.Error != "There is already a connection called “Nextcloud”. Choose another label." {
		t.Errorf("a taken label: %+v", r)
	}
	if count() != 1 {
		t.Fatalf("a taken label was saved: %d connections", count())
	}
	if connect(""); count() != 2 {
		t.Fatalf("the server's own host, free, was refused: %d", count())
	}
	page = connect("")
	if r := formRow(*page.Add, "label"); r == nil || r.Error == "" || count() != 2 {
		t.Errorf("a second connection to the same server without a label: %+v, %d", r, count())
	}

	// Edit: the second connection may not take the first one's label.
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	second := blob.Connections[1]
	v := url.Values{"op": {"edit"}, "account": {second.ID}, "label": {"NEXTCLOUD"}, "server": {second.Server}, "login": {"alice"}}
	edited := decode[view.ConnectionsPage](t, h.do(postForm("/app/settings/connections", sess, v)))
	if r := formRow(*edited.Add, "label"); r == nil || r.Error == "" {
		t.Errorf("edit to a taken label: %+v", r)
	}
	blob, _ = h.users.Catalog(sess.AccountID(), key)
	if blob.Connections[1].Label != "" {
		t.Errorf("the taken label was saved by Edit: %q", blob.Connections[1].Label)
	}
	// Its own label is not taken by itself.
	v.Set("label", "")
	if add := decode[view.ConnectionsPage](t, h.do(postForm("/app/settings/connections", sess, v))).Add; add != nil {
		if r := formRow(*add, "label"); r != nil && r.Error != "" {
			t.Errorf("a connection's own label was refused: %+v", r)
		}
	}
}
