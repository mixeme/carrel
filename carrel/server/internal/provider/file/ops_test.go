package file

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/davtest"
)

func davFiles(t *testing.T) (*davtest.Server, *Provider) {
	t.Helper()
	srv := davtest.New(t)
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
	return srv, p
}

// S-17.1 through a real WebDAV server: stat, folder, move, conditional and
// unconditional PUT, delete with the listed version, and copies of a file and
// of a folder, the body streamed.
func TestFileOperations(t *testing.T) {
	srv, p := davFiles(t)
	ctx := t.Context()
	docs := davtest.FileRoot + "docs/"
	srv.SeedFile(docs+"a.txt", "alpha")
	srv.SeedFile(docs+"b.txt", "beta")
	srv.SeedFile(docs+"tree/one.md", "1")
	srv.SeedFile(docs+"tree/deep/two.md", "22")

	e, err := p.Stat(ctx, docs+"a.txt")
	if err != nil || e.Dir || e.Size != 5 || e.Name != "a.txt" || !strings.HasPrefix(e.Type, "text/plain") || e.ETag == "" {
		t.Fatalf("stat = %+v, %v", e, err)
	}
	if d, err := p.Stat(ctx, docs+"tree/"); err != nil || !d.Dir || d.Path != docs+"tree/" {
		t.Errorf("stat folder = %+v, %v", d, err)
	}
	if _, err := p.Stat(ctx, docs+"none.txt"); !NotFound(err) {
		t.Errorf("stat of nothing: %v", err)
	}

	if err := p.MkCol(ctx, docs+"new/"); err != nil {
		t.Fatal(err)
	}
	if err := p.MkCol(ctx, docs+"new/"); !dav.IsStatus(err, http.StatusMethodNotAllowed) {
		t.Errorf("MKCOL again: %v", err)
	}

	if err := p.Move(ctx, docs+"a.txt", docs+"b.txt", false); !ErrOccupied(err) {
		t.Errorf("move onto a taken name: %v", err)
	}
	if got, _ := srv.File(docs + "b.txt"); got != "beta" {
		t.Errorf("b.txt = %q after a refused move", got)
	}
	if _, err := p.UploadCreate(ctx, docs+"b.txt", strings.NewReader("new")); !ErrOccupied(err) {
		t.Errorf("create on a taken name: %v", err)
	}
	if _, err := p.UploadOverwrite(ctx, docs+"b.txt", strings.NewReader("replaced")); err != nil {
		t.Fatal(err)
	}
	if got, _ := srv.File(docs + "b.txt"); got != "replaced" {
		t.Errorf("b.txt = %q after overwrite", got)
	}
	if err := p.Move(ctx, docs+"a.txt", docs+"new/a.txt", false); err != nil {
		t.Fatal(err)
	}
	if _, ok := srv.File(docs + "a.txt"); ok {
		t.Error("a.txt stayed after a move")
	}

	stale, _ := p.Stat(ctx, docs+"b.txt")
	time.Sleep(10 * time.Millisecond)
	srv.SeedFile(docs+"b.txt", "changed by another client")
	if err := p.Delete(ctx, docs+"b.txt", stale.ETag); !dav.IsStatus(err, http.StatusPreconditionFailed) {
		t.Errorf("delete with a stale version: %v", err)
	}
	fresh, _ := p.Stat(ctx, docs+"b.txt")
	if err := p.Delete(ctx, docs+"b.txt", fresh.ETag); err != nil {
		t.Fatal(err)
	}
	if _, ok := srv.File(docs + "b.txt"); ok {
		t.Error("b.txt stayed after delete")
	}

	if err := Copy(ctx, p, docs+"new/a.txt", p, docs+"copy.txt", false); err != nil {
		t.Fatal(err)
	}
	if err := Copy(ctx, p, docs+"new/a.txt", p, docs+"copy.txt", false); !ErrOccupied(err) {
		t.Errorf("copy onto a taken name: %v", err)
	}
	if got, _ := srv.File(docs + "copy.txt"); got != "alpha" {
		t.Errorf("copy = %q", got)
	}

	n, err := CopyTree(ctx, p, docs+"tree/", p, docs+"tree copy/", 100)
	if err != nil || n != 3 {
		t.Fatalf("copy tree: %d, %v", n, err)
	}
	if one, _ := srv.File(docs + "tree copy/one.md"); one != "1" {
		t.Errorf("one.md = %q", one)
	}
	if two, _ := srv.File(docs + "tree copy/deep/two.md"); two != "22" {
		t.Errorf("deep/two.md = %q", two)
	}
	if n, err := CopyTree(ctx, p, docs+"tree/", p, docs+"tree copy/", 100); n != 0 || !dav.IsStatus(err, http.StatusMethodNotAllowed) {
		t.Errorf("copy tree onto a taken name: %d, %v", n, err)
	}
	if n, err := CopyTree(ctx, p, docs+"tree/", p, docs+"short/", 1); n != 1 || !errors.Is(err, ErrTooMany) {
		t.Errorf("copy tree past the cap: %d, %v", n, err)
	}
}

func TestSuffixed(t *testing.T) {
	for in, want := range map[string]string{
		"report.pdf":     "report-2.pdf",
		"archive.tar.gz": "archive.tar-2.gz",
		"Folder":         "Folder-2",
		".bashrc":        ".bashrc-2",
	} {
		if got := Suffixed(in, 2); got != want {
			t.Errorf("Suffixed(%q) = %q, want %q", in, got, want)
		}
	}
}

// A listing holds the members of the folder and nothing the server names
// outside it or deeper in it: a name from there would leave the folder.
func TestListKeepsTheFolderOnly(t *testing.T) {
	p := testProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		resp := func(href string) string {
			return `<D:response><D:href>` + href + `</D:href><D:propstat><D:prop><D:resourcetype/></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`
		}
		_, _ = io.WriteString(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">`+
			resp("/docs/")+resp("/docs/ok.txt")+resp("/etc/passwd")+resp("/docs/deep/x.txt")+resp("/docs/a%2Fb")+`</D:multistatus>`)
	}))
	entries, err := p.List(t.Context(), "/docs/")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "ok.txt" {
		t.Fatalf("entries = %+v", entries)
	}
}

// Get of a file is not cut by the response cap (C-12): the provider was
// passing a capped stream, and a file over CARREL_DAV_MAX_RESPONSE_BYTES never
// arrived whole.
func TestGetIsNotCapped(t *testing.T) {
	body := strings.Repeat("y", 2<<20)
	p := testProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	rc, _, err := p.Get(t.Context(), "/big.bin", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if n, err := io.Copy(io.Discard, rc); err != nil || n != int64(len(body)) {
		t.Fatalf("read %d of %d: %v", n, len(body), err)
	}
}
