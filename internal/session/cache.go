// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"sync"
	"time"
)

// CacheConfig holds cache limits (§12). Collection, ETag, body and thumbnail
// ceilings apply per session. MaxProcessBytes is the process-wide ceiling
// the session manager enforces across every live session.
type CacheConfig struct {
	CollectionTTL  time.Duration
	MaxCollections int
	MaxETagEntries int
	// MaxBodyBytes is the per-session ceiling for cached object bodies.
	MaxBodyBytes int
	// MaxProcessBytes is the ceiling across all sessions of one process.
	// Bodies and thumbnails count; ETag maps do not, because they are small
	// and the whole point of holding them is to survive body eviction.
	MaxProcessBytes int
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
	if c.MaxBodyBytes <= 0 {
		c.MaxBodyBytes = 64 << 20 // 64 MiB — the session memory orient of §12
	}
	if c.MaxProcessBytes <= 0 {
		c.MaxProcessBytes = 256 << 20 // 256 MiB — a handful of full sessions, not ten
	}
	if c.MaxThumbBytes <= 0 {
		c.MaxThumbBytes = 16 << 20 // 16 MiB — stricter than the body ceiling
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

type bodyRef struct {
	col  collectionKey
	path string
	seq  uint64
}

// Cache holds per-session collection metadata, path→ETag maps, object bodies,
// and photo thumbnails (§12). It lives only in memory and is wiped when the
// session ends. A process-wide budget, when bound, can ask it to drop a body
// or a thumbnail; the maps stay until this session's own count limits say
// otherwise.
type Cache struct {
	cfg    CacheConfig
	now    func() time.Time
	budget *budget

	mu          sync.Mutex
	collections map[collectionKey]*collectionEntry
	order       []collectionKey
	etagCount   int

	bodies     []bodyRef
	bodyBytes  int
	localClock uint64

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

func (c *Cache) bind(b *budget) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.budget = b
	c.mu.Unlock()
}

func (c *Cache) nextSeqLocked() uint64 {
	if c.budget != nil {
		return c.budget.tick()
	}
	c.localClock++
	return c.localClock
}

func (c *Cache) publish(n int) {
	if c == nil || c.budget == nil {
		return
	}
	c.budget.set(c, n)
}

// Wipe drops every cached collection, object body and thumbnail.
func (c *Cache) Wipe() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.collections = make(map[collectionKey]*collectionEntry)
	c.order = nil
	c.etagCount = 0
	c.bodies = nil
	c.bodyBytes = 0
	c.wipeThumbsLocked()
	c.mu.Unlock()
	c.publish(0)
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
	c.dropCollectionLocked(key)
	n := c.bodyBytes + c.thumbBytes
	c.mu.Unlock()
	c.publish(n)
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
	c.evictLocked()
	n := c.bodyBytes + c.thumbBytes
	c.mu.Unlock()
	c.publish(n)
}

// PutBody stores an object body. The body is counted in bytes and evicted
// before the collection's ETag map when a ceiling is crossed (§12).
func (c *Cache) PutBody(accountID, collectionPath, objectPath string, body []byte) {
	if c == nil {
		return
	}
	key := collectionKey{AccountID: accountID, Path: collectionPath}
	c.mu.Lock()
	ent, ok := c.collections[key]
	if !ok {
		ent = &collectionEntry{key: key, etags: make(map[string]string), bodies: make(map[string][]byte)}
		c.collections[key] = ent
		c.order = append(c.order, key)
	}
	if old, exists := ent.bodies[objectPath]; exists {
		c.bodyBytes -= len(old)
		c.removeBody(key, objectPath)
	}
	dup := make([]byte, len(body))
	copy(dup, body)
	ent.bodies[objectPath] = dup
	c.bodyBytes += len(dup)
	c.bodies = append([]bodyRef{{col: key, path: objectPath, seq: c.nextSeqLocked()}}, c.bodies...)
	c.touch(keyIndex(c.order, key))
	c.evictLocked()
	n := c.bodyBytes + c.thumbBytes
	c.mu.Unlock()
	c.publish(n)
}

// GetBody returns a cached object body when present.
func (c *Cache) GetBody(accountID, collectionPath, objectPath string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := collectionKey{AccountID: accountID, Path: collectionPath}
	ent, ok := c.collections[key]
	if !ok {
		return nil, false
	}
	body, ok := ent.bodies[objectPath]
	if !ok {
		return nil, false
	}
	c.touchBody(key, objectPath)
	c.touch(keyIndex(c.order, key))
	dup := make([]byte, len(body))
	copy(dup, body)
	return dup, true
}

// BodyBytes reports how many bytes of object bodies are currently held.
func (c *Cache) BodyBytes() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bodyBytes
}

