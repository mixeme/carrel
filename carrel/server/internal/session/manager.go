package session

import (
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
)

// Manager holds every live session in the process (S-4).
type Manager struct {
	cfg  *conf.Config
	mu   sync.Mutex
	byID map[string]*Session
}

// NewManager creates a session manager.
func NewManager(cfg *conf.Config) *Manager {
	return &Manager{
		cfg:  cfg,
		byID: make(map[string]*Session),
	}
}

// Create starts a session after login. A new SID is issued (architecture.md).
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

// Get returns a live session by SID.
func (m *Manager) Get(sid string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byID[sid]
	return s, ok
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
		if s.Expired(now) {
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
