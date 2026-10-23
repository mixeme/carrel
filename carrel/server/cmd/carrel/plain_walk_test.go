package main

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
	"golang.org/x/net/html"

	"carrel.dev/carrel/server/internal/davtest"
)

// U-8: the app works without scripts. A browser with scripts off, on the
// instance as it is served — the server and its client's pages —, does what
// U-8 lists by links and forms alone: the first run and signing in;
// connecting an account and a storage; ticking the rail; making, editing
// and deleting a record of every kind; importing a file and reading the
// report; publishing a link and a guest opening it; uploading a file and
// downloading it; signing out. Then it goes by every link it reaches and
// holds that each answers a browser with a page — save what waits for the
// library (notYetPages), each named with what it waits for.
func TestTheAppWorksWithoutScripts(t *testing.T) {
	stand := davtest.New(t)
	stand.Login, stand.Password = "alice", "secret"
	stand.AddAddressBook(carddav.AddressBook{Path: davtest.AddressBookHome + "people/", Name: "People"})
	stand.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Work"})
	host, _ := url.Parse(stand.URL)
	b := startPlain(t, host.Hostname())

	b.open("/login")
	b.press("Create administrator", url.Values{"login": {"mix"}, "password": {"correct horse battery"}, "password2": {"correct horse battery"}}, nil)
	b.press("Sign in", url.Values{"login": {"mix"}, "password": {"correct horse battery"}}, nil)
	b.follow("Connect account")
	b.press("Connect", url.Values{"label": {"home"}, "server": {stand.URL}, "login": {"alice"}, "password": {"secret"}}, nil)
	b.follow("Back to app")

	// A contact: the rail ticked by its form, the new card by the arrow of
	// New (its main button opens the quick sheet, notYetPages), edited,
	// deleted.
	b.follow("Contacts")
	b.press("Tick every book", nil, nil)
	b.follow("People")
	b.press("Save", url.Values{"fn": {"Ada Lovelace"}, "tel.0": {"+1 555 0100"}}, nil)
	b.says("Ada Lovelace")
	b.follow("Edit")
	b.press("Save", url.Values{"fn": {"Ada King"}}, nil)
	b.says("Ada King")
	b.press("Delete", nil, nil)
	b.says("This book is empty")

	// An event, a task, a note: the same, in the one calendar.
	for _, k := range []struct{ section, title, edited string }{
		{"Calendar", "Standup", "Standup moved"},
		{"Tasks", "Pay the rent", "Pay the rent today"},
		{"Notes", "Reading list", "Reading list, short"},
	} {
		b.follow(k.section)
		b.press("Tick every", nil, nil)
		b.follow("Work")
		b.press("Save", url.Values{"summary": {k.title}}, nil)
		b.says(k.title)
		b.follow("Edit")
		b.press("Save", url.Values{"summary": {k.edited}}, nil)
		b.says(k.edited)
		b.press("Delete", nil, nil)
		if strings.Contains(b.page.text(), k.edited) {
			t.Errorf("%s: %q is still there after Delete", k.section, k.edited)
		}
	}

	// A file of cards imported: the file sent by the step's form, the
	// preview read, the report.
	b.follow("Contacts")
	b.follow("Import")
	b.press("Preview file", nil, map[string][2]string{"file": {"people.vcf", "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:bob\r\nFN:Bob Builder\r\nEND:VCARD\r\n"}})
	b.says("Bob Builder")
	b.press("Import 1 contact", nil, nil)
	b.says("Created 1")
	b.follow("Back to list")
	b.says("Bob Builder")

	// A link to the card, opened by a guest.
	b.follow("Bob Builder")
	b.follow("Publish a link")
	b.press("Create link", nil, nil)
	link := b.page.find(func(n *html.Node) bool { return n.Data == "input" && strings.Contains(attr(n, "value"), "/p/") })
	if link == nil {
		t.Fatalf("%s: no link made", b.page.url.Path)
	}
	guest := &plainBrowser{t: t, origin: b.origin, client: &http.Client{Timeout: 30 * time.Second}, page: b.page}
	guest.go_(attr(link, "value"))
	guest.says("Bob Builder")

	// A storage: connected, ticked, a file sent by its form and fetched back.
	b.follow("Back to list")
	b.follow("Files")
	b.follow("Connect account")
	b.press("Connect", url.Values{"label": {"store"}, "server": {stand.URL + davtest.FileRoot}, "login": {"alice"}, "password": {"secret"}}, nil)
	b.follow("Back to app")
	b.follow("Files")
	b.press("Tick every storage", nil, nil)
	b.follow("files")
	b.press("Send", nil, map[string][2]string{"file": {"hello.txt", "hello without scripts"}})
	b.says("hello.txt")
	b.follow("hello.txt")
	b.follow("Download")
	if b.page.body != "hello without scripts" {
		t.Errorf("the download: %d %.200q", b.page.status, b.page.body)
	}

	// Every screen it reaches, and the search asked by its form.
	b.open("/app/contacts")
	crawl(t, b, 300)
	b.open("/app/contacts")
	searched := b.enter("q", "Bob")
	notYet(t, searched.url.RequestURI(), searched.body, searched.doc == nil)

	b.open("/app/contacts")
	b.press("Sign out", nil, nil)
	b.says("Sign in")
}

