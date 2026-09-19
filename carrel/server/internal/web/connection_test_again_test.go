package web_test

import (
	"net/url"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

func cardOf(t *testing.T, page view.ConnectionsPage, id string) view.ConnectionCard {
	t.Helper()
	for _, c := range page.Accounts {
		if c.AccountID == id {
			return c
		}
	}
	t.Fatalf("no card for %s in %+v", id, page.Accounts)
	return view.ConnectionCard{}
}

func actionByID(actions []view.Action, id string) *view.Action {
	for i := range actions {
		if actions[i].ID == id {
			return &actions[i]
		}
	}
	return nil
}

// C-6 ConnectionCard: a saved connection is checked too, not only a new one
// — servers break later (U, /app/settings/connections). Test runs discovery
// with the stored credentials and writes nothing; its trace stays on the card
// for the session (C-9.1), and a card whose last check failed offers Test
// again.
func TestSavedConnectionIsTestedAndTestedAgain(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	srv.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work"})
	f := connectFixture(t, srv, davtest.CalendarHome+"work/", nil)

	card := cardOf(t, decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))), f.id)
	test := actionByID(card.Actions, "connection.test")
	if test == nil || test.Label != "Test" || test.Method != "POST" {
		t.Fatalf("card actions = %+v, want Test", card.Actions)
	}
	if card.Trace != nil {
		t.Error("a card nobody tested carries a trace")
	}
	before := catalogOf(t, f)
	journal := len(f.h.st.Snapshot().Audit)

	card = cardOf(t, decode[view.ConnectionsPage](t, f.h.do(postHref(f, test.Href))), f.id)
	if card.Trace == nil || !card.Trace.OK || len(card.Trace.Steps) == 0 {
		t.Fatalf("trace = %+v, want the steps of a passed check", card.Trace)
	}
	if card.State != view.CollectionOK {
		t.Errorf("state = %q after a passed check", card.State)
	}

	srv.Password = "changed"
	card = cardOf(t, decode[view.ConnectionsPage](t, f.h.do(postHref(f, test.Href))), f.id)
	if card.Trace == nil || card.Trace.OK {
		t.Fatalf("trace = %+v, want a failed check", card.Trace)
	}
	if card.State != view.CollectionUnavailable {
		t.Errorf("state = %q after a failed check, want unavailable", card.State)
	}
	again := actionByID(card.Actions, "connection.test")
	if again == nil || again.Label != "Test again" {
		t.Errorf("card actions = %+v, want Test again", card.Actions)
	}

	// The trace belongs to the session: a later GET shows it still.
	card = cardOf(t, decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))), f.id)
	if card.Trace == nil || card.Trace.OK {
		t.Error("the trace of the last check is gone from the card on the next visit")
	}

	after := catalogOf(t, f)
	if len(after.Connections) != 1 || after.Connections[0] != before.Connections[0] || len(after.Collections) != len(before.Collections) {
		t.Error("a check changed the catalog")
	}
	if n := len(f.h.st.Snapshot().Audit); n != journal {
		t.Errorf("a check wrote %d journal lines; it saves nothing", n-journal)
	}

	// Another person's connection is not tested.
	rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{"op": {"test"}, "account": {"a-NOSUCHONE0"}}))
	if rec.Code != 404 {
		t.Errorf("unknown connection: %d, want 404", rec.Code)
	}
}
