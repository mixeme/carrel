// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package contacts

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// Client is the transport surface the contacts provider needs: the plain DAV
// methods of §7 plus REPORT and a conditional PUT.
type Client interface {
	PropFind(ctx context.Context, path string, depth dav.Depth, props []xml.Name) (*dav.MultiStatus, error)
	Get(ctx context.Context, path string, rng *dav.Range) (io.ReadCloser, string, error)
	Delete(ctx context.Context, path, ifMatch string) error
	Report(ctx context.Context, path string, depth dav.Depth, body any) (*dav.MultiStatus, error)
	PutOpts(ctx context.Context, path string, body io.Reader, opts dav.PutOptions) (string, error)
}

// Cache is the part of the session cache the provider uses (§12). A nil Cache
// is allowed and means every read goes to the server.
type Cache interface {
	NeedsRefresh(accountID, collectionPath, serverCTag string) bool
	CTag(accountID, collectionPath string) (string, bool)
	GetETags(accountID, collectionPath string) (map[string]string, bool)
	SetETags(accountID, collectionPath, ctag string, etags map[string]string)
	GetBody(accountID, collectionPath, objectPath string) ([]byte, bool)
	PutBody(accountID, collectionPath, objectPath string, body []byte)
	InvalidateCollection(accountID, collectionPath string)
}

// DefaultBatchSize is how many objects one addressbook-multiget asks for. A
// whole address book in one report is a single response big enough to matter on
// a book with photos, so the list is filled a batch at a time as it is scrolled
// (§13).
const DefaultBatchSize = 50

// Options configure a Provider.
type Options struct {
	// AccountID keys the session cache and the property-loss registry.
	AccountID string
	Cache     Cache
	Losses    *model.LossRegistry
	BatchSize int
	// SkipWriteVerification turns off the read-back after a write. It exists
	// for tests and for servers where the extra round trip is not wanted;
	// with it on, property loss goes unreported (§8).
	SkipWriteVerification bool
	Now                   func() time.Time
}

// Provider reads and writes the address objects of one account.
type Provider struct {
	client    Client
	accountID string
	cache     Cache
	losses    *model.LossRegistry
	batchSize int
	verify    bool
}

// New returns a provider over one DAV client.
func New(client Client, opts Options) (*Provider, error) {
	if client == nil {
		return nil, errors.New("contacts: client is required")
	}
	batch := opts.BatchSize
	if batch <= 0 {
		batch = DefaultBatchSize
	}
	return &Provider{
		client:    client,
		accountID: opts.AccountID,
		cache:     opts.Cache,
		losses:    opts.Losses,
		batchSize: batch,
		verify:    !opts.SkipWriteVerification,
	}, nil
}

// Listing is the state of one collection: which objects it holds and at which
// version.
type Listing struct {
	Collection string
	CTag       string
	// ETags maps object path to entity tag. The path is the key everywhere
	// else in the provider; the ETag is the precondition for changing it.
	ETags map[string]string
	// FromCache reports that the listing was answered without asking the
	// server for the object map (§12).
	FromCache bool
}

