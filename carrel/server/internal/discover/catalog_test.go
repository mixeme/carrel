package discover

import (
	"slices"
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
