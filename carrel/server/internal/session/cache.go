package session

import (
	"strings"
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
type Cache struct {
	cfg *conf.Config
	mgr *Manager

	entries map[string]*cacheEntry
	order   []string // LRU, oldest first

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
	var drop []string
	for k, e := range c.entries {
		if strings.Contains(e.key, "\x00"+path+"\x00") || e.key == etagKey(path) {
			drop = append(drop, k)
		}
	}
	for _, k := range drop {
		c.remove(k)
	}
}

func (c *Cache) put(kind cacheKind, key string, data []byte) {
	if old, ok := c.entries[key]; ok {
		c.subtract(old)
		c.removeKey(key)
	}
	n := int64(len(data))
	e := &cacheEntry{kind: kind, key: key, data: data, bytes: n, at: time.Now()}
	c.entries[key] = e
	c.order = append(c.order, key)
	c.addBytes(kind, n)
	c.enforceLimits()
}

func (c *Cache) get(key string) ([]byte, bool) {
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	e.at = time.Now()
	return e.data, true
}

func (c *Cache) enforceLimits() {
	c.evictSession(kindBody, c.cfg.CacheMaxBodyBytes, &c.bodyBytes)
	c.evictSession(kindThumb, c.cfg.CacheMaxThumbBytes, &c.thumbBytes)
	c.evictSession(kindETagMap, int64(c.cfg.CacheMaxETagEntries)*64, &c.etagBytes)
	if c.mgr != nil && c.mgr.ProcessBytes() > c.cfg.CacheMaxProcessBytes {
		need := c.mgr.ProcessBytes() - c.cfg.CacheMaxProcessBytes
		c.mgr.evictProcessWide(need)
	}
}

func (c *Cache) evictSession(kind cacheKind, max int64, counter *int64) {
	for *counter > max {
		if !c.evictOne(kind) {
			break
		}
	}
}

func (c *Cache) evictOne(prefer cacheKind) bool {
	// Bodies before maps (S-4.2).
	for _, k := range c.order {
		e := c.entries[k]
		if e == nil || e.kind != prefer {
			continue
		}
		c.remove(k)
		return true
	}
	for _, k := range c.order {
		if c.entries[k] != nil {
			c.remove(k)
			return true
		}
	}
	return false
}

func (c *Cache) remove(key string) {
	e := c.entries[key]
	if e == nil {
		return
	}
	c.subtract(e)
	c.removeKey(key)
}

func (c *Cache) removeKey(key string) {
	delete(c.entries, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

func (c *Cache) subtract(e *cacheEntry) {
	switch e.kind {
	case kindBody:
		c.bodyBytes -= e.bytes
	case kindETagMap:
		c.etagBytes -= e.bytes
	case kindThumb:
		c.thumbBytes -= e.bytes
	default:
		c.otherBytes -= e.bytes
	}
}

func (c *Cache) addBytes(kind cacheKind, n int64) {
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
	return c.bodyBytes + c.etagBytes + c.thumbBytes + c.otherBytes
}

func (c *Cache) clear() {
	c.entries = make(map[string]*cacheEntry)
	c.order = nil
	c.bodyBytes = 0
	c.etagBytes = 0
	c.thumbBytes = 0
	c.otherBytes = 0
}

type evictCandidate struct {
	key  string
	kind cacheKind
	at   time.Time
}

func (c *Cache) evictionCandidates() []evictCandidate {
	out := make([]evictCandidate, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, evictCandidate{key: e.key, kind: e.kind, at: e.at})
	}
	return out
}

func (c *Cache) drop(kind cacheKind, key string) int64 {
	e := c.entries[key]
	if e == nil || e.kind != kind {
		return 0
	}
	n := e.bytes
	c.remove(key)
	return n
}

func (m *Manager) evictProcessWide(need int64) {
	type pick struct {
		sid  string
		key  string
		kind cacheKind
		at   time.Time
	}
	var all []pick
	m.mu.Lock()
	for sid, s := range m.byID {
		for _, e := range s.cache.evictionCandidates() {
			all = append(all, pick{sid: sid, key: e.key, kind: e.kind, at: e.at})
		}
	}
	m.mu.Unlock()

	// Sort: bodies first, then oldest within kind.
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			a, b := all[i], all[j]
			if bodyBefore(a.kind, b.kind) || (a.kind == b.kind && a.at.After(b.at)) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	freed := int64(0)
	for _, p := range all {
		if freed >= need {
			break
		}
		m.mu.Lock()
		s := m.byID[p.sid]
		m.mu.Unlock()
		if s == nil {
			continue
		}
		freed += s.cache.drop(p.kind, p.key)
	}
}

func bodyBefore(a, b cacheKind) bool {
	if a == kindBody && b != kindBody {
		return true
	}
	if a != kindBody && b == kindBody {
		return false
	}
	return false
}