// notYetPages are the addresses that answer a browser with JSON today: the
// screens the client has not built, each waiting for the library — a
// browser without scripts (and one with them) finds the model, not a page.
// One that becomes a page goes from here.
var notYetPages = []struct {
	path *regexp.Regexp
	why  string
}{
	{regexp.MustCompile(`^/app/(contacts|calendar|tasks|notes)/quick(\?|$)`), "the quick sheets: Typecase's task 197 p. 5"},
	{regexp.MustCompile(`^/app/(contacts|calendar|tasks|notes)/[^/]+/[^/]+/new\?pick=1$`), "New of a combined view, the quick sheet: task 197 p. 5"},
	{regexp.MustCompile(`^/app/(contacts|calendar|tasks|notes)/export$`), "the export of a combined view, no frame: task 197 p. 5"},
	{regexp.MustCompile(`^/app/duplicates`), "duplicates: task 190"},
	{regexp.MustCompile(`^/p/`), "public pages: task 196"},
}

// notYet holds what an address answered: a page; JSON only where
// notYetPages expects it; one expected that is a page now is said.
func notYet(t *testing.T, at, body string, notHTML bool) {
	t.Helper()
	json := notHTML && strings.HasPrefix(strings.TrimSpace(body), "{")
	for _, n := range notYetPages {
		if n.path.MatchString(at) {
			if !json {
				t.Errorf("%s is a page now: take it off notYetPages (%s)", at, n.why)
			}
			return
		}
	}
	if json {
		t.Errorf("%s answers a browser with JSON: %.160s", at, body)
	}
}

// crawl goes by every link of the instance it reaches from the page the
// browser holds, but signing out — each screen once: an address of the same
// path asking the same things (its query's names, not their values) is the
// same screen. Each answers a page, or a file; never a failure, and JSON
// only where notYetPages expects it.
func crawl(t *testing.T, b *plainBrowser, most int) {
	t.Helper()
	screen := func(u *url.URL) string {
		var keys []string
		for k := range u.Query() {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return u.Path + "?" + strings.Join(keys, "&")
	}
	seen := map[string]bool{}
	queue := []*url.URL{b.page.url}
	for len(queue) > 0 && len(seen) < most {
		u := queue[0]
		queue = queue[1:]
		if seen[screen(u)] {
			continue
		}
		seen[screen(u)] = true
		req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
		req.Header.Set("Accept", "text/html")
		resp, err := b.send(req)
		if err != nil {
			t.Errorf("%s: %v", u.RequestURI(), err)
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		ctype := resp.Header.Get("Content-Type")
		if resp.StatusCode >= 500 {
			t.Errorf("%s: %s", u.RequestURI(), resp.Status)
			continue
		}
		if !strings.HasPrefix(ctype, "text/html") {
			notYet(t, u.RequestURI(), string(raw), true)
			continue // a file: a download, an export
		}
		notYet(t, u.RequestURI(), string(raw), false)
		doc, err := html.Parse(strings.NewReader(string(raw)))
		if err != nil {
			continue
		}
		page := &plainPage{url: resp.Request.URL, doc: doc}
		for _, a := range page.all(func(n *html.Node) bool { return n.Data == "a" && attr(n, "href") != "" }) {
			next, err := page.url.Parse(attr(a, "href"))
			if err != nil || next.Host != page.url.Host || next.Path == "/logout" || strings.HasPrefix(next.Path, "/static/") {
				continue
			}
			next.Fragment = ""
			if !seen[screen(next)] {
				queue = append(queue, next)
			}
		}
	}
	if len(seen) < 50 {
		t.Errorf("the crawl reached %d screens only", len(seen))
	}
}
