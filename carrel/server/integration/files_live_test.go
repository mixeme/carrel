//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
)

// S-30.2 / S-17 on SFTPGo: a file past CARREL_DAV_MAX_RESPONSE_BYTES arrives
// whole; a taken name is found by Stat, because this server ignores
// If-None-Match on PUT; MOVE without overwrite and MKCOL of a taken name are
// refused with the codes the queue relies on; a folder copies with its tree.
func TestSFTPGoFiles(t *testing.T) {
	client, p := sftpgoClient(t)
	ctx := context.Background()
	root := fmt.Sprintf("/carrel-files-%d/", os.Getpid())
	_ = client.Delete(ctx, root, "")
	removeLater(t, client, root)
	if err := p.MkCol(ctx, root); err != nil {
		t.Fatalf("mkcol: %v", err)
	}
	if err := p.MkCol(ctx, root); !dav.IsStatus(err, http.StatusMethodNotAllowed) {
		t.Errorf("MKCOL of a taken name: %v", err)
	}

	const big = 11 << 20
	seedFile(ctx, t, p, root+"big.bin", func() io.Reader { return strings.NewReader(strings.Repeat("z", big)) })
	d, err := p.Open(ctx, root+"big.bin", nil)
	if err != nil {
		t.Fatal(err)
	}
	n, err := io.Copy(io.Discard, d.Body)
	d.Body.Close()
	if err != nil || n != big {
		t.Fatalf("download read %d of %d: %v", n, big, err)
	}

	if _, err := p.Stat(ctx, root+"big.bin"); err != nil {
		t.Errorf("stat of a taken name: %v", err)
	}
	if _, err := p.Stat(ctx, root+"none.bin"); !file.NotFound(err) {
		t.Errorf("stat of a free name: %v", err)
	}

	if _, err := p.UploadCreate(ctx, root+"a.txt", strings.NewReader("a")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.UploadCreate(ctx, root+"b.txt", strings.NewReader("b")); err != nil {
		t.Fatal(err)
	}
	if err := p.Move(ctx, root+"a.txt", root+"b.txt", false); !file.ErrOccupied(err) {
		t.Errorf("MOVE onto a taken name without overwrite: %v", err)
	}
	if err := p.Move(ctx, root+"a.txt", root+"c.txt", false); err != nil {
		t.Errorf("MOVE to a free name: %v", err)
	}

	if err := p.MkCol(ctx, root+"tree/"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.UploadCreate(ctx, root+"tree/one.md", strings.NewReader("1")); err != nil {
		t.Fatal(err)
	}
	if n, err := file.CopyTree(ctx, p, root+"tree/", p, root+"tree-copy/", 100); err != nil || n != 1 {
		t.Fatalf("copy tree: %d, %v", n, err)
	}
	rc, _, err := p.Get(ctx, root+"tree-copy/one.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "1" {
		t.Errorf("copied one.md = %q", got)
	}
}
