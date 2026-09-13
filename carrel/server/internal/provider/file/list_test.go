package file

import (
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
