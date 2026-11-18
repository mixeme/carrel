package discover

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"slices"
	"strconv"
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

// A storage entered by an address with a path — SFTPGo under /dav/, Nextcloud
// under /remote.php/dav/files/<login>/ — offers the folders at that address.
// A PROPFIND of depth 1 answers the address itself first; that answer is the
// storage, not one more folder in it, and the acceptance stand showed it as a
// third storage «files» beside Documents and Public (s-files-1 has two). With
// no folder in it, the address itself is the one collection.
func TestAStorageAtAPathIsNotItsOwnFolder(t *testing.T) {
	srv := davtest.New(t)
	srv.SeedFolder(davtest.FileRoot + "Documents/")
	srv.SeedFolder(davtest.FileRoot + "Public/")
	res, _, err := Run(t.Context(), loopbackGuard(), srv.URL+davtest.FileRoot, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range res.Collections {
		got = append(got, strings.TrimSuffix(c.Path, "/"))
	}
	slices.Sort(got)
	want := []string{davtest.FileRoot + "Documents", davtest.FileRoot + "Public"}
	if !res.FileOnly || !slices.Equal(got, want) {
		t.Errorf("fileOnly %v, collections %q, want %q", res.FileOnly, got, want)
	}

	empty := davtest.New(t)
	empty.SeedFolder(davtest.FileRoot)
	res, _, err = Run(t.Context(), loopbackGuard(), empty.URL+davtest.FileRoot, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Collections) != 1 || strings.TrimSuffix(res.Collections[0].Path, "/") != strings.TrimSuffix(davtest.FileRoot, "/") {
		t.Errorf("an empty storage = %+v, want its own address alone", res.Collections)
	}
}

// The files of an account on Nextcloud are one storage of that account,
// «Files» (s-files-6: «nextcloud · Files · rw»; s-plan-5: «Files у
// nextcloud (…/remote.php/dav/files/mix/)»). On the test servers' stand
// (U-15.1) a Nextcloud account gave calendars and books and no files at all,
// whatever address was entered: discovery found a principal and never asked
// for files. Nextcloud keeps them at files/<id>/ beside
// principals/users/<id>/, and no property of the principal names it.
// Davis keeps an account's files at …/dav/home/<id>/ beside its principal
// …/dav/principals/<id>/, and the principal does not say so either
// (docs/servers.md, SV-65; acceptance on our test servers, 18.11.2026). A
// principal of that shape on a server without the folder (Baikal:
// …/dav.php/principals/<id>/) is asked and stands without files, the trace
// unworried.
func TestAnAccountOnDavisHasItsFiles(t *testing.T) {
	srv := davtest.New(t)
	srv.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work", SupportedComponentSet: []string{"VEVENT"}})
	srv.SeedFolder(davtest.FileRoot + "Documents/")
	davis := davisShaped(t, srv, "carrel")

	res, trace, err := Run(t.Context(), loopbackGuard(), davis.URL+"/dav/", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var files []Collection
	for _, c := range res.Collections {
		if slices.Contains(c.Kinds, view.KindFile) {
			files = append(files, c)
		}
	}
	if want := "/dav/home/carrel/"; len(files) != 1 || files[0].Path != want || files[0].DisplayName != "Files" {
		t.Errorf("the files of the account = %+v, want one storage «Files» at %s; trace %+v", files, want, trace.Steps)
	}

	bare := davtest.New(t)
	bare.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work", SupportedComponentSet: []string{"VEVENT"}})
	// Only Baikal's own addresses: the folder is not there.
	baikal := shapedAs(t, bare, "/dav.php/principals/carrel/", [][2]string{
		{"/dav.php/principals/carrel/", davtest.Principal},
		{"/dav.php/calendars/carrel/", davtest.CalendarHome},
		{"/dav.php/addressbooks/carrel/", davtest.AddressBookHome},
	})
	res, trace, err = Run(t.Context(), loopbackGuard(), baikal.URL+"/dav.php/principals/carrel/", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Collections {
		if slices.Contains(c.Kinds, view.KindFile) {
			t.Errorf("Baikal-shaped: a storage %+v", c)
		}
	}
	for _, s := range trace.Steps {
		if s.State != view.CheckOK && strings.Contains(string(s.Detail), "files") {
			t.Errorf("Baikal-shaped: the trace worries about files: %+v", s)
		}
	}
}

func TestAnAccountOnNextcloudHasItsFiles(t *testing.T) {
	srv := davtest.New(t)
	srv.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work", SupportedComponentSet: []string{"VEVENT"}})
	srv.SeedFolder(davtest.FileRoot + "Documents/")
	nc := nextcloudShaped(t, srv, "mix")

	res, trace, err := Run(t.Context(), loopbackGuard(), nc.URL+"/remote.php/dav", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var files []Collection
	for _, c := range res.Collections {
		if slices.Contains(c.Kinds, view.KindFile) {
			files = append(files, c)
		}
	}
	want := "/remote.php/dav/files/mix/"
	if len(files) != 1 || files[0].Path != want || files[0].DisplayName != "Files" || res.FileOnly {
		t.Errorf("the files of the account = %+v (fileOnly %v), want one storage «Files» at %s; trace %+v", files, res.FileOnly, want, trace.Steps)
	}
	if !slices.ContainsFunc(res.Collections, func(c Collection) bool { return c.DisplayName == "Work" }) {
		t.Errorf("the calendar is gone: %+v", res.Collections)
	}

	// A principal of another shape is not asked for files: Baikal, Radicale
	// and davtest's own keep none there.
	res, _, err = Run(t.Context(), loopbackGuard(), srv.URL+davtest.Principal, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Collections {
		if slices.Contains(c.Kinds, view.KindFile) {
			t.Errorf("a principal of another shape gave files: %+v", c)
		}
	}
}

// nextcloudShaped serves srv at Nextcloud's addresses for the user id: a
// request's path is mapped to davtest's, and davtest's paths in the answer
// back to Nextcloud's.
func nextcloudShaped(t *testing.T, srv *davtest.Server, id string) *httptest.Server {
	t.Helper()
	return shapedAs(t, srv, "/remote.php/dav", [][2]string{
		{"/remote.php/dav/principals/users/" + id + "/", davtest.Principal},
		{"/remote.php/dav/calendars/" + id + "/", davtest.CalendarHome},
		{"/remote.php/dav/addressbooks/users/" + id + "/", davtest.AddressBookHome},
		{"/remote.php/dav/files/" + id + "/", davtest.FileRoot},
	})
}

// davisShaped is srv behind the addresses of Davis (sabre/dav, symfony):
// …/dav/principals/<id>/, its files at …/dav/home/<id>/ (WEBDAV_ENABLED).
func davisShaped(t *testing.T, srv *davtest.Server, id string) *httptest.Server {
	t.Helper()
	return shapedAs(t, srv, "/dav", [][2]string{
		{"/dav/principals/" + id + "/", davtest.Principal},
		{"/dav/calendars/" + id + "/", davtest.CalendarHome},
		{"/dav/addressbooks/" + id + "/", davtest.AddressBookHome},
		{"/dav/home/" + id + "/", davtest.FileRoot},
	})
}

// shapedAs is srv behind another server's addresses: pairs map them to
// davtest's, the principal first; whatever else is under root is the
// principal.
func shapedAs(t *testing.T, srv *davtest.Server, root string, pairs [][2]string) *httptest.Server {
	t.Helper()
	// The longer of davtest's paths first: they all begin with its
	// principal's.
	var back []string
	for _, pair := range append(append([][2]string(nil), pairs[1:]...), pairs[0]) {
		back = append(back, pair[1], pair[0])
	}
	target, _ := url.Parse(srv.URL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	direct := proxy.Director
	proxy.Director = func(r *http.Request) {
		direct(r)
		p := r.URL.Path
		mapped := false
		for _, pair := range pairs {
			if strings.HasPrefix(p, pair[0]) {
				p, mapped = pair[1]+strings.TrimPrefix(p, pair[0]), true
				break
			}
		}
		if !mapped && strings.HasPrefix(p, root) {
			p = davtest.Principal
		}
		r.URL.Path, r.URL.RawPath = p, ""
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		body = []byte(strings.NewReplacer(back...).Replace(strings.ReplaceAll(string(body), srv.URL, "")))
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
		return nil
	}
	nc := httptest.NewServer(proxy)
	t.Cleanup(nc.Close)
	return nc
}
