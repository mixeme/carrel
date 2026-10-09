package session

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"carrel.dev/carrel/server/internal/conf"
)

// touchSeq stamps recency. Two puts in the same clock tick get the same
// time.Now(), and eviction walked a map to find the oldest — map order is
// randomised, so the newest entry was evicted instead of the oldest about one
// time in eight. The counter is process-wide because evictProcessWide ranks
// entries from different sessions against each other.
var touchSeq atomic.Uint64

func nextSeq() uint64 { return touchSeq.Add(1) }

type cacheKind int

const (
	kindETagMap cacheKind = 1
	kindBody    cacheKind = 2
	kindThumb   cacheKind = 3
	kindOther   cacheKind = 4
	// kindFileThumb is the thumbnail of a file (S-17.1), budgeted apart from
	// the photos of contacts (kindThumb).
	kindFileThumb cacheKind = 5
)

type cacheEntry struct {
	kind      cacheKind
	key       string
	data      []byte
	bytes     int64
	seq       uint64    // last access, monotonic; eviction picks the lowest within a kind
	at        time.Time // last access, for reporting only — never for ordering
	fetchedAt time.Time // when stored; TTL applies to collection lists and etag maps
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
	ended      bool
	entries    map[string]*cacheEntry
	bodyBytes  int64
	etagBytes  int64
	thumbBytes int64
	otherBytes int64
	fileThumbs int64 // bytes of kindFileThumb
}

func newCache(cfg *conf.Config, mgr *Manager) *Cache {
	return &Cache{
		cfg:     cfg,
		mgr:     mgr,
		entries: make(map[string]*cacheEntry),
	}
}

func bodyKey(connID, path, version string) string {
	return "body\x00" + connID + "\x00" + path + "\x00" + version
}
func etagKey(connID, path string) string { return "etag\x00" + connID + "\x00" + path }
func thumbKey(connID, path, version string) string {
	return "thumb\x00" + connID + "\x00" + path + "\x00" + version
}
func fileThumbKey(connID, path, version string) string {
	return "fthumb\x00" + connID + "\x00" + path + "\x00" + version
}
func collKey(path string) string { return "coll\x00" + path }

// PutBody stores an object body keyed by connection, path, and version.
func (c *Cache) PutBody(connID, path, version string, data []byte) {
	c.put(kindBody, bodyKey(connID, path, version), data)
}

// GetBody returns a body for the exact connection, path, and version.
func (c *Cache) GetBody(connID, path, version string) ([]byte, bool) {
	return c.get(bodyKey(connID, path, version))
}

// PutETagMap stores a collection path→version map for one connection.
func (c *Cache) PutETagMap(connID, collection string, data []byte) {
	c.put(kindETagMap, etagKey(connID, collection), data)
}

// GetETagMap returns a cached version map.
func (c *Cache) GetETagMap(connID, collection string) ([]byte, bool) {
	return c.get(etagKey(connID, collection))
}

// PeekETagMap returns a version map whatever its age. CARREL_CACHE_COLLECTION_TTL
// says when a map is worth asking the server again; what the session has read
// is still what it has read — the numbers of the rail, the duplicates among
// the loaded lists, the links between records (S-16, C-6.3). Only the budgets
// of S-4.2 take such a map away.
func (c *Cache) PeekETagMap(connID, collection string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[etagKey(connID, collection)]
	if !ok {
		return nil, false
	}
	e.seq = nextSeq()
	e.at = time.Now()
	return e.data, true
}

// PutCollectionList stores a discovered collection list.
func (c *Cache) PutCollectionList(account string, data []byte) {
	c.put(kindOther, collKey(account), data)
}

// GetCollectionList returns a cached collection list.
func (c *Cache) GetCollectionList(account string) ([]byte, bool) {
	return c.get(collKey(account))
}

// PutThumb stores a photo thumbnail by connection and object version.
func (c *Cache) PutThumb(connID, path, version string, data []byte) {
	c.put(kindThumb, thumbKey(connID, path, version), data)
}

