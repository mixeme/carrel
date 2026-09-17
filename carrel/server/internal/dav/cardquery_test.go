package dav

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func cardText(version, uid, extra string) string {
	return strings.Join([]string{"BEGIN:VCARD", "VERSION:" + version, "UID:" + uid, "FN:Ada & Co", extra, "END:VCARD", ""}, "\r\n")
}

// xmlPaths lists every element of a document as its path of local names, with
// the attributes that matter to a query, and the text under each path.
func xmlPaths(t *testing.T, doc []byte) map[string]string {
	t.Helper()
	out := map[string]string{}
	var stack []string
	dec := xml.NewDecoder(bytes.NewReader(doc))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("body %s: %v", doc, err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			label := el.Name.Local
			for _, a := range el.Attr {
				if a.Name.Space == "" && a.Name.Local != "xmlns" {
					label += "{" + a.Name.Local + "=" + a.Value + "}"
				}
			}
			stack = append(stack, label)
			key := strings.Join(stack, "/")
			if _, ok := out[key]; !ok {
				out[key] = ""
			}
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 && strings.TrimSpace(string(el)) != "" {
				out[strings.Join(stack, "/")] += string(el)
			}
		}
	}
}

// The card query asks for vCard 4.0 by name inside address-data, keeps the
// filter RFC 6352 §10.5 nests, and reads what servers send: a card escaped or
// in CDATA, an href percent-encoded, the modification time. Without a version
// address-data names none.
func TestQueryCardsNamesTheVersion(t *testing.T) {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(cardText("4.0", "ada", "KIND:individual")))
	reply := `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:carddav">
<d:response><d:href>/book/a%20b.vcf</d:href><d:propstat><d:prop><d:getetag>"v1"</d:getetag><d:getlastmodified>Tue, 15 Sep 2026 10:00:00 GMT</d:getlastmodified><c:address-data>` +
		escaped.String() + `</c:address-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
<d:response><d:href>/book/c.vcf</d:href><d:propstat><d:prop><d:getetag>"v2"</d:getetag><c:address-data><![CDATA[` +
		cardText("4.0", "cyd", "NOTE:second") + `]]></c:address-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
</d:multistatus>`

	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, b)
		if r.Header.Get("Depth") != "1" {
			t.Errorf("Depth %q", r.Header.Get("Depth"))
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	querier, ok := testClient(t, srv.URL).CardReporter().(CardQuerier)
	if !ok {
		t.Fatal("the client's CardReporter is no CardQuerier")
	}

	objs, err := querier.QueryCards(t.Context(), "/book/", CardQuery{
		Version: VCard4, AnyOf: true,
		Matches: []CardMatch{{Prop: "FN", Text: "ada & co"}, {Prop: "UID", Text: "ada", Equals: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	paths := xmlPaths(t, bodies[0])
	for key, text := range map[string]string{
		"addressbook-query/prop/getetag":                                                           "",
		"addressbook-query/prop/getlastmodified":                                                   "",
		"addressbook-query/prop/address-data{content-type=text/vcard}{version=4.0}":                "",
		"addressbook-query/filter{test=anyof}/prop-filter{name=FN}/text-match":                     "ada & co",
		"addressbook-query/filter{test=anyof}/prop-filter{name=UID}/text-match{match-type=equals}": "ada",
	} {
		if got, ok := paths[key]; !ok || got != text {
			t.Errorf("%s: %q, %v; body %s", key, got, ok, bodies[0])
		}
	}

	if len(objs) != 2 {
		t.Fatalf("objects = %+v", objs)
	}
	want := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	if objs[0].Path != "/book/a b.vcf" || objs[0].ETag != `"v1"` || !objs[0].ModTime.Equal(want) ||
		objs[0].Card.Value("KIND") != "individual" || objs[0].Card.Value("FN") != "Ada & Co" {
		t.Errorf("escaped card: %+v", objs[0])
	}
	if objs[1].Path != "/book/c.vcf" || objs[1].Card.Value("NOTE") != "second" {
		t.Errorf("CDATA card: %+v", objs[1])
	}

	if _, err := querier.QueryCards(t.Context(), "/book/", CardQuery{}); err != nil {
		t.Fatal(err)
	}
	// The XML declaration says version="1.0" too: look at the element.
	if _, ok := xmlPaths(t, bodies[1])["addressbook-query/prop/address-data"]; !ok {
		t.Errorf("a query without a version named one: %s", bodies[1])
	}
}
