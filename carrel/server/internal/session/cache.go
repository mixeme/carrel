package session

import (
	"sort"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
)

type cacheKind int

const (
	kindETagMap cacheKind = 1
	kindBody    cacheKind = 2
	kindThumb   cacheKind = 3
	kindOther   cacheKind = 4
)

type cacheEntry struct {
	kind  cacheKind
	key   string
	data  []byte
	bytes int64
	at    time.Time
}

// Cache stores discovered collections, version maps, bodies, and related data
// for one session (S-4.2).
//
// Several requests of one session run at once — the desk asks for fragments
// while a fanout is still going — so every method takes mu. The process-wide
// budget is enforced after mu is released: the manager lock is never taken
// while a cache lock is held, and evictProcessWide takes one cache lock at a
// time, so the two locks are never nested in both orders.
type Cache struct {
	cfg *conf.Config
	mgr *Manager

	mu         sync.Mutex
	entries    map[string]*cacheEntry
	order      []string // insertion order, oldest first
	bodyBytes  int64
	etagBytes  int64
	thumbBytes int64
	otherBytes int64
}

func newCache(cfg *conf.Config, mgr *Manager) *Cache {
	return &Cache{
		cfg:     cfg,
		mgr:     mgr,
		entries: make(map[string]*cacheEntry),
	}
}

func bodyKey(path, version string) string  { return "body\x00" + path + "\x00" + version }
func etagKey(path string) string           { return "etag\x00" + path }
func thumbKey(path, version string) string { return "thumb\x00" + path + "\x00" + version }
func collKey(path string) string           { return "coll\x00" + path }

// PutBody stores an object body keyed by path and version together.
func (c *Cache) PutBody(path, version string, data []byte) {
	c.put(kindBody, bodyKey(path, version), data)
}

// GetBody returns a body for the exact path and version.
func (c *Cache) GetBody(path, version string) ([]byte, bool) {
	return c.get(bodyKey(path, version))
}

// PutETagMap stores a collection path→version map.
func (c *Cache) PutETagMap(collection string, data []byte) {
	c.put(kindETagMap, etagKey(collection), data)
}

// GetETagMap returns a cached version map.
func (c *Cache) GetETagMap(collection string) ([]byte, bool) {
	return c.get(etagKey(collection))
}

// PutCollectionList stores a discovered collection list.
func (c *Cache) PutCollectionList(account string, data []byte) {
	c.put(kindOther, collKey(account), data)
}

// GetCollectionList returns a cached collection list.
func (c *Cache) GetCollectionList(account string) ([]byte, bool) {
	return c.get(collKey(account))
}

// PutThumb stores a photo thumbnail by object version.
func (c *Cache) PutThumb(path, version string, data []byte) {
	c.put(kindThumb, thumbKey(path, version), data)
}

// GetThumb returns a cached thumbnail.
func (c *Cache) GetThumb(path, version string) ([]byte, bool) {
	return c.get(thumbKey(path, version))
}

// InvalidatePath drops cached data for one object path (after write/delete).
func (c *Cache) InvalidatePath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var drop []string
	for k, e := range c.entries {
		if strings.Contains(e.key, "\x00"+path+"\x00") ||
			e.key == etagKey(path) || e.key == collKey(path) {
			drop = append(drop, k)
		}
	}
	for _, k := range drop {
		c.removeLocked(k)
	}
}

func (c *Cache) put(kind cacheKind, key string, data []byte) {
	c.mu.Lock()
	if _, ok := c.entries[key]; ok {
		c.removeLocked(key)
	}
	n := int64(len(data))
	c.entries[key] = &cacheEntry{kind: kind, key: key, data: data, bytes: n, at: time.Now()}
	c.order = append(c.order, key)
	c.addBytesLocked(kind, n)
	c.enforceSessionLimitsLocked()
	c.mu.Unlock()

	c.enforceProcessBudget()
}

func (c *Cache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	e.at = time.Now()
	return e.data, true
}

func (c *Cache) enforceSessionLimitsLocked() {
	c.evictBytesLocked(kindBody, c.cfg.CacheMaxBodyBytes, &c.bodyBytes)
	c.evictBytesLocked(kindThumb, c.cfg.CacheMaxThumbBytes, &c.thumbBytes)
	// C-12 counts these in entries, not bytes.
	c.evictCountLocked(kindETagMap, c.cfg.CacheMaxETagEntries)
	c.evictCountLocked(kindThumb, c.cfg.CacheMaxThumbEntries)
	c.evictCountLocked(kindOther, c.cfg.CacheMaxCollections)
}

