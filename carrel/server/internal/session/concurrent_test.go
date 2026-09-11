package session

import (
	"fmt"
	"sync"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

// One session serves several requests at once: the desk asks for fragments
// while a fanout is still running. Run under -race.
func TestSessionConcurrentUse(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}

	const workers = 16
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < 50; n++ {
				key := fmt.Sprintf("/c/%d-%d", i, n)
				s.Cache().PutBody(key, "v1", []byte("body"))
				s.Cache().GetBody(key, "v1")
				s.Cache().PutETagMap(key, []byte("map"))
				s.Cache().InvalidatePath(key)
				s.AddHistory(key)
				s.NoteLoss("srv1", key)
				s.HasLoss("srv1", key)
				s.SetDraft(DraftPendingPhoto, []byte(key))
				s.Draft(DraftPendingPhoto)
				s.CSRFToken()
				s.DataKey()
				if s.AcquireFanout() {
					s.ReleaseFanout()
				}
				mgr.ProcessBytes()
			}
		}(i)
	}
	wg.Wait()

	if got := s.LiveFanouts(); got != 0 {
		t.Fatalf("live fanouts = %d, want 0", got)
	}
	if got := len(s.History()); got != 10 {
		t.Fatalf("history = %d entries, want 10", got)
	}
}

// The four-fanout ceiling is an invariant, not a hint: a burst of concurrent
// starts must not get a fifth through.
func TestFanoutCeilingUnderRace(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var granted int
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.AcquireFanout() {
				mu.Lock()
				granted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if granted != 4 {
		t.Fatalf("granted %d fanouts, want 4", granted)
	}
	if got := s.LiveFanouts(); got != 4 {
		t.Fatalf("live fanouts = %d, want 4", got)
	}
}

// S-4.2 and C-12: bodies are given up before version maps, both inside one
// session and when the process budget forces eviction across sessions.
func TestBodiesEvictedBeforeMaps(t *testing.T) {
	cfg := testCfg()
	cfg.CacheMaxBodyBytes = 100
	cfg.CacheMaxETagEntries = 64
	mgr := NewManager(cfg)
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Cache()
	c.PutETagMap("/c/one", make([]byte, 60))
	c.PutBody("/c/one/a", "v1", make([]byte, 60))
	c.PutBody("/c/one/b", "v1", make([]byte, 60)) // pushes bodies past the cap

	if _, ok := c.GetETagMap("/c/one"); !ok {
		t.Fatal("version map evicted while a body was still cached")
	}
	if _, ok := c.GetBody("/c/one/a", "v1"); ok {
		t.Fatal("oldest body should have been evicted first")
	}
	if _, ok := c.GetBody("/c/one/b", "v1"); !ok {
		t.Fatal("newest body should have survived")
	}
}

func TestProcessBudgetEvictsAcrossSessions(t *testing.T) {
	cfg := testCfg()
	cfg.CacheMaxProcessBytes = 300
	mgr := NewManager(cfg)

	var sessions []*Session
	for i := 0; i < 3; i++ {
		s, err := mgr.Create(fmt.Sprintf("a-%d", i), fmt.Sprintf("u%d", i), testDataKey(t))
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, s)
		s.Cache().PutETagMap("/c/map", make([]byte, 50))
		s.Cache().PutBody("/c/body", "v1", make([]byte, 100))
	}

	if total := mgr.ProcessBytes(); total > cfg.CacheMaxProcessBytes {
		t.Fatalf("process bytes = %d, over the %d budget", total, cfg.CacheMaxProcessBytes)
	}
	// Whatever had to go, the version maps are the last thing given up.
	for i, s := range sessions {
		if _, ok := s.Cache().GetETagMap("/c/map"); !ok {
			t.Fatalf("session %d lost its version map before every body went", i)
		}
	}
}

// After a PUT the request layer hands the session's registry to
// model.CompareAfterPut. Nothing else in the module references the interface
// yet, so without this assertion the two sides can drift apart unnoticed.
var _ model.LossAggregator = LossNotes{}
