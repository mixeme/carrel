package session

import (
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
)

func TestGetEndsExpiredSession(t *testing.T) {
	mgr := NewManager(testCfg())
	key := testDataKey(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s, err := mgr.Create("a-1", "alice", key)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.absoluteAt = now.Add(-time.Second)
	s.mu.Unlock()

	if _, ok := mgr.Get(s.ID()); ok {
		t.Fatal("expired session should not be returned")
	}
	if _, ok := mgr.Get(s.ID()); ok {
		t.Fatal("expired session should be removed")
	}
}

func TestEndAccountExceptKeepsNamedSession(t *testing.T) {
	mgr := NewManager(testCfg())
	s1, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	other, err := mgr.Create("a-2", "bob", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}

	mgr.EndAccountExcept("a-1", s1.ID())
	if _, ok := mgr.Get(s1.ID()); !ok {
		t.Fatal("kept session should remain")
	}
	if _, ok := mgr.Get(s2.ID()); ok {
		t.Fatal("other session of same account should be gone")
	}
	if _, ok := mgr.Get(other.ID()); !ok {
		t.Fatal("other account session should remain")
	}
}

func TestEndAccountEndsAllSessions(t *testing.T) {
	mgr := NewManager(testCfg())
	s1, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	other, err := mgr.Create("a-2", "bob", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}

	mgr.EndAccount("a-1")
	if _, ok := mgr.Get(s1.ID()); ok {
		t.Fatal("s1 should be gone")
	}
	if _, ok := mgr.Get(s2.ID()); ok {
		t.Fatal("s2 should be gone")
	}
	if _, ok := mgr.Get(other.ID()); !ok {
		t.Fatal("other account session should remain")
	}
}

func TestEndedSessionIsInert(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	mgr.End(s.ID())

	if _, ok := s.DataKey(); ok {
		t.Fatal("ended session should not expose data key")
	}
	if s.AcquireFanout() {
		t.Fatal("ended session should not grant fanout")
	}
	s.Cache().PutBody("conn-a", "/x", "v1", []byte("data"))
	if _, ok := s.Cache().GetBody("conn-a", "/x", "v1"); ok {
		t.Fatal("ended session cache should not accept writes")
	}
	s.SetDraft(DraftPendingPhoto, []byte("x"))
	if got := s.Draft(DraftPendingPhoto); got != nil {
		t.Fatal("ended session should not store drafts")
	}
	s.AddHistory(HistoryEntry{Query: "ghost"})
	if len(s.History()) != 0 {
		t.Fatal("ended session should not store history")
	}
	s.NoteLoss("srv1", "X-FOO")
	if s.HasLoss("srv1", "X-FOO") {
		t.Fatal("ended session should not record losses")
	}
}

func TestCacheIsolatesConnections(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Cache()
	c.PutBody("conn-a", "/same/path", "v1", []byte("from-a"))
	c.PutBody("conn-b", "/same/path", "v1", []byte("from-b"))

	gotA, ok := c.GetBody("conn-a", "/same/path", "v1")
	if !ok || string(gotA) != "from-a" {
		t.Fatalf("conn-a body = %q, ok=%v", gotA, ok)
	}
	gotB, ok := c.GetBody("conn-b", "/same/path", "v1")
	if !ok || string(gotB) != "from-b" {
		t.Fatalf("conn-b body = %q, ok=%v", gotB, ok)
	}

	c.PutETagMap("conn-a", "/cal/personal", []byte("map-a"))
	c.PutETagMap("conn-b", "/cal/personal", []byte("map-b"))
	if _, ok := c.GetETagMap("conn-a", "/cal/personal"); !ok {
		t.Fatal("conn-a etag map missing")
	}
	if _, ok := c.GetETagMap("conn-b", "/cal/personal"); !ok {
		t.Fatal("conn-b etag map missing")
	}
}

func TestInvalidateCollectionList(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Cache()
	c.PutCollectionList("alice", []byte("collections"))
	c.InvalidateCollectionList("alice")
	if _, ok := c.GetCollectionList("alice"); ok {
		t.Fatal("collection list should be invalidated")
	}
}

func TestTouchExpiresUnderManagerLock(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(conf.SessionAbsoluteTimeout + time.Hour)
	s.mu.Lock()
	s.absoluteAt = past
	s.mu.Unlock()

	if mgr.Touch(s.ID(), past) {
		t.Fatal("touch on expired session should fail")
	}
	if _, ok := mgr.Get(s.ID()); ok {
		t.Fatal("expired session should be removed")
	}
}

func TestPurgeExpiredRemovesOrphanSession(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	s.Cache().PutBody("conn-a", "/x", "v1", []byte("cached"))

	past := time.Now().Add(-conf.SessionAbsoluteTimeout - time.Hour)
	s.mu.Lock()
	s.absoluteAt = past
	s.lastSeen = past
	s.mu.Unlock()

	if n := mgr.PurgeExpired(time.Now()); n != 1 {
		t.Fatalf("purged = %d, want 1", n)
	}
	if _, ok := mgr.Get(s.ID()); ok {
		t.Fatal("orphan session should be gone without Get by SID")
	}
	if mgr.ProcessBytes() != 0 {
		t.Fatalf("cache bytes = %d after purge", mgr.ProcessBytes())
	}
}
