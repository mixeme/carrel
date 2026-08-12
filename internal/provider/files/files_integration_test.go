//go:build integration

// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package files

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
)

// liveProvider connects to the WebDAV server named in the environment
// (docs/dev-credentials.md). Without those variables the test skips: unit tests
// do not depend on a live server.
func liveProvider(t *testing.T) (*Provider, string) {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("CARREL_TEST_WEBDAV_URL"))
	user := strings.TrimSpace(os.Getenv("CARREL_TEST_WEBDAV_USER"))
	pass := os.Getenv("CARREL_TEST_WEBDAV_PASSWORD")
	if raw == "" || user == "" || pass == "" {
		t.Skip("CARREL_TEST_WEBDAV_URL, CARREL_TEST_WEBDAV_USER, CARREL_TEST_WEBDAV_PASSWORD not set")
	}
	cfg := dav.GuardConfig{
		ConnectTimeout:     15 * time.Second,
		RequestTimeout:     5 * time.Minute,
		MaxResponseBytes:   10 << 20,
		MaxRedirects:       5,
		InsecureSkipVerify: os.Getenv("CARREL_TEST_WEBDAV_ALLOW_INSECURE") == "1",
	}
	client, err := dav.NewClient(dav.NewGuard(cfg), raw, user, pass)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	p, err := New(client, Options{AccountID: "integration"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	root := NormalizeDir(parsed.Path)
	if root == "" {
		root = "/"
	}
	return p, root
}

// scratchDir is a folder of this run's own, removed afterwards, so a test never
// writes over anything already on the server.
func scratchDir(t *testing.T, p *Provider, root string) string {
	t.Helper()
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	rel := fmt.Sprintf("carrel-test-%x", suffix)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := p.MakeDir(ctx, root, rel); err != nil {
		t.Fatalf("MakeDir %s: %v", rel, err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := p.Remove(cleanup, root, rel, ""); err != nil {
			t.Logf("could not remove the scratch folder %s: %v", rel, err)
		}
	})
	return rel
}

func TestLiveListAndRoundTrip(t *testing.T) {
	p, root := liveProvider(t)
	dir := scratchDir(t, p, root)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	body := []byte("carrel integration\n")
	rel := Join(dir, "note.txt")
	if _, err := p.Upload(ctx, root, rel, strings.NewReader(string(body)), "text/plain", "", true); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	listing, err := p.List(ctx, root, dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listing.Entries) != 1 || listing.Entries[0].Name != "note.txt" {
		t.Fatalf("listing = %+v", listing.Entries)
	}
	download, err := p.Open(ctx, root, rel, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer download.Body.Close()
	got, err := io.ReadAll(download.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("downloaded %q, want %q", got, body)
	}

	// A create is conditional, so the second one is refused and the file stands.
	if _, err := p.Upload(ctx, root, rel, strings.NewReader("replacement"), "text/plain", "", true); err == nil {
		t.Fatal("a second conditional create succeeded; the first file may have been overwritten")
	}
}

// The gate of §7 and of the stage: ten megabytes through the process without it
// growing by ten megabytes. The body is generated and discarded as it flows, so a
// provider that buffered would show up as resident memory rather than as a
// failure to work.
func TestLiveLargeFileStreamsWithoutBuffering(t *testing.T) {
	p, root := liveProvider(t)
	dir := scratchDir(t, p, root)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	const size = 12 << 20
	rel := Join(dir, "large.bin")
	before := heapInUse()
	if _, err := p.Upload(ctx, root, rel, io.LimitReader(zeroes{}, size), "application/octet-stream", "", true); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	download, err := p.Open(ctx, root, rel, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	read, err := io.Copy(io.Discard, download.Body)
	download.Body.Close()
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if read != size {
		t.Fatalf("streamed %d bytes, want %d", read, size)
	}
	// A generous ceiling: the point is that the file does not land on the heap,
	// not that the allocator is idle.
	if grew := heapInUse() - before; grew > size/2 {
		t.Fatalf("heap grew by %d bytes for a %d byte file; something buffered it", grew, size)
	}

	// A range request gets the middle of it.
	part, err := p.Open(ctx, root, rel, &dav.Range{Start: 1000, End: 1099})
	if err != nil {
		t.Fatalf("ranged Open: %v", err)
	}
	defer part.Body.Close()
	chunk, err := io.ReadAll(part.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunk) != 100 {
		t.Fatalf("range returned %d bytes, want 100", len(chunk))
	}
}

func TestLiveEnsureDirAndUploadNew(t *testing.T) {
	p, root := liveProvider(t)
	dir := scratchDir(t, p, root)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	nested := Join(dir, "attachments/2026")
	if err := p.EnsureDir(ctx, root, nested); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	// Again, on a folder that now exists.
	if err := p.EnsureDir(ctx, root, nested); err != nil {
		t.Fatalf("EnsureDir twice: %v", err)
	}
	first, _, err := p.UploadNew(ctx, root, nested, "shot.png", strings.NewReader("one"), "image/png")
	if err != nil {
		t.Fatalf("UploadNew: %v", err)
	}
	second, _, err := p.UploadNew(ctx, root, nested, "shot.png", strings.NewReader("two"), "image/png")
	if err != nil {
		t.Fatalf("UploadNew again: %v", err)
	}
	if first != "shot.png" || second != "shot-2.png" {
		t.Fatalf("names = %q, %q; want the second to take a free name", first, second)
	}
}

// zeroes is an endless reader, so a large upload costs no memory to produce.
type zeroes struct{}

func (zeroes) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func heapInUse() int64 {
	var stats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&stats)
	return int64(stats.HeapInuse)
}
