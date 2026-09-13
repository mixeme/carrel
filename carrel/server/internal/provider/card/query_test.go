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
