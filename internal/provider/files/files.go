// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package files

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
)

// Client is the slice of the DAV transport a file collection needs. Unlike the
// contacts and calendar providers this one uses MKCOL and streaming PUT, and
// never REPORT: a plain collection answers no reports.
type Client interface {
	PropFind(ctx context.Context, path string, depth dav.Depth, props []xml.Name) (*dav.MultiStatus, error)
	Get(ctx context.Context, path string, rng *dav.Range) (io.ReadCloser, string, error)
	PutOpts(ctx context.Context, path string, body io.Reader, opts dav.PutOptions) (string, error)
	Delete(ctx context.Context, path, ifMatch string) error
	MkCol(ctx context.Context, path string) error
}

// Cache is the listing half of the session cache (§12).
//
// Only directory listings go in. Bodies are deliberately left out: a file
// section that kept what it served would turn the per-session memory ceiling of
// §12 into a suggestion, and a file is not reparsed on every view the way a
// vCard is, so there is nothing to save.
//
// A directory is a DAV collection, so it is cached as one: the key is the
// directory path, which gives per-directory TTL and invalidation for free.
type Cache interface {
	NeedsRefresh(accountID, collectionPath, serverCTag string) bool
	GetBody(accountID, collectionPath, objectPath string) ([]byte, bool)
	PutBody(accountID, collectionPath, objectPath string, body []byte)
	SetETags(accountID, collectionPath, ctag string, etags map[string]string)
	InvalidateCollection(accountID, collectionPath string)
}

// Options configure a provider.
type Options struct {
	AccountID string
	Cache     Cache
	// MaxEntries caps how many members of one directory are reported. A folder
	// with tens of thousands of files is not something this section is for
	// (§23.10), and a listing that large is an unbounded page as well as an
	// unbounded PROPFIND.
	MaxEntries int
}

// DefaultMaxEntries is the per-directory ceiling when Options leaves it unset.
const DefaultMaxEntries = 2000

// Provider reads and writes one WebDAV account's file collections.
type Provider struct {
	client     Client
	accountID  string
	cache      Cache
	maxEntries int
}

// New returns a provider over client.
func New(client Client, opts Options) (*Provider, error) {
	if client == nil {
		return nil, errors.New("files: client is required")
	}
	max := opts.MaxEntries
	if max <= 0 {
		max = DefaultMaxEntries
	}
	return &Provider{
		client:     client,
		accountID:  opts.AccountID,
		cache:      opts.Cache,
		maxEntries: max,
	}, nil
}