// Paths returns the object paths of a listing, sorted, so a caller can page
// through them in a stable order.
func (l *Listing) Paths() []string {
	if l == nil {
		return nil
	}
	out := make([]string, 0, len(l.ETags))
	for path := range l.ETags {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

// List returns the object map of a collection.
//
// The collection tag decides whether the server is asked at all: unchanged means
// the cached map still describes the collection and no PROPFIND is needed, which
// is what keeps a second visit to an address book off the wire (§12).
func (p *Provider) List(ctx context.Context, collection string) (*Listing, error) {
	collection = normalizeCollection(collection)
	if collection == "" {
		return nil, errors.New("contacts: collection path is required")
	}

	cached, hasCached := p.cachedETags(collection)
	if hasCached && !p.cache.NeedsRefresh(p.accountID, collection, "") {
		return &Listing{Collection: collection, ETags: cached, FromCache: true, CTag: p.cachedCTag(collection)}, nil
	}

	ctag, err := p.fetchCTag(ctx, collection)
	if err != nil {
		return nil, err
	}
	if hasCached && ctag != "" {
		if previous, ok := p.cache.CTag(p.accountID, collection); ok && previous == ctag {
			// Same version of the collection: keep the map and restamp it
			// rather than reading every ETag again.
			p.cache.SetETags(p.accountID, collection, ctag, cached)
			return &Listing{Collection: collection, CTag: ctag, ETags: cached, FromCache: true}, nil
		}
	}

	etags, err := p.fetchETags(ctx, collection)
	if err != nil {
		return nil, err
	}
	if p.cache != nil {
		p.cache.SetETags(p.accountID, collection, ctag, etags)
	}
	return &Listing{Collection: collection, CTag: ctag, ETags: etags}, nil
}

func (p *Provider) cachedETags(collection string) (map[string]string, bool) {
	if p.cache == nil {
		return nil, false
	}
	return p.cache.GetETags(p.accountID, collection)
}

func (p *Provider) cachedCTag(collection string) string {
	if p.cache == nil {
		return ""
	}
	ctag, _ := p.cache.CTag(p.accountID, collection)
	return ctag
}

// fetchCTag reads the collection tag. A server that does not publish one leaves
// the answer empty, and the cache falls back to its soft TTL (§12).
func (p *Provider) fetchCTag(ctx context.Context, collection string) (string, error) {
	ms, err := p.client.PropFind(ctx, collection, dav.DepthZero, []xml.Name{dav.GetCTagName})
	if err != nil {
		return "", fmt.Errorf("contacts: read collection tag of %s: %w", collection, err)
	}
	for _, resp := range ms.Responses {
		var ctag dav.GetCTag
		if err := resp.DecodeProp(&ctag); err == nil {
			return strings.TrimSpace(ctag.Tag), nil
		}
	}
	return "", nil
}

func (p *Provider) fetchETags(ctx context.Context, collection string) (map[string]string, error) {
	props := []xml.Name{dav.GetETagName, dav.ResourceTypeName, dav.GetContentTypeName}
	ms, err := p.client.PropFind(ctx, collection, dav.DepthOne, props)
	if err != nil {
		return nil, fmt.Errorf("contacts: list %s: %w", collection, err)
	}

	etags := make(map[string]string, len(ms.Responses))
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil {
			continue
		}
		if normalizeCollection(path) == collection {
			continue
		}
		var resType dav.ResourceType
		if err := resp.DecodeProp(&resType); err == nil && resType.Is(dav.CollectionName) {
			continue
		}
		var etag dav.GetETag
		if err := resp.DecodeProp(&etag); err != nil {
			continue
		}
		value := strings.TrimSpace(etag.ETag)
		if value == "" {
			continue
		}
		etags[path] = value
	}
	return etags, nil
}

// MultigetResult is what one batched read produced.
//
// Failures are carried alongside the objects rather than returned as one error:
// a single card this build cannot parse should cost that card, not the whole
// address book.
type MultigetResult struct {
	Objects []*model.Object
	Failed  []ObjectFailure
}

// ObjectFailure is one object that could not be read.
type ObjectFailure struct {
	Path string
	Err  error
}

func (f ObjectFailure) Error() string { return fmt.Sprintf("%s: %v", f.Path, f.Err) }

// Multiget fetches the given objects, in batches, from the cache where the ETag
// says the cached body is still current and from the server otherwise (§12,
// §13). The objects come back in the order the paths were given.
func (p *Provider) Multiget(ctx context.Context, collection string, paths []string, etags map[string]string) (*MultigetResult, error) {
	collection = normalizeCollection(collection)
	result := &MultigetResult{}
	if len(paths) == 0 {
		return result, nil
	}

	found := make(map[string]*model.Object, len(paths))
	var wanted []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		etag := etags[path]
		if body, ok := p.cachedBody(collection, path, etag); ok {
			obj, err := model.ParseVCard(path, etag, body)
			if err != nil {
				result.Failed = append(result.Failed, ObjectFailure{Path: path, Err: err})
				continue
			}
			found[path] = obj
			continue
		}
		wanted = append(wanted, path)
	}

	for start := 0; start < len(wanted); start += p.batchSize {
		end := start + p.batchSize
		if end > len(wanted) {
			end = len(wanted)
		}
		batch := wanted[start:end]
		objects, failures, err := p.multigetBatch(ctx, collection, batch)
		if err != nil {
			return nil, err
		}
		result.Failed = append(result.Failed, failures...)
		for _, obj := range objects {
			found[obj.Path] = obj
		}
	}

	for _, path := range paths {
		if obj, ok := found[path]; ok {
			result.Objects = append(result.Objects, obj)
		}
	}
	return result, nil
}

