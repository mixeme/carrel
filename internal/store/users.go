// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"fmt"
	"sort"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// Users returns every account, ordered by login.
func (s *Store) Users() []*User {
	var out []*User
	s.read(func(state *State) {
		out = make([]*User, 0, len(state.Users))
		for _, u := range state.Users {
			out = append(out, u.Clone())
		}
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Login < out[j].Login })
	return out
}

// User returns one account by ID.
func (s *Store) User(id string) (*User, error) {
	var out *User
	s.read(func(state *State) {
		if u := findUser(state, id); u != nil {
			out = u.Clone()
		}
	})
	if out == nil {
		return nil, ErrNotFound
	}
	return out, nil
}

// UserByLogin returns one account by login. The login is normalized first.
func (s *Store) UserByLogin(login string) (*User, error) {
	login = NormalizeLogin(login)
	var out *User
	s.read(func(state *State) {
		if u := findUserByLogin(state, login); u != nil {
			out = u.Clone()
		}
	})
	if out == nil {
		return nil, ErrNotFound
	}
	return out, nil
}

// CreateFirstAdmin sets up the first administrator on an empty volume. It is
// refused once any administrator exists, so the setup screen cannot be used to
// mint a second one (§5.1).
func (s *Store) CreateFirstAdmin(login, email, password string, ip string) (*User, error) {
	login = NormalizeLogin(login)
	email = NormalizeEmail(email)
	if err := validateNewUser(login, email, password); err != nil {
		return nil, err
	}

	var out *User
	err := s.update(func(state *State) error {
		for _, u := range state.Users {
			if u.Role == RoleAdmin {
				return ErrNotBootstrap
			}
		}
		if err := s.checkLoginFree(state, login); err != nil {
			return err
		}
		u, err := s.newUser(state, login, email, RoleAdmin, password, "")
		if err != nil {
			return err
		}
		// The first administrator picks their own password, so there is
		// nothing to force a change of.
		u.MustChangePassword = false
		state.Users = append(state.Users, u)
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionBootstrap,
			ActorLogin:  login,
			ActorID:     u.ID,
			TargetID:    u.ID,
			TargetLogin: login,
			IP:          ip,
		})
		out = u.Clone()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateUserWithPassword adds an account with a temporary password chosen by
// the administrator. The user must replace it at first login, and until they
// do the administrator can sign in as them — the UI has to say so (§5.2).
func (s *Store) CreateUserWithPassword(actor Actor, login, email string, role Role, tempPassword string) (*User, error) {
	login = NormalizeLogin(login)
	email = NormalizeEmail(email)
	if err := validateNewUser(login, email, tempPassword); err != nil {
		return nil, err
	}
	if !role.Valid() {
		return nil, fmt.Errorf("store: unknown role %q", role)
	}

	var out *User
	err := s.update(func(state *State) error {
		if err := s.checkLoginFree(state, login); err != nil {
			return err
		}
		u, err := s.newUser(state, login, email, role, tempPassword, actor.ID)
		if err != nil {
			return err
		}
		u.MustChangePassword = true
		state.Users = append(state.Users, u)
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionUserCreate,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
			Detail:      "temporary password",
		})
		out = u.Clone()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// newUser builds a fully credentialed account. It must be called inside
// update, because it reads the escrow configuration from the state it is
// mutating.
func (s *Store) newUser(state *State, login, email string, role Role, password, createdBy string) (*User, error) {
	id, err := newID()
	if err != nil {
		return nil, err
	}
	u := &User{
		ID:        id,
		Login:     login,
		Email:     email,
		Role:      role,
		CreatedAt: s.now(),
		CreatedBy: createdBy,
	}
	if err := s.setCredentials(u, password, state.Settings.Escrow); err != nil {
		return nil, err
	}
	return u, nil
}

// setCredentials derives everything that hangs off a password: the login
// verifier, a fresh KEK salt, a fresh DEK, and — when escrow is on — a copy of
// that DEK sealed to the recovery public key. The two derivations use separate
// salts, which is what keeps the stored verifier from revealing the KEK (§4).
func (s *Store) setCredentials(u *User, password string, esc EscrowSettings) error {
	auth, err := crypto.HashPassword(password, s.opts.Auth)
	if err != nil {
		return err
	}
	kekSalt, err := crypto.NewSalt()
	if err != nil {
		return err
	}
	kek, err := crypto.DeriveKey(password, kekSalt, s.opts.KEK)
	if err != nil {
		return err
	}
	defer kek.Zero()

	dek, err := crypto.NewDEK()
	if err != nil {
		return err
	}
	defer dek.Zero()

	wrapped, err := crypto.WrapDEK(kek, dek)
	if err != nil {
		return err
	}

	var escrowed []byte
	if esc.Active() {
		if escrowed, err = esc.Config.SealDEK(dek); err != nil {
			return err
		}
	}

	u.Auth = auth
	u.KEKSalt = kekSalt
	u.KEKParams = s.opts.KEK
	u.WrappedDEK = wrapped
	u.EscrowDEK = escrowed
	return nil
}

// Authenticate verifies a login password and returns the account together with
// its unwrapped DEK. The caller owns the key: put it in the session keyring
// and wipe it on logout (§4).
//
// An unknown login, a wrong password and an account that never set one all
// return ErrAuth. The account is still opened for a disabled user so that the
// answer to "is this login disabled?" costs a correct password.
func (s *Store) Authenticate(login, password string) (*User, crypto.Key, error) {
	login = NormalizeLogin(login)

	var (
		stored *User
		params crypto.Params
		salt   []byte
		wrap   []byte
	)
	s.read(func(state *State) {
		if u := findUserByLogin(state, login); u != nil {
			stored = u.Clone()
			params = u.KEKParams
			salt = cloneBytes(u.KEKSalt)
			wrap = cloneBytes(u.WrappedDEK)
		}
	})
	if stored == nil || !stored.Activated() {
		// Spend a comparable amount of work on an unknown login so that the
		// response time does not disclose which logins exist.
		decoyVerify(s.opts.Auth, password)
		return nil, nil, ErrAuth
	}
	if !stored.Auth.Verify(password) {
		return nil, nil, ErrAuth
	}
	if stored.Disabled {
		return nil, nil, ErrUserDisabled
	}

	kek, err := crypto.DeriveKey(password, salt, params)
	if err != nil {
		return nil, nil, err
	}
	defer kek.Zero()

	dek, err := crypto.UnwrapDEK(kek, wrap)
	if err != nil {
		return nil, nil, ErrAuth
	}
	return stored, dek, nil
}

// decoyVerify burns roughly one password derivation. It cannot make the timing
// identical, only unremarkable.
func decoyVerify(p crypto.Params, password string) {
	salt := make([]byte, crypto.SaltLen)
	if key, err := crypto.DeriveKey(password, salt, p); err == nil {
		key.Zero()
	}
}

// RecordLogin stamps the last successful login and writes the audit entry.
func (s *Store) RecordLogin(actor Actor) error {
	return s.update(func(state *State) error {
		u := findUser(state, actor.ID)
		if u == nil {
			return ErrNotFound
		}
		u.LastLoginAt = s.now()
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionLogin,
			ActorID:     u.ID,
			ActorLogin:  u.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
		})
		return nil
	})
}

