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
	"carrel.dev/carrel/server/internal/model"
)

// docs/servers.md, SV-61: Google keeps a colon in a card's value escaped —
// «item2.URL;TYPE=PREF:https\://ada.example», «X-SOCIALPROFILE:https\://…»
// (the live matrix, 16.11.2026). RFC 6350 §3.4 escapes only the backslash,
// the comma, the semicolon and the line end, and go-vcard leaves «\:» as it
// is: the card showed «https\://ada.example», no link, the form the same,
// and a new address saved was reported not kept — Carrel sent «https://»
// and read back «https\://». «\:» is a colon.
func TestAnEscapedColonIsAColon(t *testing.T) {
	doc := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:g\r\nFN:Ada\r\nN:Ada;;;;\r\n" +
		"item2.URL;TYPE=PREF:https\\://ada.example\r\nitem2.X-ABLabel:Work\r\n" +
		"X-SOCIALPROFILE;TYPE=TWITTER:https\\://x.example/ada\r\nNOTE:a\\\\:b\r\nEND:VCARD\r\n"
	obj, err := Parse("/book/g.vcf", "v1", strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	for prop, want := range map[string]string{"URL": "https://ada.example", "X-SOCIALPROFILE": "https://x.example/ada", "NOTE": "a\\:b"} {
		if got := obj.PropText(prop); len(got) != 1 || got[0] != want {
			t.Errorf("%s = %q, want %q", prop, got, want)
		}
	}
	// A list reads the card as the record does: the record and the form of
	// it come from one read, the check before a save from the other, and two
	// reads of one card refused every save of a Google card with a web page.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		var data bytes.Buffer
		xml.EscapeText(&data, []byte(doc))
		io.WriteString(w, `<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:carddav"><d:response><d:href>/book/g.vcf</d:href><d:propstat><d:prop><d:getetag>"v1"</d:getetag><c:address-data>`+
			data.String()+`</c:address-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
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
	objs, err := p.Query(t.Context(), "/book/")
	if err != nil || len(objs) != 1 {
		t.Fatalf("query: %d cards, %v", len(objs), err)
	}
	if got := objs[0].PropText("URL"); len(got) != 1 || got[0] != "https://ada.example" {
		t.Errorf("the list reads URL %q", got)
	}

	// Sent as «https://» and read back as the server keeps it: no loss.
	sent := model.NewObject("/book/g.vcf", `"1"`, map[string][]model.RawValue{"URL": {{Text: "https://ada.example", Group: "item2", Params: map[string]string{"TYPE": "PREF"}}}})
	got := model.NewObject("/book/g.vcf", `"2"`, map[string][]model.RawValue{"URL": obj.Props()["URL"]})
	if loss := model.CompareAfterPut("google", sent, got, nil); loss != nil {
		t.Errorf("loss %+v", loss)
	}
}
