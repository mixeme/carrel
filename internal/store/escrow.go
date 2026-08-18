// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

var (
	// ErrEscrowNotConfigured is returned when an escrow operation is asked
	// for on an instance that has no recovery key pair.
	ErrEscrowNotConfigured = errors.New("store: escrow is not configured")
	// ErrEscrowConfigured is returned when the setup flow runs a second
	// time. Generating a new key pair would orphan every deposited copy.
	ErrEscrowConfigured = errors.New("store: escrow is already configured")
	// ErrEscrowNotDeposited is returned when there is no copy of the
	// account's DEK to recover from.
	ErrEscrowNotDeposited = errors.New("store: this account has no deposited key copy")
	// ErrEscrowDeposited is returned when an account already has a copy, so
	// opting in again would only replace one valid copy with another.
	ErrEscrowDeposited = errors.New("store: this account is already covered by escrow")
)

// EnableEscrow turns on key deposit and generates the recovery key pair. It
// applies only to users created afterwards: an existing DEK cannot be
// deposited without its owner's password, and the server does not have it.
// Everyone else joins from their own profile, through EscrowOptIn (§5.4).
//
// Losing the master password makes the scheme useless — the UI must say so
// before this is called.
func (s *Store) EnableEscrow(actor Actor, masterPassword string) error {
	esc, err := crypto.NewEscrowWithParams(masterPassword, s.opts.Master)
	if err != nil {
		return err
	}
	return s.update(func(state *State) error {
		if state.Settings.Escrow.Config != nil {
			return ErrEscrowConfigured
		}
		state.Settings.Escrow.Enabled = true
		state.Settings.Escrow.Config = esc
		appendAudit(state, s.now(), AuditEntry{
			Action:     ActionEscrowEnable,
			ActorID:    actor.ID,
			ActorLogin: actor.Login,
			IP:         actor.IP,
		})
		return nil
	})
}

// ResumeEscrow switches deposit back on for an instance that already has a key
// pair, keeping the copies taken before it was switched off. The master
// password is required not because the operation needs it, but because an
// administrator who cannot produce it is turning on a scheme that can no
// longer recover anything (§5.4).
func (s *Store) ResumeEscrow(actor Actor, masterPassword string) error {
	return s.update(func(state *State) error {
		esc := state.Settings.Escrow.Config
		if esc == nil {
			return ErrEscrowNotConfigured
		}
		if !esc.CheckMasterPassword(masterPassword) {
			return crypto.ErrWrongMasterPassword
		}
		if state.Settings.Escrow.Enabled {
			return nil
		}
		state.Settings.Escrow.Enabled = true
		appendAudit(state, s.now(), AuditEntry{
			Action:     ActionEscrowEnable,
			ActorID:    actor.ID,
			ActorLogin: actor.Login,
			IP:         actor.IP,
			Detail:     "existing key pair",
		})
		return nil
	})
}

// DisableEscrow stops depositing DEK copies for new users. The key pair and
// the copies already taken are left alone: dropping them silently would make
// the profile's "who can recover your data" answer wrong for anyone who opted
// in. Removing a copy is the user's own action (§5.4).
func (s *Store) DisableEscrow(actor Actor) error {
	return s.update(func(state *State) error {
		state.Settings.Escrow.Enabled = false
		appendAudit(state, s.now(), AuditEntry{
			Action:     ActionEscrowDisable,
			ActorID:    actor.ID,
			ActorLogin: actor.Login,
			IP:         actor.IP,
		})
		return nil
	})
}

// ChangeEscrowMasterPassword re-seals the recovery private key. The deposited
// copies are encrypted to the public key, which does not change, so nobody has
// to opt in again (§5.4).
func (s *Store) ChangeEscrowMasterPassword(actor Actor, current, next string) error {
	return s.update(func(state *State) error {
		esc := state.Settings.Escrow.Config
		if esc == nil {
			return ErrEscrowNotConfigured
		}
		if err := esc.ChangeMasterPassword(current, next); err != nil {
			return err
		}
		appendAudit(state, s.now(), AuditEntry{
			Action:     ActionSettings,
			ActorID:    actor.ID,
			ActorLogin: actor.Login,
			IP:         actor.IP,
			Detail:     "escrow master password",
		})
		return nil
	})
}

// EscrowOptIn deposits a copy of an existing user's DEK. It takes the user's
// own password because that is the only thing that can open their DEK — the
// server cannot deposit an account into escrow behind its owner's back, which
// is exactly why enabling the scheme is not retroactive (§5.4).
func (s *Store) EscrowOptIn(actor Actor, userID, password string) error {
	return s.update(func(state *State) error {
		if !state.Settings.Escrow.Active() {
			return ErrEscrowNotConfigured
		}
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		if len(u.EscrowDEK) > 0 {
			return ErrEscrowDeposited
		}
		if !u.Activated() || !u.Auth.Verify(password) {
			return ErrAuth
		}

		dek, err := unwrapDEK(u, password)
		if err != nil {
			return err
		}
		defer dek.Zero()

		sealed, err := state.Settings.Escrow.Config.SealDEK(dek)
		if err != nil {
			return err
		}
		u.EscrowDEK = sealed
		// The user has just been shown what they were agreeing to; repeating
		// the first-login notice afterwards would be noise.
		u.EscrowNoticeSeen = true

		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionEscrowOptIn,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
		})
		return nil
	})
}

// EscrowOptOut deletes the deposited copy, which is the only way to make the
// account unrecoverable again. It works whether or not deposit is still on for
// new users: the copy belongs to its owner (§5.4).
func (s *Store) EscrowOptOut(actor Actor, userID string) error {
	return s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		if len(u.EscrowDEK) == 0 {
			return ErrEscrowNotDeposited
		}
		u.EscrowDEK = nil

		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionEscrowOptOut,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
		})
		return nil
	})
}

// RecoverUser is the whole point of escrow (§5.4): the master password opens
// the recovery private key, that decrypts the deposited copy of the user's
// DEK, and the DEK is re-wrapped under a temporary password the administrator
// hands over. The key itself never changes, so the account's data survives —
// this is the alternative ResetPassword refuses in favour of.
//
// The returned user is a copy for the caller to notify with: every recovery
// goes into the audit log and into an email to its subject, whatever the
// administrator would prefer. The caller must also end the account's sessions
// and send that mail.
func (s *Store) RecoverUser(actor Actor, userID, masterPassword, tempPassword string) (*User, error) {
	if err := ValidatePassword(tempPassword); err != nil {
		return nil, err
	}

	var out *User
	err := s.update(func(state *State) error {
		esc := state.Settings.Escrow.Config
		if esc == nil {
			return ErrEscrowNotConfigured
		}
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		if len(u.EscrowDEK) == 0 {
			return ErrEscrowNotDeposited
		}

		dek, err := esc.RecoverDEK(masterPassword, u.EscrowDEK)
		if err != nil {
			return err
		}
		defer dek.Zero()

		if err := s.rewrapDEK(u, tempPassword, dek); err != nil {
			return err
		}
		// A password the administrator knows is not one the user may keep.
		u.MustChangePassword = true
		u.EscrowRecoveredAt = s.now()

		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionEscrowRecover,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
		})
		out = u.Clone()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EscrowCoverage counts the accounts holding a deposited copy, for the line in
// the panel that says how many people the master password reaches.
func (s *Store) EscrowCoverage() int {
	n := 0
	s.read(func(state *State) {
		for _, u := range state.Users {
			if len(u.EscrowDEK) > 0 {
				n++
			}
		}
	})
	return n
}
