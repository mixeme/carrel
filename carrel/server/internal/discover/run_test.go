package discover

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/view"
)

// fakeDAV answers the PROPFINDs S-6 walks through. Each hook decides what one
// step replies with, so a test can fail exactly one of them.
type fakeDAV struct {
	principal string // empty: the server has no current-user-principal
	homeSets  bool   // false: both home-set lookups come back 404
	dirs      []string
	// selfKind makes the entered address answer as a collection of its own —
	// "calendar" or "addressbook" — the way a handed-over link does (S-6).
	selfKind string
	selfName string
}

func (f fakeDAV) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/.well-known/") {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		asked := string(body)

		switch {
		case strings.Contains(asked, "current-user-principal"):
			if f.principal == "" {
				f.reply(w, notFoundProp("/", "current-user-principal"))
				return
			}
			f.reply(w, `<D:response><D:href>/</D:href><D:propstat><D:prop>`+
				`<D:current-user-principal><D:href>`+f.principal+`</D:href></D:current-user-principal>`+
				`</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`)
		case strings.Contains(asked, "calendar-home-set"):
			f.reply(w, notFoundProp(f.principal, "C:calendar-home-set"))
		case strings.Contains(asked, "addressbook-home-set"):
			f.reply(w, notFoundProp(f.principal, "A:addressbook-home-set"))
		// The entered address answering for itself. Discovery asks for the
		// component set here and ReadDir never does, so the file walk below
		// keeps its own answer.
		case f.selfKind != "" && strings.Contains(asked, "supported-calendar-component-set"):
			mark, comps := `<C:calendar/>`, `<C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>`
			if f.selfKind == "addressbook" {
				mark, comps = `<A:addressbook/>`, ``
			}
			f.reply(w, `<D:response><D:href>/</D:href><D:propstat><D:prop>`+
				`<D:resourcetype><D:collection/>`+mark+`</D:resourcetype>`+
				`<D:displayname>`+f.selfName+`</D:displayname>`+comps+
				`</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`)
		default: // ReadDir: resourcetype and friends
			var b strings.Builder
			for _, d := range f.dirs {
				b.WriteString(`<D:response><D:href>` + d + `</D:href><D:propstat><D:prop>` +
					`<D:resourcetype><D:collection/></D:resourcetype>` +
					`</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`)
			}
			f.reply(w, b.String())
		}
	})
}

func (fakeDAV) reply(w http.ResponseWriter, responses string) {
	w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>`+
		`<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" `+
		`xmlns:A="urn:ietf:params:xml:ns:carddav">`+responses+`</D:multistatus>`)
}

func notFoundProp(href, prop string) string {
	if href == "" {
		href = "/"
	}
	return `<D:response><D:href>` + href + `</D:href><D:propstat><D:prop><` + prop + `/></D:prop>` +
		`<D:status>HTTP/1.1 404 Not Found</D:status></D:propstat></D:response>`
}

func runAgainst(t *testing.T, f fakeDAV) (*Result, *view.DiagnosticTrace, error) {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)

	cfg := &conf.Config{
		DAVConnectTimeout:   5 * time.Second,
		DAVRequestTimeout:   10 * time.Second,
		DAVMaxResponseBytes: 1 << 20,
		DAVMaxRedirects:     5,
		DAVSSRFAllowlist:    "127.0.0.1",
	}
	return Run(t.Context(), dav.NewGuard(cfg), srv.URL, "", "", "")
}

// S-6: the entered address is itself a calendar. Mail.ru hands out a
// calendar's own link, and such an address names no principal — but a file
// collection is one whose resource type carries neither mark, so the mark is
// what decides. The file walk must not run: dirs are there to prove it did
// not, and that a handed-over calendar does not arrive as a folder.
func TestEnteredAddressIsACalendar(t *testing.T) {
	res, trace, err := runAgainst(t, fakeDAV{selfKind: "calendar", selfName: "Основной", dirs: []string{"/", "/docs/"}})
	if err != nil {
		t.Fatalf("discovery refused a handed-over calendar: %v", err)
	}
	if res.FileOnly {
		t.Error("the calendar came back as a file root")
	}
	if len(res.Collections) != 1 {
		t.Fatalf("collections = %#v, want the one that was handed over", res.Collections)
	}
	col := res.Collections[0]
	if col.DisplayName != "Основной" {
		t.Errorf("name = %q, want the one the server shows", col.DisplayName)
	}
	if len(col.Kinds) != 1 || col.Kinds[0] != view.KindEvent {
		t.Errorf("kinds = %v, want the components the server named", col.Kinds)
	}
	if !trace.OK {
		t.Error("a handed-over calendar is a successful check")
	}
}

