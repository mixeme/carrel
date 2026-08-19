// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"sync"
	"time"
)

// CacheConfig holds per-session cache limits (§12).
type CacheConfig struct {
	CollectionTTL  time.Duration
	MaxCollections int
	MaxETagEntries int
	// MaxThumbBytes is the stricter ceiling for photo thumbnails (§12).
	MaxThumbBytes int
	// MaxThumbEntries caps how many thumbnails one session may keep.
	MaxThumbEntries int
}

func (c CacheConfig) withDefaults() CacheConfig {
	if c.CollectionTTL <= 0 {
		c.CollectionTTL = 60 * time.Second
	}
	if c.MaxCollections <= 0 {
		c.MaxCollections = 256
	}
	if c.MaxETagEntries <= 0 {
		c.MaxETagEntries = 4096
	}
	if c.MaxThumbBytes <= 0 {
		c.MaxThumbBytes = 16 << 20 // 16 MiB — stricter than the ~64 MiB session orient
	}
	if c.MaxThumbEntries <= 0 {
		c.MaxThumbEntries = 512
	}
	return c
}

type collectionKey struct {
	AccountID string
	Path      string
}

type collectionEntry struct {
	key       collectionKey
	ctag      string
	fetchedAt time.Time
	etags     map[string]string
	bodies    map[string][]byte
}

// Cache holds per-session collection metadata, path→ETag maps, object bodies,
// and photo thumbnails (§12). It lives only in memory and is wiped when the
// session ends.
type Cache struct {
	cfg CacheConfig
	now func() time.Time

	mu          sync.Mutex
	collections map[collectionKey]*collectionEntry
	order       []collectionKey
	etagCount   int

	thumbs     map[thumbKey]*thumbEntry
	thumbOrder []thumbKey
	thumbBytes int
}

// NewCache returns an empty session cache.
func NewCache(cfg CacheConfig, now func() time.Time) *Cache {
	if now == nil {
		now = time.Now
	}
	return &Cache{
		cfg:         cfg.withDefaults(),
		now:         now,
		collections: make(map[collectionKey]*collectionEntry),
		thumbs:      make(map[thumbKey]*thumbEntry),
	}
}

// Wipe drops every cached collection, object body and thumbnail.
func (c *Cache) Wipe() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.collections = make(map[collectionKey]*collectionEntry)
	c.order = nil
	c.etagCount = 0
	c.wipeThumbsLocked()
}

// InvalidateAll drops cached data for every collection.
func (c *Cache) InvalidateAll() { c.Wipe() }

// InvalidateCollection drops cached data for one collection.
func (c *Cache) InvalidateCollection(accountID, collectionPath string) {
	if c == nil {
		return
	}
	key := collectionKey{AccountID: accountID, Path: collectionPath}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ent, ok := c.collections[key]; ok {
		c.etagCount -= len(ent.etags)
		delete(c.collections, key)
		c.removeKey(key)
	}
}

