// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTest(t *testing.T, idle, absolute time.Duration) (*Manager, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	return New(Options{Idle: idle, Absolute: absolute, Now: c.now}), c
}

func testUser() User {
	return User{ID: "u1", Login: "ada", Admin: false}
}

func mustDEK(t *testing.T) crypto.Key {
	t.Helper()
	dek, err := crypto.NewDEK()
	if err != nil {
		t.Fatalf("NewDEK: %v", err)
	}
	return dek
}

func mustCreate(t *testing.T, m *Manager, u User) *Session {
	t.Helper()
	s, err := m.Create(u, mustDEK(t))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return s
}

func TestCreateAndGet(t *testing.T) {
	m, _ := newTest(t, time.Hour, 24*time.Hour)
	s := mustCreate(t, m, testUser())

	if len(s.ID) < 32 || len(s.CSRF) < 32 {
		t.Errorf("identifier %d chars, CSRF %d chars: too short to be unguessable", len(s.ID), len(s.CSRF))
	}
	if s.ID == s.CSRF {
		t.Error("session identifier doubles as the CSRF token")
	}

	got, ok := m.Get(s.ID)
	if !ok || got.ID != s.ID {
		t.Fatalf("Get returned %v, %v", got, ok)
	}
	if _, ok := m.Get("not a session"); ok {
		t.Error("Get accepted an unknown identifier")
	}
}

// Every login gets a new identifier, so a value planted in the browser before
// the login cannot survive it (§24.5).
func TestEachLoginGetsFreshID(t *testing.T) {
	m, _ := newTest(t, time.Hour, 24*time.Hour)
	first := mustCreate(t, m, testUser())
	second := mustCreate(t, m, testUser())

	if first.ID == second.ID {
		t.Fatal("two logins produced the same session identifier")
	}
	if first.CSRF == second.CSRF {
		t.Error("two logins produced the same CSRF token")
	}
}

