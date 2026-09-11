package model

import "sync"

type mapAgg struct {
	mu   sync.Mutex
	seen map[string]map[string]struct{}
}

func newMapAgg() *mapAgg {
	return &mapAgg{seen: make(map[string]map[string]struct{})}
}

func (m *mapAgg) Has(serverID, prop string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.seen[serverID]
	if set == nil {
		return false
	}
	_, ok := set[prop]
	return ok
}

func (m *mapAgg) Note(serverID, prop string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.seen[serverID] == nil {
		m.seen[serverID] = make(map[string]struct{})
	}
	m.seen[serverID][prop] = struct{}{}
}
