// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ratelimit throttles guessing against endpoints that take a secret:
// the login form, invite links, and escrow recovery (§24.3).
package ratelimit

import (
	"sync"
	"time"
)

// Options tune a Limiter. Zero values fall back to the defaults.
type Options struct {
	// Free is how many failures are forgiven before delays begin. It keeps a
	// mistyped password from locking anyone out.
	Free int
	// Base is the delay after the first failure past Free; each further
	// failure doubles it.
	Base time.Duration
	// Max caps the delay, so an attacker cannot push a key into a lockout
	// that outlasts the person whose account it is.
	Max time.Duration
	// Forget clears a key that has been quiet for this long.
	Forget time.Duration
	// Now overrides the clock.
	Now func() time.Time
}

// Defaults for the login form. They cost a person one retry pause and an
// attacker several hours per hundred guesses.
const (
	DefaultFree   = 3
	DefaultBase   = 2 * time.Second
	DefaultMax    = 5 * time.Minute
	DefaultForget = time.Hour
)

func (o Options) withDefaults() Options {
	if o.Free < 0 {
		o.Free = 0
	}
	if o.Free == 0 {
		o.Free = DefaultFree
	}
	if o.Base <= 0 {
		o.Base = DefaultBase
	}
	if o.Max <= 0 {
		o.Max = DefaultMax
	}
	if o.Forget <= 0 {
		o.Forget = DefaultForget
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o
}

type entry struct {
	failures int
	retryAt  time.Time
	seen     time.Time
}

// Limiter counts failures per key and makes the next attempt wait. Keys are
// caller-chosen: the login form uses both the client address and the submitted
// login, so neither one attacker address nor one targeted account can be
// hammered (§24.5).
//
// It is a deliberate non-goal to be exact under concurrency: two requests
// arriving together may both pass. What matters is that sustained guessing
// slows down.
type Limiter struct {
	opts Options

	mu      sync.Mutex
	entries map[string]*entry
}

// New returns a Limiter with the given options.
func New(opts Options) *Limiter {
	return &Limiter{opts: opts.withDefaults(), entries: make(map[string]*entry)}
}

// Allow reports whether an attempt on key may proceed. When it may not, the
// second result is how long the caller should tell the client to wait.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok {
		return true, 0
	}
	now := l.opts.Now()
	if now.Sub(e.seen) >= l.opts.Forget {
		delete(l.entries, key)
		return true, 0
	}
	if now.Before(e.retryAt) {
		return false, e.retryAt.Sub(now)
	}
	return true, 0
}

// AllowAll applies Allow to several keys and reports the longest wait among
// those that refuse. Login checks the address and the account together.
func (l *Limiter) AllowAll(keys ...string) (bool, time.Duration) {
	ok := true
	var longest time.Duration
	for _, k := range keys {
		allowed, wait := l.Allow(k)
		if !allowed {
			ok = false
			if wait > longest {
				longest = wait
			}
		}
	}
	return ok, longest
}

// Fail records an unsuccessful attempt and returns the delay now in force.
func (l *Limiter) Fail(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.opts.Now()
	e, ok := l.entries[key]
	if !ok || now.Sub(e.seen) >= l.opts.Forget {
		e = &entry{}
		l.entries[key] = e
	}
	e.failures++
	e.seen = now

	delay := l.delayFor(e.failures)
	e.retryAt = now.Add(delay)
	return delay
}

// FailAll records a failure against several keys at once.
func (l *Limiter) FailAll(keys ...string) {
	for _, k := range keys {
		l.Fail(k)
	}
}

// delayFor doubles from Base once the free attempts are used up, up to Max.
func (l *Limiter) delayFor(failures int) time.Duration {
	over := failures - l.opts.Free
	if over <= 0 {
		return 0
	}
	delay := l.opts.Base
	for i := 1; i < over; i++ {
		delay *= 2
		if delay >= l.opts.Max {
			return l.opts.Max
		}
	}
	if delay > l.opts.Max {
		return l.opts.Max
	}
	return delay
}

// Reset forgets a key. A successful login clears the counters it was throttled
// under, so one person's typos do not follow them around.
func (l *Limiter) Reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.entries, k)
	}
}

// Sweep drops keys that have gone quiet and returns how many it removed. It
// keeps a long-running instance from accumulating one entry per address ever
// seen.
func (l *Limiter) Sweep() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.opts.Now()
	n := 0
	for k, e := range l.entries {
		if now.Sub(e.seen) >= l.opts.Forget {
			delete(l.entries, k)
			n++
		}
	}
	return n
}

// Len returns the number of tracked keys.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}