// Entry is one member of a collection.
type Entry struct {
	// Path is the absolute DAV path, decoded.
	Path string `json:"path"`
	// Rel is the path inside the collection root the listing was made against.
	Rel  string `json:"rel"`
	Name string `json:"name"`
	// Dir marks a collection. Its resourcetype says so; the absence of
	// getcontentlength is only a hint and is not relied on.
	Dir         bool      `json:"dir,omitempty"`
	Size        int64     `json:"size,omitempty"`
	HasSize     bool      `json:"has_size,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
	ETag        string    `json:"etag,omitempty"`
	ModTime     time.Time `json:"mod_time,omitempty"`
}

// Listing is one directory of one collection.
type Listing struct {
	// Root is the collection from discovery; Rel is the directory inside it.
	Root    string  `json:"root"`
	Rel     string  `json:"rel"`
	Dir     string  `json:"dir"`
	Entries []Entry `json:"entries"`
	// Truncated says the directory holds more than MaxEntries members, so the
	// list on screen is not the whole of it.
	Truncated bool `json:"truncated,omitempty"`
	FromCache bool `json:"-"`
}

var listProps = []xml.Name{
	dav.ResourceTypeName,
	dav.DisplayNameName,
	dav.GetContentLengthName,
	dav.GetContentTypeName,
	dav.GetETagName,
	dav.GetLastModifiedName,
}

// List reads one directory. Folders come first and then files, both by name,
// because a listing sorted by anything else is a listing nobody can scan.
func (p *Provider) List(ctx context.Context, root, rel string) (*Listing, error) {
	dir, err := Resolve(root, rel)
	if err != nil {
		return nil, err
	}
	dir = NormalizeDir(dir)
	clean, _ := CleanRelative(rel)

	if cached, ok := p.cachedListing(dir); ok {
		cached.FromCache = true
		return cached, nil
	}

	ms, err := p.client.PropFind(ctx, dir, dav.DepthOne, listProps)
	if err != nil {
		return nil, fmt.Errorf("files: list %s: %w", dir, err)
	}
	out := &Listing{Root: NormalizeDir(root), Rel: clean, Dir: dir}
	etags := make(map[string]string)
	for _, resp := range ms.Responses {
		entryPath, err := resp.Path()
		if err != nil {
			continue
		}
		if NormalizeDir(entryPath) == dir {
			continue
		}
		entry, ok := decodeEntry(resp, entryPath)
		if !ok {
			continue
		}
		entry.Rel = Join(clean, entry.Name)
		if len(out.Entries) >= p.maxEntries {
			out.Truncated = true
			break
		}
		if entry.ETag != "" {
			etags[entry.Path] = entry.ETag
		}
		out.Entries = append(out.Entries, entry)
	}
	sortEntries(out.Entries)
	p.storeListing(dir, etags, out)
	return out, nil
}

// Stat reads the metadata of one member without its body.
//
// A path is asked for as it was given and then, if that is not there, with a
// trailing slash. Servers disagree about whether a collection answers under both
// forms — some redirect, some answer, some do not — and the caller does not know
// which of the two a name is until the answer comes back.
func (p *Provider) Stat(ctx context.Context, root, rel string) (Entry, error) {
	entry, err := p.stat(ctx, root, rel)
	if err != nil && dav.IsNotFound(err) && !strings.HasSuffix(rel, "/") {
		if asDir, dirErr := p.stat(ctx, root, rel+"/"); dirErr == nil {
			return asDir, nil
		}
	}
	return entry, err
}

func (p *Provider) stat(ctx context.Context, root, rel string) (Entry, error) {
	target, err := Resolve(root, rel)
	if err != nil {
		return Entry{}, err
	}
	if strings.HasSuffix(rel, "/") {
		target = NormalizeDir(target)
	}
	ms, err := p.client.PropFind(ctx, target, dav.DepthZero, listProps)
	if err != nil {
		return Entry{}, fmt.Errorf("files: read %s: %w", target, err)
	}
	for _, resp := range ms.Responses {
		entryPath, err := resp.Path()
		if err != nil {
			continue
		}
		entry, ok := decodeEntry(resp, entryPath)
		if !ok {
			continue
		}
		if clean, relErr := Relative(root, entryPath); relErr == nil {
			entry.Rel = clean
		}
		return entry, nil
	}
	return Entry{}, fmt.Errorf("files: %s is not there", target)
}

// Download is an open stream and what is known about it. The caller closes Body.
type Download struct {
	Body        io.ReadCloser
	Name        string
	ContentType string
	Size        int64
	HasSize     bool
	ETag        string
}

// Open starts a download. The body is the server's, unbuffered: §7 exists so
// this method can hand a stream to the browser rather than a slice to the heap.
func (p *Provider) Open(ctx context.Context, root, rel string, rng *dav.Range) (*Download, error) {
	target, err := Resolve(root, rel)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(target, "/") {
		return nil, errors.New("files: that is a folder, not a file")
	}
	return p.openPath(ctx, target, Base(rel), rng)
}

// OpenAbsolute starts a download of a path already checked against a collection.
// It exists for attachments, whose path comes out of an `ATTACH` URI rather than
// out of the URL of the request (§23.10).
func (p *Provider) OpenAbsolute(ctx context.Context, target string, rng *dav.Range) (*Download, error) {
	if target == "" || !strings.HasPrefix(target, "/") {
		return nil, errors.New("files: absolute path is required")
	}
	return p.openPath(ctx, target, path.Base(target), rng)
}

func (p *Provider) openPath(ctx context.Context, target, name string, rng *dav.Range) (*Download, error) {
	body, ctype, err := p.client.Get(ctx, target, rng)
	if err != nil {
		return nil, fmt.Errorf("files: read %s: %w", target, err)
	}
	return &Download{Body: body, Name: name, ContentType: ctype}, nil
}

// Upload streams a body into the collection.
//
// A create carries `If-None-Match: *` so it cannot silently replace a file
// somebody else put there; a replace carries the version it was read at, the
// same precondition every other write in Carrel uses (§9).
func (p *Provider) Upload(ctx context.Context, root, rel string, body io.Reader, contentType, ifMatch string, create bool) (string, error) {
	target, err := Resolve(root, rel)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(target, "/") {
		return "", errors.New("files: a file needs a name")
	}
	if contentType == "" {
		contentType = TypeForName(Base(rel))
	}
	etag, err := p.client.PutOpts(ctx, target, body, dav.PutOptions{
		ContentType: contentType,
		IfMatch:     ifMatch,
		IfNoneMatch: create && ifMatch == "",
	})
	if err != nil {
		return "", fmt.Errorf("files: write %s: %w", target, err)
	}
	p.invalidate(parentOf(target))
	return strings.TrimSpace(etag), nil
}

// UploadNew stores a body under a name that is free, and returns the name it
// settled on with the path it was written to.
//
// The free name is found by asking the server rather than by listing first: a
// conditional create is refused when something is already there, and a refusal
// is the answer. Listing and then writing would be two requests and still race.
func (p *Provider) UploadNew(ctx context.Context, root, dirRel, filename string, body io.ReadSeeker, contentType string) (string, string, error) {
	name, err := CleanName(filename)
	if err != nil {
		return "", "", err
	}
	stem, ext := splitExt(name)
	for attempt := 0; attempt < 20; attempt++ {
		candidate := name
		if attempt > 0 {
			candidate = fmt.Sprintf("%s-%d%s", stem, attempt+1, ext)
		}
		if _, err := body.Seek(0, io.SeekStart); err != nil {
			return "", "", err
		}
		rel := Join(dirRel, candidate)
		target, resolveErr := Resolve(root, rel)
		if resolveErr != nil {
			return "", "", resolveErr
		}
		_, err := p.Upload(ctx, root, rel, body, contentType, "", true)
		if err == nil {
			return candidate, target, nil
		}
		if !isTaken(err) {
			return "", "", err
		}
	}
	return "", "", errors.New("files: could not find a free name for the upload")
}

// MakeDir creates one collection.
func (p *Provider) MakeDir(ctx context.Context, root, rel string) error {
	target, err := Resolve(root, rel)
	if err != nil {
		return err
	}
	if NormalizeDir(target) == NormalizeDir(root) {
		return errors.New("files: a folder needs a name")
	}
	if err := p.client.MkCol(ctx, NormalizeDir(target)); err != nil {
		return fmt.Errorf("files: create folder %s: %w", target, err)
	}
	p.invalidate(parentOf(NormalizeDir(target)))
	return nil
}

// EnsureDir creates a collection when it is not there yet. It is how the
// attachment folder of §23.10 comes into being: the folder is named once in the
// settings and the person is not then asked to go and create it.
func (p *Provider) EnsureDir(ctx context.Context, root, rel string) error {
	clean, err := CleanRelative(rel)
	if err != nil {
		return err
	}
	if clean == "" {
		return nil
	}
	if entry, err := p.Stat(ctx, root, clean); err == nil {
		if !entry.Dir {
			return fmt.Errorf("files: %s is a file, not a folder", clean)
		}
		return nil
	} else if !dav.IsNotFound(err) {
		return err
	}
	if parent, ok := Parent(clean); ok && parent != "" {
		if err := p.EnsureDir(ctx, root, parent); err != nil {
			return err
		}
	}
	// A server that says the collection is already there has done what was
	// asked, whatever it thinks of being asked twice.
	if err := p.MakeDir(ctx, root, clean); err != nil && !isTaken(err) {
		return err
	}
	return nil
}

// Remove deletes one member. A folder is removed with everything in it, which
// is what DELETE on a collection means, so the interface confirms first.
func (p *Provider) Remove(ctx context.Context, root, rel, ifMatch string) error {
	target, err := Resolve(root, rel)
	if err != nil {
		return err
	}
	if NormalizeDir(target) == NormalizeDir(root) {
		return errors.New("files: the collection itself cannot be deleted")
	}
	if err := p.client.Delete(ctx, target, ifMatch); err != nil {
		return fmt.Errorf("files: delete %s: %w", target, err)
	}
	p.invalidate(parentOf(target))
	p.invalidate(NormalizeDir(target))
	return nil
}

// Invalidate drops the cached listing of one directory, which is what the
// refresh button of §12 does to a file view.
func (p *Provider) Invalidate(root, rel string) {
	if target, err := Resolve(root, rel); err == nil {
		p.invalidate(NormalizeDir(target))
	}
}

func (p *Provider) invalidate(dir string) {
	if p.cache != nil && dir != "" {
		p.cache.InvalidateCollection(p.accountID, dir)
	}
}

// listingKey names the cached listing inside the directory's cache entry. The
// leading control byte keeps it clear of any real member path.
const listingKey = "\x01listing"

func (p *Provider) cachedListing(dir string) (*Listing, bool) {
	if p.cache == nil || p.cache.NeedsRefresh(p.accountID, dir, "") {
		return nil, false
	}
	raw, ok := p.cache.GetBody(p.accountID, dir, listingKey)
	if !ok {
		return nil, false
	}
	var out Listing
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return &out, true
}

func (p *Provider) storeListing(dir string, etags map[string]string, listing *Listing) {
	if p.cache == nil {
		return
	}
	raw, err := json.Marshal(listing)
	if err != nil {
		return
	}
	// SetETags stamps the entry with the time it was read at, which is what
	// gives the listing the soft TTL of §12; PutBody alone would not.
	p.cache.SetETags(p.accountID, dir, "", etags)
	p.cache.PutBody(p.accountID, dir, listingKey, raw)
}

func decodeEntry(resp dav.Response, entryPath string) (Entry, bool) {
	var resType dav.ResourceType
	if err := resp.DecodeProp(&resType); err != nil && !dav.IsNotFound(err) {
		return Entry{}, false
	}
	entry := Entry{
		Path: entryPath,
		Dir:  resType.Is(dav.CollectionName),
		Name: nameOf(entryPath),
	}
	if entry.Name == "" {
		return Entry{}, false
	}
	if entry.Dir {
		entry.Path = NormalizeDir(entryPath)
	}
	var length dav.GetContentLength
	if err := resp.DecodeProp(&length); err == nil {
		entry.Size, entry.HasSize = length.Bytes()
	}
	var ctype dav.GetContentType
	if err := resp.DecodeProp(&ctype); err == nil {
		if t, _, perr := mime.ParseMediaType(strings.TrimSpace(ctype.Type)); perr == nil {
			entry.ContentType = t
		}
	}
	if entry.ContentType == "" && !entry.Dir {
		entry.ContentType = TypeForName(entry.Name)
	}
	var etag dav.GetETag
	if err := resp.DecodeProp(&etag); err == nil {
		entry.ETag = strings.TrimSpace(etag.ETag)
	}
	var modified dav.GetLastModified
	if err := resp.DecodeProp(&modified); err == nil {
		if at, ok := modified.Time(); ok {
			entry.ModTime = at
		}
	}
	return entry, true
}

// nameOf is the last segment of a DAV path. Hrefs arrive decoded, but a server
// that hands back an encoded one is common enough to be worth undoing.
func nameOf(p string) string {
	trimmed := strings.TrimSuffix(p, "/")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		trimmed = trimmed[i+1:]
	}
	if decoded, err := url.PathUnescape(trimmed); err == nil {
		trimmed = decoded
	}
	return trimmed
}

func parentOf(target string) string {
	trimmed := strings.TrimSuffix(target, "/")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		return trimmed[:i+1]
	}
	return "/"
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		a, b := strings.ToLower(entries[i].Name), strings.ToLower(entries[j].Name)
		if a == b {
			return entries[i].Name < entries[j].Name
		}
		return a < b
	})
}

// isTaken reports whether a conditional create was refused because something is
// already at that path.
func isTaken(err error) bool {
	return dav.IsPreconditionFailed(err) || dav.StatusCode(err) == 405
}

func splitExt(name string) (string, string) {
	ext := path.Ext(name)
	return strings.TrimSuffix(name, ext), ext
}

// TypeForName guesses a media type from an extension, for a server that reports
// none. It is a guess for display only: what is actually served to a browser is
// sent with `nosniff` and an explicit `Content-Disposition` (§24.4).
func TypeForName(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if ext == "" {
		return ""
	}
	if t := mime.TypeByExtension(ext); t != "" {
		if parsed, _, err := mime.ParseMediaType(t); err == nil {
			return parsed
		}
		return t
	}
	switch ext {
	case ".md", ".markdown":
		return "text/markdown"
	case ".ics":
		return "text/calendar"
	case ".vcf":
		return "text/vcard"
	}
	return ""
}
