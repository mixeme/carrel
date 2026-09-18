package web

import (
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
)

type bucket struct {
	mu       sync.Mutex
	tokens   float64
	last     time.Time
	capacity float64
	rate     float64 // tokens per second
}

func newBucket(capacity float64, perMinute int) *bucket {
	return &bucket{
		tokens:   capacity,
		last:     time.Now(),
		capacity: capacity,
		rate:     float64(perMinute) / 60.0,
	}
}

func (b *bucket) allow(now time.Time, n float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens < n {
		return false
	}
	b.tokens -= n
	return true
}

// room reports whether one more request would pass, without taking it.
func (b *bucket) room(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tokens+now.Sub(b.last).Seconds()*b.rate >= 1
}

// full reports whether the bucket has refilled completely: nobody is being
// limited by it, so it can be forgotten and rebuilt on the next request.
func (b *bucket) full(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tokens+now.Sub(b.last).Seconds()*b.rate >= b.capacity
}

// bucketMap is one keyed family of buckets. Keys arrive from the network
// (addresses, cookie values, typed logins), so the map must not grow with the
// attacker: once it holds more than sweepAbove entries, a lookup first drops
// every bucket that has refilled. A full bucket carries no state worth
// keeping, and rebuilding it costs one allocation.
type bucketMap struct {
	perMinute int
	buckets   map[string]*bucket
	lastSweep time.Time
}

const (
	sweepAbove    = 4096
	sweepInterval = time.Second
)

func newBucketMap(perMinute int) *bucketMap {
	return &bucketMap{perMinute: perMinute, buckets: make(map[string]*bucket)}
}

func (m *bucketMap) get(key string, now time.Time) *bucket {
	if len(m.buckets) > sweepAbove && now.Sub(m.lastSweep) >= sweepInterval {
		m.lastSweep = now
		for k, b := range m.buckets {
			if b.full(now) {
				delete(m.buckets, k)
			}
		}
	}
	b, ok := m.buckets[key]
	if !ok {
		b = newBucket(float64(m.perMinute), m.perMinute)
		b.last = now
		m.buckets[key] = b
	}
	return b
}

// Limiter applies architecture.md rate buckets in memory.
type Limiter struct {
	mu           sync.Mutex
	loginAccount *bucketMap
	loginIP      *bucketMap
	anonIP       *bucketMap
	session      *bucketMap
	// link is the bucket of one published link once its secret matched: the
	// link is an identity then, as a session is (S-25).
	link *bucketMap
}

func NewLimiter() *Limiter {
	return &Limiter{
		loginAccount: newBucketMap(conf.LoginRatePerAccount),
		loginIP:      newBucketMap(conf.LoginRatePerIP),
		anonIP:       newBucketMap(conf.AnonTokenRatePerIP),
		session:      newBucketMap(conf.SessionRateLimit),
		link:         newBucketMap(conf.SessionRateLimit),
	}
}

func (l *Limiter) take(m *bucketMap, key string, now time.Time) *bucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	return m.get(key, now)
}

// size returns how many buckets one family holds (tests).
func (l *Limiter) size(m *bucketMap) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(m.buckets)
}

func (l *Limiter) allowLogin(login, ip string, now time.Time) bool {
	login = normalizeLogin(login)
	if !l.take(l.loginAccount, login, now).allow(now, 1) {
		return false
	}
	return l.take(l.loginIP, ip, now).allow(now, 1)
}

func (l *Limiter) allowAnon(ip string, now time.Time) bool {
	return l.take(l.anonIP, ip, now).allow(now, 1)
}

// anonRoom reports whether the address may try a secret now. A published
// link spends the address's bucket only on a miss: the bucket is there
// against guessing, and a matching secret is not a guess (S-25).
func (l *Limiter) anonRoom(ip string, now time.Time) bool {
	return l.take(l.anonIP, ip, now).room(now)
}

// allowLink takes one request of a published link whose secret matched.
func (l *Limiter) allowLink(id string, now time.Time) bool {
	return l.take(l.link, id, now).allow(now, 1)
}

func (l *Limiter) allowSession(sid string, now time.Time) bool {
	if sid == "" {
		return true
	}
	return l.take(l.session, sid, now).allow(now, 1)
}
