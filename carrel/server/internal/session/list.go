package session

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"
)

// Info is one live session as its owner's account page shows it (C-6
// SessionRow). The session ID is a cookie and never leaves the server:
// PublicID stands for it.
type Info struct {
	PublicID string
	Agent    string
	Address  string
	Started  time.Time
	LastSeen time.Time
}

// PublicID is the name a session is shown and ended by: derived from the
// SID one way, so knowing it opens nothing.
func publicID(sid string) string {
	sum := sha256.Sum256([]byte("carrel.session.public|" + sid))
	return hex.EncodeToString(sum[:8])
}

// PublicID is this session's shown name.
func (s *Session) PublicID() string { return publicID(s.id) }

// SetClient notes what the session was started from: the browser and the
// address, as the owner will recognise them. Memory only, like the session.
func (s *Session) SetClient(agent, address string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agent, s.address = agent, address
}

// SessionsOf lists an account's live sessions, the most recently seen first.
func (m *Manager) SessionsOf(accountID string) []Info {
	now := time.Now()
	m.mu.Lock()
	var list []*Session
	for _, s := range m.byID {
		if s.accountID == accountID && !s.ended && !s.expiredAt(now) {
			list = append(list, s)
		}
	}
	m.mu.Unlock()
	out := make([]Info, 0, len(list))
	for _, s := range list {
		s.mu.Lock()
		out = append(out, Info{PublicID: publicID(s.id), Agent: s.agent, Address: s.address, Started: s.created, LastSeen: s.lastSeen})
		s.mu.Unlock()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}

// EndPublic ends one of an account's sessions by its shown name; another
// account's session answers like none.
func (m *Manager) EndPublic(accountID, public string) bool {
	m.mu.Lock()
	var found *Session
	for id, s := range m.byID {
		if s.accountID == accountID && publicID(id) == public {
			found = s
			delete(m.byID, id)
			break
		}
	}
	m.mu.Unlock()
	if found == nil {
		return false
	}
	found.end()
	return true
}
