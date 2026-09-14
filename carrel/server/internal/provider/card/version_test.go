package card

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
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
