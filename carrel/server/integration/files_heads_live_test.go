//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
)

// docs/servers.md, SV-57 (ФБ11, решение владельца 03.10.2026): what a folder
// holds comes from the folder's one PROPFIND where the server says it, and
// the size of an image from one ranged read of its head. Nextcloud counts
// the folders and the files inside each folder; SFTPGo and OpenCloud say
// nothing, and the folder has no count. Every server answered the range 206;
// the size is read either way.
func TestNextcloudFolderHeads(t *testing.T) {
	client, p, home := nextcloudFiles(t)
	said := true
	runFolderHeads(t, client, p, fmt.Sprintf("%scarrel-heads-%d/", home, os.Getpid()), &said)
}

func TestSFTPGoFolderHeads(t *testing.T) {
	client, p := sftpgoClient(t)
	said := false
	runFolderHeads(t, client, p, fmt.Sprintf("/carrel-heads-%d/", os.Getpid()), &said)
}

func TestOpenCloudFolderHeads(t *testing.T) {
	client, p, space := opencloudClient(t)
	said := false
	runFolderHeads(t, client, p, fmt.Sprintf("%s/carrel-heads-%d/", space, os.Getpid()), &said)
}

// Stalwart and Davis serve files beside CalDAV and CardDAV (TestStalwartFiles,
// TestDavisFiles). Probed on the Linux gates' stands 03.10.2026: neither
// counts, both answer the range 206 (SV-57).
func TestStalwartFolderHeads(t *testing.T) {
	endpoint, user, pass := stalwartConfig(t)
	said := false
	client, err := dav.NewClient(testGuard(t), endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := file.New(client)
	if err != nil {
		t.Fatal(err)
	}
	runFolderHeads(t, client, p, fmt.Sprintf("/dav/file/%s/carrel-heads-%d/", user, os.Getpid()), &said)
}

func TestDavisFolderHeads(t *testing.T) {
	endpoint, user, pass := davisConfig(t)
	said := false
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	client, err := dav.NewClient(testGuard(t), u.Scheme+"://"+u.Host+"/", user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := file.New(client)
	if err != nil {
		t.Fatal(err)
	}
	runFolderHeads(t, client, p, fmt.Sprintf("/dav/home/%s/carrel-heads-%d/", user, os.Getpid()), &said)
}

// runFolderHeads lays out a folder with three members and a camera's JPEG
// beside it, lists the parent once and reads the picture's head once. said
// is whether the server is known to count (nil — not probed yet: what it
// answers is logged, not held).
func runFolderHeads(t *testing.T, client *dav.Client, p *file.Provider, root string, said *bool) {
	t.Helper()
	ctx := context.Background()
	_ = client.Delete(ctx, root, "")
	removeLater(t, client, root)
	for _, dir := range []string{root, root + "sub/", root + "sub/inner/"} {
		if err := p.MkCol(ctx, dir); err != nil {
			t.Fatalf("mkcol %s: %v", dir, err)
		}
	}
	for _, name := range []string{"a.md", "b.md"} {
		seedFile(ctx, t, p, root+"sub/"+name, func() io.Reader { return strings.NewReader(name) })
	}
	picture := cameraPicture(t, 640, 480, 20000)
	seedFile(ctx, t, p, root+"photo.jpg", func() io.Reader { return bytes.NewReader(picture) })

	entries, err := p.List(ctx, root)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var sub *file.Entry
	for i := range entries {
		if entries[i].Name == "sub" {
			sub = &entries[i]
		}
	}
	if sub == nil {
		t.Fatalf("no sub in %+v", entries)
	}
	switch {
	case said == nil:
		t.Logf("sub holds %v by the server's own answer", sub.Items)
	case *said && (sub.Items == nil || *sub.Items != 3):
		t.Errorf("sub holds %v by the server's answer, want 3 (SV-57)", sub.Items)
	case !*said && sub.Items != nil:
		t.Errorf("the server now counts: sub holds %d — revisit SV-57", *sub.Items)
	}
	w, h, ok, err := p.Dimensions(ctx, root+"photo.jpg")
	if err != nil || !ok || w != 640 || h != 480 {
		t.Errorf("the picture's head = %d×%d %v %v, want 640×480", w, h, ok, err)
	}
	d, err := p.Open(ctx, root+"photo.jpg", &dav.Range{Start: 0, End: 99})
	if err != nil {
		t.Fatalf("ranged GET: %v", err)
	}
	d.Body.Close()
	if d.Status != 206 {
		t.Errorf("ranged GET answered %d, want 206 (SV-57: every server of the lineup did)", d.Status)
	}
}

// cameraPicture is a JPEG with an APP1 Exif segment of exif bytes before its
// frame header.
func cameraPicture(t *testing.T, w, h, exif int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	raw := b.Bytes()
	payload := append([]byte("Exif\x00\x00"), make([]byte, exif)...)
	out := append([]byte(nil), raw[:2]...)
	out = append(out, 0xFF, 0xE1)
	out = binary.BigEndian.AppendUint16(out, uint16(len(payload)+2))
	out = append(out, payload...)
	return append(out, raw[2:]...)
}
