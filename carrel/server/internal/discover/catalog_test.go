package discover

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

func loopbackGuard() *dav.Guard {
	return dav.NewGuard(&conf.Config{
		DAVConnectTimeout:   5 * time.Second,
		DAVRequestTimeout:   10 * time.Second,
		DAVMaxResponseBytes: 1 << 20,
		DAVMaxRedirects:     5,
		DAVSSRFAllowlist:    "127.0.0.1",
	})
}

// The catalog needs what S-6 says discovery takes and the struct used to drop:
// both homes, colour, max-resource-size, the change tag, rights.
func TestDiscoveryFillsTheCatalog(t *testing.T) {
	srv := davtest.New(t)
	work := davtest.CalendarHome + "work/"
	plain := davtest.CalendarHome + "plain/"
	people := davtest.AddressBookHome + "people/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work", SupportedComponentSet: []string{"VTODO", "VEVENT"}, MaxResourceSize: 1 << 20})
	srv.AddCalendar(caldav.Calendar{Path: plain, Name: "Plain", ReadOnly: true})
	srv.SetColour(work, "#FF0000FF")
	srv.AddAddressBook(carddav.AddressBook{Path: people, Name: "People", ReadOnly: true, MaxResourceSize: 4096})

	res, trace, err := Run(t.Context(), loopbackGuard(), srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !trace.OK {
		t.Fatalf("trace: %+v", trace.Steps)
	}
	if res.CalendarHome != davtest.CalendarHome || res.AddressBookHome != davtest.AddressBookHome {
		t.Errorf("homes = %q, %q", res.CalendarHome, res.AddressBookHome)
	}
	byPath := make(map[string]Collection)
	for _, c := range res.Collections {
		byPath[c.Path] = c
	}
	w, ok := byPath[work]
	if !ok {
		t.Fatalf("collections = %+v, want %s", res.Collections, work)
	}
	if w.Colour != "#FF0000FF" || w.MaxResourceSize != 1<<20 || w.ChangeTag == "" || w.DisplayName != "Work" {
		t.Errorf("work = %+v", w)
	}
	if !slices.Equal(w.Kinds, []view.Kind{view.KindEvent, view.KindTask}) {
		t.Errorf("work kinds = %v, want event, task in that order", w.Kinds)
	}
	if w.ReadOnly {
		t.Error("work is writable")
	}
	if p := byPath[plain]; p.Colour != "" || !p.ReadOnly {
		t.Errorf("plain = %+v, want no colour (none sent) and read-only", p)
	}
	b := byPath[people]
	if !b.ReadOnly || b.MaxResourceSize != 4096 || b.ChangeTag == "" || !slices.Equal(b.Kinds, []view.Kind{view.KindContact}) {
		t.Errorf("people = %+v", b)
	}
	if !traceHas(trace, stepProperties, view.CheckOK) {
		t.Error("the properties step belongs on the trace")
	}
}

// docs/servers.md, SV-30: a server with a trash bin keeps a deleted calendar at
// its address, among the children of the home, and calls it a calendar to any
// request that does not ask about the deletion. Discovery asks, in the
// PROPFIND it already makes, and leaves the trashed one out — otherwise a
// collection the person deleted comes back at the next discovery (S-6). The
// trace says how many were left out, because a collection quietly missing is
// its own kind of lie.
func TestDiscoverySkipsWhatTheServerPutInItsTrashBin(t *testing.T) {
	srv := davtest.New(t)
	live := davtest.CalendarHome + "live/"
	gone := davtest.CalendarHome + "gone/"
	srv.AddCalendar(caldav.Calendar{Path: live, Name: "Live"})
	srv.AddCalendar(caldav.Calendar{Path: gone, Name: "Deleted last week"})
	srv.Trash(gone, "2026-09-17T21:53:30+00:00")

	res, trace, err := Run(t.Context(), loopbackGuard(), srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Collections {
		if c.Path == gone {
			t.Errorf("the trashed calendar is in the catalog: %+v", c)
		}
	}
	var found bool
	for _, c := range res.Collections {
		found = found || c.Path == live
	}
	if !found {
		t.Fatalf("collections = %+v, want %s", res.Collections, live)
	}
	var said string
	for _, st := range trace.Steps {
		if st.Name == stepProperties {
			said = string(st.Detail)
		}
	}
	if !strings.Contains(said, "trash bin") {
		t.Errorf("the trace does not say what was left out: %q", said)
	}
}

// docs/servers.md, SV-29: a server that sends no displayname at all. What the
// collection is, is its name then — the last segment of the address is a
// technical word to the person whose contacts are in it (iCloud's only book is
// at .../carddavhome/card/). Two nameless ones of a kind in one home are told
// apart by that segment, because two identical rows are worse.
func TestACollectionWithoutANameIsNamedByItsKind(t *testing.T) {
	srv := davtest.New(t)
	only := davtest.AddressBookHome + "card/"
	srv.AddAddressBook(carddav.AddressBook{Path: only})
	cal := davtest.CalendarHome + "events/"
	srv.AddCalendar(caldav.Calendar{Path: cal})

	res, _, err := Run(t.Context(), loopbackGuard(), srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, c := range res.Collections {
		byPath[c.Path] = c.DisplayName
	}
	if byPath[only] != "Address book" || byPath[cal] != "Calendar" {
		t.Fatalf("names = %+v", byPath)
	}

	// A second nameless book in the same home: now the address tells them apart.
	second := davtest.AddressBookHome + "other/"
	srv.AddAddressBook(carddav.AddressBook{Path: second})
	res, _, err = Run(t.Context(), loopbackGuard(), srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	byPath = map[string]string{}
	for _, c := range res.Collections {
		byPath[c.Path] = c.DisplayName
	}
	if byPath[only] != "Address book (card)" || byPath[second] != "Address book (other)" {
		t.Errorf("names = %+v", byPath)
	}
	if byPath[cal] != "Calendar" {
		t.Errorf("the calendar is alone and keeps its plain name: %q", byPath[cal])
	}
}
