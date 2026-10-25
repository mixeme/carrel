package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/view"
)

// Session holds per-login state that dies with the session (S-4).
//
// One session serves several requests at once, so everything that changes
// after Create is behind mu: the deadlines, the drafts, the history, the loss
// registry, the live-fanout count, the kept tasks, the transfer queue, the
// readings of collections, the refusals of servers and the last checks of
// connections.
// Cache has its own lock.
// No method calls into a task that locks while mu is held: a task takes mu
// through AcquireFanout and ReleaseFanout under its own lock.
type Session struct {
	id        string
	accountID string
	login     string

	mu         sync.Mutex
	ended      bool
	dataKey    [crypt.KeyLen]byte
	csrf       []byte
	created    time.Time
	lastSeen   time.Time
	absoluteAt time.Time
	drafts     Drafts
	history    []HistoryEntry
	losses     LossRegistry
	fanout     int
	tasks      []Task // newest first
	transfers  []Transfer
	unlockAt   time.Time
	readings   map[string]Reading               // by connection and collection path (S-22)
	ticks      map[string]time.Time             // tasks marked done, by collection and UID (ticks.go)
	refusals   map[string]bool                  // what a server refused in a collection (refusals.go)
	checks     map[string]*view.DiagnosticTrace // last check of each connection (C-6)
	agent      string                           // the browser, as the account page names it
	address    string                           // the client address at sign-in

	cache *Cache
	mgr   *Manager
}

// ID returns the session identifier (cookie value).
func (s *Session) ID() string { return s.id }

// AccountID returns the owning account.
func (s *Session) AccountID() string { return s.accountID }

// Login returns the account login.
func (s *Session) Login() string { return s.login }

// DataKey returns a copy of the data key when the session is live. Callers
// must not retain it longer than the session lives.
func (s *Session) DataKey() ([crypt.KeyLen]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return [crypt.KeyLen]byte{}, false
	}
	var k [crypt.KeyLen]byte
	copy(k[:], s.dataKey[:])
	return k, true
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
	if s.mgr != nil {
		s.mgr.Touch(s.id, now)
		return
	}
	s.touchAt(now)
}

func (s *Session) touchAt(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.lastSeen = now
}

func (s *Session) expiredAt(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return true
	}
	if !now.Before(s.absoluteAt) {
		return true
	}
	return !now.Before(s.lastSeen.Add(s.mgr.idleTimeout()))
}

// Expired reports whether idle or absolute lifetime has ended (S-4).
func (s *Session) Expired(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !now.Before(s.absoluteAt) {
		return true
	}
	return !now.Before(s.lastSeen.Add(s.mgr.idleTimeout()))
}

// UnlockUntil returns the end of the S-10.7 secret-release window, or nil
// when the sheet was not confirmed or the window has passed. The window is
// per session: it dies with the cookie, and another device of the same
// person confirms its own password.
func (s *Session) UnlockUntil(now time.Time) *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended || !now.Before(s.unlockAt) {
		return nil
	}
	t := s.unlockAt
	return &t
}

// SetUnlockUntil opens the secret-release window until t.
func (s *Session) SetUnlockUntil(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.unlockAt = t
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
	if s.ended {
		return
	}
	s.drafts.set(slot, data)
}

// NoteLoss records that a server dropped a property (S-7.1).
func (s *Session) NoteLoss(serverID, prop string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
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

// LossNotes is one session's loss registry, safe for concurrent use. Its
// method set is model.LossAggregator, matched structurally — session stays
// below model and does not import it.
type LossNotes struct{ s *Session }

func (n LossNotes) Note(serverID, prop string)     { n.s.NoteLoss(serverID, prop) }
func (n LossNotes) Has(serverID, prop string) bool { return n.s.HasLoss(serverID, prop) }
func (n LossNotes) Props(serverID string) []string { return n.s.LostProps(serverID) }

// Losses returns the aggregator to hand to model.CompareAfterPut after a PUT.
func (s *Session) Losses() LossNotes { return LossNotes{s} }

// AcquireFanout reserves a live fanout slot (S-15). At most MaxLiveFanouts
// per session.
func (s *Session) AcquireFanout() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended || s.fanout >= conf.MaxLiveFanouts {
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

// Task is a fanout the session owns (S-15.1). The session does not know what
// a task asks: it keeps it for the screens that read it, bounds how many it
// keeps, and cancels them when it ends. fanout.Task satisfies it. Running
// must not lock: the session asks it under its own lock.
type Task interface {
	ID() string
	Running() bool
	Cancel()
}

// KeepTask stores a started task, newest first. Past MaxKeptFanouts the
// oldest finished task goes, a running one only when no finished one is left;
// whatever goes is cancelled. An ended session cancels the task at once.
func (s *Session) KeepTask(t Task) {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		t.Cancel()
		return
	}
	s.tasks = append([]Task{t}, s.tasks...)
	var gone []Task
	for len(s.tasks) > conf.MaxKeptFanouts {
		victim := len(s.tasks) - 1
		for i := len(s.tasks) - 1; i > 0; i-- {
			if !s.tasks[i].Running() {
				victim = i
				break
			}
		}
		gone = append(gone, s.tasks[victim])
		s.tasks = append(s.tasks[:victim], s.tasks[victim+1:]...)
	}
	s.mu.Unlock()
	for _, g := range gone {
		g.Cancel()
	}
}

// Task returns a kept task of this session. A task ID of another session is
// not found here.
func (s *Session) Task(id string) (Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tasks {
		if t.ID() == id {
			return t, true
		}
	}
	return nil, false
}

// Tasks returns the kept tasks, newest first.
func (s *Session) Tasks() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Task(nil), s.tasks...)
}

// AddHistory writes a finished search at the top of the ten-line history
// (S-15.3). The query is kept as typed without outer spaces; an empty one is
// not written; a repeat replaces its row instead of adding a second.
func (s *Session) AddHistory(e HistoryEntry) {
	e.Query = strings.TrimSpace(e.Query)
	if e.Query == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	out := make([]HistoryEntry, 0, conf.SearchHistoryLen)
	out = append(out, e)
	for _, h := range s.history {
		if h.Query != e.Query && len(out) < conf.SearchHistoryLen {
			out = append(out, h)
		}
	}
	s.history = out
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
		absoluteAt: now.Add(mgr.absoluteTimeout()),
		cache:      newCache(mgr.cfg, mgr),
		losses:     LossRegistry{byServer: make(map[string]map[string]struct{})},
		mgr:        mgr,
	}
	return s, nil
}

// end wipes sensitive material and unregisters the session (S-2, S-4).
func (s *Session) end() {
	s.mu.Lock()
	s.ended = true
	crypt.Wipe(s.dataKey[:])
	crypt.Wipe(s.csrf)
	s.drafts.clear()
	s.history = nil
	s.losses.byServer = nil
	s.fanout = 0
	tasks := s.tasks
	s.tasks = nil
	transfers := s.transfers
	s.transfers = nil
	s.unlockAt = time.Time{}
	s.readings = nil
	s.ticks = nil
	s.refusals = nil
	s.mu.Unlock()

	for _, t := range tasks {
		t.Cancel()
	}
	for _, t := range transfers {
		t.Discard()
	}
	s.cache.markEnded()
}
