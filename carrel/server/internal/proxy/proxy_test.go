package proxy_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/proxy"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
)

const (
	work   = davtest.CalendarHome + "work/"
	people = davtest.AddressBookHome + "people/"
	docs   = davtest.FileRoot + "docs/"
)

type fixture struct {
	t        *testing.T
	far      *davtest.Server
	users    *user.Service
	userID   string
	account  string
	password string
	deviceID string
	srv      *httptest.Server
}

// newFixture: alice with one connection holding a calendar, a book and a
// folder, and a device that sees the calendar and the folder, not the book.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	return newFixtureAt(t, "")
}

func newFixtureAt(t *testing.T, publicURL string) *fixture {
	t.Helper()
	far := davtest.New(t)
	far.Login, far.Password = "alice", "davpass"
	far.AddCalendar(caldav.Calendar{Path: work, Name: "Work", SupportedComponentSet: []string{"VEVENT"}})
	far.AddAddressBook(carddav.AddressBook{Path: people, Name: "People"})
	far.SeedFile(docs+"readme.txt", "hello from the files")

	dir := t.TempDir()
	key, err := crypt.CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir, cfg.DAVSSRFAllowlist, cfg.DAVloomPort, cfg.PublicURL = dir, "127.0.0.1", 8081, publicURL
	users := user.New(st, session.NewManager(cfg), cfg)
	ctx := context.Background()
	if err := users.SetupFirstAdmin(ctx, "alice", "password123"); err != nil {
		t.Fatal(err)
	}
	id, dataKey, _, ok := users.Authenticate(ctx, "alice", "password123")
	if !ok {
		t.Fatal("no sign-in")
	}
	account, err := users.AddConnection(id, dataKey, sealed.Connection{Server: far.URL, Login: "alice", Password: "davpass"}, []sealed.Collection{
		{Path: work, Name: "Work", Kinds: []view.Kind{view.KindEvent}},
		{Path: people, Name: "People", Kinds: []view.Kind{view.KindContact}},
		{Path: docs, Name: "Docs", Kinds: []view.Kind{view.KindFile}},
	})
	if err != nil {
		t.Fatal(err)
	}
	deviceID, password, err := users.IssueDevice(ctx, id, dataKey, "Phone", user.DeviceScope{
		Kinds: []view.Kind{view.KindEvent, view.KindContact, view.KindFile},
		Cols:  []store.DeviceScope{{AccountID: account, Path: work}, {AccountID: account, Path: docs}},
	})
	if err != nil {
		t.Fatal(err)
	}
	px := proxy.New(cfg, users, dav.NewGuard(cfg), "test", "abc")
	srv := httptest.NewServer(px.Handler())
	t.Cleanup(srv.Close)
	return &fixture{t: t, far: far, users: users, userID: id, account: account, password: password, deviceID: deviceID, srv: srv}
}

