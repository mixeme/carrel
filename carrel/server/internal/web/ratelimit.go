package web

import (
	"net"
	"net/http"
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

// Limiter applies architecture.md rate buckets in memory.
type Limiter struct {
	loginAccount map[string]*bucket
	loginIP      map[string]*bucket
	anonIP       map[string]*bucket
	session      map[string]*bucket
	mu           sync.Mutex
}

func NewLimiter() *Limiter {
	return &Limiter{
		loginAccount: make(map[string]*bucket),
		loginIP:      make(map[string]*bucket),
		anonIP:       make(map[string]*bucket),
		session:      make(map[string]*bucket),
	}
}

func (l *Limiter) loginAccountBucket(key string) *bucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.loginAccount[key]
	if !ok {
		b = newBucket(float64(conf.LoginRatePerAccount), conf.LoginRatePerAccount)
		l.loginAccount[key] = b
	}
	return b
}

func (l *Limiter) loginIPBucket(key string) *bucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.loginIP[key]
	if !ok {
		b = newBucket(float64(conf.LoginRatePerIP), conf.LoginRatePerIP)
		l.loginIP[key] = b
	}
	return b
}

func (l *Limiter) anonIPBucket(key string) *bucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.anonIP[key]
	if !ok {
		b = newBucket(float64(conf.AnonTokenRatePerIP), conf.AnonTokenRatePerIP)
		l.anonIP[key] = b
	}
	return b
}

func (l *Limiter) sessionBucket(sid string) *bucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.session[sid]
	if !ok {
		b = newBucket(float64(conf.SessionRateLimit), conf.SessionRateLimit)
		l.session[sid] = b
	}
	return b
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (l *Limiter) allowLogin(login, ip string, now time.Time) bool {
	login = normalizeLogin(login)
	if !l.loginAccountBucket(login).allow(now, 1) {
		return false
	}
	return l.loginIPBucket(ip).allow(now, 1)
}

func (l *Limiter) allowAnon(ip string, now time.Time) bool {
	return l.anonIPBucket(ip).allow(now, 1)
}

func (l *Limiter) allowSession(sid string, now time.Time) bool {
	if sid == "" {
		return true
	}
	return l.sessionBucket(sid).allow(now, 1)
}
