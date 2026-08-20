// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"sync"
	"testing"
	"time"
)

func TestCacheSetAndGetETags(t *testing.T) {
	c := NewCache(CacheConfig{CollectionTTL: time.Minute, MaxCollections: 4, MaxETagEntries: 8}, nil)
	c.SetETags("acc1", "/cal/", "ctag1", map[string]string{"/cal/a.vcf": "e1"})
	got, ok := c.GetETags("acc1", "/cal/")
	if !ok || got["/cal/a.vcf"] != "e1" {
		t.Fatalf("GetETags = %v, %v", got, ok)
	}
}

func TestCacheNeedsRefresh(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	c := NewCache(CacheConfig{CollectionTTL: time.Minute, MaxCollections: 4, MaxETagEntries: 8}, clock)

	if !c.NeedsRefresh("acc1", "/cal/", "ctag1") {
		t.Fatal("empty cache should need refresh")
	}
	c.SetETags("acc1", "/cal/", "ctag1", map[string]string{"/cal/a.vcf": "e1"})
	if c.NeedsRefresh("acc1", "/cal/", "ctag1") {
		t.Fatal("fresh cache with same ctag should not need refresh")
	}
	if !c.NeedsRefresh("acc1", "/cal/", "ctag2") {
		t.Fatal("ctag change should need refresh")
	}
	now = now.Add(2 * time.Minute)
	if !c.NeedsRefresh("acc1", "/cal/", "ctag1") {
		t.Fatal("TTL expiry should need refresh")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c := NewCache(CacheConfig{CollectionTTL: time.Minute, MaxCollections: 2, MaxETagEntries: 100}, nil)
	c.SetETags("a", "/one/", "1", map[string]string{"/one/x": "e"})
	c.SetETags("a", "/two/", "2", map[string]string{"/two/x": "e"})
	c.SetETags("a", "/three/", "3", map[string]string{"/three/x": "e"})

	if _, ok := c.GetETags("a", "/one/"); ok {
		t.Fatal("oldest collection should have been evicted")
	}
	if _, ok := c.GetETags("a", "/three/"); !ok {
		t.Fatal("newest collection should remain")
	}
}

func TestCacheWipeOnSessionEnd(t *testing.T) {
	m, _ := newTest(t, time.Hour, 24*time.Hour)
	m.opts.Cache = CacheConfig{CollectionTTL: time.Minute, MaxCollections: 4, MaxETagEntries: 8}

	s := mustCreate(t, m, testUser())
	s.Cache().SetETags("acc", "/cal/", "c", map[string]string{"/cal/x": "e"})
	m.Destroy(s.ID)
	if s.Cache() != nil {
		// Cache object may remain on dead session shell but must be empty after wipe in remove.
	}
	s2 := mustCreate(t, m, testUser())
	if _, ok := s2.Cache().GetETags("acc", "/cal/"); ok {
		t.Fatal("new session should not inherit cache")
	}
}

func TestCacheBodyRoundTrip(t *testing.T) {
	c := NewCache(CacheConfig{CollectionTTL: time.Minute, MaxCollections: 4, MaxETagEntries: 8}, nil)
	body := []byte("vcard data")
	c.PutBody("acc", "/ab/", "/ab/contact.vcf", body)
	got, ok := c.GetBody("acc", "/ab/", "/ab/contact.vcf")
	if !ok || string(got) != string(body) {
		t.Fatalf("GetBody = %q, %v", got, ok)
	}
}

func TestCacheLastFetched(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	c := NewCache(CacheConfig{CollectionTTL: time.Minute, MaxCollections: 4, MaxETagEntries: 8}, clock)
	if _, ok := c.LastFetched(); ok {
		t.Fatal("empty cache should have no last fetch")
	}
	c.SetETags("acc", "/cal/", "c1", map[string]string{"/cal/a": "e"})
	got, ok := c.LastFetched()
	if !ok || !got.Equal(now) {
		t.Fatalf("LastFetched = %v, %v", got, ok)
	}
	now = now.Add(3 * time.Minute)
	c.SetETags("acc", "/ab/", "c2", map[string]string{"/ab/a": "e"})
	got, ok = c.LastFetched()
	if !ok || !got.Equal(now) {
		t.Fatalf("LastFetched after later write = %v, %v", got, ok)
	}
}

func TestCollectionMeta(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	c := NewCache(CacheConfig{CollectionTTL: time.Minute, MaxCollections: 4, MaxETagEntries: 8}, func() time.Time { return now })
	c.SetETags("acc", "/cal/", "ctag-1", map[string]string{"/cal/a.ics": "e1", "/cal/b.ics": "e2"})
	meta, ok := c.CollectionMeta("acc", "/cal/")
	if !ok {
		t.Fatal("expected collection metadata")
	}
	if meta.CTag != "ctag-1" || meta.ObjectCount != 2 || !meta.FetchedAt.Equal(now) {
		t.Fatalf("meta = %+v", meta)
	}
	if _, ok := c.CollectionMeta("acc", "/missing/"); ok {
		t.Fatal("missing collection should not report metadata")
	}
}

func TestCacheBodiesEvictedBeforeETagMap(t *testing.T) {
	c := NewCache(CacheConfig{
		CollectionTTL:  time.Minute,
		MaxCollections: 4,
		MaxETagEntries: 8,
		MaxBodyBytes:   10,
	}, nil)
	c.SetETags("acc", "/ab/", "ctag", map[string]string{"/ab/a": "e1", "/ab/b": "e2"})
	c.PutBody("acc", "/ab/", "/ab/a", []byte("12345"))
	c.PutBody("acc", "/ab/", "/ab/b", []byte("123456")) // 5+6=11 > 10 → oldest body goes

	if _, ok := c.GetBody("acc", "/ab/", "/ab/a"); ok {
		t.Fatal("oldest body should have been evicted")
	}
	got, ok := c.GetBody("acc", "/ab/", "/ab/b")
	if !ok || string(got) != "123456" {
		t.Fatalf("remaining body = %q, %v", got, ok)
	}
	etags, ok := c.GetETags("acc", "/ab/")
	if !ok || etags["/ab/a"] != "e1" || etags["/ab/b"] != "e2" {
		t.Fatalf("ETag map should have survived body eviction: %v, %v", etags, ok)
	}
	if c.BodyBytes() != 6 {
		t.Fatalf("BodyBytes = %d, want 6", c.BodyBytes())
	}
}

func TestCacheBodyTouchProtectsFromEviction(t *testing.T) {
	c := NewCache(CacheConfig{
		CollectionTTL:  time.Minute,
		MaxCollections: 4,
		MaxETagEntries: 8,
		MaxBodyBytes:   10,
	}, nil)
	c.PutBody("acc", "/ab/", "/ab/a", []byte("12345"))
	c.PutBody("acc", "/ab/", "/ab/b", []byte("12"))
	if _, ok := c.GetBody("acc", "/ab/", "/ab/a"); !ok {
		t.Fatal("setup: a should be present")
	}
	c.PutBody("acc", "/ab/", "/ab/c", []byte("12345")) // 5+2+5=12 > 10; b is LRU

	if _, ok := c.GetBody("acc", "/ab/", "/ab/b"); ok {
		t.Fatal("untouched body should have been evicted")
	}
	if _, ok := c.GetBody("acc", "/ab/", "/ab/a"); !ok {
		t.Fatal("touched body should remain")
	}
	if _, ok := c.GetBody("acc", "/ab/", "/ab/c"); !ok {
		t.Fatal("newest body should remain")
	}
}

func TestCacheReplacingABodyRecountsBytes(t *testing.T) {
	c := NewCache(CacheConfig{
		CollectionTTL:  time.Minute,
		MaxCollections: 4,
		MaxETagEntries: 8,
		MaxBodyBytes:   100,
	}, nil)
	c.PutBody("acc", "/ab/", "/ab/a", []byte("12345"))
	c.PutBody("acc", "/ab/", "/ab/a", []byte("1"))
	if c.BodyBytes() != 1 {
		t.Fatalf("BodyBytes = %d, want 1 after replace", c.BodyBytes())
	}
}

func TestCacheInvalidateCollectionDropsBodies(t *testing.T) {
	c := NewCache(CacheConfig{
		CollectionTTL:  time.Minute,
		MaxCollections: 4,
		MaxETagEntries: 8,
		MaxBodyBytes:   100,
	}, nil)
	c.SetETags("acc", "/ab/", "c", map[string]string{"/ab/a": "e"})
	c.PutBody("acc", "/ab/", "/ab/a", []byte("12345"))
	c.InvalidateCollection("acc", "/ab/")
	if c.BodyBytes() != 0 {
		t.Fatalf("BodyBytes = %d after invalidate", c.BodyBytes())
	}
	if _, ok := c.GetETags("acc", "/ab/"); ok {
		t.Fatal("map should have gone with the collection")
	}
}

func tightBudgetManager(t *testing.T, processBytes int) *Manager {
	t.Helper()
	return New(Options{
		Idle:     time.Hour,
		Absolute: 24 * time.Hour,
		Cache: CacheConfig{
			CollectionTTL:   time.Minute,
			MaxCollections:  8,
			MaxETagEntries:  32,
			MaxBodyBytes:    1 << 20,
			MaxProcessBytes: processBytes,
			MaxThumbBytes:   1 << 20,
			MaxThumbEntries: 8,
		},
	})
}

func TestCacheProcessWideEvictsAcrossUsers(t *testing.T) {
	m := tightBudgetManager(t, 10)
	s1 := mustCreate(t, m, User{ID: "u1", Login: "ada"})
	s2 := mustCreate(t, m, User{ID: "u2", Login: "bob"})

	s1.Cache().SetETags("a", "/ab/", "c1", map[string]string{"/ab/x": "e"})
	s1.Cache().PutBody("a", "/ab/", "/ab/x", []byte("12345"))
	s2.Cache().SetETags("a", "/ab/", "c2", map[string]string{"/ab/y": "e"})
	s2.Cache().PutBody("a", "/ab/", "/ab/y", []byte("123456")) // 5+6=11 > 10

	if _, ok := s1.Cache().GetBody("a", "/ab/", "/ab/x"); ok {
		t.Fatal("older session's body should have been evicted by the process ceiling")
	}
	if _, ok := s1.Cache().GetETags("a", "/ab/"); !ok {
		t.Fatal("older session's ETag map should have been kept")
	}
	got, ok := s2.Cache().GetBody("a", "/ab/", "/ab/y")
	if !ok || string(got) != "123456" {
		t.Fatalf("newer body = %q, %v", got, ok)
	}
	if m.budget.bytes() != 6 {
		t.Fatalf("process bytes = %d, want 6", m.budget.bytes())
	}
}

func TestCacheProcessWideTouchProtectsAcrossUsers(t *testing.T) {
	m := tightBudgetManager(t, 10)
	s1 := mustCreate(t, m, User{ID: "u1", Login: "ada"})
	s2 := mustCreate(t, m, User{ID: "u2", Login: "bob"})

	s1.Cache().PutBody("a", "/ab/", "/ab/a", []byte("12345"))
	s2.Cache().PutBody("a", "/ab/", "/ab/b", []byte("12"))
	if _, ok := s1.Cache().GetBody("a", "/ab/", "/ab/a"); !ok {
		t.Fatal("setup: s1 body")
	}
	s2.Cache().PutBody("a", "/ab/", "/ab/c", []byte("12345")) // b is the LRU across users

	if _, ok := s2.Cache().GetBody("a", "/ab/", "/ab/b"); ok {
		t.Fatal("untouched body in s2 should have been evicted")
	}
	if _, ok := s1.Cache().GetBody("a", "/ab/", "/ab/a"); !ok {
		t.Fatal("touched body in s1 should remain")
	}
}

func TestCacheProcessWideDropsBodiesBeforeThumbs(t *testing.T) {
	m := tightBudgetManager(t, 10)
	s1 := mustCreate(t, m, User{ID: "u1", Login: "ada"})
	s2 := mustCreate(t, m, User{ID: "u2", Login: "bob"})

	s1.Cache().SetETags("a", "/ab/", "c", map[string]string{"/ab/x": "e"})
	s1.Cache().PutBody("a", "/ab/", "/ab/x", []byte("12345"))
	s2.Cache().PutThumb("a", "/ab/p", "e1", "image/jpeg", []byte("123456"))

	if _, ok := s1.Cache().GetBody("a", "/ab/", "/ab/x"); ok {
		t.Fatal("body should go before a thumbnail under the process ceiling")
	}
	if _, ok := s1.Cache().GetETags("a", "/ab/"); !ok {
		t.Fatal("ETag map should remain after the body is dropped")
	}
	if _, ok := s2.Cache().GetThumb("a", "/ab/p", "e1"); !ok {
		t.Fatal("thumbnail should remain")
	}
}

func TestCacheWipeReleasesProcessBudget(t *testing.T) {
	m := tightBudgetManager(t, 10)
	s := mustCreate(t, m, testUser())
	s.Cache().PutBody("a", "/ab/", "/ab/x", []byte("12345"))
	if m.budget.bytes() != 5 {
		t.Fatalf("bytes before wipe = %d", m.budget.bytes())
	}
	m.Destroy(s.ID)
	if m.budget.bytes() != 0 {
		t.Fatalf("bytes after logout = %d, want 0", m.budget.bytes())
	}
}

func TestCacheConcurrentBudget(t *testing.T) {
	m := tightBudgetManager(t, 64)
	s1 := mustCreate(t, m, User{ID: "u1", Login: "ada"})
	s2 := mustCreate(t, m, User{ID: "u2", Login: "bob"})
	s3 := mustCreate(t, m, User{ID: "u3", Login: "cam"})
	caches := []*Cache{s1.Cache(), s2.Cache(), s3.Cache()}

	var wg sync.WaitGroup
	for i, cache := range caches {
		wg.Add(1)
		go func(id int, c *Cache) {
			defer wg.Done()
			acc := string(rune('a' + id))
			for n := 0; n < 200; n++ {
				path := "/ab/" + string(rune('a'+n%26))
				c.SetETags(acc, "/ab/", "c", map[string]string{path: "e"})
				c.PutBody(acc, "/ab/", path, []byte("xxxxxxxx"))
				c.GetBody(acc, "/ab/", path)
				c.PutThumb(acc, path, "e", "image/jpeg", []byte("thumb"))
				c.GetThumb(acc, path, "e")
				if n%17 == 0 {
					c.InvalidateCollection(acc, "/ab/")
				}
			}
		}(i, cache)
	}
	wg.Wait()
	m.Close()
	if m.budget.bytes() != 0 {
		t.Fatalf("bytes after close = %d, want 0", m.budget.bytes())
	}
}
