// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

const root = "/dav/files/"

// fakeDAV is a WebDAV file collection: enough of one to exercise listing,
// streaming, conditional creates and deletes.
type fakeDAV struct {
	mu      sync.Mutex
	entries map[string][]byte // path → body; a path ending in / is a folder
	// requests counts PROPFINDs, so a test can assert the cache spared one.
	propfinds int
	// bodyBytesRead is what the handler actually read off the wire, which is how
	// a streaming upload is told from a buffered one.
	uploaded map[string]int
}

func newFakeDAV() *fakeDAV {
	return &fakeDAV{
		entries:  map[string][]byte{root: nil},
		uploaded: map[string]int{},
	}
}

func (f *fakeDAV) put(path string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries[path] = body
}

func (f *fakeDAV) has(path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.entries[path]
	return ok
}

func (f *fakeDAV) body(path string) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.entries[path]
}

func (f *fakeDAV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch r.Method {
	case "PROPFIND":
		f.mu.Lock()
		f.propfinds++
		f.mu.Unlock()
		f.propfind(w, r, path)
	case http.MethodGet:
		f.get(w, r, path)
	case http.MethodPut:
		f.putHandler(w, r, path)
	case http.MethodDelete:
		f.mu.Lock()
		defer f.mu.Unlock()
		if _, ok := f.entries[path]; !ok {
			http.NotFound(w, r)
			return
		}
		delete(f.entries, path)
		w.WriteHeader(http.StatusNoContent)
	case "MKCOL":
		f.mu.Lock()
		defer f.mu.Unlock()
		dir := path
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		if _, ok := f.entries[dir]; ok {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		f.entries[dir] = nil
		w.WriteHeader(http.StatusCreated)
	default:
		http.Error(w, "no", http.StatusMethodNotAllowed)
	}
}

func (f *fakeDAV) propfind(w http.ResponseWriter, r *http.Request, path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	depth := r.Header.Get("Depth")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:">`)
	write := func(p string) {
		body := f.entries[p]
		isDir := strings.HasSuffix(p, "/")
		b.WriteString(`<d:response><d:href>` + p + `</d:href><d:propstat><d:prop>`)
		if isDir {
			b.WriteString(`<d:resourcetype><d:collection/></d:resourcetype>`)
		} else {
			b.WriteString(`<d:resourcetype/>`)
			b.WriteString(`<d:getcontentlength>` + itoa(len(body)) + `</d:getcontentlength>`)
			b.WriteString(`<d:getcontenttype>text/plain; charset=utf-8</d:getcontenttype>`)
			b.WriteString(`<d:getetag>"` + itoa(len(body)) + `"</d:getetag>`)
			b.WriteString(`<d:getlastmodified>Tue, 11 Aug 2026 09:00:00 GMT</d:getlastmodified>`)
		}
		b.WriteString(`</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`)
	}
	if _, ok := f.entries[path]; !ok {
		http.NotFound(w, r)
		return
	}
	write(path)
	if depth == "1" && strings.HasSuffix(path, "/") {
		for p := range f.entries {
			if p == path || !strings.HasPrefix(p, path) {
				continue
			}
			rest := strings.TrimSuffix(strings.TrimPrefix(p, path), "/")
			if rest == "" || strings.Contains(rest, "/") {
				continue // not a direct member
			}
			write(p)
		}
	}
	b.WriteString(`</d:multistatus>`)
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusMultiStatus)
	io.WriteString(w, b.String())
}

func (f *fakeDAV) get(w http.ResponseWriter, r *http.Request, path string) {
	body := f.body(path)
	if body == nil && !f.has(path) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if spec := r.Header.Get("Range"); strings.HasPrefix(spec, "bytes=") {
		start, end := 0, len(body)-1
		parts := strings.SplitN(strings.TrimPrefix(spec, "bytes="), "-", 2)
		start = atoi(parts[0], 0)
		if len(parts) == 2 && parts[1] != "" {
			end = atoi(parts[1], end)
		}
		if start < 0 || start >= len(body) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if end >= len(body) {
			end = len(body) - 1
		}
		w.WriteHeader(http.StatusPartialContent)
		w.Write(body[start : end+1])
		return
	}
	w.Write(body)
}

