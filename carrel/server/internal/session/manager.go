package session

import (
	"sync"
	"sync/atomic"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
)

// Manager holds every live session in the process (S-4).
type Manager struct {
	cfg  *conf.Config
	mu   sync.Mutex
	byID map[string]*Session
	// idle and absolute are the lifetimes of sessions, set by the
	// administrator. Idle applies to live sessions at once; absolute to the
	// ones started after it changed.
	idle, absolute atomic.Int64
}

// SetLifetimes changes how long sessions live; zero keeps the default.
func (m *Manager) SetLifetimes(idle, absolute time.Duration) {
	if idle <= 0 {
		idle = conf.SessionIdleTimeout
	}
	if absolute <= 0 {
		absolute = conf.SessionAbsoluteTimeout
	}
	m.idle.Store(int64(idle))
	m.absolute.Store(int64(absolute))
}

func (m *Manager) idleTimeout() time.Duration {
	if m == nil || m.idle.Load() == 0 {
		return conf.SessionIdleTimeout
	}
	return time.Duration(m.idle.Load())
}

func (m *Manager) absoluteTimeout() time.Duration {
	if m == nil || m.absolute.Load() == 0 {
		return conf.SessionAbsoluteTimeout
	}
	return time.Duration(m.absolute.Load())
}

// NewManager creates a session manager.
func NewManager(cfg *conf.Config) *Manager {
	return &Manager{
		cfg:  cfg,
		byID: make(map[string]*Session),
	}
}

// Create starts a session after login. A new SID is issued (security.md).
func (m *Manager) Create(accountID, login string, dataKey [crypt.KeyLen]byte) (*Session, error) {
	now := time.Now()
	s, err := newSession(m, accountID, login, dataKey, now)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.byID[s.id] = s
	m.mu.Unlock()
	return s, nil
}

// Get returns a live session by SID. Expired sessions are ended and reported
// as missing (S-4.1).
func (m *Manager) Get(sid string) (*Session, bool) {
	m.mu.Lock()
	s, ok := m.byID[sid]
	if !ok {
		m.mu.Unlock()
		return nil, false
	}
	if s.ended || s.expiredAt(time.Now()) {
		delete(m.byID, sid)
		m.mu.Unlock()
		s.end()
		return nil, false
	}
	m.mu.Unlock()
	return s, true
}

// Touch records activity under the same lock order as Get.
func (m *Manager) Touch(sid string, now time.Time) bool {
	m.mu.Lock()
	s, ok := m.byID[sid]
	if !ok || s.ended {
		m.mu.Unlock()
		return false
	}
	if s.expiredAt(now) {
		delete(m.byID, sid)
		m.mu.Unlock()
		s.end()
		return false
	}
	s.touchAt(now)
	m.mu.Unlock()
	return true
}

// End terminates one session and wipes its keys and cache.
func (m *Manager) End(sid string) {
	m.mu.Lock()
	s, ok := m.byID[sid]
	if ok {
		delete(m.byID, sid)
	}
	m.mu.Unlock()
	if ok {
		s.end()
	}
}

// LiveCount reports non-expired sessions for one account (S-10.6).
func (m *Manager) LiveCount(accountID string) int {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.byID {
		if s.accountID != accountID || s.ended || s.expiredAt(now) {
			continue
		}
		n++
	}
	return n
}

// EndAccount terminates every live session for one account (S-10).
func (m *Manager) EndAccount(accountID string) {
	m.mu.Lock()
	var ids []string
	for id, s := range m.byID {
		if s.accountID == accountID {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.End(id)
	}
}

// EndAccountExcept terminates every live session for one account except the
// named SID. Used after a password change so the browser that submitted the
// form keeps its session (S-10).
func (m *Manager) EndAccountExcept(accountID, exceptSID string) {
	m.mu.Lock()
	var ids []string
	for id, s := range m.byID {
		if s.accountID == accountID && id != exceptSID {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.End(id)
	}
}

// EndAll wipes every session (process restart / shutdown).
func (m *Manager) EndAll() {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.byID))
	for _, s := range m.byID {
		sessions = append(sessions, s)
	}
	m.byID = make(map[string]*Session)
	m.mu.Unlock()
	for _, s := range sessions {
		s.end()
	}
}

// PurgeExpired ends sessions past idle or absolute timeout.
func (m *Manager) PurgeExpired(now time.Time) int {
	m.mu.Lock()
	var expired []string
	for id, s := range m.byID {
		if s.expiredAt(now) {
			expired = append(expired, id)
		}
	}
	m.mu.Unlock()
	for _, id := range expired {
		m.End(id)
	}
	return len(expired)
}

// ProcessBytes reports cache bytes charged to the process-wide budget.
func (m *Manager) ProcessBytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	for _, s := range m.byID {
		total += s.cache.totalBytes()
	}
	return total
}
