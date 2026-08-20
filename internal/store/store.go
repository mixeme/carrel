// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// StateFile is the single state file on the data volume. Everything in it is
// sealed with the server key (§4).
const StateFile = "state.enc"

var (
	// ErrNotFound is returned for any record that does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrLoginTaken is returned when a login is already used by a user.
	ErrLoginTaken = errors.New("store: login is already in use")
	// ErrLastAdmin guards the invariant that at least one administrator can
	// always log in (§5.5).
	ErrLastAdmin = errors.New("store: the last administrator cannot be removed or demoted")
	// ErrNotBootstrap is returned when the first-administrator flow runs on a
	// volume that already has one.
	ErrNotBootstrap = errors.New("store: the service is already set up")
	// ErrAuth is the single failure returned for an unknown login, a wrong
	// password and an account that has never set one. Callers must not tell
	// them apart in the response either.
	ErrAuth = errors.New("store: authentication failed")
	// ErrUserDisabled is returned after the password checks out but the
	// account is switched off.
	ErrUserDisabled = errors.New("store: account is disabled")
	// ErrUserUnconfirmed is returned after the password checks out but the
	// self-registration email has not been confirmed yet (§5.2).
	ErrUserUnconfirmed = errors.New("store: account is awaiting email confirmation")
	// ErrRegistrationClosed is returned when a visitor tries to sign up
	// while the public form is off.
	ErrRegistrationClosed = errors.New("store: self-registration is not enabled")
	// ErrInviteInvalid covers unknown, revoked, expired and already-accepted
	// invites alike, so probing a token learns nothing (§24.3).
	ErrInviteInvalid = errors.New("store: invite is not usable")
	// ErrInviteNotByEmail is returned when an operation applies only to
	// invitations that were emailed.
	ErrInviteNotByEmail = errors.New("store: invite was not sent by email")
	// ErrEscrowActive is returned when a destructive password reset is asked
	// for on an account that can be recovered instead (§5.5).
	ErrEscrowActive = errors.New("store: escrow is active for this user; recover instead of resetting")
	// ErrUnsupportedVersion means the volume was written by a newer build.
	ErrUnsupportedVersion = errors.New("store: state file was written by a newer version")
)

// Options tune a Store. The zero value is what production uses; tests and
// future cost migrations override the Argon2id profiles.
type Options struct {
	// Auth and KEK are the Argon2id profiles for new records. Existing
	// records keep the parameters they were written with.
	Auth crypto.Params
	KEK  crypto.Params
	// Master is the strengthened profile for the escrow master password. It
	// is written into the escrow configuration when the scheme is set up
	// and read back from there afterwards (§5.4).
	Master crypto.Params
	// Now overrides the clock.
	Now func() time.Time
}

func (o Options) withDefaults() Options {
	if o.Auth == (crypto.Params{}) {
		o.Auth = crypto.AuthParams()
	}
	if o.KEK == (crypto.Params{}) {
		o.KEK = crypto.KEKParams()
	}
	if o.Master == (crypto.Params{}) {
		o.Master = crypto.MasterParams()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o
}

// Store is the encrypted state on the data volume. It is safe for concurrent
// use: readers get copies, and every mutation is written to disk before it
// becomes visible.
type Store struct {
	path string
	key  crypto.Key
	opts Options

	mu    sync.RWMutex
	state *State
}

// Open loads the state from dataDir, creating the server key on first run. A
// volume with no state file yields an empty store in bootstrap mode; nothing
// is written until the first change.
func Open(dataDir string) (*Store, error) { return OpenWith(dataDir, Options{}) }

// OpenWith is Open with explicit options.
func OpenWith(dataDir string, opts Options) (*Store, error) {
	opts = opts.withDefaults()

	key, err := crypto.LoadOrCreateServerKey(dataDir)
	if err != nil {
		return nil, err
	}

	s := &Store{
		path: filepath.Join(dataDir, StateFile),
		key:  key,
		opts: opts,
	}

	state, err := s.load()
	if err != nil {
		key.Zero()
		return nil, err
	}
	s.state = state
	return s, nil
}

// Close wipes the server key. The state on disk stays where it is.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key.Zero()
	return nil
}

