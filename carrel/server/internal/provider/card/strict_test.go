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

// Б3, its rest: go-vcard skips a line it cannot take apart — no colon after
// the name, an open quote — and reads the card without it: the field is gone
// from the screen, and nothing says so. A calendar record with such a line is
// refused (go-ical), and a card is now too: «invalid vCard», named with the
// line, as every other card Carrel cannot read (v0.2.390).

const noColon = "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:nc\r\nFN:No Colon\r\nNOTE the line has no colon\r\nEND:VCARD\r\n"

func TestACardWithALineWithoutAColonIsNotRead(t *testing.T) {
	_, err := Parse("/book/nc.vcf", "v1", strings.NewReader(noColon))
	if err == nil || !strings.Contains(err.Error(), "NOTE the line has no colon") {
		t.Errorf("Parse: %v, want the line named", err)
	}
	// A card without any such line reads as before; so does a vCard 2.1
	// value broken with «=», whose next line has no colon of its own.
	for _, ok := range []string{
		vcf("ok", "Ok"),
		"BEGIN:VCARD\r\nVERSION:2.1\r\nUID:qp\r\nFN:Qp\r\nNOTE;ENCODING=QUOTED-PRINTABLE:first half=\r\n and the second\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:2.1\r\nUID:qp\r\nFN:Qp\r\nNOTE;ENCODING=QUOTED-PRINTABLE:first half=\r\nand the second\r\nEND:VCARD\r\n",
	} {
		if _, err := Parse("/book/ok.vcf", "v1", strings.NewReader(ok)); err != nil {
			t.Errorf("a healthy card: %v\n%s", err, ok)
		}
	}
}

func TestAnImportedCardWithALineWithoutAColonIsBroken(t *testing.T) {
	objs, broken, err := Split([]byte(vcf("ada", "Ada") + noColon))
	if len(objs) != 1 || len(broken) != 1 || err != nil {
		t.Fatalf("objs %d, broken %+v, err %v", len(objs), broken, err)
	}
	if broken[0].Name != "No Colon" || !strings.Contains(broken[0].Reason, "NOTE the line has no colon") {
		t.Errorf("broken = %+v", broken[0])
	}
}

func TestABookReadPastACardWithALineWithoutAColon(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, `<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:carddav">`)
		for _, item := range []struct{ name, data string }{{"ada", vcf("ada", "Ada")}, {"nc", noColon}} {
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
	objs, err := p.Query(t.Context(), "/book/")
	if len(objs) != 1 || UID(objs[0]) != "ada" {
		t.Fatalf("read %d cards, want Ada alone; error %v", len(objs), err)
	}
	if err == nil || !strings.Contains(err.Error(), "/book/nc.vcf") {
		t.Errorf("the card is not named: %v", err)
	}
}