func TestRotateKeepsKeyringDropsOldID(t *testing.T) {
	m, _ := newTest(t, time.Hour, 24*time.Hour)
	s := mustCreate(t, m, testUser())
	before := s.DEK().Clone()
	defer before.Zero()

	rotated, err := m.Rotate(s.ID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if rotated.ID == s.ID {
		t.Fatal("rotation reused the identifier")
	}
	if _, ok := m.Get(s.ID); ok {
		t.Error("the old identifier still works after rotation")
	}
	if !crypto.Equal(rotated.DEK(), before) {
		t.Error("rotation lost the data key")
	}
	if m.Len() != 1 {
		t.Errorf("manager holds %d sessions after rotation, want 1", m.Len())
	}
}

func TestDestroyWipesKey(t *testing.T) {
	m, _ := newTest(t, time.Hour, 24*time.Hour)
	s := mustCreate(t, m, testUser())

	// Hold the same backing array the session holds: wiping there must show
	// through here (§24.6).
	key := s.DEK()
	m.Destroy(s.ID)

	for i, b := range key {
		if b != 0 {
			t.Fatalf("byte %d of the data key survived Destroy", i)
		}
	}
	if _, ok := m.Get(s.ID); ok {
		t.Error("destroyed session still resolves")
	}
}

func TestDestroyUserEndsEverySession(t *testing.T) {
	m, _ := newTest(t, time.Hour, 24*time.Hour)
	u := testUser()
	a := mustCreate(t, m, u)
	b := mustCreate(t, m, u)
	other := mustCreate(t, m, User{ID: "u2", Login: "bob"})

	if n := m.DestroyUser(u.ID); n != 2 {
		t.Errorf("DestroyUser ended %d sessions, want 2", n)
	}
	for _, s := range []*Session{a, b} {
		if _, ok := m.Get(s.ID); ok {
			t.Error("a session of the disabled user survived")
		}
	}
	if _, ok := m.Get(other.ID); !ok {
		t.Error("another user's session was ended too")
	}
	if m.Count(u.ID) != 0 {
		t.Error("Count still reports sessions for the disabled user")
	}
}

func TestIdleTimeout(t *testing.T) {
	m, c := newTest(t, 30*time.Minute, 24*time.Hour)
	s := mustCreate(t, m, testUser())

	c.advance(20 * time.Minute)
	if _, ok := m.Get(s.ID); !ok {
		t.Fatal("session expired before the idle timeout")
	}
	// The request just made counts as activity, so the clock restarts.
	c.advance(20 * time.Minute)
	if _, ok := m.Get(s.ID); !ok {
		t.Fatal("activity did not extend the idle timeout")
	}
	c.advance(31 * time.Minute)
	if _, ok := m.Get(s.ID); ok {
		t.Error("idle session survived the timeout")
	}
}

func TestAbsoluteTimeout(t *testing.T) {
	m, c := newTest(t, time.Hour, 2*time.Hour)
	s := mustCreate(t, m, testUser())

	// Constant activity must not extend a session past its hard deadline.
	for i := 0; i < 4; i++ {
		c.advance(30 * time.Minute)
		m.Get(s.ID)
	}
	if _, ok := m.Get(s.ID); ok {
		t.Error("session outlived its absolute deadline")
	}
}

func TestSweepDropsExpired(t *testing.T) {
	m, c := newTest(t, 10*time.Minute, 24*time.Hour)
	s := mustCreate(t, m, testUser())
	key := s.DEK()

	c.advance(11 * time.Minute)
	if n := m.Sweep(); n != 1 {
		t.Fatalf("Sweep removed %d sessions, want 1", n)
	}
	for i, b := range key {
		if b != 0 {
			t.Fatalf("byte %d of the data key survived the sweep", i)
		}
	}
	if m.Len() != 0 {
		t.Errorf("manager holds %d sessions after the sweep", m.Len())
	}
}

func TestCloseWipesEverything(t *testing.T) {
	m, _ := newTest(t, time.Hour, 24*time.Hour)
	a := mustCreate(t, m, testUser())
	b := mustCreate(t, m, User{ID: "u2", Login: "bob"})
	keys := []crypto.Key{a.DEK(), b.DEK()}

	m.Close()

	if m.Len() != 0 {
		t.Errorf("manager holds %d sessions after Close", m.Len())
	}
	for _, key := range keys {
		for i, v := range key {
			if v != 0 {
				t.Fatalf("byte %d of a data key survived Close", i)
			}
		}
	}
}

func TestSweeperWipesOnShutdown(t *testing.T) {
	m := New(Options{Idle: time.Hour, Absolute: time.Hour})
	s := mustCreate(t, m, testUser())
	key := s.DEK()

	ctx, cancel := context.WithCancel(context.Background())
	m.StartSweeper(ctx, time.Hour)
	cancel()

	deadline := time.Now().Add(2 * time.Second)
	for m.Len() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if m.Len() != 0 {
		t.Fatal("sessions survived cancellation of the sweeper context")
	}
	for i, b := range key {
		if b != 0 {
			t.Fatalf("byte %d of the data key survived shutdown", i)
		}
	}
}

func TestMustChangePassword(t *testing.T) {
	m, _ := newTest(t, time.Hour, 24*time.Hour)
	u := testUser()
	u.MustChangePassword = true
	s := mustCreate(t, m, u)

	if !s.MustChangePassword() {
		t.Fatal("flag lost on session creation")
	}
	m.SetMustChangePassword(u.ID, false)
	if s.MustChangePassword() {
		t.Error("flag not cleared after the password change")
	}
}

func TestCheckCSRF(t *testing.T) {
	m, _ := newTest(t, time.Hour, 24*time.Hour)
	s := mustCreate(t, m, testUser())
	other := mustCreate(t, m, testUser())

	if !CheckCSRF(s, s.CSRF) {
		t.Error("the session's own token was rejected")
	}
	if CheckCSRF(s, other.CSRF) {
		t.Error("another session's token was accepted")
	}
	if CheckCSRF(s, "") || CheckCSRF(nil, s.CSRF) {
		t.Error("an empty token or a missing session was accepted")
	}
}

func TestSessionsSnapshot(t *testing.T) {
	m, c := newTest(t, time.Hour, 24*time.Hour)
	u := testUser()
	s := mustCreate(t, m, u)

	c.advance(5 * time.Minute)
	m.Get(s.ID)

	infos := m.Sessions(u.ID)
	if len(infos) != 1 {
		t.Fatalf("Sessions returned %d entries, want 1", len(infos))
	}
	if infos[0].Login != u.Login || infos[0].ID != s.ID {
		t.Errorf("snapshot = %+v, want the live session", infos[0])
	}
	if !infos[0].LastSeen.After(infos[0].CreatedAt) {
		t.Error("last seen was not updated by the request")
	}
}
