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
	// DraftIssuedDevice is a davloom device password just issued (S-27.2):
	// the next GET of the page shows it once and clears it.
	DraftIssuedDevice
	// DraftRewrite is the rewrite of a replaced address a saved card asks
	// about (S-15.4): the pairs, and once scanned, the records it would
	// write with the version each was read at.
	DraftRewrite
	// DraftNewAttach is the files of a new event, task or note waiting for
	// Save under the UID it will have, with what was typed (S-20).
	DraftNewAttach
	// DraftBackupJob is a backup job as its form was typed when Browse
	// opened the folder chooser over it (s-backup-2): the chooser's answer
	// is the form again with what was typed and the folder chosen.
	DraftBackupJob
	// DraftProfileLink is the link to an Apple profile last issued (S-23):
	// the page keeps its card while the link lives (s-plan-1). The state
	// has only its hash, so the link is here, in memory, or nowhere.
	DraftProfileLink
	// DraftDavloomAdd is Add device as it was typed when Create password
	// asked the person's password first (s-davloom-5): the password's sheet
	// issues the device from it; its Cancel brings the form back as typed.
	DraftDavloomAdd
	// DraftIssuedTrigger is a backup job's external trigger just issued
	// through the password's sheet (193 p. 3): the job's page shows its
	// secret once, and clears it.
	DraftIssuedTrigger
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
