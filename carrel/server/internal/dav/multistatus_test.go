package dav

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const propFindReply = `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:response>
    <D:href>/cal/personal/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>Personal</D:displayname>
        <CS:getctag>ctag-17</CS:getctag>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
    <D:propstat>
      <D:prop>
        <D:getetag/>
      </D:prop>
      <D:status>HTTP/1.1 404 Not Found</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

// A PROPFIND is asked for property values, so the decoder has to carry them
// back: a reply reduced to bare names cannot name a collection or spot a
// changed ctag.
func TestPropFindKeepsPropertyValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(propFindReply))
	}))
	defer srv.Close()

	client := testClient(t, srv.URL)
	ms, err := client.PropFind(t.Context(), "", DepthOne, []xml.Name{
		{Space: "DAV:", Local: "displayname"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ms.Responses) != 1 {
		t.Fatalf("responses = %d", len(ms.Responses))
	}
	got := ms.Responses[0]
	if got.Href != "/cal/personal/" {
		t.Fatalf("href = %q", got.Href)
	}
	name := xml.Name{Space: "DAV:", Local: "displayname"}
	if v := string(got.Props[name]); v != "Personal" {
		t.Fatalf("displayname = %q, want %q", v, "Personal")
	}
	ctag := xml.Name{Space: "http://calendarserver.org/ns/", Local: "getctag"}
	if v := string(got.Props[ctag]); v != "ctag-17" {
		t.Fatalf("getctag = %q, want %q", v, "ctag-17")
	}
	// The trailing 404 propstat lists properties the resource does not have.
	// It must not be mistaken for the resource being missing, and its names
	// must not join the ones that were actually returned.
	if got.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", got.StatusCode)
	}
	if _, ok := got.Props[xml.Name{Space: "DAV:", Local: "getetag"}]; ok {
		t.Fatal("property from the 404 propstat was kept as present")
	}
}

// A reply of exactly CARREL_DAV_MAX_RESPONSE_BYTES is complete, not an
// overrun; one byte more is refused.
func TestResponseLimitBoundary(t *testing.T) {
	const limit = 4096
	for _, tc := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{"under", limit - 1, false},
		{"exact", limit, false},
		{"over", limit + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(strings.Repeat("x", tc.size)))
			}))
			defer srv.Close()

			c := testConfig()
			c.DAVSSRFAllowlist = "127.0.0.1"
			c.DAVMaxResponseBytes = limit
			client, err := NewClient(NewGuard(c), srv.URL, "", "", "")
			if err != nil {
				t.Fatal(err)
			}
			rc, _, err := client.Get(t.Context(), "", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()

			buf := make([]byte, 1024)
			var read int
			for {
				n, err := rc.Read(buf)
				read += n
				if err != nil {
					if tc.wantErr {
						if err != ErrResponseTooLarge {
							t.Fatalf("err = %v, want ErrResponseTooLarge", err)
						}
						return
					}
					if err.Error() != "EOF" {
						t.Fatalf("err = %v, want EOF", err)
					}
					if read != tc.size {
						t.Fatalf("read %d bytes, want %d", read, tc.size)
					}
					return
				}
			}
		})
	}
}

func testClient(t *testing.T, endpoint string) *Client {
	t.Helper()
	c := testConfig()
	c.DAVSSRFAllowlist = "127.0.0.1"
	client, err := NewClient(NewGuard(c), endpoint, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}
