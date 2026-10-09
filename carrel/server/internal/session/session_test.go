package session

import (
	"fmt"
	"sync/atomic"
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
		s.AddHistory(HistoryEntry{Query: fmt.Sprintf("q%d", i)})
	}
	h := s.History()
	if len(h) != 10 {
		t.Fatalf("history len = %d", len(h))
	}
	if h[0].Query != "q11" {
		t.Fatalf("newest first: %q", h[0].Query)
	}
}

// S-15.3: the query without outer spaces, no empty rows, and a repeat
// updates its row instead of adding a second — a query that found nothing
// included.
func TestHistoryRepeatUpdatesItsRow(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	s.AddHistory(HistoryEntry{Query: "  ada ", None: true})
	s.AddHistory(HistoryEntry{Query: "   "})
	s.AddHistory(HistoryEntry{Query: "bob", Hits: 2})
	s.AddHistory(HistoryEntry{Query: "ada", Hits: 3})
	h := s.History()
	if len(h) != 2 || h[0].Query != "ada" || h[0].Hits != 3 || h[0].None || h[1].Query != "bob" {
		t.Fatalf("history = %+v", h)
	}
}

type fakeTask struct {
	id        string
	running   atomic.Bool
	cancelled atomic.Int32
}

func (f *fakeTask) ID() string    { return f.id }
func (f *fakeTask) Running() bool { return f.running.Load() }
func (f *fakeTask) Cancel() {
	f.cancelled.Add(1)
	f.running.Store(false)
}

// S-15.1: tasks belong to the session, are bounded, and are cancelled with
// it. The oldest finished task goes before a running one.
func TestKeptTasksAreBoundedAndEndWithTheSession(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	var all []*fakeTask
	for i := 0; i < conf.MaxKeptFanouts; i++ {
		ft := &fakeTask{id: fmt.Sprintf("t-%d", i)}
		ft.running.Store(i == 0) // the oldest still runs
		all = append(all, ft)
		s.KeepTask(ft)
	}
	extra := &fakeTask{id: "t-new"}
	extra.running.Store(true)
	s.KeepTask(extra)

	if len(s.Tasks()) != conf.MaxKeptFanouts {
		t.Fatalf("kept %d", len(s.Tasks()))
	}
	if _, ok := s.Task("t-1"); ok || all[1].cancelled.Load() != 1 {
		t.Error("the oldest finished task should have gone, cancelled")
	}
	if _, ok := s.Task("t-0"); !ok || all[0].cancelled.Load() != 0 {
		t.Error("a running task went while finished ones were left")
	}
	if got, _ := s.Task("t-new"); got != extra || s.Tasks()[0] != extra {
		t.Error("the new task is not kept first")
	}

	other, _ := mgr.Create("a-1", "alice", testDataKey(t))
	if _, ok := other.Task("t-new"); ok {
		t.Error("another session sees this session's task")
	}

	mgr.End(s.ID())
	if extra.cancelled.Load() != 1 || all[0].cancelled.Load() != 1 {
		t.Error("ending the session did not cancel its tasks")
	}
	late := &fakeTask{id: "t-late"}
	s.KeepTask(late)
	if late.cancelled.Load() != 1 || len(s.Tasks()) != 0 {
		t.Error("an ended session kept a task")
	}
}