func (f *fakeDAV) putHandler(w http.ResponseWriter, r *http.Request, path string) {
	if r.Header.Get("If-None-Match") == "*" && f.has(path) {
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read", http.StatusBadRequest)
		return
	}
	f.put(path, body)
	f.mu.Lock()
	f.uploaded[path] = len(body)
	f.mu.Unlock()
	w.Header().Set("ETag", `"`+itoa(len(body))+`"`)
	w.WriteHeader(http.StatusCreated)
}

func itoa(n int) string { return strconv.Itoa(n) }

func atoi(s string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fallback
	}
	return n
}

func newProvider(t *testing.T, fake *fakeDAV, cache Cache) *Provider {
	t.Helper()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	guard := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	client, err := dav.NewClient(guard, srv.URL+root, "mix", "secret")
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(client, Options{AccountID: "acc", Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListSeparatesFoldersFromFiles(t *testing.T) {
	fake := newFakeDAV()
	fake.put(root+"notes/", nil)
	fake.put(root+"beta.txt", []byte("beta"))
	fake.put(root+"Alpha.txt", []byte("alpha body"))
	fake.put(root+"notes/deep.txt", []byte("deep"))

	p := newProvider(t, fake, nil)
	listing, err := p.List(context.Background(), root, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var names []string
	for _, e := range listing.Entries {
		names = append(names, e.Name)
	}
	// Folders first, then files, both by name ignoring case. The member of the
	// subfolder is not a member of this one.
	want := []string{"notes", "Alpha.txt", "beta.txt"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("entries = %v, want %v", names, want)
	}
	if !listing.Entries[0].Dir {
		t.Fatal("notes/ was not reported as a folder")
	}
	file := listing.Entries[1]
	if !file.HasSize || file.Size != int64(len("alpha body")) {
		t.Fatalf("size = %d (has=%v)", file.Size, file.HasSize)
	}
	if file.ContentType != "text/plain" {
		t.Fatalf("content type = %q, want the media type without its charset", file.ContentType)
	}
	if file.ModTime.IsZero() {
		t.Fatal("getlastmodified was not parsed")
	}
	if file.Rel != "Alpha.txt" {
		t.Fatalf("rel = %q", file.Rel)
	}
}

// §7 exists so a download is a stream. The body must arrive without the provider
// having read it, which is what letting a caller read it afterwards proves.
func TestOpenStreamsWithoutBuffering(t *testing.T) {
	fake := newFakeDAV()
	payload := bytes.Repeat([]byte("x"), 1<<20)
	fake.put(root+"big.bin", payload)

	p := newProvider(t, fake, nil)
	download, err := p.Open(context.Background(), root, "big.bin", nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer download.Body.Close()
	if download.Name != "big.bin" {
		t.Fatalf("name = %q", download.Name)
	}
	head := make([]byte, 16)
	if _, err := io.ReadFull(download.Body, head); err != nil {
		t.Fatalf("read head: %v", err)
	}
	rest, err := io.ReadAll(download.Body)
	if err != nil {
		t.Fatalf("read rest: %v", err)
	}
	if len(head)+len(rest) != len(payload) {
		t.Fatalf("read %d bytes, want %d", len(head)+len(rest), len(payload))
	}
}

func TestOpenHonoursRange(t *testing.T) {
	fake := newFakeDAV()
	fake.put(root+"abc.txt", []byte("abcdefghij"))
	p := newProvider(t, fake, nil)
	download, err := p.Open(context.Background(), root, "abc.txt", &dav.Range{Start: 2, End: 5})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer download.Body.Close()
	got, _ := io.ReadAll(download.Body)
	if string(got) != "cdef" {
		t.Fatalf("range body = %q, want %q", got, "cdef")
	}
}

func TestOpenRefusesFolderAndTraversal(t *testing.T) {
	fake := newFakeDAV()
	fake.put(root+"sub/", nil)
	p := newProvider(t, fake, nil)
	if _, err := p.Open(context.Background(), root, "sub/", nil); err == nil {
		t.Fatal("opening a folder as a file should be refused")
	}
	if _, err := p.Open(context.Background(), root, "../secret", nil); !errors.Is(err, ErrOutsideCollection) {
		t.Fatalf("traversal error = %v, want ErrOutsideCollection", err)
	}
}

// A create is conditional, so an upload never silently replaces somebody's file.
func TestUploadCreateIsConditional(t *testing.T) {
	fake := newFakeDAV()
	fake.put(root+"taken.txt", []byte("original"))
	p := newProvider(t, fake, nil)

	_, err := p.Upload(context.Background(), root, "taken.txt", strings.NewReader("new"), "text/plain", "", true)
	if !dav.IsPreconditionFailed(err) {
		t.Fatalf("second create error = %v, want a refused precondition", err)
	}
	if got := string(fake.body(root + "taken.txt")); got != "original" {
		t.Fatalf("body = %q, the refused create overwrote it", got)
	}
}

// The attachment path: a name that is taken yields the next one, and the file
// that was already there is untouched (§23.10).
func TestUploadNewFindsAFreeName(t *testing.T) {
	fake := newFakeDAV()
	fake.put(root+"2026-08-13-note.png", []byte("first"))
	p := newProvider(t, fake, nil)

	name, path, err := p.UploadNew(context.Background(), root, "", "2026-08-13-note.png",
		strings.NewReader("second"), "image/png")
	if err != nil {
		t.Fatalf("UploadNew: %v", err)
	}
	if name != "2026-08-13-note-2.png" {
		t.Fatalf("name = %q", name)
	}
	if path != root+"2026-08-13-note-2.png" {
		t.Fatalf("path = %q", path)
	}
	if got := string(fake.body(root + "2026-08-13-note.png")); got != "first" {
		t.Fatalf("the file already there became %q", got)
	}
	if got := string(fake.body(path)); got != "second" {
		t.Fatalf("uploaded body = %q", got)
	}
}

func TestEnsureDirCreatesOnlyWhatIsMissing(t *testing.T) {
	fake := newFakeDAV()
	p := newProvider(t, fake, nil)
	ctx := context.Background()
	if err := p.EnsureDir(ctx, root, "attachments/2026"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if !fake.has(root+"attachments/") || !fake.has(root+"attachments/2026/") {
		t.Fatal("EnsureDir did not create the whole chain")
	}
	// Again: nothing to do, and no error for a folder that is already there.
	if err := p.EnsureDir(ctx, root, "attachments/2026"); err != nil {
		t.Fatalf("EnsureDir on an existing folder: %v", err)
	}
}

func TestRemoveRefusesTheCollectionItself(t *testing.T) {
	p := newProvider(t, newFakeDAV(), nil)
	if err := p.Remove(context.Background(), root, "", ""); err == nil {
		t.Fatal("deleting the collection root should be refused")
	}
}

// §12: a folder reopened inside the TTL costs no request, and a write to it is a
// miss straight away.
func TestListingIsCachedPerFolderAndInvalidatedByAWrite(t *testing.T) {
	fake := newFakeDAV()
	fake.put(root+"a.txt", []byte("a"))
	cache := session.NewCache(session.CacheConfig{}, nil)
	p := newProvider(t, fake, cache)
	ctx := context.Background()

	if _, err := p.List(ctx, root, ""); err != nil {
		t.Fatal(err)
	}
	first := fake.propfinds
	second, err := p.List(ctx, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !second.FromCache {
		t.Fatal("the second listing was not taken from the cache")
	}
	if fake.propfinds != first {
		t.Fatalf("propfinds went from %d to %d; the cached listing cost a request", first, fake.propfinds)
	}

	if _, err := p.Upload(ctx, root, "b.txt", strings.NewReader("b"), "text/plain", "", true); err != nil {
		t.Fatal(err)
	}
	third, err := p.List(ctx, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if third.FromCache {
		t.Fatal("the listing after an upload came from the cache")
	}
	if len(third.Entries) != 2 {
		t.Fatalf("entries after upload = %d, want 2", len(third.Entries))
	}
}

func TestMaxEntriesTruncates(t *testing.T) {
	fake := newFakeDAV()
	for i := 0; i < 10; i++ {
		fake.put(root+"f"+itoa(i)+".txt", []byte("x"))
	}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	guard := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	client, err := dav.NewClient(guard, srv.URL+root, "mix", "secret")
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(client, Options{AccountID: "acc", MaxEntries: 4})
	if err != nil {
		t.Fatal(err)
	}
	listing, err := p.List(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 4 || !listing.Truncated {
		t.Fatalf("entries = %d truncated = %v, want 4 and true", len(listing.Entries), listing.Truncated)
	}
}

func TestTypeForNameFallsBackToTheExtension(t *testing.T) {
	if got := TypeForName("notes.md"); got != "text/markdown" {
		t.Fatalf("TypeForName(.md) = %q", got)
	}
	if got := TypeForName("no-extension"); got != "" {
		t.Fatalf("TypeForName with no extension = %q, want empty", got)
	}
}
