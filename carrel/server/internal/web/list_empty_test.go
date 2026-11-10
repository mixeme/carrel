package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// An empty list says so in its frame's words and offers the ways to fill
// it, of every kind (U-6.0; «kinds are equal»): «This list is empty.» ·
// «Add a task, or import a file of them.» (states-21), «This notebook is
// empty.» · «Write a note, or import a file of them.» (states-28), «This
// book is empty.» · «Add a contact, or import a file of them.» (states-10).
func TestAnEmptyListSaysSoOfEveryKind(t *testing.T) {
	app := newMockupApp(t)
	nc := app.servers["nextcloud"]
	for _, slug := range []string{"rabota", "dnevnik"} {
		for _, p := range nc.Paths(davtest.CalendarHome + slug + "/") {
			if body, _, ok := nc.Object(p); ok && (strings.Contains(body, "BEGIN:VTODO") || strings.Contains(body, "BEGIN:VJOURNAL")) {
				nc.Unseed(p)
			}
		}
	}
	for _, tc := range []struct {
		segment, slug, title, body string
		empty                      func(*harness) *view.Notice
	}{
		{"tasks", "rabota", "This list is empty.", "Add a task, or import a file of them.", func(h *harness) *view.Notice {
			return decode[view.TasksPage](t, h.do(get(app.listPath(t, "tasks", "rabota"), app.sess))).Empty
		}},
		{"notes", "dnevnik", "This notebook is empty.", "Write a note, or import a file of them.", func(h *harness) *view.Notice {
			return decode[view.NotesPage](t, h.do(get(app.listPath(t, "notes", "dnevnik"), app.sess))).Empty
		}},
		{"contacts", "rassylki", "This book is empty.", "Add a contact, or import a file of them.", func(h *harness) *view.Notice {
			return decode[view.ContactsListPage](t, h.do(get(app.listPath(t, "contacts", "rassylki"), app.sess))).Empty
		}},
	} {
		n := tc.empty(app.h)
		if n == nil {
			t.Errorf("%s: an empty list says nothing", tc.slug)
			continue
		}
		if string(n.Title) != tc.title || string(n.Body) != tc.body {
			t.Errorf("%s: %q · %q, want %q · %q", tc.slug, n.Title, n.Body, tc.title, tc.body)
		}
		ids := []string{}
		for _, a := range n.Actions {
			ids = append(ids, a.ID)
		}
		if strings.Join(ids, " ") != "record.new collection.import" {
			t.Errorf("%s: actions %q, want New and Import", tc.slug, ids)
		}
	}
}

// A list this account may only read says so by its title, of every kind:
// the feed «Праздники РФ» in the tasks (states-23), «Общие с семьёй» in the
// notes (states-30), «Команда» in the contacts (states-4).
func TestAReadOnlyListSaysSoOfEveryKind(t *testing.T) {
	app := newMockupApp(t)
	feed := ""
	key, _ := app.sess.DataKey()
	blob, _ := app.h.users.Catalog(app.sess.AccountID(), key)
	for _, s := range blob.Subscriptions {
		if s.Name == "Праздники РФ" {
			feed = "/app/tasks/" + s.ID + "/" + s.FeedCollection().Path
		}
	}
	if feed == "" {
		t.Fatal("the set has no «Праздники РФ»")
	}
	if p := decode[view.TasksPage](t, app.h.do(get(feed, app.sess))); !p.ReadOnly {
		t.Errorf("the feed's tasks are not read-only")
	} else if p.Subtitle != "Subscriptions · 2 tasks · a feed can only be read" {
		// A feed is named by the section of the rail it stands in, and
		// why it cannot be written is its kind, not a server's grant
		// (states-23).
		t.Errorf("the feed's line = %q", p.Subtitle)
	}
	if p := decode[view.NotesPage](t, app.h.do(get(app.listPath(t, "notes", "zapiski-obshchie"), app.sess))); !p.ReadOnly {
		t.Errorf("«Общие с семьёй» is not read-only")
	}
	if p := decode[view.TasksPage](t, app.h.do(get(app.listPath(t, "tasks", "dela"), app.sess))); p.ReadOnly {
		t.Errorf("«Дела» is read-only")
	}
	if p := decode[view.ContactsListPage](t, app.h.do(get(app.listPath(t, "contacts", "komanda"), app.sess))); !p.ReadOnly {
		t.Errorf("«Команда» is not read-only")
	}
}
