package web_test

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

func formRow(f view.Form, name string) *view.FormRow {
	for i := range f.Sets {
		for j := range f.Sets[i].Rows {
			if f.Sets[i].Rows[j].Name == name {
				return &f.Sets[i].Rows[j]
			}
		}
	}
	return nil
}

// Without Edit a changed address or password meant deleting the connection
// and making it again, losing the ticks and the duplicate decisions (U,
// /app/settings/connections). Edit keeps the a-…, saves only after discovery
// answers, and offers the difference of the catalog as re-discovery does
// (S-6.1): a collection at the same path keeps its tick.
func TestEditConnectionAfterPasswordAndAddressChange(t *testing.T) {
	work := davtest.CalendarHome + "work/"
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	f := connectFixture(t, srv, work, nil)
	tick(t, f, sealed.Selection{Kind: view.KindEvent, AccountID: f.id, Path: work})

	card := cardOf(t, decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))), f.id)
	edit := actionByID(card.Actions, "connection.edit")
	if edit == nil || edit.Method != "GET" {
		t.Fatalf("card actions = %+v, want Edit", card.Actions)
	}
	u, _ := url.Parse(edit.Href)
	page := decode[view.ConnectionsPage](t, f.h.do(get(u.RequestURI(), f.sess)))
	if r := formRow(page.Add, "server"); r == nil || string(r.Value) != catalogOf(t, f).Connections[0].Server {
		t.Errorf("server row = %+v, want the stored address", r)
	}
	if r := formRow(page.Add, "login"); r == nil || r.Value != "alice" {
		t.Errorf("login row = %+v", r)
	}
	if r := formRow(page.Add, "password"); r == nil || r.Value != "" || r.Hint == "" {
		t.Errorf("password row = %+v, want it empty with a hint that empty keeps it", r)
	}
	save := actionByID(page.Add.Actions, "connection.edit.save")
	if save == nil || save.Method != "POST" {
		t.Fatalf("edit form actions = %+v", page.Add.Actions)
	}
	post := func(v url.Values) view.ConnectionsPage {
		t.Helper()
		su, _ := url.Parse(save.Href)
		for k, vs := range su.Query() {
			v[k] = vs
		}
		return decode[view.ConnectionsPage](t, f.h.do(postForm(su.Path, f.sess, v)))
	}

	// The password changed on the server; a wrong one saves nothing.
	srv.Password = "changed"
	before := catalogOf(t, f).Connections[0]
	page = post(url.Values{"server": {before.Server}, "login": {"alice"}, "password": {"wrong"}})
	if noticeTitled(page.Shell.Notices, "Could not connect") == nil {
		t.Errorf("notices = %+v, want the failure with its trace", page.Shell.Notices)
	}
	if got := catalogOf(t, f).Connections[0]; !reflect.DeepEqual(got, before) {
		t.Error("a failed edit changed the connection")
	}
	if formRow(page.Add, "login") == nil || actionByID(page.Add.Actions, "connection.edit.save") == nil {
		t.Error("a failed edit does not give the edit form back")
	}

	page = post(url.Values{"server": {before.Server}, "login": {"alice"}, "password": {"changed"}})
	blob := catalogOf(t, f)
	if len(blob.Connections) != 1 || blob.Connections[0].ID != f.id || blob.Connections[0].Password != "changed" {
		t.Fatalf("connections = %+v, want the same ID with the new password", blob.Connections)
	}
	if card := cardOf(t, page, f.id); card.Trace == nil || !card.Trace.OK {
		t.Error("the card does not carry the trace of the check that saved it")
	}

	// The server moved: the same calendar and one more at the new address.
	moved := davtest.New(t)
	moved.Login, moved.Password = "alice", "changed"
	moved.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	moved.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "home/", Name: "Home"})
	page = post(url.Values{"server": {moved.URL}, "login": {"alice"}, "password": {""}})
	blob = catalogOf(t, f)
	if blob.Connections[0].ID != f.id || blob.Connections[0].Password != "changed" {
		t.Fatalf("connection = %+v: an empty password keeps the stored one", blob.Connections[0])
	}
	if _, ok := blob.Collection(f.id, davtest.CalendarHome+"home/"); ok {
		t.Error("a collection joined the catalog without the person applying it")
	}
	if len(blob.Selected) != 1 || blob.Selected[0].Path != work {
		t.Errorf("selected = %+v, want the tick kept", blob.Selected)
	}
	n := noticeTitled(page.Shell.Notices, "Collections changed on the server")
	if n == nil {
		t.Fatalf("notices = %+v, want the difference offered", page.Shell.Notices)
	}
	if rec := f.h.do(get("/app/calendar/"+f.id+"/"+url.PathEscape(work), f.sess)); rec.Code != 200 {
		t.Errorf("the ticked calendar at the new address: %d", rec.Code)
	}
	if auditCount(f.h, "connection.edit") != 2 {
		t.Errorf("connection.edit lines = %d, want 2", auditCount(f.h, "connection.edit"))
	}
}
