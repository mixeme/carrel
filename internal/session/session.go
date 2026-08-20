// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// ErrNotFound is returned for an unknown, expired or already-destroyed
// session.
var ErrNotFound = errors.New("session: not found")

// IDLen is the length in bytes of a session identifier and of a CSRF token.
const IDLen = 32

// User is what the handlers hand over when a login succeeds. The session layer
// does not import the store; it keeps only this much of an account.
type User struct {
	ID                 string
	Login              string
	Admin              bool
	MustChangePassword bool
	// EscrowNotice asks the UI to show the deposit notice once (§5.4).
	EscrowNotice bool
}

// Session is one logged-in browser. It exists only in memory: nothing here
// reaches the volume, so a restart logs everyone out and takes every DEK with
// it (§4).
//
// The exported fields never change after Create and are safe to read from any
// goroutine. Everything mutable sits behind mu and is reached through methods.
type Session struct {
	ID        string
	UserID    string
	Login     string
	Admin     bool
	CSRF      string
	CreatedAt time.Time

	mu           sync.Mutex
	dek          crypto.Key
	lastSeen     time.Time
	deadline     time.Time
	mustChange   bool
	escrowNotice bool
	dead         bool
	cache        *Cache
	losses       *model.LossRegistry
	conflicts    map[string]ConflictDraft
	photos       map[string]PhotoDraft
	imports      map[string]ImportDraft
}

// Cache returns the session's DAV collection cache (§12).
func (s *Session) Cache() *Cache {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cache
}

// DEK returns the user's data key. It aliases the session's own copy: do not
// wipe it, the manager does that when the session ends.
func (s *Session) DEK() crypto.Key {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dek
}

// MustChangePassword reports whether the user is still on a temporary password
// and has to be held on the change-password screen (§5.2).
func (s *Session) MustChangePassword() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mustChange
}

// EscrowNotice reports whether the deposit notice is still owed to the user.
func (s *Session) EscrowNotice() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.escrowNotice
}

// ClearEscrowNotice marks the notice as shown.
func (s *Session) ClearEscrowNotice() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.escrowNotice = false
}

// Info is a snapshot of a session for the admin UI (§5.5).
type Info struct {
	ID        string
	UserID    string
	Login     string
	CreatedAt time.Time
	LastSeen  time.Time
	ExpiresAt time.Time
}

// Options configure a Manager. Zero values fall back to the defaults.
type Options struct {
	// Idle ends a session that has gone quiet.
	Idle time.Duration
	// Absolute caps the lifetime of a session regardless of activity, so a
	// stolen cookie cannot be renewed forever.
	Absolute time.Duration
	// Now overrides the clock.
	Now func() time.Time
	// Cache holds per-session collection cache limits (§12).
	Cache CacheConfig
	// OnEnd is called with the identifier of a session that has just ended,
	// however it ended. Work started on behalf of a session and outliving the
	// request that started it — a fan-out poll above all — is stopped from
	// here, so a logout or an expiry leaves nothing running (§16).
	OnEnd func(sessionID string)
}

// Default session lifetimes. The administrator overrides them from global
// settings (§5.5).
const (
	DefaultIdle     = 12 * time.Hour
	DefaultAbsolute = 7 * 24 * time.Hour
)

