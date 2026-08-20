// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"sync"
	"sync/atomic"
)

// evictKind is the order §12 requires under a memory ceiling: bodies first,
// because they are large, then thumbnails, because they are large and cheap
// to rebuild. ETag maps are not on this list — they are small and save a
// deep PROPFIND, so they stay until a per-session count limit says otherwise.
type evictKind int

const (
	evictBody evictKind = iota
	evictThumb
)

// budget is the process-wide cache ceiling of §12. Each session cache still
// holds its own data — nothing is copied here, so users cannot see each
// other — and the budget only tracks how many bytes each cache currently
// accounts for. When the total exceeds max, the least-recently-used body
// across every live session is dropped; thumbs follow only after no bodies
// remain.
type budget struct {
	max   int64
	clock uint64 // monotonic recency, shared so LRU can reach across users

	mu    sync.Mutex
	total int64
	used  map[*Cache]int
}

func newBudget(maxBytes int) *budget {
	if maxBytes < 0 {
		maxBytes = 0
	}
	return &budget{
		max:  int64(maxBytes),
		used: make(map[*Cache]int),
	}
}

func (b *budget) tick() uint64 {
	if b == nil {
		return 0
	}
	return atomic.AddUint64(&b.clock, 1)
}

func (b *budget) bytes() int64 {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.total
}

// set records that c now accounts for n bytes of bodies and thumbnails, then
// evicts across sessions if the process ceiling is crossed. n == 0 unregisters
// the cache, which is the logout / wipe path.
func (b *budget) set(c *Cache, n int) {
	if b == nil || c == nil {
		return
	}
	b.adjust(c, n)
	if b.max > 0 && b.bytes() > b.max {
		b.evictUntilUnder()
	}
}

func (b *budget) adjust(c *Cache, n int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	old := b.used[c]
	b.total += int64(n - old)
	if n <= 0 {
		delete(b.used, c)
	} else {
		b.used[c] = n
	}
}

func (b *budget) evictUntilUnder() {
	for {
		b.mu.Lock()
		if b.max <= 0 || b.total <= b.max {
			b.mu.Unlock()
			return
		}
		caches := make([]*Cache, 0, len(b.used))
		for c := range b.used {
			caches = append(caches, c)
		}
		b.mu.Unlock()

		var (
			best     *Cache
			bestKind evictKind
			bestSeq  uint64
			found    bool
		)
		for _, c := range caches {
			kind, seq, ok := c.peekVictim()
			if !ok {
				continue
			}
			if !found || kind < bestKind || (kind == bestKind && seq < bestSeq) {
				best, bestKind, bestSeq = c, kind, seq
				found = true
			}
		}
		if !found {
			return
		}
		best.dropVictim(bestKind)
	}
}
