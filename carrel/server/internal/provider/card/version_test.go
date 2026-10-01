package card

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
)

// sabreLike answers a card query the way sabre/dav does: in 4.0 when the
// address-data asks for it, else in 3.0 without what 3.0 cannot hold. With
// refuse a query that names a version is refused as unsupported (409). asked
// records the version of each query, "" for none.
func sabreLike(t *testing.T, refuse bool) (*Provider, *[]string) {
	t.Helper()
	asked := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		four := bytes.Contains(body, []byte(`version="4.0"`))
		if four {
			*asked = append(*asked, "4.0")
		} else {
			*asked = append(*asked, "")
		}
		if four && refuse {
			http.Error(w, "supported-address-data", http.StatusConflict)
			return
		}
		card := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:ada\r\nFN:Ada\r\nEND:VCARD\r\n"
		if four {
			card = "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:ada\r\nFN:Ada\r\nKIND:individual\r\nEND:VCARD\r\n"
		}
		var data bytes.Buffer
		_ = xml.EscapeText(&data, []byte(card))
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:carddav">`+
			`<d:response><d:href>/book/ada.vcf</d:href><d:propstat><d:prop><d:getetag>"v1"</d:getetag><c:address-data>`+
			data.String()+`</c:address-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
	}))
	t.Cleanup(srv.Close)
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
	return p, asked
}

// S-7: every card read asks for vCard 4.0, so a converting server hands over
// the card with what only 4.0 holds, and one REPORT does it.
func TestCardReadsAskForVCard4(t *testing.T) {
	p, asked := sabreLike(t, false)
	list, err := p.Query(t.Context(), "/book/")
	if err != nil || len(list) != 1 {
		t.Fatalf("query: %v, %v", list, err)
	}
	one, err := p.FindUID(t.Context(), "/book/", "ada")
	if err != nil {
		t.Fatal(err)
	}
	found, err := p.Search(t.Context(), "/book/", "ada")
	if err != nil || len(found) != 1 {
		t.Fatalf("search: %v, %v", found, err)
	}
	for _, o := range []string{list[0].PropText("VERSION")[0], one.PropText("VERSION")[0], found[0].PropText("VERSION")[0]} {
		if o != "4.0" {
			t.Errorf("VERSION read = %q, want 4.0", o)
		}
	}
	if k := one.PropText("KIND"); len(k) != 1 || k[0] != "individual" {
		t.Errorf("KIND = %v: the card came converted", k)
	}
	if len(*asked) != 3 || (*asked)[0] != "4.0" || (*asked)[1] != "4.0" || (*asked)[2] != "4.0" {
		t.Errorf("queries asked %q, want 4.0 each, one a read", *asked)
	}
}

// A server that refuses 4.0 is asked again without a version, once a read.
func TestCardReadFallsBackWhenVersionIsRefused(t *testing.T) {
	p, asked := sabreLike(t, true)
	objs, err := p.Query(t.Context(), "/book/")
	if err != nil || len(objs) != 1 || objs[0].PropText("VERSION")[0] != "3.0" {
		t.Fatalf("query: %v, %v", objs, err)
	}
	if len(*asked) != 2 || (*asked)[0] != "4.0" || (*asked)[1] != "" {
		t.Errorf("queries asked %q, want 4.0 then none", *asked)
	}
}

// SV-1 on a converting stand: every read of a card asks for vCard 4.0 — the
// query, the search, the UID lookup, the multiget of a book without the
// query, and the GET of one card. sabre/dav (Baikal, Nextcloud) answers a
// read that does not ask in 3.0 without KIND, and an edit of what it handed
// over wrote 3.0 back. The stand converts: a GET without Accept is 3.0.
func TestCardReadsOfAConvertingServerKeepVCard4(t *testing.T) {
	const card = "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nKIND:individual\nEND:VCARD\n"
	for _, refuseQuery := range []bool{false, true} {
		p, srv := davProvider(t)
		srv.ConvertsCards, srv.RefuseCardQuery = true, refuseQuery
		srv.Seed(book+"ada.vcf", card)

		rc, _, err := p.t.Get(t.Context(), book+"ada.vcf", nil)
		if err != nil {
			t.Fatal(err)
		}
		bare, _ := io.ReadAll(rc)
		rc.Close()
		if !bytes.Contains(bare, []byte("VERSION:3.0")) || bytes.Contains(bare, []byte("KIND")) {
			t.Fatalf("the stand does not convert a GET without Accept:\n%s", bare)
		}

		reads := map[string]func() (*model.Object, error){
			"get": func() (*model.Object, error) { return p.Get(t.Context(), book+"ada.vcf") },
			"query": func() (*model.Object, error) {
				objs, err := p.Query(t.Context(), book)
				if err != nil || len(objs) != 1 {
					return nil, fmt.Errorf("%d cards, %v", len(objs), err)
				}
				return objs[0], nil
			},
			"uid": func() (*model.Object, error) { return p.FindUID(t.Context(), book, "ada") },
		}
		for name, read := range reads {
			obj, err := read()
			if err != nil {
				t.Errorf("refused query %v, %s: %v", refuseQuery, name, err)
				continue
			}
			if v := obj.PropText("VERSION"); len(v) != 1 || v[0] != "4.0" {
				t.Errorf("refused query %v, %s: VERSION %v, want 4.0", refuseQuery, name, v)
			}
			if k := obj.PropText("KIND"); len(k) != 1 || k[0] != "individual" {
				t.Errorf("refused query %v, %s: KIND %v — the card came converted", refuseQuery, name, k)
			}
		}
	}
}

// A server that answers the GET's Accept with 406 Not Acceptable is asked
// again without it, as a refused report is: the card it holds is read, in
// whatever version it gives.
func TestCardGetFallsBackWhenVersionIsRefused(t *testing.T) {
	var accepts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accepts = append(accepts, r.Header.Get("Accept"))
		if strings.Contains(r.Header.Get("Accept"), "version=4.0") {
			http.Error(w, "not acceptable", http.StatusNotAcceptable)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = io.WriteString(w, "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:ada\r\nFN:Ada\r\nEND:VCARD\r\n")
	}))
	t.Cleanup(srv.Close)
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
	obj, err := p.Get(t.Context(), "/book/ada.vcf")
	if err != nil {
		t.Fatal(err)
	}
	if UID(obj) != "ada" || obj.ETag != "v1" {
		t.Errorf("read %q at %q", UID(obj), obj.ETag)
	}
	if len(accepts) != 2 || !strings.Contains(accepts[0], "version=4.0") || accepts[1] != "" {
		t.Errorf("GETs asked %q, want 4.0 then nothing", accepts)
	}
}
