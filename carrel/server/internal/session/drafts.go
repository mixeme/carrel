package session

import "time"

// HistoryEntry is one line in the ten-entry query history (S-15).
type HistoryEntry struct {
	Query string
	At    time.Time
}

// Drafts holds multi-step edits that survive one request but not the session.
type Drafts struct {
	RejectedEdit  []byte
	PendingPhoto  []byte
	PendingImport []byte
}

func (d *Drafts) clear() {
	d.RejectedEdit = nil
	d.PendingPhoto = nil
	d.PendingImport = nil
}

// LossRegistry records properties a server has already been seen dropping (S-7).
type LossRegistry struct {
	byServer map[string]map[string]struct{}
}

// Note records that a server dropped a property.
func (r *LossRegistry) Note(serverID, prop string) {
	if r.byServer == nil {
		r.byServer = make(map[string]map[string]struct{})
	}
	set := r.byServer[serverID]
	if set == nil {
		set = make(map[string]struct{})
		r.byServer[serverID] = set
	}
	set[prop] = struct{}{}
}

// Has reports whether a loss was already noted for a server/property pair.
func (r *LossRegistry) Has(serverID, prop string) bool {
	set := r.byServer[serverID]
	if set == nil {
		return false
	}
	_, ok := set[prop]
	return ok
}

// Props returns lost property names for one server.
func (r *LossRegistry) Props(serverID string) []string {
	set := r.byServer[serverID]
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	return out
}
