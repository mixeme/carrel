package file

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/davtest"
)

// S-17 through a real WebDAV server: one PROPFIND, names, sizes, versions,
// folders marked, the folder itself left out.
func TestListReadsOneFolder(t *testing.T) {
	srv := davtest.New(t)
	docs := davtest.FileRoot + "docs/"
	srv.SeedFile(docs+"contract & terms.pdf", "0123456789")
	srv.SeedFile(docs+"sub/deep.md", "x")
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

	entries, err := p.List(t.Context(), docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	byName := map[string]Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	f := byName["contract & terms.pdf"]
	if f.Dir || f.Size != 10 || f.ETag == "" || strings.Contains(f.ETag, `"`) || f.Modified.IsZero() || f.Path != docs+"contract & terms.pdf" {
		t.Errorf("file = %+v", f)
	}
	if d := byName["sub"]; !d.Dir || d.Path != docs+"sub/" {
		t.Errorf("folder = %+v", d)
	}
	if n := srv.CountRequests("PROPFIND", docs); n != 1 {
		t.Errorf("PROPFIND requests = %d", n)
	}
}

// docs/servers.md, SV-48: a multistatus response may carry its own status
// instead of propstat blocks (RFC 4918 §14.24), and that is how Xandikos says
// a file is not there — 207, one response, 404, and an href that is the
// whole URL percent-encoded. Read without that status, the response had no
// properties and status 0, and Stat took the file for present: every free
// name looked taken, and a name check before an upload could not be trusted.
// The body below is Xandikos's answer as the server sent it.
func TestStatReadsAStatusThatIsNotInAPropstat(t *testing.T) {
	p := testProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Fatalf("method %q", r.Method)
		}
		w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<ns0:multistatus xmlns:ns0="DAV:"><ns0:response>`+
			`<ns0:href>http%3A//127.0.0.1%3A8081/user/statprobe/none.bin</ns0:href>`+
			`<ns0:status>HTTP/1.1 404 Not Found</ns0:status>`+
			`</ns0:response></ns0:multistatus>`)
	}))
	if _, err := p.Stat(t.Context(), "/user/statprobe/none.bin"); !NotFound(err) {
		t.Fatalf("Stat of a file the server says is not there = %v, want not found", err)
	}
}