func (c *Cache) evictLocked() {
	for c.bodyBytes > c.cfg.MaxBodyBytes {
		c.evictOldestBody()
	}
	c.evictThumbs()
	for len(c.collections) > c.cfg.MaxCollections {
		c.evictOldestCollection()
	}
	for c.etagCount > c.cfg.MaxETagEntries {
		c.evictOldestCollection()
	}
}

func (c *Cache) evictOldestBody() {
	if len(c.bodies) == 0 {
		return
	}
	ref := c.bodies[len(c.bodies)-1]
	c.bodies = c.bodies[:len(c.bodies)-1]
	ent, ok := c.collections[ref.col]
	if !ok {
		return
	}
	body, ok := ent.bodies[ref.path]
	if !ok {
		return
	}
	c.bodyBytes -= len(body)
	delete(ent.bodies, ref.path)
}

func (c *Cache) evictOldestCollection() {
	if len(c.order) == 0 {
		return
	}
	c.dropCollectionLocked(c.order[len(c.order)-1])
}

func (c *Cache) dropCollectionLocked(key collectionKey) {
	ent, ok := c.collections[key]
	if !ok {
		return
	}
	c.etagCount -= len(ent.etags)
	for _, body := range ent.bodies {
		c.bodyBytes -= len(body)
	}
	kept := c.bodies[:0]
	for _, ref := range c.bodies {
		if ref.col != key {
			kept = append(kept, ref)
		}
	}
	c.bodies = kept
	delete(c.collections, key)
	c.removeKey(key)
}

func (c *Cache) peekVictim() (evictKind, uint64, bool) {
	if c == nil {
		return 0, 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bodyBytes > 0 && len(c.bodies) > 0 {
		return evictBody, c.bodies[len(c.bodies)-1].seq, true
	}
	if c.thumbBytes > 0 && len(c.thumbOrder) > 0 {
		key := c.thumbOrder[len(c.thumbOrder)-1]
		if ent, ok := c.thumbs[key]; ok {
			return evictThumb, ent.seq, true
		}
		return evictThumb, 0, true
	}
	return 0, 0, false
}

func (c *Cache) dropVictim(kind evictKind) {
	if c == nil {
		return
	}
	c.mu.Lock()
	switch kind {
	case evictBody:
		c.evictOldestBody()
	case evictThumb:
		c.evictOldestThumb()
	}
	n := c.bodyBytes + c.thumbBytes
	c.mu.Unlock()
	if c.budget != nil {
		c.budget.adjust(c, n)
	}
}

func (c *Cache) touchBody(key collectionKey, path string) {
	idx := -1
	for i, ref := range c.bodies {
		if ref.col == key && ref.path == path {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	ref := c.bodies[idx]
	ref.seq = c.nextSeqLocked()
	copy(c.bodies[1:idx+1], c.bodies[0:idx])
	c.bodies[0] = ref
}

func (c *Cache) removeBody(key collectionKey, path string) {
	idx := -1
	for i, ref := range c.bodies {
		if ref.col == key && ref.path == path {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	c.bodies = append(c.bodies[:idx], c.bodies[idx+1:]...)
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