func (s *Store) now() time.Time { return s.opts.Now().UTC() }

func (s *Store) load() (*State, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return newState(s.now()), nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: read %s: %w", s.path, err)
	}

	plaintext, err := crypto.OpenState(s.key, raw)
	if err != nil {
		return nil, fmt.Errorf("store: decrypt %s: %w", s.path, err)
	}
	defer crypto.Zero(plaintext)

	var state State
	if err := json.Unmarshal(plaintext, &state); err != nil {
		return nil, fmt.Errorf("store: parse %s: %w", s.path, err)
	}
	if state.Version > StateVersion {
		return nil, fmt.Errorf("%w: file is version %d, this build handles %d", ErrUnsupportedVersion, state.Version, StateVersion)
	}
	if state.Version < StateVersion {
		migrate(&state)
	}
	return &state, nil
}

// migrate brings an older state up to StateVersion. There is nothing to do
// yet; the hook exists so the first format change has an obvious home.
func migrate(state *State) {
	state.Version = StateVersion
}

// update applies fn to a copy of the state and swaps it in only after the new
// state has reached the disk. A failed write leaves the store exactly as it
// was, in memory and on the volume.
func (s *Store) update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next, err := cloneState(s.state)
	if err != nil {
		return err
	}
	if err := fn(next); err != nil {
		return err
	}
	if err := s.persist(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

// read runs fn under the read lock. fn must copy anything it wants to keep.
func (s *Store) read(fn func(*State)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.state)
}

func (s *Store) persist(state *State) error {
	plaintext, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("store: encode state: %w", err)
	}
	defer crypto.Zero(plaintext)

	ciphertext, err := crypto.SealState(s.key, plaintext)
	if err != nil {
		return fmt.Errorf("store: encrypt state: %w", err)
	}
	return writeAtomic(s.path, ciphertext)
}

// writeAtomic replaces path in one step: a reader either sees the whole old
// file or the whole new one, and a crash mid-write cannot leave a truncated
// state on the volume.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("store: create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	cleanup := func(cause error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return cause
	}

	if err := tmp.Chmod(0o600); err != nil && !errors.Is(err, os.ErrInvalid) {
		// Windows reports Chmod on some filesystems as unsupported; the file
		// was created 0600 by CreateTemp in any case.
		if !isUnsupported(err) {
			return cleanup(fmt.Errorf("store: chmod %s: %w", tmpName, err))
		}
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(fmt.Errorf("store: write %s: %w", tmpName, err))
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(fmt.Errorf("store: sync %s: %w", tmpName, err))
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("store: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("store: replace %s: %w", path, err)
	}
	syncDir(dir)
	return nil
}

// syncDir flushes the rename itself. It is best effort: directories cannot be
// opened for sync on Windows, and a missed flush costs the last change on a
// power loss, not the store.
func syncDir(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Sync()
}

func isUnsupported(err error) bool {
	return errors.Is(err, os.ErrInvalid) || errors.Is(err, errors.ErrUnsupported)
}

// cloneState deep-copies through JSON. The state is small and written rarely,
// and a round trip cannot miss a field the way hand-written copying can.
func cloneState(state *State) (*State, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("store: clone state: %w", err)
	}
	var out State
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("store: clone state: %w", err)
	}
	return &out, nil
}

// NeedsBootstrap reports whether the volume has no administrator yet, which is
// what sends the first visitor to the setup screen (§5.1).
func (s *Store) NeedsBootstrap() bool {
	empty := true
	s.read(func(state *State) {
		for _, u := range state.Users {
			if u.Role == RoleAdmin {
				empty = false
				return
			}
		}
	})
	return empty
}

func newID() (string, error) {
	b, err := crypto.Random(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