func (f *fixture) do(method, path, body string, header map[string]string) (*http.Response, string) {
	f.t.Helper()
	req, _ := http.NewRequest(method, f.srv.URL+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	if _, set := header["Authorization"]; !set {
		req.SetBasicAuth("alice", f.password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, string(data)
}

func (f *fixture) col(path string) string {
	return "/c/" + f.account + "/" + url.PathEscape(path) + "/"
}

// S-27.4: the root answers a browser without a password and names what it
// is and where the source is, and nothing of anybody's collections.
func TestRootPage(t *testing.T) {
	f := newFixture(t)
	resp, err := http.Get(f.srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var page view.AboutPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil || page.Title != "davloom" || page.Source.Href != "https://carrel.dev" || page.Commit != "abc" {
		t.Fatalf("page = %+v %v", page, err)
	}
	if resp.Header.Get("Set-Cookie") != "" {
		t.Error("the proxy set a cookie")
	}
}

// Basic auth: no header, the sign-in password, a wrong one and a revoked
// device all answer 401 with the realm; the device password opens.
func TestDevicePasswordOnly(t *testing.T) {
	f := newFixture(t)
	for name, auth := range map[string]string{"none": "", "sign-in password": basic("alice", "password123"), "wrong": basic("alice", "nope")} {
		resp, _ := f.do("PROPFIND", "/principal/", "", map[string]string{"Authorization": auth, "Depth": "0"})
		if resp.StatusCode != 401 || !strings.Contains(resp.Header.Get("WWW-Authenticate"), `realm="davloom"`) {
			t.Errorf("%s: %d %q", name, resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
		}
	}
	if resp, _ := f.do("PROPFIND", "/principal/", "", map[string]string{"Depth": "0"}); resp.StatusCode != 207 {
		t.Fatalf("device password: %d", resp.StatusCode)
	}
	if err := f.users.RevokeDevice(f.userID, f.deviceID); err != nil {
		t.Fatal(err)
	}
	if resp, _ := f.do("PROPFIND", "/principal/", "", map[string]string{"Depth": "0"}); resp.StatusCode != 401 {
		t.Errorf("revoked device: %d", resp.StatusCode)
	}
}

// S-27.3: the principal names the homes; a home lists the device's
// collections by /c/{account}/{col}/ with the far server's ctag; a
// collection outside the scope is neither listed nor reachable.
func TestHomesAreTheScope(t *testing.T) {
	f := newFixture(t)
	_, principal := f.do("PROPFIND", "/principal/", "", map[string]string{"Depth": "0"})
	if !strings.Contains(principal, "<d:href>/cal/</d:href>") || !strings.Contains(principal, "<d:href>/card/</d:href>") {
		t.Errorf("principal:\n%s", principal)
	}
	_, cal := f.do("PROPFIND", "/cal/", "", map[string]string{"Depth": "1"})
	if !strings.Contains(cal, f.col(work)) || !strings.Contains(cal, "<c:calendar/>") {
		t.Errorf("cal home:\n%s", cal)
	}
	_, card := f.do("PROPFIND", "/card/", "", map[string]string{"Depth": "1"})
	if strings.Contains(card, "/c/") {
		t.Errorf("a book outside the scope is listed:\n%s", card)
	}
	if resp, _ := f.do("PROPFIND", f.col(people), "", map[string]string{"Depth": "1"}); resp.StatusCode != 404 {
		t.Errorf("a book outside the scope answers %d", resp.StatusCode)
	}
	_, files := f.do("PROPFIND", "/files/", "", map[string]string{"Depth": "1"})
	if !strings.Contains(files, f.col(docs)) {
		t.Errorf("files home:\n%s", files)
	}
}

// Below a collection everything passes to its server: a PUT lands there,
// the listing comes back with local hrefs only, a multiget with local hrefs
// finds the record, a file comes back byte for byte.
func TestRequestsPassThrough(t *testing.T) {
	f := newFixture(t)
	event := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\nBEGIN:VEVENT\r\nUID:e1\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20261001T100000Z\r\nSUMMARY:Through the proxy\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req, _ := http.NewRequest("PUT", f.srv.URL+f.col(work)+"e1.ics", strings.NewReader(event))
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	req.Header.Set("If-None-Match", "*")
	req.SetBasicAuth("alice", f.password)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode/100 != 2 {
		t.Fatalf("put: %v %v", err, resp)
	}
	resp.Body.Close()
	if !strings.Contains(strings.Join(f.far.Paths(work), " "), "e1.ics") {
		t.Fatalf("not on the far server: %v", f.far.Paths(work))
	}

	_, listing := f.do("PROPFIND", f.col(work), `<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:getetag/></d:prop></d:propfind>`, map[string]string{"Depth": "1"})
	if !strings.Contains(listing, f.col(work)+"e1.ics") || strings.Contains(listing, ">"+work) {
		t.Errorf("listing:\n%s", listing)
	}
	multiget := `<?xml version="1.0"?><c:calendar-multiget xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:calendar-data/></d:prop><d:href>` + f.col(work) + `e1.ics</d:href></c:calendar-multiget>`
	resp, got := f.do("REPORT", f.col(work), multiget, map[string]string{"Depth": "1"})
	if resp.StatusCode != 207 || !strings.Contains(got, "Through the proxy") || !strings.Contains(got, f.col(work)+"e1.ics") {
		t.Errorf("multiget %d:\n%s", resp.StatusCode, got)
	}
	resp, body := f.do("GET", f.col(docs)+"readme.txt", "", nil)
	if resp.StatusCode != 200 || body != "hello from the files" {
		t.Errorf("file: %d %q", resp.StatusCode, body)
	}
	if resp, _ := f.do("GET", f.col(docs)+"../x", "", nil); resp.StatusCode != 404 {
		t.Errorf("dot-dot: %d", resp.StatusCode)
	}
}

func basic(login, password string) string {
	req, _ := http.NewRequest("GET", "/", nil)
	req.SetBasicAuth(login, password)
	return req.Header.Get("Authorization")
}

// Published at CARREL_PUBLIC_URL, the proxy is reached under /dav/ and its
// reverse proxy strips the prefix on the way in: hrefs go out with /dav, and
// a request is taken with the prefix or without it.
func TestHrefsCarryThePublicPrefix(t *testing.T) {
	f := newFixtureAt(t, "https://carrel.example")
	for _, p := range []string{"/cal/", "/dav/cal/"} {
		resp, body := f.do("PROPFIND", p, "", map[string]string{"Depth": "1"})
		if resp.StatusCode != 207 || !strings.Contains(body, "<d:href>/dav"+f.col(work)+"</d:href>") {
			t.Errorf("%s: %d %s", p, resp.StatusCode, body)
		}
	}
	_, listing := f.do("PROPFIND", "/dav"+f.col(work), "", map[string]string{"Depth": "1"})
	if !strings.Contains(listing, "/dav"+f.col(work)) {
		t.Errorf("listing: %s", listing)
	}
}