// NeedsRefresh reports whether collection metadata should be fetched again.
func (c *Cache) NeedsRefresh(accountID, collectionPath, serverCTag string) bool {
	if c == nil {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ent, ok := c.collections[collectionKey{AccountID: accountID, Path: collectionPath}]
	if !ok {
		return true
	}
	if c.now().Sub(ent.fetchedAt) >= c.cfg.CollectionTTL {
		return true
	}
	if serverCTag != "" && ent.ctag != serverCTag {
		return true
	}
	return false
}

// CollectionMeta holds sync metadata Carrel already keeps for a collection (§23.8).
type CollectionMeta struct {
	FetchedAt   time.Time
	CTag        string
	ObjectCount int
}

// LastFetched is the most recent collection read in this session, if any.
func (c *Cache) LastFetched() (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var latest time.Time
	for _, ent := range c.collections {
		if ent != nil && ent.fetchedAt.After(latest) {
			latest = ent.fetchedAt
		}
	}
	if latest.IsZero() {
		return time.Time{}, false
	}
	return latest, true
}

// CollectionMeta returns cached read time, server tag and object count when the
// collection is in the session cache.
func (c *Cache) CollectionMeta(accountID, collectionPath string) (CollectionMeta, bool) {
	if c == nil {
		return CollectionMeta{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ent, ok := c.collections[collectionKey{AccountID: accountID, Path: collectionPath}]
	if !ok {
		return CollectionMeta{}, false
	}
	return CollectionMeta{
		FetchedAt:   ent.fetchedAt,
		CTag:        ent.ctag,
		ObjectCount: len(ent.etags),
	}, true
}

// CTag returns the collection tag the cached map was read at. A caller that
// finds the server still reporting the same tag can keep the map instead of
// reading every ETag again (§12).
func (c *Cache) CTag(accountID, collectionPath string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ent, ok := c.collections[collectionKey{AccountID: accountID, Path: collectionPath}]
	if !ok {
		return "", false
	}
	return ent.ctag, true
}

// GetETags returns a copy of the cached path→ETag map for one collection.
func (c *Cache) GetETags(accountID, collectionPath string) (map[string]string, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ent, ok := c.collections[collectionKey{AccountID: accountID, Path: collectionPath}]
	if !ok {
		return nil, false
	}
	c.touch(keyIndex(c.order, ent.key))
	out := make(map[string]string, len(ent.etags))
	for k, v := range ent.etags {
		out[k] = v
	}
	return out, true
}

// SetETags stores collection metadata and ETag map, evicting LRU entries when
// limits are exceeded.
func (c *Cache) SetETags(accountID, collectionPath, ctag string, etags map[string]string) {
	if c == nil {
		return
	}
	key := collectionKey{AccountID: accountID, Path: collectionPath}
	c.mu.Lock()
	defer c.mu.Unlock()

	ent, ok := c.collections[key]
	if ok {
		c.etagCount -= len(ent.etags)
	} else {
		ent = &collectionEntry{key: key, etags: make(map[string]string), bodies: make(map[string][]byte)}
		c.collections[key] = ent
		c.order = append(c.order, key)
	}
	ent.ctag = ctag
	ent.fetchedAt = c.now()
	ent.etags = make(map[string]string, len(etags))
	for k, v := range etags {
		ent.etags[k] = v
	}
	c.etagCount += len(ent.etags)
	c.touch(keyIndex(c.order, key))
	c.evict()
}

// PutBody stores a minimal object body for refresh tests.
func (c *Cache) PutBody(accountID, collectionPath, objectPath string, body []byte) {
	if c == nil {
		return
	}
	key := collectionKey{AccountID: accountID, Path: collectionPath}
	c.mu.Lock()
	defer c.mu.Unlock()
	ent, ok := c.collections[key]
	if !ok {
		ent = &collectionEntry{key: key, etags: make(map[string]string), bodies: make(map[string][]byte)}
		c.collections[key] = ent
		c.order = append(c.order, key)
	}
	dup := make([]byte, len(body))
	copy(dup, body)
	ent.bodies[objectPath] = dup
	c.touch(keyIndex(c.order, key))
	c.evict()
}

// GetBody returns a cached object body when present.
func (c *Cache) GetBody(accountID, collectionPath, objectPath string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ent, ok := c.collections[collectionKey{AccountID: accountID, Path: collectionPath}]
	if !ok {
		return nil, false
	}
	body, ok := ent.bodies[objectPath]
	if !ok {
		return nil, false
	}
	c.touch(keyIndex(c.order, collectionKey{AccountID: accountID, Path: collectionPath}))
	dup := make([]byte, len(body))
	copy(dup, body)
	return dup, true
}

func (c *Cache) evict() {
	for len(c.collections) > c.cfg.MaxCollections {
		c.evictOldestCollection()
	}
	for c.etagCount > c.cfg.MaxETagEntries {
		c.evictOldestCollection()
	}
}

func (c *Cache) evictOldestCollection() {
	if len(c.order) == 0 {
		return
	}
	key := c.order[len(c.order)-1]
	if ent, ok := c.collections[key]; ok {
		c.etagCount -= len(ent.etags)
		delete(c.collections, key)
	}
	c.order = c.order[:len(c.order)-1]
}

func (c *Cache) touch(idx int) {
	if idx < 0 || idx >= len(c.order) {
		return
	}
	key := c.order[idx]
	copy(c.order[1:idx+1], c.order[0:idx])
	c.order[0] = key
}

func (c *Cache) removeKey(key collectionKey) {
	idx := keyIndex(c.order, key)
	if idx < 0 {
		return
	}
	c.order = append(c.order[:idx], c.order[idx+1:]...)
}

func keyIndex(order []collectionKey, key collectionKey) int {
	for i, k := range order {
		if k == key {
			return i
		}
	}
	return -1
}
