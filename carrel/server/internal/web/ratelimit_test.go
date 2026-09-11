package web

import (
	"fmt"
	"testing"
	"time"
)

// Ф2-2. Keys come from the network, so a family of buckets must not grow
// with the attacker: once past the threshold, refilled buckets are dropped.
func TestLimiterDropsRefilledBuckets(t *testing.T) {
	l := NewLimiter()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	n := sweepAbove * 3
	for i := 0; i < n; i++ {
		l.allowAnon(fmt.Sprintf("2001:db8::%x", i), now)
	}
	if got := l.size(l.anonIP); got != n {
		t.Fatalf("before refill: %d buckets, want %d", got, n)
	}
	// One key is kept busy: it must survive the sweep, still limited.
	busy := "203.0.113.9"
	for i := 0; i < 40; i++ {
		l.allowAnon(busy, now.Add(59*time.Second))
	}
	// Half a second past the refill window: the idle buckets are full, the
	// busy one has a quarter of a token.
	later := now.Add(60*time.Second + 500*time.Millisecond)
	l.allowAnon("203.0.113.10", later)
	if got := l.size(l.anonIP); got > 3 {
		t.Fatalf("after refill: %d buckets, want the busy one, the new one and little else", got)
	}
	if l.allowAnon(busy, later) {
		t.Fatal("busy bucket was reset by the sweep")
	}
}

func TestLimiterSweepIsRateLimitedItself(t *testing.T) {
	l := NewLimiter()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	for i := 0; i < sweepAbove+10; i++ {
		l.allowSession(fmt.Sprintf("sid-%d", i), now)
	}
	// Within the same second nothing is swept even though every bucket is
	// nearly full — a sweep per request would be an O(n) cost the attacker
	// controls.
	l.allowSession("sid-x", now.Add(500*time.Millisecond))
	if got := l.size(l.session); got != sweepAbove+11 {
		t.Fatalf("swept within the interval: %d", got)
	}
}
