//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
)

// S-30.2 / S-17 on OpenCloud: the second file server after SFTPGo, and the
// first whose address is a space — a path of its own under the host, not the
// root of it. Every path Carrel builds has to carry that prefix, and a
// collection that is not at "/" is what this test holds. The codes are the
// ones the transfer queue reads: a taken folder answers 405, a taken name
// answers 412, and Stat is what a conditional write asks first.
//
// The ceiling lifted from the body of a GET is held by TestSFTPGoFiles; here
// the file is small enough to stay under it, because what is new is the
// prefix, not the size.

func opencloudConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_OPENCLOUD_URL")
	user = os.Getenv("CARREL_OPENCLOUD_USER")
	pass = os.Getenv("CARREL_OPENCLOUD_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_OPENCLOUD_URL, CARREL_OPENCLOUD_USER, CARREL_OPENCLOUD_PASS required")
	}
	return endpoint, user, pass
}

// opencloudClient dials the space and says where it sits: the paths below are
// read from the root of the host, so they all begin with that.
func opencloudClient(t *testing.T) (*dav.Client, *file.Provider, string) {
	endpoint, user, pass := opencloudConfig(t)
	client, err := dav.NewClient(testGuard(t), endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := file.New(client)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	return client, p, strings.TrimSuffix(u.Path, "/")
}

func TestOpenCloudFiles(t *testing.T) {
	client, p, space := opencloudClient(t)
	ctx := context.Background()
	root := fmt.Sprintf("%s/carrel-oc-%d/", space, os.Getpid())
	_ = client.Delete(ctx, root, "")
	removeLater(t, client, root)

	if err := p.MkCol(ctx, root); err != nil {
		t.Fatalf("mkcol %s: %v", root, err)
	}
	if err := p.MkCol(ctx, root); !dav.IsStatus(err, http.StatusMethodNotAllowed) {
		t.Errorf("MKCOL of a taken name: %v, want 405", err)
	}

	note := root + "note.txt"
	if _, err := p.UploadCreate(ctx, note, strings.NewReader("first")); err != nil {
		t.Fatalf("create: %v", err)
	}
	// This server drops the precondition the way SFTPGo does (SV-8), so the
	// second write goes through and Carrel looks a name up before writing
	// instead of trusting If-None-Match. The day it answers 412, this fails and
	// the entry can be revisited.
	if _, err := p.UploadCreate(ctx, note, strings.NewReader("second!")); err != nil {
		t.Errorf("PUT of a taken name: %v, want it overwritten (docs/servers.md, SV-8)", err)
	}
	e, err := p.Stat(ctx, note)
	if err != nil || e.Size != int64(len("second!")) {
		t.Errorf("stat of a taken name: %+v %v", e, err)
	}
	if _, err := p.Stat(ctx, root+"none.txt"); !file.NotFound(err) {
		t.Errorf("stat of a free name: %v, want 404", err)
	}

	// A file goes out as it came in, prefix and all.
	const size = 3 << 20
	big := root + "big.bin"
	if _, err := p.UploadOverwrite(ctx, big, strings.NewReader(strings.Repeat("z", size))); err != nil {
		t.Fatalf("put: %v", err)
	}
	d, err := p.Open(ctx, big, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	n, err := io.Copy(io.Discard, d.Body)
	d.Body.Close()
	if err != nil || n != size {
		t.Fatalf("read %d of %d: %v", n, size, err)
	}

	// MOVE is the rename and the move inside one storage; without overwrite a
	// taken name is 412, which is the question the queue asks about.
	moved := root + "moved.txt"
	if err := p.Move(ctx, note, moved, false); err != nil {
		t.Fatalf("move to a free name: %v", err)
	}
	if _, err := p.UploadCreate(ctx, note, strings.NewReader("again")); err != nil {
		t.Fatalf("create after move: %v", err)
	}
	if err := p.Move(ctx, note, moved, false); !file.ErrOccupied(err) {
		t.Errorf("MOVE onto a taken name without overwrite: %v, want 412", err)
	}

	// A folder copies with its tree: GET and PUT a file at a time (S-17.1).
	tree, copyOf := root+"tree/", root+"tree-copy/"
	if err := p.MkCol(ctx, tree); err != nil {
		t.Fatalf("mkcol tree: %v", err)
	}
	if _, err := p.UploadCreate(ctx, tree+"one.md", strings.NewReader("1")); err != nil {
		t.Fatalf("seed tree: %v", err)
	}
	if n, err := file.CopyTree(ctx, p, tree, p, copyOf, 100); err != nil || n != 1 {
		t.Fatalf("copy tree: %d entries, %v", n, err)
	}
	rc, _, err := p.Get(ctx, copyOf+"one.md", nil)
	if err != nil {
		t.Fatalf("get the copy: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "1" {
		t.Errorf("the copy holds %q", got)
	}
}
