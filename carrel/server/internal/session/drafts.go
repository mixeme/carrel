package session

import "time"

// HistoryEntry is one line in the ten-entry query history (S-15.3): the query
// as typed, how it ended, and when the poll finished.
type HistoryEntry struct {
	Query string
	Hits  int
	// None: at least one source answered and nothing was found. Zero hits
	// with None false means no source answered at all.
	None bool
	At   time.Time
}

// DraftSlot names one multi-step edit held between requests.
type DraftSlot int

const (
	// DraftRejectedEdit is the edit a conflicting write handed back (S-8).
	DraftRejectedEdit DraftSlot = iota
	// DraftPendingPhoto is an uploaded photo awaiting confirmation (S-13).
	DraftPendingPhoto
	// DraftPendingImport is a parsed import awaiting confirmation (S-19).
	DraftPendingImport
	// DraftMail is the letter being written (S-26.2); it never reaches disk.
	DraftMail
	numDraftSlots
)

// Drafts holds multi-step edits that survive one request but not the session.
// Access goes through Session so the slots stay behind the session lock.
type Drafts struct {
	slots [numDraftSlots][]byte
}

func (d *Drafts) get(slot DraftSlot) []byte {
	if slot < 0 || slot >= numDraftSlots {
		return nil
	}
	return d.slots[slot]
}

func (d *Drafts) set(slot DraftSlot, data []byte) {
	if slot < 0 || slot >= numDraftSlots {
		return
	}
	d.slots[slot] = data
}

func (d *Drafts) clear() {
	for i := range d.slots {
		d.slots[i] = nil
	}
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
