// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// ObjectSet is what a component query returns: whole objects rather than the
// expanded occurrences an agenda wants, because a task or a note has nothing to
// expand.
type ObjectSet struct {
	Collection string
	Component  string
	Objects    []*model.Object
	// FromCache reports that nothing left the process to answer this, which
	// §16 asks the progress panel to distinguish.
	FromCache bool
}

// SearchProps are the properties a cross-source search looks at in a calendar
// collection (§16).
var SearchProps = []string{"SUMMARY", "DESCRIPTION"}

// QueryComponent reports the objects of one component kind in a collection,
// optionally limited to [from, to). Tasks and notes pass a zero range.
func (p *Provider) QueryComponent(ctx context.Context, collection, component string, from, to time.Time) (*ObjectSet, error) {
	collection = normalizeCollection(collection)
	if collection == "" {
		return nil, errors.New("calendar: collection path is required")
	}
	component = strings.ToUpper(strings.TrimSpace(component))
	if component == "" {
		return nil, errors.New("calendar: component name is required")
	}
	ctag := p.refreshedCTag(ctx, collection)
	key := componentKey(component, from, to, ctag)
	if objects, ok := p.cachedSet(collection, key); ok {
		return &ObjectSet{Collection: collection, Component: component, Objects: objects, FromCache: true}, nil
	}
	ms, err := p.client.Report(ctx, collection, dav.DepthZero, dav.NewCalendarComponentQuery(component, from, to))
	if err != nil {
		return nil, fmt.Errorf("calendar: query %s in %s: %w", component, collection, err)
	}
	objects := p.objectsFromReport(collection, ms, component)
	sortObjects(objects)
	p.putCachedSet(collection, key, objects)
	return &ObjectSet{Collection: collection, Component: component, Objects: objects}, nil
}

// Search returns the objects of the given components whose searched properties
// contain text. It is one report per component and property, because a CalDAV
// filter has no "any of" at that level; a cancelled context stops it part way
// and what was already found is returned.
func (p *Provider) Search(ctx context.Context, collection, text string, components ...string) (*ObjectSet, error) {
	collection = normalizeCollection(collection)
	if collection == "" {
		return nil, errors.New("calendar: collection path is required")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("calendar: search text is required")
	}
	if len(components) == 0 {
		components = []string{dav.CompEvent, dav.CompTodo, dav.CompJournal}
	}
	found := make(map[string]*model.Object)
	var firstErr error
	for _, component := range components {
		for _, property := range SearchProps {
			if err := ctx.Err(); err != nil {
				return p.searchResult(collection, found), err
			}
			query := dav.NewCalendarTextQuery(component, property, text)
			ms, err := p.client.Report(ctx, collection, dav.DepthZero, query)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("calendar: search %s in %s: %w", component, collection, err)
				}
				continue
			}
			for _, obj := range p.objectsFromReport(collection, ms, component) {
				found[obj.Path] = obj
			}
		}
	}
	if len(found) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return p.searchResult(collection, found), nil
}

func (p *Provider) searchResult(collection string, found map[string]*model.Object) *ObjectSet {
	objects := make([]*model.Object, 0, len(found))
	for _, obj := range found {
		objects = append(objects, obj)
	}
	sortObjects(objects)
	return &ObjectSet{Collection: collection, Objects: objects}
}

// objectsFromReport parses the calendar bodies of a multistatus, keeping only
// the requested component. A server that answers a filter it does not implement
// with everything it has would otherwise put events in a task list.
func (p *Provider) objectsFromReport(collection string, ms *dav.MultiStatus, component string) []*model.Object {
	var out []*model.Object
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
		body := []byte(data.Data)
		obj, err := model.ParseICal(path, strings.TrimSpace(etag.ETag), body)
		if err != nil {
			continue
		}
		if component != "" && obj.Component() != component {
			continue
		}
		p.putCachedBody(collection, path, obj.ETag, body)
		out = append(out, obj)
	}
	return out
}

// refreshedCTag returns the collection tag, dropping cached bodies when the
// collection has moved on since they were stored (§12).
func (p *Provider) refreshedCTag(ctx context.Context, collection string) string {
	if p.cache == nil {
		return "notag"
	}
	previousETags, hadPrevious := p.cache.GetETags(p.accountID, collection)
	previousCTag, _ := p.cache.CTag(p.accountID, collection)
	listing, err := p.List(ctx, collection)
	if err != nil {
		return "notag"
	}
	if hadPrevious && (previousCTag != listing.CTag || !sameETags(previousETags, listing.ETags)) {
		p.cache.InvalidateCollection(p.accountID, collection)
		p.cache.SetETags(p.accountID, collection, listing.CTag, listing.ETags)
	}
	if listing.CTag == "" {
		return "notag"
	}
	return listing.CTag
}

// cachedRef is one member of a cached result set. The bodies stay under their
// own path-and-ETag keys, so a set cached at an older version cannot serve a
// stale body: a missing member is a miss for the whole set.
type cachedRef struct {
	Path string `json:"p"`
	ETag string `json:"e"`
}

func (p *Provider) cachedSet(collection, key string) ([]*model.Object, bool) {
	if p.cache == nil {
		return nil, false
	}
	raw, ok := p.cache.GetBody(p.accountID, collection, key)
	if !ok {
		return nil, false
	}
	var refs []cachedRef
	if err := json.Unmarshal(raw, &refs); err != nil {
		return nil, false
	}
	out := make([]*model.Object, 0, len(refs))
	for _, ref := range refs {
		body, ok := p.cachedBody(collection, ref.Path, ref.ETag)
		if !ok {
			return nil, false
		}
		obj, err := model.ParseICal(ref.Path, ref.ETag, body)
		if err != nil {
			return nil, false
		}
		out = append(out, obj)
	}
	return out, true
}

func (p *Provider) putCachedSet(collection, key string, objects []*model.Object) {
	if p.cache == nil {
		return
	}
	refs := make([]cachedRef, 0, len(objects))
	for _, obj := range objects {
		if obj.ETag == "" {
			// Without a version there is nothing to key the body on, so the
			// set is not cached at all rather than cached unsafely.
			return
		}
		refs = append(refs, cachedRef{Path: obj.Path, ETag: obj.ETag})
	}
	if raw, err := json.Marshal(refs); err == nil {
		p.cache.PutBody(p.accountID, collection, key, raw)
	}
}

func componentKey(component string, from, to time.Time, ctag string) string {
	return "\x01comp\x00" + component + "\x00" + timeKey(from) + "\x00" + timeKey(to) + "\x00" + ctag
}

func timeKey(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func sortObjects(objects []*model.Object) {
	sort.SliceStable(objects, func(i, j int) bool { return objects[i].Path < objects[j].Path })
}