// The same for an address book: the two marks are read the same way.
func TestEnteredAddressIsAnAddressBook(t *testing.T) {
	res, _, err := runAgainst(t, fakeDAV{selfKind: "addressbook", selfName: "Contacts", dirs: []string{"/", "/docs/"}})
	if err != nil {
		t.Fatalf("discovery refused a handed-over address book: %v", err)
	}
	if res.FileOnly || len(res.Collections) != 1 {
		t.Fatalf("fileOnly %v, collections %#v", res.FileOnly, res.Collections)
	}
	col := res.Collections[0]
	if col.DisplayName != "Contacts" || len(col.Kinds) != 1 || col.Kinds[0] != view.KindContact {
		t.Errorf("collection = %+v", col)
	}
}

// S-6: a server with no principal is a file collection, not a refusal. The
// old test for this asserted the fields of a struct literal it had just
// written, so neither Run nor fileCollections was ever executed.
func TestNoPrincipalGivesFileCollections(t *testing.T) {
	res, trace, err := runAgainst(t, fakeDAV{
		dirs: []string{"/", "/docs/", "/photos/"},
	})
	if err != nil {
		t.Fatalf("discovery refused a file-only server: %v", err)
	}
	if !res.FileOnly {
		t.Error("FileOnly should be set when there is no principal")
	}
	if len(res.Collections) != 2 {
		t.Fatalf("collections = %#v, want /docs/ and /photos/ (root skipped)", res.Collections)
	}
	for _, c := range res.Collections {
		if len(c.Kinds) != 1 || c.Kinds[0] != view.KindFile {
			t.Errorf("collection %q kinds = %v, want [file]", c.Path, c.Kinds)
		}
	}
	if !trace.OK {
		t.Error("a file-only server is a successful check")
	}
	if !traceHas(trace, stepPrincipal, view.CheckBad) {
		t.Error("the failed principal step should still be on the trace")
	}
	if !traceHas(trace, stepCollections, view.CheckOK) {
		t.Error("the collections step should be green")
	}
}

// A server with no collections at all still answers, and the root becomes the
// one file collection.
func TestNoPrincipalNoDirsFallsBackToRoot(t *testing.T) {
	res, _, err := runAgainst(t, fakeDAV{dirs: []string{"/"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Collections) != 1 || res.Collections[0].Path != "/" {
		t.Fatalf("collections = %#v, want the root alone", res.Collections)
	}
}

// A principal whose home-sets both fail is a bad check, not a green one: the
// run reached no collections at all.
func TestPrincipalWithoutHomeSetsIsNotOK(t *testing.T) {
	_, trace, err := runAgainst(t, fakeDAV{principal: "/principals/me/"})
	if err != nil {
		t.Fatal(err)
	}
	if trace.OK {
		t.Error("trace.OK should be false when no collection was found")
	}
	if !traceHas(trace, stepPrincipal, view.CheckOK) {
		t.Error("the principal step succeeded and should be green")
	}
	if !traceHas(trace, stepHomeSet, view.CheckBad) {
		t.Error("a failed home-set lookup belongs on the trace (S-6)")
	}
}

// S-6: every step leaves a trace, including the well-known probes that failed.
func TestTraceRecordsEveryStep(t *testing.T) {
	_, trace, err := runAgainst(t, fakeDAV{dirs: []string{"/", "/docs/"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{stepWellKnown, stepDirect, stepPrincipal, stepCollections} {
		if !traceHasStep(trace, want) {
			t.Errorf("step %q missing from the trace", want)
		}
	}
	for i, s := range trace.Steps {
		if s.Detail == "" {
			t.Errorf("step %d (%s) has no detail — the trace should say what the server answered", i, s.Name)
		}
	}
}

func traceHasStep(tr *view.DiagnosticTrace, name string) bool {
	for _, s := range tr.Steps {
		if string(s.Name) == name {
			return true
		}
	}
	return false
}

func traceHas(tr *view.DiagnosticTrace, name string, state view.CheckState) bool {
	for _, s := range tr.Steps {
		if string(s.Name) == name && s.State == state {
			return true
		}
	}
	return false
}
