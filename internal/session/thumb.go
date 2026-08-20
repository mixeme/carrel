// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

// Thumb holds a decoded photo thumbnail ready to serve (§12).
type Thumb struct {
	MediaType string
	Bytes     []byte
}

type thumbKey struct {
	AccountID string
	Path      string
	ETag      string
}

type thumbEntry struct {
	key   thumbKey
	media string
	data  []byte
	seq   uint64
}

// PutThumb stores a photo thumbnail keyed by the object's path and ETag (§12).
// A changed ETag is a miss rather than a stale hit.
func (c *Cache) PutThumb(accountID, objectPath, etag, mediaType string, data []byte) {
	if c == nil || etag == "" || len(data) == 0 {
		return
	}
	key := thumbKey{AccountID: accountID, Path: objectPath, ETag: etag}
	c.mu.Lock()

	if ent, ok := c.thumbs[key]; ok {
		c.thumbBytes -= len(ent.data)
		delete(c.thumbs, key)
		c.removeThumbKey(key)
	}
	dup := make([]byte, len(data))
	copy(dup, data)
	c.thumbs[key] = &thumbEntry{key: key, media: mediaType, data: dup, seq: c.nextSeqLocked()}
	c.thumbOrder = append([]thumbKey{key}, c.thumbOrder...)
	c.thumbBytes += len(dup)
	c.evictLocked()
	n := c.bodyBytes + c.thumbBytes
	c.mu.Unlock()
	c.publish(n)
}

// GetThumb returns a cached thumbnail when the object's ETag still matches.
func (c *Cache) GetThumb(accountID, objectPath, etag string) (Thumb, bool) {
	if c == nil || etag == "" {
		return Thumb{}, false
	}
	key := thumbKey{AccountID: accountID, Path: objectPath, ETag: etag}
	c.mu.Lock()
	defer c.mu.Unlock()
	ent, ok := c.thumbs[key]
	if !ok {
		return Thumb{}, false
	}
	c.touchThumb(keyIndexThumb(c.thumbOrder, key))
	dup := make([]byte, len(ent.data))
	copy(dup, ent.data)
	return Thumb{MediaType: ent.media, Bytes: dup}, true
}

// ThumbBytes reports how many bytes of thumbnails are currently held.
func (c *Cache) ThumbBytes() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.thumbBytes
}

// ThumbCount reports how many thumbnails are currently held.
func (c *Cache) ThumbCount() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.thumbs)
}

func (c *Cache) wipeThumbsLocked() {
	c.thumbs = make(map[thumbKey]*thumbEntry)
	c.thumbOrder = nil
	c.thumbBytes = 0
}

func (c *Cache) evictThumbs() {
	for len(c.thumbs) > c.cfg.MaxThumbEntries || c.thumbBytes > c.cfg.MaxThumbBytes {
		c.evictOldestThumb()
	}
}

func (c *Cache) evictOldestThumb() {
	if len(c.thumbOrder) == 0 {
		return
	}
	key := c.thumbOrder[len(c.thumbOrder)-1]
	if ent, ok := c.thumbs[key]; ok {
		c.thumbBytes -= len(ent.data)
		delete(c.thumbs, key)
	}
	c.thumbOrder = c.thumbOrder[:len(c.thumbOrder)-1]
}

func (c *Cache) touchThumb(idx int) {
	if idx < 0 || idx >= len(c.thumbOrder) {
		return
	}
	key := c.thumbOrder[idx]
	copy(c.thumbOrder[1:idx+1], c.thumbOrder[0:idx])
	c.thumbOrder[0] = key
	if ent, ok := c.thumbs[key]; ok {
		ent.seq = c.nextSeqLocked()
	}
}

func (c *Cache) removeThumbKey(key thumbKey) {
	idx := keyIndexThumb(c.thumbOrder, key)
	if idx < 0 {
		return
	}
	c.thumbOrder = append(c.thumbOrder[:idx], c.thumbOrder[idx+1:]...)
}

func keyIndexThumb(order []thumbKey, key thumbKey) int {
	for i, k := range order {
		if k == key {
			return i
		}
	}
	return -1
}