// ChangePassword re-wraps the DEK under a key derived from the new password.
// The DEK itself does not change, so everything encrypted under it stays
// readable — this is the safe counterpart to ResetPassword (§5.5).
func (s *Store) ChangePassword(userID, current, next string) error {
	if err := ValidatePassword(next); err != nil {
		return err
	}
	return s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		if !u.Activated() || !u.Auth.Verify(current) {
			return ErrAuth
		}

		oldKEK, err := crypto.DeriveKey(current, u.KEKSalt, u.KEKParams)
		if err != nil {
			return err
		}
		defer oldKEK.Zero()

		dek, err := crypto.UnwrapDEK(oldKEK, u.WrappedDEK)
		if err != nil {
			return ErrAuth
		}
		defer dek.Zero()

		auth, err := crypto.HashPassword(next, s.opts.Auth)
		if err != nil {
			return err
		}
		salt, err := crypto.NewSalt()
		if err != nil {
			return err
		}
		newKEK, err := crypto.DeriveKey(next, salt, s.opts.KEK)
		if err != nil {
			return err
		}
		defer newKEK.Zero()

		wrapped, err := crypto.WrapDEK(newKEK, dek)
		if err != nil {
			return err
		}

		u.Auth = auth
		u.KEKSalt = salt
		u.KEKParams = s.opts.KEK
		u.WrappedDEK = wrapped
		u.MustChangePassword = false

		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionPasswordChange,
			ActorID:     u.ID,
			ActorLogin:  u.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
		})
		return nil
	})
}