// GetThumb returns a cached thumbnail.
func (c *Cache) GetThumb(connID, path, version string) ([]byte, bool) {
	return c.get(thumbKey(connID, path, version))
}

// PutFileThumb stores the thumbnail of a file by connection, path and file
// version (S-4.2, S-17.1). It has a budget of its own
// (CARREL_CACHE_MAX_FILE_THUMB_BYTES, …_ENTRIES), apart from the photos of
// contacts: a book of contacts and a folder of pictures do not push each
// other out, and a file's thumbnail is the dearer to make again — its file
// is read whole from the far server. No bytes — the file has no thumbnail.
func (c *Cache) PutFileThumb(connID, path, version string, data []byte) {
	c.put(kindFileThumb, fileThumbKey(connID, path, version), data)
}

// GetFileThumb returns what PutFileThumb stored for that version.
func (c *Cache) GetFileThumb(connID, path, version string) ([]byte, bool) {
	return c.get(fileThumbKey(connID, path, version))
}

// PutImageSize stores what the head of an image said of its size, by
// connection, path and file version (S-4.2, S-17): one ranged read per
// version. It is kept with the bodies — its key is one — so taking a path or
// a connection out takes it too.
func (c *Cache) PutImageSize(connID, path, version string, data []byte) {
	c.put(kindBody, bodyKey(connID, path, "size\x00"+version), data)
}

// GetImageSize returns what PutImageSize stored for that version.
func (c *Cache) GetImageSize(connID, path, version string) ([]byte, bool) {
	return c.get(bodyKey(connID, path, "size\x00"+version))
}

// InvalidateCollectionList drops a cached collection list for one account.
func (c *Cache) InvalidateCollectionList(account string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended {
		return
	}
	if e := c.entries[collKey(account)]; e != nil {
		c.removeLocked(collKey(account))
	}
}

// InvalidateConnection drops everything cached of one connection: bodies,
// version maps, thumbnails and its collection list. A connection that moved
// to another server or was taken out has nothing the session read of it
// that still holds.
func (c *Cache) InvalidateConnection(connID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended {
		return
	}
	var drop []string
	for k, e := range c.entries {
		for _, kind := range []string{"body", "etag", "thumb"} {
			if strings.HasPrefix(e.key, kind+"\x00"+connID+"\x00") {
				drop = append(drop, k)
			}
		}
		if e.key == collKey(connID) {
			drop = append(drop, k)
		}
	}
	for _, k := range drop {
		c.removeLocked(k)
	}
}

// InvalidateMapsUnder drops every version map whose key starts with prefix on
// a connection. A write Carrel makes to a collection changes what a whole read
// of it would find; the next read goes to the server. Bodies stay: they are
// keyed by version, and a stale one cannot be reached (S-4.2).
func (c *Cache) InvalidateMapsUnder(connID, prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended {
		return
	}
	start := etagKey(connID, prefix)
	var drop []string
	for k, e := range c.entries {
		if strings.HasPrefix(e.key, start) {
			drop = append(drop, k)
		}
	}
	for _, k := range drop {
		c.removeLocked(k)
	}
}

// InvalidatePath drops cached data for one object path on a connection.
func (c *Cache) InvalidatePath(connID, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended {
		return
	}
	var drop []string
	prefix := "\x00" + path + "\x00"
	connPrefix := connID + "\x00"
	for k, e := range c.entries {
		if strings.HasPrefix(e.key, "body\x00"+connPrefix) && strings.Contains(e.key, prefix) ||
			strings.HasPrefix(e.key, "thumb\x00"+connPrefix) && strings.Contains(e.key, prefix) ||
			e.key == etagKey(connID, path) {
			drop = append(drop, k)
		}
	}
	for _, k := range drop {
		c.removeLocked(k)
	}
}

