package store

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"carrel.dev/carrel/server/internal/crypt"
)

// Store holds the encrypted instance state (S-3).
type Store struct {
	mu        sync.RWMutex
	path      string
	serverKey [crypt.KeyLen]byte
	state     State
	firstRun  bool
	writeFn   func(path string, data []byte) error
}

// Open loads state.enc or enters first-run mode when the file is absent.
func Open(dataDir string, serverKey [crypt.KeyLen]byte) (*Store, error) {
	return openAt(filepath.Join(dataDir, StateFile), serverKey, atomicWrite)
}

func openAt(path string, serverKey [crypt.KeyLen]byte, write func(string, []byte) error) (*Store, error) {
	st := &Store{
		path:      path,
		serverKey: serverKey,
		writeFn:   write,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			st.firstRun = true
			st.state = emptyState()
			return st, nil
		}
		return nil, err
	}
	_, state, err := decodeFile(data, serverKey)
	if err != nil {
		return nil, err
	}
	st.state = state
	return st, nil
}

// FirstRun reports whether the volume had no state file when opened.
func (s *Store) FirstRun() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.firstRun
}

// Snapshot returns a copy of the current state.
func (s *Store) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.clone()
}

// ConnectionCount returns how many DAV connections an account has without
// opening the sealed blob (S-3).
func (s *Store) ConnectionCount(accountID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u := s.state.userByID(accountID)
	if u == nil {
		return 0
	}
	return u.ConnCount
}

// Mutate applies fn to a copy of the state and persists atomically. On any
// error the in-memory state and audit log stay unchanged — a failed write does
// not leave a journal line behind (S-3).
func (s *Store) Mutate(fn func(*State) ([]AuditEntry, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.state.clone()
	entries, err := fn(&next)
	if err != nil {
		return err
	}
	if err := validateStateKDF(&next); err != nil {
		return err
	}
	for _, e := range entries {
		if err := validateAudit(e); err != nil {
			return err
		}
		next.Audit = append(next.Audit, e)
	}
	trimAudit(&next.Audit)

	payload, err := encodeFile(FormatVersion, s.serverKey, next)
	if err != nil {
		return err
	}
	if err := s.writeFn(s.path, payload); err != nil {
		return err
	}
	s.state = next
	s.firstRun = false
	return nil
}

// SealBackupKey seals a data key for one backup job's external trigger.
func (s *Store) SealBackupKey(jobID string, dataKey [crypt.KeyLen]byte) ([]byte, error) {
	return crypt.SealBackupKey(s.serverKey, jobID, dataKey)
}

// OpenBackupKey opens a backup job's key envelope.
func (s *Store) OpenBackupKey(jobID string, envelope []byte) ([crypt.KeyLen]byte, error) {
	return crypt.OpenBackupKey(s.serverKey, jobID, envelope)
}

// SealLink seals a published link's envelope under the server key (S-25).
func (s *Store) SealLink(hash, plaintext []byte) ([]byte, error) {
	return crypt.SealLink(s.serverKey, hash, plaintext)
}

// OpenLink opens a published link's envelope.
func (s *Store) OpenLink(hash, ciphertext []byte) ([]byte, error) {
	return crypt.OpenLink(s.serverKey, hash, ciphertext)
}
