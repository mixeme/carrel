package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
)

// Session holds per-login state that dies with the session (S-4).
//
// One session serves several requests at once, so everything that changes
// after Create is behind mu: the deadlines, the drafts, the history, the loss
// registry, and the live-fanout count. Cache has its own lock.
type Session struct {
	id        string
	accountID string
	login     string

	mu         sync.Mutex
	dataKey    [crypt.KeyLen]byte
	csrf       []byte
	created    time.Time
	lastSeen   time.Time
	absoluteAt time.Time
	drafts     Drafts
	history    []HistoryEntry
	losses     LossRegistry
	fanout     int

	cache *Cache
	mgr   *Manager
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
	s.mu.Lock()
	defer s.mu.Unlock()
	var k [crypt.KeyLen]byte
	copy(k[:], s.dataKey[:])
	return k
}

// CSRFToken is the token for mutating requests.
func (s *Session) CSRFToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return base64.RawURLEncoding.EncodeToString(s.csrf)
}

// CheckCSRF validates a mutating request token (constant time).
func (s *Session) CheckCSRF(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(raw) != len(s.csrf) {
		return false
	}
	return subtle.ConstantTimeCompare(raw, s.csrf) == 1
}

// Touch records activity and extends the idle deadline.
func (s *Session) Touch(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSeen = now
}

// Expired reports whether idle or absolute lifetime has ended (S-4).
func (s *Session) Expired(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !now.Before(s.absoluteAt) {
		return true
	}
	return !now.Before(s.lastSeen.Add(conf.SessionIdleTimeout))
}

// Cache returns the session cache (S-4.2).
func (s *Session) Cache() *Cache { return s.cache }

// Draft reads one draft slot. Drafts survive a request, not the session.
func (s *Session) Draft(slot DraftSlot) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.drafts.get(slot)
}

// SetDraft writes one draft slot; a nil value clears it.
func (s *Session) SetDraft(slot DraftSlot, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drafts.set(slot, data)
}

// NoteLoss records that a server dropped a property (S-7.1).
func (s *Session) NoteLoss(serverID, prop string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.losses.Note(serverID, prop)
}

// HasLoss reports whether a loss was already noted for a server/property pair,
// so the screen aggregates instead of repeating itself.
func (s *Session) HasLoss(serverID, prop string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.losses.Has(serverID, prop)
}

// LostProps returns the property names one server has been seen dropping.
func (s *Session) LostProps(serverID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.losses.Props(serverID)
}

// AcquireFanout reserves a live fanout slot (S-15). At most MaxLiveFanouts
// per session.
func (s *Session) AcquireFanout() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fanout >= conf.MaxLiveFanouts {
		return false
	}
	s.fanout++
	return true
}

// ReleaseFanout frees a fanout slot.
func (s *Session) ReleaseFanout() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fanout > 0 {
		s.fanout--
	}
}

// LiveFanouts returns how many fanouts are currently running.
func (s *Session) LiveFanouts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fanout
}

// AddHistory prepends a query to the ten-line history (S-15).
func (s *Session) AddHistory(query string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append([]HistoryEntry{{Query: query, At: time.Now()}}, s.history...)
	if len(s.history) > 10 {
		s.history = s.history[:10]
	}
}

// History returns a copy of the query history.
func (s *Session) History() []HistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	crypt.Wipe(s.dataKey[:])
	crypt.Wipe(s.csrf)
	s.drafts.clear()
	s.history = nil
	s.losses.byServer = nil
	s.fanout = 0
	s.mu.Unlock()

	s.cache.clear()
}
