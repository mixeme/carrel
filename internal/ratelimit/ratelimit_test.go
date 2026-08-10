// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ratelimit

import (
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTest(o Options) (*Limiter, *clock) {
	c := &clock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	o.Now = c.now
	return New(o), c
}

func TestFreeAttemptsThenProgressiveDelay(t *testing.T) {
	l, c := newTest(Options{Free: 2, Base: time.Second, Max: time.Minute})

	// A mistyped password must not cost a wait.
	for i := 0; i < 2; i++ {
		if d := l.Fail("ip:10.0.0.1"); d != 0 {
			t.Fatalf("failure %d already delays by %v", i+1, d)
		}
		if ok, _ := l.Allow("ip:10.0.0.1"); !ok {
			t.Fatalf("blocked after %d failures, within the free allowance", i+1)
		}
	}

	// From here each failure doubles the wait.
	for i, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second} {
		if d := l.Fail("ip:10.0.0.1"); d != want {
			t.Errorf("failure %d delays by %v, want %v", i+3, d, want)
		}
		if ok, wait := l.Allow("ip:10.0.0.1"); ok {
			t.Errorf("attempt allowed during the %v cool-off", want)
		} else if wait != want {
			t.Errorf("reported wait %v, want %v", wait, want)
		}
		c.advance(want)
		if ok, _ := l.Allow("ip:10.0.0.1"); !ok {
			t.Errorf("still blocked after waiting %v", want)
		}
	}
}

func TestDelayIsCapped(t *testing.T) {
	l, _ := newTest(Options{Free: 1, Base: time.Second, Max: 4 * time.Second})

	var last time.Duration
	for i := 0; i < 20; i++ {
		last = l.Fail("key")
	}
	if last != 4*time.Second {
		t.Errorf("delay reached %v, want the cap of 4s", last)
	}
}

// Two keys are throttled per attempt: the address and the account. Neither one
// attacker nor one target can be hammered, and one does not lock out the other
// (§24.5).
func TestKeysAreIndependent(t *testing.T) {
	l, _ := newTest(Options{Free: 1, Base: time.Second, Max: time.Minute})

	l.FailAll("ip:10.0.0.1", "user:ada")
	l.FailAll("ip:10.0.0.1", "user:ada")

	if ok, _ := l.Allow("ip:10.0.0.2"); !ok {
		t.Error("a different address was blocked")
	}
	if ok, _ := l.Allow("user:bob"); !ok {
		t.Error("a different account was blocked")
	}
	if ok, _ := l.AllowAll("ip:10.0.0.2", "user:ada"); ok {
		t.Error("the targeted account was not throttled from a new address")
	}
}

func TestAllowAllReportsLongestWait(t *testing.T) {
	l, _ := newTest(Options{Free: 1, Base: time.Second, Max: time.Minute})

	// Two failures past the free one put "a" at 1s; four put "b" at 4s.
	l.Fail("a")
	l.Fail("a")
	for i := 0; i < 4; i++ {
		l.Fail("b")
	}

	ok, wait := l.AllowAll("a", "b")
	if ok {
		t.Fatal("AllowAll passed while both keys are throttled")
	}
	if wait != 4*time.Second {
		t.Errorf("wait = %v, want the longer 4s", wait)
	}
}

func TestResetOnSuccess(t *testing.T) {
	l, _ := newTest(Options{Free: 1, Base: time.Second, Max: time.Minute})

	l.Fail("user:ada")
	l.Fail("user:ada")
	if ok, _ := l.Allow("user:ada"); ok {
		t.Fatal("expected a cool-off")
	}

	l.Reset("user:ada")
	if ok, _ := l.Allow("user:ada"); !ok {
		t.Error("a successful login did not clear the counter")
	}
}

func TestForgetAndSweep(t *testing.T) {
	l, c := newTest(Options{Free: 1, Base: time.Second, Max: time.Minute, Forget: time.Hour})

	l.Fail("key")
	l.Fail("key")
	c.advance(2 * time.Hour)

	if ok, _ := l.Allow("key"); !ok {
		t.Error("a key quiet for longer than Forget is still throttled")
	}
	l.Fail("other")
	c.advance(2 * time.Hour)
	if n := l.Sweep(); n == 0 {
		t.Error("Sweep kept a key that had gone quiet")
	}
	if l.Len() != 0 {
		t.Errorf("limiter holds %d keys after the sweep", l.Len())
	}
}
