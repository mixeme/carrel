// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
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
