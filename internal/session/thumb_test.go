// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import "testing"

func TestCacheThumbRoundTripAndETagMiss(t *testing.T) {
	c := NewCache(CacheConfig{MaxThumbBytes: 1 << 20, MaxThumbEntries: 8}, nil)
	data := []byte("jpeg-bytes")
	c.PutThumb("acc", "/ab/a.vcf", `"e1"`, "image/jpeg", data)

	got, ok := c.GetThumb("acc", "/ab/a.vcf", `"e1"`)
	if !ok || string(got.Bytes) != string(data) || got.MediaType != "image/jpeg" {
		t.Fatalf("GetThumb = %+v, %v", got, ok)
	}
	if _, ok := c.GetThumb("acc", "/ab/a.vcf", `"e2"`); ok {
		t.Fatal("changed ETag must miss")
	}
	if c.ThumbCount() != 1 || c.ThumbBytes() != len(data) {
		t.Fatalf("count=%d bytes=%d", c.ThumbCount(), c.ThumbBytes())
	}
}

func TestCacheThumbLRUEvictionByCount(t *testing.T) {
	c := NewCache(CacheConfig{MaxThumbBytes: 1 << 20, MaxThumbEntries: 2}, nil)
	c.PutThumb("a", "/1", "1", "image/jpeg", []byte("one"))
	c.PutThumb("a", "/2", "2", "image/jpeg", []byte("two"))
	c.GetThumb("a", "/1", "1") // touch oldest so /2 is LRU
	c.PutThumb("a", "/3", "3", "image/jpeg", []byte("three"))

	if _, ok := c.GetThumb("a", "/2", "2"); ok {
		t.Fatal("/2 should have been evicted")
	}
	if _, ok := c.GetThumb("a", "/1", "1"); !ok {
		t.Fatal("/1 should remain after touch")
	}
	if _, ok := c.GetThumb("a", "/3", "3"); !ok {
		t.Fatal("/3 should remain")
	}
}

func TestCacheThumbLRUEvictionByBytes(t *testing.T) {
	c := NewCache(CacheConfig{MaxThumbBytes: 10, MaxThumbEntries: 100}, nil)
	c.PutThumb("a", "/1", "1", "image/jpeg", []byte("12345"))
	c.PutThumb("a", "/2", "2", "image/jpeg", []byte("123456")) // 5+6=11 > 10 → evict /1
	if _, ok := c.GetThumb("a", "/1", "1"); ok {
		t.Fatal("oldest thumb should be evicted by byte ceiling")
	}
	if got, ok := c.GetThumb("a", "/2", "2"); !ok || string(got.Bytes) != "123456" {
		t.Fatalf("remaining = %v %v", got, ok)
	}
}

func TestCacheWipeClearsThumbs(t *testing.T) {
	c := NewCache(CacheConfig{}, nil)
	c.PutThumb("a", "/1", "1", "image/jpeg", []byte("x"))
	c.Wipe()
	if c.ThumbCount() != 0 || c.ThumbBytes() != 0 {
		t.Fatal("wipe must clear thumbnails")
	}
}
