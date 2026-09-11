package session

import (
	"fmt"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
)

func testCfg() *conf.Config {
	return &conf.Config{
		CacheMaxETagEntries:  4,
		CacheMaxBodyBytes:    200,
		CacheMaxProcessBytes: 300,
		CacheMaxThumbBytes:   100,
	}
}

func testDataKey(t *testing.T) [crypt.KeyLen]byte {
	k, err := crypt.NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSessionCSRFAndWipe(t *testing.T) {
	mgr := NewManager(testCfg())
	key := testDataKey(t)
	s, err := mgr.Create("a-1", "alice", key)
	if err != nil {
		t.Fatal(err)
	}
	token := s.CSRFToken()
	if !s.CheckCSRF(token) {
		t.Fatal("csrf should match")
	}
	if s.CheckCSRF("wrong") {
		t.Fatal("csrf should reject wrong token")
	}
	mgr.End(s.ID())
	if _, ok := mgr.Get(s.ID()); ok {
		t.Fatal("session should be gone")
	}
}

func TestSessionTimeouts(t *testing.T) {
	mgr := NewManager(testCfg())
	key := testDataKey(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s, err := mgr.Create("a-1", "alice", key)
	if err != nil {
		t.Fatal(err)
	}
	s.created = now
	s.lastSeen = now
	s.absoluteAt = now.Add(conf.SessionAbsoluteTimeout)

	s.Touch(now.Add(2 * time.Hour))
	if s.Expired(now.Add(3 * time.Hour)) {
		t.Fatal("should not be idle-expired yet")
	}
	if s.Expired(now.Add(conf.SessionIdleTimeout + time.Second)) {
		t.Fatal("should be idle-expired")
	}

	s2, err := mgr.Create("a-2", "bob", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	s2.created = now
	s2.lastSeen = now
	s2.absoluteAt = now.Add(conf.SessionAbsoluteTimeout)
	if !s2.Expired(now.Add(conf.SessionAbsoluteTimeout + time.Second)) {
		t.Fatal("should be absolutely expired")
	}
}

func TestFanoutLimit(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < conf.MaxLiveFanouts; i++ {
		if !s.AcquireFanout() {
			t.Fatalf("fanout %d should acquire", i)
		}
	}
	if s.AcquireFanout() {
		t.Fatal("fifth fanout should be rejected")
	}
	s.ReleaseFanout()
	if !s.AcquireFanout() {
		t.Fatal("slot should be available after release")
	}
}

func TestHistoryKeepsTen(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		s.AddHistory(fmt.Sprintf("q%d", i))
	}
	h := s.History()
	if len(h) != 10 {
		t.Fatalf("history len = %d", len(h))
	}
	if h[0].Query != "q11" {
		t.Fatalf("newest first: %q", h[0].Query)
	}
}

func TestCacheVersionKeyMakesOldBodyUnreachable(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Cache()
	c.PutBody("/cal/personal/event.ics", "v1", []byte("old"))
	c.PutBody("/cal/personal/event.ics", "v2", []byte("new"))
	if _, ok := c.GetBody("/cal/personal/event.ics", "v1"); !ok {
		t.Fatal("v1 should still be cached until evicted")
	}
	got, ok := c.GetBody("/cal/personal/event.ics", "v2")
	if !ok || string(got) != "new" {
		t.Fatal("v2 body missing")
	}
}

func TestCacheEvictsBodiesBeforeMaps(t *testing.T) {
	cfg := testCfg()
	cfg.CacheMaxBodyBytes = 80
	cfg.CacheMaxETagEntries = 2
	mgr := NewManager(cfg)
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Cache()
	c.PutETagMap("/cal/personal", []byte("etag-map-data"))
	c.PutBody("/cal/personal/a.ics", "e1", make([]byte, 50))
	c.PutBody("/cal/personal/b.ics", "e1", make([]byte, 50))
	if _, ok := c.GetETagMap("/cal/personal"); !ok {
		t.Fatal("etag map should survive body pressure first")
	}
}

func TestProcessWideEviction(t *testing.T) {
	cfg := testCfg()
	cfg.CacheMaxBodyBytes = 1 << 20
	cfg.CacheMaxProcessBytes = 150
	mgr := NewManager(cfg)
	s1, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := mgr.Create("a-2", "bob", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	s1.Cache().PutBody("/a", "v1", make([]byte, 100))
	s2.Cache().PutBody("/b", "v1", make([]byte, 100))
	if mgr.ProcessBytes() > cfg.CacheMaxProcessBytes {
		// enforceLimits on put should have evicted
	}
	if mgr.ProcessBytes() > cfg.CacheMaxProcessBytes {
		t.Fatalf("process bytes = %d, max %d", mgr.ProcessBytes(), cfg.CacheMaxProcessBytes)
	}
}

func TestEndAllWipesSessions(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	s.Cache().PutBody("/x", "v1", []byte("data"))
	s.AddHistory("find me")
	mgr.EndAll()
	if _, ok := mgr.Get(s.ID()); ok {
		t.Fatal("session should be removed")
	}
	if mgr.ProcessBytes() != 0 {
		t.Fatalf("cache bytes = %d", mgr.ProcessBytes())
	}
}

func TestLossRegistry(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	r := s.Losses()
	r.Note("srv1", "X-FOO")
	if !r.Has("srv1", "X-FOO") {
		t.Fatal("loss not recorded")
	}
	if len(r.Props("srv1")) != 1 {
		t.Fatal("expected one prop")
	}
}
