// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package calendar reads and writes calendar objects over CalDAV.
package calendar

import (
	"context"
	"encoding/json"
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

type Client interface {
	PropFind(context.Context, string, dav.Depth, []xml.Name) (*dav.MultiStatus, error)
	Get(context.Context, string, *dav.Range) (io.ReadCloser, string, error)
	Delete(context.Context, string, string) error
	Report(context.Context, string, dav.Depth, any) (*dav.MultiStatus, error)
	PutOpts(context.Context, string, io.Reader, dav.PutOptions) (string, error)
}

type Cache interface {
	NeedsRefresh(string, string, string) bool
	CTag(string, string) (string, bool)
	GetETags(string, string) (map[string]string, bool)
	SetETags(string, string, string, map[string]string)
	GetBody(string, string, string) ([]byte, bool)
	PutBody(string, string, string, []byte)
	InvalidateCollection(string, string)
}

const DefaultBatchSize = 50

type Options struct {
	AccountID             string
	Cache                 Cache
	Losses                *model.LossRegistry
	BatchSize             int
	SkipWriteVerification bool
	Location              *time.Location
}

type Provider struct {
	client    Client
	accountID string
	cache     Cache
	losses    *model.LossRegistry
	batchSize int
	verify    bool
	location  *time.Location
}

func New(client Client, opts Options) (*Provider, error) {
	if client == nil {
		return nil, errors.New("calendar: client is required")
	}
	batch := opts.BatchSize
	if batch <= 0 {
		batch = DefaultBatchSize
	}
	loc := opts.Location
	if loc == nil {
		loc = time.Local
	}
	return &Provider{
		client: client, accountID: opts.AccountID, cache: opts.Cache,
		losses: opts.Losses, batchSize: batch,
		verify: !opts.SkipWriteVerification, location: loc,
	}, nil
}

type Listing struct {
	Collection string
	CTag       string
	ETags      map[string]string
	FromCache  bool
}

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

func (p *Provider) List(ctx context.Context, collection string) (*Listing, error) {
	collection = normalizeCollection(collection)
	if collection == "" {
		return nil, errors.New("calendar: collection path is required")
	}
	cached, hasCached := p.cachedETags(collection)
	if hasCached && !p.cache.NeedsRefresh(p.accountID, collection, "") {
		return &Listing{Collection: collection, ETags: cached, CTag: p.cachedCTag(collection), FromCache: true}, nil
	}
	ctag, err := p.fetchCTag(ctx, collection)
	if err != nil {
		return nil, err
	}
	if hasCached && ctag != "" {
		if previous, ok := p.cache.CTag(p.accountID, collection); ok && previous == ctag {
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

func (p *Provider) fetchCTag(ctx context.Context, collection string) (string, error) {
	ms, err := p.client.PropFind(ctx, collection, dav.DepthZero, []xml.Name{dav.GetCTagName})
	if err != nil {
		return "", fmt.Errorf("calendar: read collection tag of %s: %w", collection, err)
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
		return nil, fmt.Errorf("calendar: list %s: %w", collection, err)
	}
	etags := make(map[string]string, len(ms.Responses))
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil || normalizeCollection(path) == collection {
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
		if value := strings.TrimSpace(etag.ETag); value != "" {
			etags[path] = value
		}
	}
	return etags, nil
}

type MultigetResult struct {
	Objects []*model.Object
	Failed  []ObjectFailure
}

type ObjectFailure struct {
	Path string
	Err  error
}

func (f ObjectFailure) Error() string { return fmt.Sprintf("%s: %v", f.Path, f.Err) }

func (p *Provider) Multiget(ctx context.Context, collection string, paths []string, etags map[string]string) (*MultigetResult, error) {
	collection = normalizeCollection(collection)
	result := &MultigetResult{}
	found := make(map[string]*model.Object, len(paths))
	var wanted []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		etag := etags[path]
		if body, ok := p.cachedBody(collection, path, etag); ok {
			obj, err := model.ParseICal(path, etag, body)
			if err != nil {
				result.Failed = append(result.Failed, ObjectFailure{Path: path, Err: err})
			} else {
				found[path] = obj
			}
			continue
		}
		wanted = append(wanted, path)
	}
	for start := 0; start < len(wanted); start += p.batchSize {
		end := start + p.batchSize
		if end > len(wanted) {
			end = len(wanted)
		}
		objects, failures, err := p.multigetBatch(ctx, collection, wanted[start:end])
		if err != nil {
			return nil, err
		}
		result.Failed = append(result.Failed, failures...)
		for _, obj := range objects {
			found[obj.Path] = obj
		}
	}
	for _, path := range paths {
		if obj := found[path]; obj != nil {
			result.Objects = append(result.Objects, obj)
		}
	}
	return result, nil
}

func (p *Provider) multigetBatch(ctx context.Context, collection string, paths []string) ([]*model.Object, []ObjectFailure, error) {
	ms, err := p.client.Report(ctx, collection, dav.DepthZero, dav.NewCalendarMultiget(paths))
	if err != nil {
		return nil, nil, fmt.Errorf("calendar: multiget in %s: %w", collection, err)
	}
	var objects []*model.Object
	var failures []ObjectFailure
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil {
			continue
		}
		var data dav.CalendarData
		if err := resp.DecodeProp(&data); err != nil {
			failures = append(failures, ObjectFailure{Path: path, Err: err})
			continue
		}
		var etag dav.GetETag
		_ = resp.DecodeProp(&etag)
		obj, err := model.ParseICal(path, strings.TrimSpace(etag.ETag), []byte(data.Data))
		if err != nil {
			failures = append(failures, ObjectFailure{Path: path, Err: err})
			continue
		}
		p.putCachedBody(collection, path, obj.ETag, []byte(data.Data))
		objects = append(objects, obj)
	}
	return objects, failures, nil
}

func (p *Provider) Get(ctx context.Context, collection, objectPath string) (*model.Object, error) {
	collection = normalizeCollection(collection)
	if objectPath == "" {
		return nil, errors.New("calendar: object path is required")
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
				return nil, fmt.Errorf("calendar: read %s: %w", objectPath, failure.Err)
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
		return nil, fmt.Errorf("calendar: read %s: %w", objectPath, err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("calendar: read %s: %w", objectPath, err)
	}
	obj, err := model.ParseICal(objectPath, etag, raw)
	if err != nil {
		return nil, err
	}
	p.putCachedBody(collection, objectPath, etag, raw)
	return obj, nil
}

func (p *Provider) fetchObjectETag(ctx context.Context, path string) (string, error) {
	ms, err := p.client.PropFind(ctx, path, dav.DepthZero, []xml.Name{dav.GetETagName})
	if err != nil {
		return "", fmt.Errorf("calendar: read version of %s: %w", path, err)
	}
	for _, resp := range ms.Responses {
		var etag dav.GetETag
		if err := resp.DecodeProp(&etag); err == nil {
			return strings.TrimSpace(etag.ETag), nil
		}
	}
	return "", nil
}

type Agenda struct {
	Collection  string
	From        time.Time
	To          time.Time
	Occurrences []model.Occurrence
	FromCache   bool
}

func (p *Provider) Query(ctx context.Context, collection string, from, to time.Time) (*Agenda, error) {
	collection = normalizeCollection(collection)
	if collection == "" {
		return nil, errors.New("calendar: collection path is required")
	}
	if !to.After(from) {
		return nil, errors.New("calendar: query end must be after start")
	}
	ctag := "notag"
	if p.cache != nil {
		previousETags, hadPrevious := p.cache.GetETags(p.accountID, collection)
		previousCTag, _ := p.cache.CTag(p.accountID, collection)
		listing, err := p.List(ctx, collection)
		if err != nil {
			return nil, err
		}
		if hadPrevious && (previousCTag != listing.CTag || !sameETags(previousETags, listing.ETags)) {
			p.cache.InvalidateCollection(p.accountID, collection)
			p.cache.SetETags(p.accountID, collection, listing.CTag, listing.ETags)
		}
		if listing.CTag != "" {
			ctag = listing.CTag
		}
	}
	key := rangeKey(from, to, ctag)
	if p.cache != nil {
		if raw, ok := p.cache.GetBody(p.accountID, collection, key); ok {
			var occurrences []model.Occurrence
			if err := json.Unmarshal(raw, &occurrences); err == nil {
				return &Agenda{Collection: collection, From: from, To: to, Occurrences: occurrences, FromCache: true}, nil
			}
		}
	}
	ms, err := p.client.Report(ctx, collection, dav.DepthZero, dav.NewCalendarQuery(from, to))
	if err != nil {
		return nil, fmt.Errorf("calendar: query %s: %w", collection, err)
	}
	var occurrences []model.Occurrence
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil {
			continue
		}
		var data dav.CalendarData
		if err := resp.DecodeProp(&data); err != nil {
			continue
		}
		var etag dav.GetETag
		_ = resp.DecodeProp(&etag)
		obj, err := model.ParseICal(path, strings.TrimSpace(etag.ETag), []byte(data.Data))
		if err != nil {
			continue
		}
		expanded, err := obj.ExpandOccurrences(from, to, p.location)
		if err != nil {
			return nil, fmt.Errorf("calendar: expand %s: %w", path, err)
		}
		occurrences = append(occurrences, expanded...)
	}
	sort.SliceStable(occurrences, func(i, j int) bool {
		if occurrences[i].Start.Equal(occurrences[j].Start) {
			return occurrences[i].Summary < occurrences[j].Summary
		}
		return occurrences[i].Start.Before(occurrences[j].Start)
	})
	if p.cache != nil {
		if raw, err := json.Marshal(occurrences); err == nil {
			p.cache.PutBody(p.accountID, collection, key, raw)
		}
	}
	return &Agenda{Collection: collection, From: from, To: to, Occurrences: occurrences}, nil
}

func rangeKey(from, to time.Time, ctag string) string {
	return "\x01range\x00" + from.UTC().Format(time.RFC3339Nano) + "\x00" +
		to.UTC().Format(time.RFC3339Nano) + "\x00" + ctag
}

func sameETags(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for path, etag := range a {
		if b[path] != etag {
			return false
		}
	}
	return true
}

func (p *Provider) cachedBody(collection, objectPath, etag string) ([]byte, bool) {
	if p.cache == nil || etag == "" {
		return nil, false
	}
	return p.cache.GetBody(p.accountID, collection, bodyKey(objectPath, etag))
}

func (p *Provider) putCachedBody(collection, objectPath, etag string, body []byte) {
	if p.cache != nil && etag != "" {
		p.cache.PutBody(p.accountID, collection, bodyKey(objectPath, etag), body)
	}
}

func bodyKey(objectPath, etag string) string { return objectPath + "\x00" + etag }

func normalizeCollection(path string) string {
	path = strings.TrimSpace(path)
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}
