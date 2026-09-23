package card

import (
	"errors"
	"testing"
	"time"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/davtest"
)

const book = davtest.AddressBookHome + "people/"

func davProvider(t *testing.T) (*Provider, *davtest.Server) {
	t.Helper()
	srv := davtest.New(t)
	srv.AddAddressBook(carddav.AddressBook{Path: book, Name: "People"})
	g := dav.NewGuard(&conf.Config{
		DAVConnectTimeout: 5 * time.Second, DAVRequestTimeout: 10 * time.Second,
		DAVMaxResponseBytes: 1 << 20, DAVMaxRedirects: 5, DAVSSRFAllowlist: "127.0.0.1",
	})
	client, err := dav.NewClient(g, srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(client)
	if err != nil {
		t.Fatal(err)
	}
	return p, srv
}

func vcf(uid, fn string) string {
	return "BEGIN:VCARD\nVERSION:4.0\nUID:" + uid + "\nFN:" + fn + "\nX-CUSTOM:keep\nEND:VCARD\n"
}

func TestQueryReadsCardsFromOneReport(t *testing.T) {
	p, srv := davProvider(t)
	srv.Seed(book+"ada.vcf", vcf("ada", "Ada Lovelace"))
	srv.Seed(book+"bob.vcf", vcf("bob", "Bob, the Builder"))

	objs, err := p.Query(t.Context(), book)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 {
		t.Fatalf("objs = %+v", objs)
	}
	for _, o := range objs {
		if o.ETag == "" || len(o.PropText("X-CUSTOM")) != 1 {
			t.Errorf("%s: etag %q, props %v", o.Path, o.ETag, o.Props())
		}
		if UID(o) == "bob" && o.PropText("FN")[0] != "Bob, the Builder" {
			t.Errorf("FN = %q, want unescaped", o.PropText("FN")[0])
		}
	}
	if n := srv.CountRequests("GET", book); n != 0 {
		t.Errorf("GET requests = %d: the REPORT body was fetched again", n)
	}
}

// S-15.2: one REPORT, any shown property. davtest matches case-sensitively;
// a real server's default collation does not.
func TestSearchLooksInShownPropertiesWithOneReport(t *testing.T) {
	p, srv := davProvider(t)
	srv.Seed(book+"ada.vcf", vcf("ada", "Ada Lovelace"))
	srv.Seed(book+"fan.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:fan\nFN:Bob\nEMAIL:lovelace-fan@example.com\nEND:VCARD\n")
	srv.Seed(book+"cyd.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:cyd\nFN:Cyd\nNOTE:Lovelace\nEND:VCARD\n")

	objs, err := p.Search(t.Context(), book, "ovelace")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, o := range objs {
		got[UID(o)] = true
	}
	if len(objs) != 2 || !got["ada"] || !got["fan"] {
		t.Fatalf("found %v; a NOTE is not a shown property", got)
	}
	if n := srv.CountRequests("REPORT", book); n != 1 {
		t.Errorf("REPORT requests = %d, want 1", n)
	}
}

func TestFindUIDIsExact(t *testing.T) {
	p, srv := davProvider(t)
	srv.Seed(book+"a.vcf", vcf("abc", "Short"))
	srv.Seed(book+"b.vcf", vcf("abcd", "Long"))
	obj, err := p.FindUID(t.Context(), book, "abc")
	if err != nil || obj.PropText("FN")[0] != "Short" {
		t.Fatalf("obj = %+v, err = %v", obj, err)
	}
	if _, err := p.FindUID(t.Context(), book, "zzz"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing = %v", err)
	}
}

func TestNewObjectIsASkeleton(t *testing.T) {
	obj := NewObject("0123456789abcdef0123456789abcdef")
	if UID(obj) != "0123456789abcdef0123456789abcdef" || obj.PropText("VERSION")[0] != "4.0" || len(obj.Props()) != 2 {
		t.Fatalf("skeleton = %v", obj.Props())
	}
}

// docs/servers.md, SV-26: a server whose addressbook-query answers 207 with no
// responses at all while the same book lists its cards and hands them over to
// a multiget — Google. An empty answer is checked against the book itself, and
// what is there is read the round-about way; a search over it is then ours to
// do, and it says so. The same server answers 400 to an allprop inside
// address-data, so a body that carries one fails here outright.
func TestHollowQueryIsCheckedAgainstTheBook(t *testing.T) {
	p, srv := davProvider(t)
	srv.HollowCardQuery = true
	srv.RefuseAllPropInAddressData = true
	srv.Seed(book+"ada.vcf", vcf("ada", "Ada Lovelace"))
	srv.Seed(book+"bob.vcf", vcf("bob", "Bob Builder"))

	objs, err := p.Query(t.Context(), book)
	if err != nil || len(objs) != 2 {
		t.Fatalf("the whole book: %d objects, err %v", len(objs), err)
	}
	obj, err := p.FindUID(t.Context(), book, "ada")
	if err != nil || obj.PropText("FN")[0] != "Ada Lovelace" {
		t.Fatalf("FindUID = %+v, err %v", obj, err)
	}
	if _, err := p.FindUID(t.Context(), book, "zzz"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a UID that is nowhere = %v, want ErrNotFound", err)
	}
	found, err := p.Search(t.Context(), book, "ovelace")
	if !errors.Is(err, ErrServerDoesNotQuery) {
		t.Fatalf("search err = %v, want the note that the searching was ours", err)
	}
	if len(found) != 1 || UID(found[0]) != "ada" {
		t.Errorf("found %+v, want the one card that holds the text", found)
	}
}

// An empty book is an empty book: the listing says so, and nothing is fetched.
func TestAnEmptyBookIsTakenAsEmpty(t *testing.T) {
	p, srv := davProvider(t)
	objs, err := p.Query(t.Context(), book)
	if err != nil || len(objs) != 0 {
		t.Fatalf("objects = %+v, err = %v", objs, err)
	}
	if n := srv.CountRequests("GET", book); n != 0 {
		t.Errorf("GET requests = %d", n)
	}
}

// docs/servers.md, SV-40: Xandikos takes match-type and answers 207 with no
// responses at all — the same answer as for a card that is not there — while
// the same filter without it hands the card over. Nothing on the wire says
// the filter was the problem, so there is nothing to fall back from: a single
// card would simply be missing, and its page a 404. FindUID therefore asks
// for the default match, a substring, and checks the UID itself. The stand
// answers nothing to any match-type, so this passes only while we send none.
func TestFindUIDAsksForNoOptionalMatchType(t *testing.T) {
	p, srv := davProvider(t)
	srv.DeafToMatchType = true
	srv.Seed(book+"ada.vcf", vcf("ada", "Ada Lovelace"))
	srv.Seed(book+"ada-2.vcf", vcf("ada-2", "Ada the Second"))

	obj, err := p.FindUID(t.Context(), book, "ada")
	if err != nil || obj == nil || UID(obj) != "ada" {
		t.Fatalf("FindUID = %+v, err %v", obj, err)
	}
	// A substring filter can bring back more than was asked for: "ada" is
	// inside "ada-2". The exact UID is ours to check, and it is.
	if obj.PropText("FN")[0] != "Ada Lovelace" {
		t.Errorf("FN = %q, want the card whose UID is exactly the one asked for", obj.PropText("FN")[0])
	}
	if _, err := p.FindUID(t.Context(), book, "zzz"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a UID that is nowhere = %v, want ErrNotFound", err)
	}
}

// docs/servers.md, SV-42: a server that falls over on the filter instead of
// refusing it — Xandikos answers 500 to any text-match over a book holding
// one value with a character outside ASCII, whatever the text asked for,
// while the same book without a filter hands every card over. Left as it
// came, a search would die on any book with one accented name. It is read
// the round-about way instead, and the note says the searching was ours.
func TestASearchSurvivesAServerThatBreaksOnTheFilter(t *testing.T) {
	p, srv := davProvider(t)
	srv.BreaksOnTextMatch = true
	srv.Seed(book+"ada.vcf", vcf("ada", "Ada Lovelace"))
	srv.Seed(book+"rene.vcf", vcf("rene", "René Descartes"))

	found, err := p.Search(t.Context(), book, "ovelace")
	if !errors.Is(err, ErrServerDoesNotQuery) {
		t.Fatalf("search err = %v, want the note that the searching was ours", err)
	}
	if len(found) != 1 || UID(found[0]) != "ada" {
		t.Errorf("found %+v, want the one card that holds the text", found)
	}
	obj, err := p.FindUID(t.Context(), book, "rene")
	if err != nil || obj == nil || UID(obj) != "rene" {
		t.Fatalf("FindUID = %+v, err %v", obj, err)
	}
	// Without a filter there is nothing to have broken on: a 500 then is
	// the answer, and it goes to whoever asked.
	srv.Refuse("REPORT", 500, "boom")
	if _, err := p.Query(t.Context(), book); err == nil {
		t.Error("a whole-book read that the server answered 500 came back as success")
	}
}