func (o Options) withDefaults() Options {
	if o.Idle <= 0 {
		o.Idle = DefaultIdle
	}
	if o.Absolute <= 0 {
		o.Absolute = DefaultAbsolute
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	o.Cache = o.Cache.withDefaults()
	return o
}

// Manager is the in-memory session store and DEK keyring.
type Manager struct {
	opts   Options
	budget *budget

	mu       sync.Mutex
	sessions map[string]*Session
	byUser   map[string]map[string]struct{}
}

// New returns a Manager with the given options.
func New(opts Options) *Manager {
	opts = opts.withDefaults()
	return &Manager{
		opts:     opts,
		budget:   newBudget(opts.Cache.MaxProcessBytes),
		sessions: make(map[string]*Session),
		byUser:   make(map[string]map[string]struct{}),
	}
}

func (m *Manager) now() time.Time { return m.opts.Now() }

// Create starts a session for a user who has just proved their password. The
// identifier is fresh every time, which is the session-fixation defence of
// §24.5: whatever cookie the browser arrived with is never reused.
//
// The manager takes ownership of dek and wipes it when the session ends.
func (m *Manager) Create(u User, dek crypto.Key) (*Session, error) {
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	csrf, err := randomID()
	if err != nil {
		return nil, err
	}

	now := m.now()
	s := &Session{
		ID:           id,
		UserID:       u.ID,
		Login:        u.Login,
		Admin:        u.Admin,
		CSRF:         csrf,
		CreatedAt:    now,
		dek:          dek,
		lastSeen:     now,
		deadline:     now.Add(m.opts.Absolute),
		mustChange:   u.MustChangePassword,
		escrowNotice: u.EscrowNotice,
		cache:        NewCache(m.opts.Cache, m.opts.Now),
	}
	s.cache.bind(m.budget)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.insert(s)
	return s, nil
}

// insert must be called with the manager lock held.
func (m *Manager) insert(s *Session) {
	m.sessions[s.ID] = s
	ids, ok := m.byUser[s.UserID]
	if !ok {
		ids = make(map[string]struct{})
		m.byUser[s.UserID] = ids
	}
	ids[s.ID] = struct{}{}
}

// detach must be called with the manager lock held. It unlinks a session
// without touching its key material.
func (m *Manager) detach(s *Session) {
	delete(m.sessions, s.ID)
	if ids, ok := m.byUser[s.UserID]; ok {
		delete(ids, s.ID)
		if len(ids) == 0 {
			delete(m.byUser, s.UserID)
		}
	}
}

// Get returns a live session and marks it as seen. An expired session is
// destroyed on the way out, so its DEK does not linger until the next sweep.
func (m *Manager) Get(id string) (*Session, bool) {
	if id == "" {
		return nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, false
	}
	now := m.now()
	if m.expired(s, now) {
		m.remove(s)
		return nil, false
	}
	s.mu.Lock()
	s.lastSeen = now
	s.mu.Unlock()
	return s, true
}

// expired must be called with the manager lock held.
func (m *Manager) expired(s *Session, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dead || !now.Before(s.deadline) || now.Sub(s.lastSeen) >= m.opts.Idle
}

// remove must be called with the manager lock held. It wipes the DEK before
// dropping the session, so the key does not wait for the collector (§24.6).
func (m *Manager) remove(s *Session) {
	s.mu.Lock()
	if s.cache != nil {
		s.cache.Wipe()
		s.cache = nil
	}
	s.wipeScratch()
	s.dek.Zero()
	s.dek = nil
	s.dead = true
	s.mu.Unlock()
	m.detach(s)
	if m.opts.OnEnd != nil {
		m.opts.OnEnd(s.ID)
	}
}

// Rotate replaces a session with an identical one under a new identifier,
// carrying the keyring across. The old identifier stops working at once.
func (m *Manager) Rotate(id string) (*Session, error) {
	next, err := randomID()
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	old, ok := m.sessions[id]
	if !ok {
		return nil, ErrNotFound
	}

	old.mu.Lock()
	fresh := &Session{
		ID:           next,
		UserID:       old.UserID,
		Login:        old.Login,
		Admin:        old.Admin,
		CSRF:         old.CSRF,
		CreatedAt:    old.CreatedAt,
		dek:          old.dek,
		lastSeen:     m.now(),
		deadline:     old.deadline,
		mustChange:   old.mustChange,
		escrowNotice: old.escrowNotice,
		cache:        old.cache,
		losses:       old.losses,
		conflicts:    old.conflicts,
		photos:       old.photos,
	}
	// The key and scratch moved to the new session; the old shell must not wipe them.
	old.dek = nil
	old.cache = nil
	old.losses = nil
	old.conflicts = nil
	old.photos = nil
	old.dead = true
	old.mu.Unlock()

	m.detach(old)
	m.insert(fresh)
	return fresh, nil
}

// Destroy ends one session — logout.
func (m *Manager) Destroy(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		m.remove(s)
	}
}

// DestroyUser ends every session of one user and reports how many went. It is
// what "disable user" and "kill sessions" call; disabling has to take effect
// immediately (§5.5).
func (m *Manager) DestroyUser(userID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	n := 0
	for id := range m.byUser[userID] {
		if s, ok := m.sessions[id]; ok {
			m.remove(s)
			n++
		}
	}
	return n
}

// Count returns the number of live sessions of one user, for the admin list.
func (m *Manager) Count(userID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepLocked()
	return len(m.byUser[userID])
}

// Sessions returns snapshots of one user's live sessions.
func (m *Manager) Sessions(userID string) []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepLocked()

	out := make([]Info, 0, len(m.byUser[userID]))
	for id := range m.byUser[userID] {
		s, ok := m.sessions[id]
		if !ok {
			continue
		}
		s.mu.Lock()
		out = append(out, Info{
			ID:        s.ID,
			UserID:    s.UserID,
			Login:     s.Login,
			CreatedAt: s.CreatedAt,
			LastSeen:  s.lastSeen,
			ExpiresAt: s.deadline,
		})
		s.mu.Unlock()
	}
	return out
}

// SetMustChangePassword updates the flag on every live session of a user, so a
// password change lifts the forced-change screen without a re-login.
func (m *Manager) SetMustChangePassword(userID string, must bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.byUser[userID] {
		if s, ok := m.sessions[id]; ok {
			s.mu.Lock()
			s.mustChange = must
			s.mu.Unlock()
		}
	}
}

// Sweep drops expired sessions and returns how many it removed. Call it
// periodically: without it an abandoned session keeps its DEK in memory until
// someone happens to ask for it by ID.
func (m *Manager) Sweep() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sweepLocked()
}

func (m *Manager) sweepLocked() int {
	now := m.now()
	n := 0
	for _, s := range m.sessions {
		if m.expired(s, now) {
			m.remove(s)
			n++
		}
	}
	return n
}

// StartSweeper runs Sweep until ctx is cancelled, then wipes what is left.
// This is the SIGTERM path: shutdown must not leave keys behind (§24.6).
func (m *Manager) StartSweeper(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = time.Minute
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				m.Close()
				return
			case <-t.C:
				m.Sweep()
			}
		}
	}()
}

// Close ends every session and wipes every key.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		m.remove(s)
	}
}

// Len returns the number of live sessions.
func (m *Manager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// CheckCSRF compares a submitted token with the session's own, in constant
// time. Every mutating request goes through it, htmx fragments included
// (§24.5).
func CheckCSRF(s *Session, token string) bool {
	if s == nil || s.CSRF == "" || token == "" {
		return false
	}
	return crypto.Equal([]byte(s.CSRF), []byte(token))
}

func randomID() (string, error) {
	b, err := crypto.Random(IDLen)
	if err != nil {
		return "", err
	}
	defer crypto.Zero(b)
	return base64.RawURLEncoding.EncodeToString(b), nil
}