func (p *Provider) multigetBatch(ctx context.Context, collection string, paths []string) ([]*model.Object, []ObjectFailure, error) {
	report := dav.NewAddressBookMultiget(paths, dav.GetETagName, dav.AddressDataName)
	ms, err := p.client.Report(ctx, collection, dav.DepthZero, report)
	if err != nil {
		return nil, nil, fmt.Errorf("contacts: multiget in %s: %w", collection, err)
	}

	var (
		objects  []*model.Object
		failures []ObjectFailure
	)
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil {
			continue
		}
		var data dav.AddressData
		if err := resp.DecodeProp(&data); err != nil {
			failures = append(failures, ObjectFailure{Path: path, Err: err})
			continue
		}
		var etag dav.GetETag
		_ = resp.DecodeProp(&etag)

		obj, err := model.ParseVCard(path, strings.TrimSpace(etag.ETag), []byte(data.Data))
		if err != nil {
			failures = append(failures, ObjectFailure{Path: path, Err: err})
			continue
		}
		p.putCachedBody(collection, path, obj.ETag, []byte(data.Data))
		objects = append(objects, obj)
	}
	return objects, failures, nil
}

// Get reads one object with its ETag.
//
// It goes through a multiget because a plain GET carries the body but not the
// version, and without the version there is no precondition to write back with
// (§9). A server that will not answer the report is fallen back to the plain
// pair of requests.
func (p *Provider) Get(ctx context.Context, collection, objectPath string) (*model.Object, error) {
	collection = normalizeCollection(collection)
	if objectPath == "" {
		return nil, errors.New("contacts: object path is required")
	}

	objects, failures, err := p.multigetBatch(ctx, collection, []string{objectPath})
	if err == nil {
		for _, obj := range objects {
			if obj.Path == objectPath {
				return obj, nil
			}
		}
		for _, failure := range failures {
			if failure.Path == objectPath {
				return nil, fmt.Errorf("contacts: read %s: %w", objectPath, failure.Err)
			}
		}
	}
	return p.getDirect(ctx, collection, objectPath)
}

func (p *Provider) getDirect(ctx context.Context, collection, objectPath string) (*model.Object, error) {
	etag, err := p.fetchObjectETag(ctx, objectPath)
	if err != nil {
		return nil, err
	}
	body, _, err := p.client.Get(ctx, objectPath, nil)
	if err != nil {
		return nil, fmt.Errorf("contacts: read %s: %w", objectPath, err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("contacts: read %s: %w", objectPath, err)
	}
	obj, err := model.ParseVCard(objectPath, etag, raw)
	if err != nil {
		return nil, err
	}
	p.putCachedBody(collection, objectPath, obj.ETag, raw)
	return obj, nil
}

func (p *Provider) fetchObjectETag(ctx context.Context, objectPath string) (string, error) {
	ms, err := p.client.PropFind(ctx, objectPath, dav.DepthZero, []xml.Name{dav.GetETagName})
	if err != nil {
		return "", fmt.Errorf("contacts: read version of %s: %w", objectPath, err)
	}
	for _, resp := range ms.Responses {
		var etag dav.GetETag
		if err := resp.DecodeProp(&etag); err == nil {
			return strings.TrimSpace(etag.ETag), nil
		}
	}
	return "", nil
}

// cachedBody keys a body by path and ETag together, so a body cached under an
// older version of an object is a miss rather than a stale hit (§12).
func (p *Provider) cachedBody(collection, objectPath, etag string) ([]byte, bool) {
	if p.cache == nil || etag == "" {
		return nil, false
	}
	return p.cache.GetBody(p.accountID, collection, bodyKey(objectPath, etag))
}

func (p *Provider) putCachedBody(collection, objectPath, etag string, body []byte) {
	if p.cache == nil || etag == "" {
		return
	}
	p.cache.PutBody(p.accountID, collection, bodyKey(objectPath, etag), body)
}

func bodyKey(objectPath, etag string) string { return objectPath + "\x00" + etag }
