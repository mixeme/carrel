package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
)

// Session holds per-login state that dies with the session (S-4).
type Session struct {
	id        string
	accountID string
	login     string

	dataKey [crypt.KeyLen]byte
	csrf    []byte

	created    time.Time
	lastSeen   time.Time
	absoluteAt time.Time

	cache   *Cache
	drafts  Drafts
	history []HistoryEntry
	losses  LossRegistry
	fanout  int

	mgr *Manager
}

// ID returns the session identifier (cookie value).
func (s *Session) ID() string { return s.id }

// AccountID returns the owning account.
func (s *Session) AccountID() string { return s.accountID }

// Login returns the account login.
func (s *Session) Login() string { return s.login }

// DataKey returns a copy of the data key. Callers must not retain it longer
// than the session lives.
func (s *Session) DataKey() [crypt.KeyLen]byte {
	var k [crypt.KeyLen]byte
	copy(k[:], s.dataKey[:])
	return k
}

// CSRFToken is the token for mutating requests.
func (s *Session) CSRFToken() string {
	return base64.RawURLEncoding.EncodeToString(s.csrf)
}

// CheckCSRF validates a mutating request token (constant time).
func (s *Session) CheckCSRF(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false
	}
	if len(raw) != len(s.csrf) {
		return false
	}
	return subtle.ConstantTimeCompare(raw, s.csrf) == 1
}

// Touch records activity and extends the idle deadline.
func (s *Session) Touch(now time.Time) {
	s.lastSeen = now
}

// Expired reports whether idle or absolute lifetime has ended (S-4).
func (s *Session) Expired(now time.Time) bool {
	if !now.Before(s.absoluteAt) {
		return true
	}
	return !now.Before(s.lastSeen.Add(conf.SessionIdleTimeout))
}

// Cache returns the session cache (S-4.2).
func (s *Session) Cache() *Cache { return s.cache }

// Drafts returns draft storage for multi-step edits.
func (s *Session) Drafts() *Drafts { return &s.drafts }

// Losses returns the per-server loss registry (S-7).
func (s *Session) Losses() *LossRegistry { return &s.losses }

// AcquireFanout reserves a live fanout slot (S-15). At most MaxLiveFanouts
// per session.
func (s *Session) AcquireFanout() bool {
	if s.fanout >= conf.MaxLiveFanouts {
		return false
	}
	s.fanout++
	return true
}

// ReleaseFanout frees a fanout slot.
func (s *Session) ReleaseFanout() {
	if s.fanout > 0 {
		s.fanout--
	}
}

// LiveFanouts returns how many fanouts are currently running.
func (s *Session) LiveFanouts() int { return s.fanout }

// AddHistory prepends a query to the ten-line history (S-15).
func (s *Session) AddHistory(query string) {
	s.history = append([]HistoryEntry{{Query: query, At: time.Now()}}, s.history...)
	if len(s.history) > 10 {
		s.history = s.history[:10]
	}
}

// History returns a copy of the query history.
func (s *Session) History() []HistoryEntry {
	out := make([]HistoryEntry, len(s.history))
	copy(out, s.history)
	return out
}

func newSession(mgr *Manager, accountID, login string, dataKey [crypt.KeyLen]byte, now time.Time) (*Session, error) {
	csrf := make([]byte, 32)
	if _, err := rand.Read(csrf); err != nil {
		return nil, err
	}
	id := make([]byte, 24)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	s := &Session{
		id:         base64.RawURLEncoding.EncodeToString(id),
		accountID:  accountID,
		login:      login,
		dataKey:    dataKey,
		csrf:       csrf,
		created:    now,
		lastSeen:   now,
		absoluteAt: now.Add(conf.SessionAbsoluteTimeout),
		cache:      newCache(mgr.cfg, mgr),
		losses:     LossRegistry{byServer: make(map[string]map[string]struct{})},
		mgr:        mgr,
	}
	return s, nil
}

// end wipes sensitive material and unregisters the session (S-2, S-4).
func (s *Session) end() {
	crypt.Wipe(s.dataKey[:])
	crypt.Wipe(s.csrf)
	s.cache.clear()
	s.drafts.clear()
	s.history = nil
	s.losses.byServer = nil
	s.fanout = 0
}