func (c *Cache) put(kind cacheKind, key string, data []byte) {
	c.mu.Lock()
	if c.ended {
		c.mu.Unlock()
		return
	}
	if _, ok := c.entries[key]; ok {
		c.removeLocked(key)
	}
	n := int64(len(data))
	now := time.Now()
	c.entries[key] = &cacheEntry{
		kind: kind, key: key, data: data, bytes: n,
		seq: nextSeq(), at: now, fetchedAt: now,
	}
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
	if c.ttlExpiredLocked(e) {
		c.removeLocked(key)
		return nil, false
	}
	e.seq = nextSeq()
	e.at = time.Now()
	return e.data, true
}

// ttlExpiredLocked reports whether a collection list or version map has lived
// past CARREL_CACHE_COLLECTION_TTL. Bodies and thumbnails are keyed by object
// version and are not TTL-governed (S-4.2).
func (c *Cache) ttlExpiredLocked(e *cacheEntry) bool {
	ttl := c.cfg.CacheCollectionTTL
	if ttl <= 0 {
		return false
	}
	switch e.kind {
	case kindETagMap, kindOther:
		return time.Since(e.fetchedAt) > ttl
	default:
		return false
	}
}

func (c *Cache) enforceSessionLimitsLocked() {
	c.evictBytesLocked(kindBody, c.cfg.CacheMaxBodyBytes, &c.bodyBytes)
	c.evictBytesLocked(kindThumb, c.cfg.CacheMaxThumbBytes, &c.thumbBytes)
	c.evictBytesLocked(kindFileThumb, c.cfg.CacheMaxFileThumbBytes, &c.fileThumbs)
	// C-12 counts these in entries, not bytes.
	c.evictCountLocked(kindETagMap, c.cfg.CacheMaxETagEntries)
	c.evictCountLocked(kindThumb, c.cfg.CacheMaxThumbEntries)
	c.evictCountLocked(kindFileThumb, c.cfg.CacheMaxFileThumbEntries)
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

// evictOneLocked drops the least recently used entry of kind — same rule as
// evictProcessWide uses within a kind. It does not fall back to another kind:
// a body budget is not paid for with someone else's version map.
func (c *Cache) evictOneLocked(kind cacheKind) bool {
	var victim string
	var oldest uint64
	for k, e := range c.entries {
		if e.kind != kind {
			continue
		}
		if victim == "" || e.seq < oldest {
			victim = k
			oldest = e.seq
		}
	}
	if victim == "" {
		return false
	}
	c.removeLocked(victim)
	return true
}

func (c *Cache) removeLocked(key string) {
	e := c.entries[key]
	if e == nil {
		return
	}
	c.addBytesLocked(e.kind, -e.bytes)
	delete(c.entries, key)
}

func (c *Cache) addBytesLocked(kind cacheKind, n int64) {
	switch kind {
	case kindBody:
		c.bodyBytes += n
	case kindETagMap:
		c.etagBytes += n
	case kindThumb:
		c.thumbBytes += n
	case kindFileThumb:
		c.fileThumbs += n
	default:
		c.otherBytes += n
	}
}

func (c *Cache) totalBytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bodyBytes + c.etagBytes + c.thumbBytes + c.otherBytes + c.fileThumbs
}

func (c *Cache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
	c.bodyBytes = 0
	c.etagBytes = 0
	c.thumbBytes = 0
	c.otherBytes = 0
	c.fileThumbs = 0
}

func (c *Cache) markEnded() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ended = true
	c.entries = make(map[string]*cacheEntry)
	c.bodyBytes = 0
	c.etagBytes = 0
	c.thumbBytes = 0
	c.otherBytes = 0
	c.fileThumbs = 0
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
	seq  uint64
}

func (c *Cache) evictionCandidates() []evictCandidate {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]evictCandidate, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, evictCandidate{key: e.key, kind: e.kind, seq: e.seq})
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
		seq  uint64
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
			all = append(all, pick{sid: sid, key: e.key, kind: e.kind, seq: e.seq})
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if ai, bi := evictRank(a.kind), evictRank(b.kind); ai != bi {
			return ai < bi
		}
		return a.seq < b.seq
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
	case kindThumb, kindFileThumb:
		return 1
	case kindOther:
		return 2
	default:
		return 3
	}
}
