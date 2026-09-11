//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
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

func radicaleEndpoint(t *testing.T) string {
	t.Helper()
	if u := os.Getenv("CARREL_RADICALE_URL"); u != "" {
		return u
	}
	return "http://127.0.0.1:5232/test/"
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

	_ = client.Delete(ctx, "/cal/", "")
	_ = client.Delete(ctx, "/contacts/", "")

	resp, err := davRequest(ctx, g, "MKCOL", base+"/cal/", "")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("MKCOL /cal/: %s", resp.Status)
	}

	calBody := `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set>
    <D:prop>
      <D:displayname>Personal</D:displayname>
      <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
    </D:prop>
  </D:set>
</C:mkcalendar>`
	resp, err = davRequest(ctx, g, "MKCALENDAR", base+"/cal/personal/", calBody)
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

	result, trace, err := discover.Run(context.Background(), testGuard(t), endpoint, radicaleUser, radicalePass)
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

func TestRadicalePutIfMatch(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	client := testClient(t, endpoint)
	ctx := context.Background()
	path := "/note.txt"
	_ = client.Delete(ctx, path, "")

	etag, err := client.Put(ctx, path, strings.NewReader("first"), "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := client.Put(ctx, path, strings.NewReader("second"), `"wrong"`); err == nil {
		t.Fatal("expected wrong If-Match to fail")
	}
	if _, err := client.Put(ctx, path, strings.NewReader("third"), `"`+etag+`"`); err != nil {
		t.Fatalf("update with etag: %v", err)
	}
}

func TestRadicaleGetStreams(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	client := testClient(t, endpoint)
	ctx := context.Background()
	path := "/stream.bin"
	payload := bytes.Repeat([]byte("x"), 256*1024)
	_ = client.Delete(ctx, path, "")
	if _, err := client.Put(ctx, path, bytes.NewReader(payload), ""); err != nil {
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
