// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"fmt"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// DefaultEmailChangeTTL is how long a confirmation link stays valid.
const DefaultEmailChangeTTL = 24 * time.Hour

// RequestEmailChange starts a confirmation flow for a new address. The token
// is returned once and only its digest is stored (§5.3).
func (s *Store) RequestEmailChange(actor Actor, userID, newEmail string, ttl time.Duration) (string, error) {
	newEmail = NormalizeEmail(newEmail)
	if err := ValidateEmail(newEmail); err != nil {
		return "", err
	}
	if newEmail == "" {
		return "", ErrNotFound // reuse: empty address is not confirmable
	}

	token, err := crypto.NewToken()
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		ttl = DefaultEmailChangeTTL
	}

	err = s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		if u.Email == newEmail {
			return fmt.Errorf("store: email is unchanged")
		}
		u.PendingEmail = newEmail
		u.EmailChangeTokenHash = crypto.HashToken(token)
		u.EmailChangeExpires = s.now().Add(ttl)
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionUserEmail,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
			Detail:      "confirmation requested",
		})
		return nil
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// ConfirmEmailChange applies a pending address after the bearer token checks
// out. Unknown, expired and already-cleared tokens all yield ErrInviteInvalid
// so the endpoint reveals nothing (§24.3).
func (s *Store) ConfirmEmailChange(token string) (*User, error) {
	if token == "" {
		return nil, ErrInviteInvalid
	}
	digest := crypto.HashToken(token)

	var out *User
	err := s.update(func(state *State) error {
		now := s.now()
		var u *User
		for _, candidate := range state.Users {
			if len(candidate.EmailChangeTokenHash) == 0 {
				continue
			}
			if crypto.Equal(candidate.EmailChangeTokenHash, digest) {
				u = candidate
			}
		}
		if u == nil || u.PendingEmail == "" || !now.Before(u.EmailChangeExpires) {
			return ErrInviteInvalid
		}
		u.Email = u.PendingEmail
		u.PendingEmail = ""
		u.EmailChangeTokenHash = nil
		u.EmailChangeExpires = time.Time{}

		appendAudit(state, now, AuditEntry{
			Action:      ActionEmailConfirm,
			ActorID:     u.ID,
			ActorLogin:  u.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			Detail:      u.Email,
		})
		out = u.Clone()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