func (c *Cache) evictBytesLocked(kind cacheKind, max int64, counter *int64) {
	if max <= 0 {
		return
	}
	for *counter > max {
		if !c.evictOneLocked(kind) {
			return
		}
	}
}

func (c *Cache) evictCountLocked(kind cacheKind, max int) {
	if max <= 0 {
		return
	}
	for c.countLocked(kind) > max {
		if !c.evictOneLocked(kind) {
			return
		}
	}
}

func (c *Cache) countLocked(kind cacheKind) int {
	var n int
	for _, e := range c.entries {
		if e.kind == kind {
			n++
		}
	}
	return n
}

// evictOneLocked drops the oldest entry of kind. It does not fall back to
// another kind: a body budget is not paid for with someone else's version map.
func (c *Cache) evictOneLocked(kind cacheKind) bool {
	for _, k := range c.order {
		if e := c.entries[k]; e != nil && e.kind == kind {
			c.removeLocked(k)
			return true
		}
	}
	return false
}

func (c *Cache) removeLocked(key string) {
	e := c.entries[key]
	if e == nil {
		return
	}
	c.addBytesLocked(e.kind, -e.bytes)
	delete(c.entries, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

func (c *Cache) addBytesLocked(kind cacheKind, n int64) {
	switch kind {
	case kindBody:
		c.bodyBytes += n
	case kindETagMap:
		c.etagBytes += n
	case kindThumb:
		c.thumbBytes += n
	default:
		c.otherBytes += n
	}
}

func (c *Cache) totalBytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bodyBytes + c.etagBytes + c.thumbBytes + c.otherBytes
}

func (c *Cache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
	c.order = nil
	c.bodyBytes = 0
	c.etagBytes = 0
	c.thumbBytes = 0
	c.otherBytes = 0
}

// enforceProcessBudget trims caches across every session once this one has
// pushed the process past CARREL_CACHE_MAX_PROCESS_BYTES. Called with no cache
// lock held.
func (c *Cache) enforceProcessBudget() {
	if c.mgr == nil || c.cfg.CacheMaxProcessBytes <= 0 {
		return
	}
	total := c.mgr.ProcessBytes()
	if total <= c.cfg.CacheMaxProcessBytes {
		return
	}
	c.mgr.evictProcessWide(total - c.cfg.CacheMaxProcessBytes)
}

type evictCandidate struct {
	key  string
	kind cacheKind
	at   time.Time
}

func (c *Cache) evictionCandidates() []evictCandidate {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]evictCandidate, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, evictCandidate{key: e.key, kind: e.kind, at: e.at})
	}
	return out
}

// drop removes one entry if it is still present with the expected kind and
// reports how many bytes that freed.
func (c *Cache) drop(kind cacheKind, key string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[key]
	if e == nil || e.kind != kind {
		return 0
	}
	n := e.bytes
	c.removeLocked(key)
	return n
}

// evictProcessWide frees need bytes across every live session. Bodies go
// before version maps (S-4.2, C-12), oldest first within a kind.
func (m *Manager) evictProcessWide(need int64) {
	type pick struct {
		sid  string
		key  string
		kind cacheKind
		at   time.Time
	}

	m.mu.Lock()
	caches := make(map[string]*Cache, len(m.byID))
	for sid, s := range m.byID {
		caches[sid] = s.cache
	}
	m.mu.Unlock()

	var all []pick
	for sid, c := range caches {
		for _, e := range c.evictionCandidates() {
			all = append(all, pick{sid: sid, key: e.key, kind: e.kind, at: e.at})
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if ai, bi := evictRank(a.kind), evictRank(b.kind); ai != bi {
			return ai < bi
		}
		return a.at.Before(b.at)
	})

	var freed int64
	for _, p := range all {
		if freed >= need {
			return
		}
		if c := caches[p.sid]; c != nil {
			freed += c.drop(p.kind, p.key)
		}
	}
}

// evictRank orders kinds by how early they are given up: bodies first, then
// thumbnails, then collection lists, and version maps last — a map is small,
// and losing it costs another round trip to the far server.
func evictRank(k cacheKind) int {
	switch k {
	case kindBody:
		return 0
	case kindThumb:
		return 1
	case kindOther:
		return 2
	default:
		return 3
	}
}
