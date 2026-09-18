//go:build integration

package integration

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/view"
)

const (
	radicaleUser = "test"
	radicalePass = "test"
)

// radicaleEndpoint is the address of the Radicale of the Linux machine, and it
// skips the test when nothing is listening there. Radicale runs in
// docker-compose beside the gates of that machine and nowhere else, so on any
// other machine the whole set used to fail — fourteen red lines that say
// nothing, in which a real failure is easy to lose. Baikal and SoGo skip
// themselves when the credentials name no server; this is the same courtesy
// for a server named by a default. A server that answers is tested: only a
// refused connection is a skip, never a wrong answer.
func radicaleEndpoint(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("CARREL_RADICALE_URL")
	if addr == "" {
		addr = "http://127.0.0.1:5232/test/"
	}
	u, err := url.Parse(addr)
	if err != nil {
		t.Fatalf("CARREL_RADICALE_URL = %q: %v", addr, err)
	}
	host := u.Host
	if u.Port() == "" {
		host = u.Hostname() + ":80"
		if u.Scheme == "https" {
			host = u.Hostname() + ":443"
		}
	}
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		t.Skipf("no Radicale at %s (%v): it runs on the Linux machine, with scripts/linux-gates.sh", addr, err)
	}
	_ = conn.Close()
	return addr
}

func testGuard(t *testing.T) *dav.Guard {
	t.Helper()
	c, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	c.DAVSSRFAllowlist = "127.0.0.1,localhost"
	c.DAVConnectTimeout = 5 * time.Second
	c.DAVRequestTimeout = 30 * time.Second
	return dav.NewGuard(c)
}

func testClient(t *testing.T, endpoint string) *dav.Client {
	t.Helper()
	client, err := dav.NewClient(testGuard(t), endpoint, radicaleUser, radicalePass, "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func davRequest(ctx context.Context, g *dav.Guard, method, url, body string) (*http.Response, error) {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	}
	req.SetBasicAuth(radicaleUser, radicalePass)
	return g.HTTPClient().Do(req)
}

func resetRadicale(t *testing.T, endpoint string) {
	t.Helper()
	ctx := context.Background()
	g := testGuard(t)
	client := testClient(t, endpoint)
	base := strings.TrimRight(endpoint, "/")

	// Paths are relative to the home: dav.Client resolves a leading "/" from
	// the host root, where Radicale sees another principal and answers 403.
	_ = client.Delete(ctx, "cal/", "")
	_ = client.Delete(ctx, "contacts/", "")

	// Radicale's default owner_only rights allow calendars and address books
	// directly in the home and nothing deeper: no plain folder in between.
	calBody := `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set>
    <D:prop>
      <D:displayname>Personal</D:displayname>
      <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
    </D:prop>
  </D:set>
</C:mkcalendar>`
	resp, err := davRequest(ctx, g, "MKCALENDAR", base+"/cal/", calBody)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("MKCALENDAR: %s", resp.Status)
	}

	bookBody := `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set>
    <D:prop>
      <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
      <D:displayname>Contacts</D:displayname>
    </D:prop>
  </D:set>
</D:mkcol>`
	resp, err = davRequest(ctx, g, "MKCOL", base+"/contacts/", bookBody)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("MKCOL contacts: %s", resp.Status)
	}
}

func TestRadicaleDiscoveryFindsBothKinds(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	resetRadicale(t, endpoint)

	result, trace, err := discover.Run(context.Background(), testGuard(t), endpoint, radicaleUser, radicalePass, "")
	if err != nil {
		t.Fatal(err)
	}
	if !trace.OK {
		t.Fatalf("trace not ok: %+v", trace.Steps)
	}
	var cal, card int
	for _, c := range result.Collections {
		for _, k := range c.Kinds {
			switch k {
			case view.KindEvent, view.KindTask, view.KindNote:
				cal++
			case view.KindContact:
				card++
			}
		}
	}
	if cal == 0 || card == 0 {
		t.Fatalf("collections = %+v, want calendar and address book", result.Collections)
	}
}

const calEventPath = "cal/event.ics"

func icsEvent(uid, summary string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Carrel//Integration//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20260101T120000Z\r\nDTSTART:20260101T100000Z\r\n" +
		"SUMMARY:" + summary + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

// largeICS is one event with a long folded DESCRIPTION: Radicale refuses
// several VEVENTs with different UIDs in one resource (400).
func largeICS(minBytes int) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Carrel//Integration//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:large@carrel.test\r\nDTSTAMP:20260101T120000Z\r\n" +
		"DTSTART:20260101T100000Z\r\nSUMMARY:Large\r\nDESCRIPTION:x")
	chunk := "\r\n " + strings.Repeat("x", 73)
	for b.Len() < minBytes {
		b.WriteString(chunk)
	}
	b.WriteString("\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	return b.String()
}

func TestRadicalePutIfMatch(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	resetRadicale(t, endpoint)
	client := testClient(t, endpoint)
	ctx := context.Background()
	path := calEventPath
	_ = client.Delete(ctx, path, "")

	etag, err := client.Put(ctx, path, strings.NewReader(icsEvent("put-if-match@carrel.test", "first")), "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := client.Put(ctx, path, strings.NewReader(icsEvent("put-if-match@carrel.test", "second")), `"wrong"`); err == nil {
		t.Fatal("expected wrong If-Match to fail")
	}
	if _, err := client.Put(ctx, path, strings.NewReader(icsEvent("put-if-match@carrel.test", "third")), `"`+etag+`"`); err != nil {
		t.Fatalf("update with etag: %v", err)
	}
}

func TestRadicaleGetStreams(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	resetRadicale(t, endpoint)
	client := testClient(t, endpoint)
	ctx := context.Background()
	path := "cal/large.ics"
	payload := largeICS(256 * 1024)
	_ = client.Delete(ctx, path, "")
	if _, err := client.Put(ctx, path, strings.NewReader(payload), ""); err != nil {
		t.Fatalf("put: %v", err)
	}
	body, etag, err := client.Get(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if etag == "" {
		t.Fatal("missing etag")
	}
	got, err := io.ReadAll(io.LimitReader(body, 4096))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4096 {
		t.Fatalf("read %d bytes, want 4096", len(got))
	}
}