// ResetPassword is the destructive administrator action from §5.5: the old DEK
// is gone, so a new one is generated and everything sealed under the old one is
// dropped. When the account is covered by escrow it is refused, because a
// recovery would keep the data.
func (s *Store) ResetPassword(actor Actor, userID, tempPassword string) error {
	if err := ValidatePassword(tempPassword); err != nil {
		return err
	}
	return s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		if len(u.EscrowDEK) > 0 && state.Settings.Escrow.Config != nil {
			return ErrEscrowActive
		}
		if err := s.setCredentials(u, tempPassword, state.Settings.Escrow); err != nil {
			return err
		}
		// The blob was sealed under the DEK that just went away.
		u.Secrets = nil
		u.MustChangePassword = true

		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionPasswordReset,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
			Detail:      "connections destroyed",
		})
		return nil
	})
}

// SetDisabled switches an account on or off. Disabling keeps the data and only
// bars the login; the caller must also kill the user's sessions (§5.5).
func (s *Store) SetDisabled(actor Actor, userID string, disabled bool) error {
	return s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		if u.Disabled == disabled {
			return nil
		}
		if disabled && isLastActiveAdmin(state, u) {
			return ErrLastAdmin
		}
		u.Disabled = disabled
		action := ActionUserEnable
		if disabled {
			action = ActionUserDisable
		}
		appendAudit(state, s.now(), AuditEntry{
			Action:      action,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
		})
		return nil
	})
}

// SetRole promotes or demotes an account. The last administrator cannot be
// demoted (§5.5).
func (s *Store) SetRole(actor Actor, userID string, role Role) error {
	if !role.Valid() {
		return fmt.Errorf("store: unknown role %q", role)
	}
	return s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		if u.Role == role {
			return nil
		}
		if role != RoleAdmin && isLastActiveAdmin(state, u) {
			return ErrLastAdmin
		}
		u.Role = role
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionUserRole,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
			Detail:      string(role),
		})
		return nil
	})
}

// SetEmail replaces the address used for invites and service notices. The
// confirmation flow that precedes it lives in the mail package (§5.3).
func (s *Store) SetEmail(actor Actor, userID, email string) error {
	email = NormalizeEmail(email)
	if err := ValidateEmail(email); err != nil {
		return err
	}
	return s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		u.Email = email
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionUserEmail,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
		})
		return nil
	})
}

// DeleteUser removes an account and its encrypted blob for good. The caller is
// responsible for having confirmed the login with the administrator (§5.5).
func (s *Store) DeleteUser(actor Actor, userID string) error {
	return s.update(func(state *State) error {
		idx := -1
		for i, u := range state.Users {
			if u.ID == userID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return ErrNotFound
		}
		u := state.Users[idx]
		if isLastActiveAdmin(state, u) {
			return ErrLastAdmin
		}
		state.Users = append(state.Users[:idx], state.Users[idx+1:]...)
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionUserDelete,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
		})
		return nil
	})
}

// MarkEscrowNoticeSeen records that the user has been shown the escrow notice,
// so it is not repeated at every login (§5.4).
func (s *Store) MarkEscrowNoticeSeen(userID string) error {
	return s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		u.EscrowNoticeSeen = true
		return nil
	})
}

func validateNewUser(login, email, password string) error {
	if err := ValidateLogin(login); err != nil {
		return err
	}
	if err := ValidateEmail(email); err != nil {
		return err
	}
	return ValidatePassword(password)
}

// checkLoginFree rejects a login already held by an account or by an invite
// that could still be accepted.
func (s *Store) checkLoginFree(state *State, login string) error {
	if findUserByLogin(state, login) != nil {
		return ErrLoginTaken
	}
	now := s.now()
	for _, i := range state.Invites {
		if i.Login == login && i.Usable(now) {
			return ErrLoginTaken
		}
	}
	return nil
}

func findUser(state *State, id string) *User {
	for _, u := range state.Users {
		if u.ID == id {
			return u
		}
	}
	return nil
}

func findUserByLogin(state *State, login string) *User {
	for _, u := range state.Users {
		if u.Login == login {
			return u
		}
	}
	return nil
}

// isLastActiveAdmin reports whether u is the only administrator who can still
// log in. Deleting, demoting or disabling them would leave the instance with
// no way back in.
func isLastActiveAdmin(state *State, u *User) bool {
	if u.Role != RoleAdmin || u.Disabled {
		return false
	}
	for _, other := range state.Users {
		if other.ID == u.ID || other.Role != RoleAdmin || other.Disabled {
			continue
		}
		return false
	}
	return true
}
