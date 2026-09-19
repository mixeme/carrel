package web_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// connections-tail.md: a connection could be made and never taken out. The
// card has Disconnect, and it opens the sheet of "what exactly goes" — the
// brother of the collection's delete sheet (s-profile-9), without an export
// or an archive: nothing is deleted on the server. Confirmed, the connection
// leaves Carrel whole and its links stop opening.
func TestDisconnectAConnection(t *testing.T) {
	work := davtest.CalendarHome + "work/"
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	srv.Seed(work+"meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nSUMMARY:Review\nEND:VEVENT"))
	f := connectFixture(t, srv, work, nil)
	tick(t, f, sealed.Selection{Kind: view.KindEvent, AccountID: f.id, Path: work})
	key, _ := f.sess.DataKey()

	// A published link of this connection, and a device that sees it.
	conn, _ := catalogOf(t, f).Connection(f.id)
	link, err := f.h.users.Publish(f.sess.AccountID(), key, conn, user.PublishTarget{AccountID: f.id, Kind: "calendar", Col: work, Title: "Work"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	card := cardOf(t, decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))), f.id)
	act := actionByID(card.Actions, "connection.disconnect")
	if act == nil || act.Method != "GET" {
		t.Fatalf("card actions = %+v, want Disconnect", card.Actions)
	}
	u, _ := url.Parse(act.Href)
	sheet := decode[view.ConnectionRemoveSheet](t, f.h.do(get(u.RequestURI(), f.sess)))
	host, _ := url.Parse(conn.Server)
	if !strings.Contains(string(sheet.Subject), host.Host) {
		t.Errorf("subject = %q", sheet.Subject)
	}
	if !strings.Contains(string(sheet.Warning), "nothing is deleted on") {
		t.Errorf("warning = %q", sheet.Warning)
	}
	refs := strings.Join(textLines(sheet.References), "|")
	if !strings.Contains(refs, "1 collection") || !strings.Contains(refs, "Published link") {
		t.Errorf("references = %q", refs)
	}
	confirm := actionByID(sheet.Actions, "connection.disconnect.confirm")
	cancel := actionByID(sheet.Actions, "connection.disconnect.cancel")
	if confirm == nil || !confirm.Danger || cancel == nil {
		t.Fatalf("sheet actions = %+v", sheet.Actions)
	}

	before := auditCount(f.h, "connection.remove")
	resp := f.h.do(postHref(f, confirm.Href))
	if resp.Code != 303 || resp.Header().Get("Location") != "/app/settings/connections" {
		t.Fatalf("disconnect: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	blob := catalogOf(t, f)
	if len(blob.Connections) != 0 || len(blob.Collections) != 0 || len(blob.Selected) != 0 {
		t.Errorf("after disconnect the catalog holds %+v", blob)
	}
	if _, ok := f.h.users.OpenPublished(link); ok {
		t.Error("the link of the connection still opens")
	}
	if auditCount(f.h, "connection.remove") != before+1 {
		t.Error("the journal has no connection.remove line")
	}
	// The collection is gone from the app, and the server still has it.
	if rec := f.h.do(get(f.path("calendar"), f.sess)); rec.Code != 404 {
		t.Errorf("the collection still opens: %d", rec.Code)
	}
	if _, _, ok := f.srv.Object(work + "meet.ics"); !ok {
		t.Error("Disconnect deleted something on the server")
	}
	// The sheet of a connection that is no longer there is not a sheet.
	if rec := f.h.do(get(u.RequestURI(), f.sess)); rec.Code != 404 {
		t.Errorf("the sheet of a gone connection: %d", rec.Code)
	}
}

func textLines(in []view.Text) []string {
	out := make([]string, 0, len(in))
	for _, t := range in {
		out = append(out, string(t))
	}
	return out
}
