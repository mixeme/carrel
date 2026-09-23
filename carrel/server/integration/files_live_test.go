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
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/view"
)

// S-30.2 / S-17 on SFTPGo: a file past CARREL_DAV_MAX_RESPONSE_BYTES arrives
// whole; a taken name is found by Stat, because this server ignores
// If-None-Match on PUT; MOVE without overwrite and MKCOL of a taken name are
// refused with the codes the queue relies on; a folder copies with its tree.
func TestSFTPGoFiles(t *testing.T) {
	client, p := sftpgoClient(t)
	runFiles(t, client, p, fmt.Sprintf("/carrel-files-%d/", os.Getpid()))
}

// runFiles is S-17 against whatever server answers: the same checks, a root
// of the caller's choosing. It is not SFTPGo's alone — Stalwart serves
// calendars, address books and files from one engine, which no other server
// in the lineup does, so plain WebDAV there is worth its own run.
func runFiles(t *testing.T, client *dav.Client, p *file.Provider, root string) {
	t.Helper()
	ctx := context.Background()
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

// S-17.1 live: a folder uploaded through the pages arrives as a tree. The
// browser names every part by its path inside the chosen folder, the service
// creates the folders it needs before the files, and each file gets a row of
// its own in the queue. A taken name is still a question and nothing is
// overwritten (S-17.3).
func TestSFTPGoFolderUpload(t *testing.T) {
	endpoint, user, pass := sftpgoConfig(t)
	client, p := sftpgoClient(t)
	ctx := context.Background()
	root := fmt.Sprintf("/carrel-folder-%d-%d/", os.Getpid(), time.Now().Unix()%100000)
	_ = client.Delete(ctx, root, "")
	if err := p.MkCol(ctx, root); err != nil {
		t.Fatalf("mkcol %s: %v", root, err)
	}
	removeLater(t, client, root)
	// A folder and a file in it, for the clash the upload runs into below.
	if err := p.MkCol(ctx, root+"trip/"); err != nil {
		t.Fatalf("mkcol %strip/: %v", root, err)
	}
	if _, err := p.UploadCreate(ctx, root+"trip/taken.txt", strings.NewReader("original")); err != nil {
		t.Fatalf("seed the taken name: %v", err)
	}

	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	col := liveFileCollection(t, app, root)
	list := "/app/files/" + connID + "/" + url.PathEscape(col) + "/"
	t.Logf("uploading into %s", list)

	rec := app.uploadFiles(list+"upload/", map[string]string{
		"trip/a.txt":      "first",
		"trip/2020/b.txt": "second",
		"loose.txt":       "flat",
		"trip/taken.txt":  "mine",
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("the upload answered %d: %s", rec.Code, rec.Body.String())
	}
	for path, want := range map[string]string{
		root + "trip/a.txt":      "first",
		root + "trip/2020/b.txt": "second",
		root + "loose.txt":       "flat",
		root + "trip/taken.txt":  "original", // never overwritten
	} {
		if got := liveFileBody(ctx, t, p, path); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}

	page := liveDecode[view.FilesPage](t, "the folder after the upload", app.get(list))
	if page.Queue == nil {
		t.Fatal("no queue after a folder upload")
	}
	states := map[string]view.OpState{}
	for _, row := range page.Queue.Rows {
		states[string(row.Name)] = row.State
	}
	if states["trip/2020/b.txt"] != view.OpDone || states["loose.txt"] != view.OpDone ||
		states["trip/taken.txt"] != view.OpIdle {
		t.Errorf("queue rows = %+v", states)
	}
}

// liveFileCollection is the collection the catalog holds for this folder. A
// file server comes as one collection per folder under its root (S-6), so the
// folder the test made is a collection of its own.
func liveFileCollection(t *testing.T, app *liveApp, root string) string {
	t.Helper()
	want := strings.TrimSuffix(root, "/")
	var paths []string
	for _, c := range liveCatalog(t, app).Collections {
		if !c.Has(view.KindFile) {
			continue
		}
		paths = append(paths, c.Path)
		if strings.TrimSuffix(c.Path, "/") == want {
			return c.Path
		}
	}
	t.Fatalf("no file collection at %s; the catalog holds %v", root, paths)
	return ""
}

func liveFileBody(ctx context.Context, t *testing.T, p *file.Provider, path string) string {
	t.Helper()
	d, err := p.Open(ctx, path, nil)
	if err != nil {
		return ""
	}
	defer d.Body.Close()
	body, err := io.ReadAll(d.Body)
	if err != nil {
		return ""
	}
	return string(body)
}