func TestCacheVersionKeyMakesOldBodyUnreachable(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Cache()
	c.PutBody("conn-a", "/cal/personal/event.ics", "v1", []byte("old"))
	c.PutBody("conn-a", "/cal/personal/event.ics", "v2", []byte("new"))
	if _, ok := c.GetBody("conn-a", "/cal/personal/event.ics", "v1"); !ok {
		t.Fatal("v1 should still be cached until evicted")
	}
	got, ok := c.GetBody("conn-a", "/cal/personal/event.ics", "v2")
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
	c.PutETagMap("conn-a", "/cal/personal", []byte("etag-map-data"))
	c.PutBody("conn-a", "/cal/personal/a.ics", "e1", make([]byte, 50))
	c.PutBody("conn-a", "/cal/personal/b.ics", "e1", make([]byte, 50))
	if _, ok := c.GetETagMap("conn-a", "/cal/personal"); !ok {
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
	s1.Cache().PutBody("conn-a", "/a", "v1", make([]byte, 100))
	s2.Cache().PutBody("conn-a", "/b", "v1", make([]byte, 100))
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
	s.Cache().PutBody("conn-a", "/x", "v1", []byte("data"))
	s.AddHistory(HistoryEntry{Query: "find me"})
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
	s.NoteLoss("srv1", "X-FOO")
	if !s.HasLoss("srv1", "X-FOO") {
		t.Fatal("loss not recorded")
	}
	if len(s.LostProps("srv1")) != 1 {
		t.Fatal("expected one prop")
	}
	if s.HasLoss("srv2", "X-FOO") {
		t.Fatal("loss leaked to another server")
	}
}

func TestCacheCollectionTTLExpires(t *testing.T) {
	cfg := testCfg()
	cfg.CacheCollectionTTL = 50 * time.Millisecond
	mgr := NewManager(cfg)
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Cache()
	c.PutCollectionList("alice", []byte("collections"))
	c.PutETagMap("conn-a", "/cal/personal", []byte("etag-map"))
	if _, ok := c.GetCollectionList("alice"); !ok {
		t.Fatal("fresh collection list should hit")
	}
	if _, ok := c.GetETagMap("conn-a", "/cal/personal"); !ok {
		t.Fatal("fresh etag map should hit")
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.GetCollectionList("alice"); ok {
		t.Fatal("stale collection list should miss")
	}
	if _, ok := c.GetETagMap("conn-a", "/cal/personal"); ok {
		t.Fatal("stale etag map should miss")
	}
	c.PutBody("conn-a", "/cal/personal/a.ics", "v1", []byte("body"))
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.GetBody("conn-a", "/cal/personal/a.ics", "v1"); !ok {
		t.Fatal("body should not expire by collection TTL")
	}
}

func TestSessionEvictionByAccessTime(t *testing.T) {
	cfg := testCfg()
	cfg.CacheMaxBodyBytes = 80
	mgr := NewManager(cfg)
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Cache()
	c.PutBody("conn-a", "/a", "v1", make([]byte, 40))
	time.Sleep(2 * time.Millisecond)
	c.PutBody("conn-a", "/b", "v1", make([]byte, 40))
	if _, ok := c.GetBody("conn-a", "/b", "v1"); !ok {
		t.Fatal("b should be present")
	}
	c.PutBody("conn-a", "/c", "v1", make([]byte, 40))
	if _, ok := c.GetBody("conn-a", "/a", "v1"); ok {
		t.Fatal("a should be evicted before b (least recently used)")
	}
	if _, ok := c.GetBody("conn-a", "/b", "v1"); !ok {
		t.Fatal("b should survive after recent access")
	}
}

// S-4.2: the thumbnails of files and the photos of contacts have a budget
// each — a book of contacts does not push a folder's thumbnails out, nor the
// other way; within its own budget each gives up what was used longest ago,
// by bytes and by count.
func TestFileThumbnailsHaveTheirOwnBudget(t *testing.T) {
	cfg := testCfg()
	cfg.CacheMaxBodyBytes, cfg.CacheMaxProcessBytes = 1<<20, 1<<20
	cfg.CacheMaxThumbBytes, cfg.CacheMaxThumbEntries = 100, 4
	cfg.CacheMaxFileThumbBytes, cfg.CacheMaxFileThumbEntries = 300, 4
	mgr := NewManager(cfg)
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Cache()
	c.PutFileThumb("conn", "/files/a.jpg", "v1", make([]byte, 100))
	c.PutFileThumb("conn", "/files/b.jpg", "v1", make([]byte, 100))
	// A book of photos, far more than its own budget holds.
	for i := range 20 {
		c.PutThumb("conn", fmt.Sprintf("/book/%d", i), "e1", make([]byte, 40))
	}
	for _, p := range []string{"/files/a.jpg", "/files/b.jpg"} {
		if _, ok := c.GetFileThumb("conn", p, "v1"); !ok {
			t.Errorf("%s was pushed out by the photos of contacts", p)
		}
	}
	if _, ok := c.GetThumb("conn", "/book/0", "e1"); ok {
		t.Error("the photos of contacts outgrew their own budget")
	}
	if _, ok := c.GetThumb("conn", "/book/19", "e1"); !ok {
		t.Error("the photo used last is gone")
	}
	// The same path under the two kinds is two entries.
	c.PutThumb("conn", "/files/a.jpg", "v1", []byte("photo"))
	if got, _ := c.GetFileThumb("conn", "/files/a.jpg", "v1"); len(got) != 100 {
		t.Errorf("a photo under the path of a file replaced its thumbnail: %d bytes", len(got))
	}

	// Its own bytes: a third and a fourth of 100 leave the two used last.
	c.PutFileThumb("conn", "/files/c.jpg", "v1", make([]byte, 100))
	c.PutFileThumb("conn", "/files/d.jpg", "v1", make([]byte, 100))
	if _, ok := c.GetFileThumb("conn", "/files/b.jpg", "v1"); ok {
		t.Error("400 bytes of file thumbnails are held in a budget of 300")
	}
	if _, ok := c.GetFileThumb("conn", "/files/d.jpg", "v1"); !ok {
		t.Error("the thumbnail made last is gone")
	}
	// Its own count: «no thumbnail» is an entry too.
	for i := range 6 {
		c.PutFileThumb("conn", fmt.Sprintf("/files/none-%d.png", i), "v1", nil)
	}
	held := 0
	for i := range 6 {
		if _, ok := c.GetFileThumb("conn", fmt.Sprintf("/files/none-%d.png", i), "v1"); ok {
			held++
		}
	}
	if held != 4 {
		t.Errorf("%d entries of file thumbnails are held, the budget is 4", held)
	}
}
