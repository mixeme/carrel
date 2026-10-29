package card

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
)

// davtest decodes vCards itself, so a broken resource needs a raw REPORT.
func brokenBook(t *testing.T) *Provider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Errorf("unexpected %s", r.Method)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, `<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:carddav">`)
		for _, item := range []struct{ name, data string }{
			{"ada", vcf("ada", "Ada")},
			{"broken", "BEGIN:VCARD\nVERSION:4.0\nUID:broken\nFN:Broken\n"},
			{"bob", vcf("bob", "Bob")},
		} {
			var data bytes.Buffer
			xml.EscapeText(&data, []byte(item.data))
			io.WriteString(w, `<d:response><d:href>/book/`+item.name+`.vcf</d:href><d:propstat><d:prop><d:getetag>"v1"</d:getetag><c:address-data>`+data.String()+`</c:address-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`)
		}
		io.WriteString(w, `</d:multistatus>`)
	}))
	t.Cleanup(srv.Close)
	g := dav.NewGuard(&conf.Config{DAVConnectTimeout: time.Second, DAVRequestTimeout: 5 * time.Second, DAVMaxResponseBytes: 1 << 20, DAVSSRFAllowlist: "127.0.0.1"})
	client, err := dav.NewClient(g, srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(client)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUnreadableCardKeepsItsNeighbours(t *testing.T) {
	p := brokenBook(t)
	objs, err := p.Query(t.Context(), "/book/")
	if len(objs) != 2 {
		t.Fatalf("read %d healthy cards, want 2; error: %v", len(objs), err)
	}
	if err == nil || !strings.Contains(err.Error(), "/book/broken.vcf") {
		t.Fatalf("missing diagnosis: %v", err)
	}
	if UID(objs[0]) != "ada" || UID(objs[1]) != "bob" {
		t.Fatalf("wrong neighbours: %v", objs)
	}
	for _, o := range objs {
		if o.ETag != "v1" || len(o.PropText("X-CUSTOM")) != 1 {
			t.Fatalf("lost version or unknown property: %v", o)
		}
	}
}

func TestUnreadableCardDoesNotHideHealthyUID(t *testing.T) {
	p := brokenBook(t)
	obj, err := p.FindUID(t.Context(), "/book/", "bob")
	if err != nil || UID(obj) != "bob" {
		t.Fatalf("healthy UID: %v, %v", obj, err)
	}
	_, err = p.FindUID(t.Context(), "/book/", "broken")
	if err == nil || err == ErrNotFound {
		t.Fatalf("unreadable UID reported absent: %v", err)
	}
}
