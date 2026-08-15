// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"fmt"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// Register creates an account from the public form (§5.2). The user cannot
// sign in until they follow the returned confirmation token. An unconfirmed
// account with the same login and email is reused: credentials are replaced
// and a fresh token is issued, so a missed message can be retried without
// occupying the login forever.
func (s *Store) Register(login, email, password, ip string) (*User, string, error) {
	login = NormalizeLogin(login)
	email = NormalizeEmail(email)
	if email == "" {
		return nil, "", fmt.Errorf("store: email is required")
	}
	if err := validateNewUser(login, email, password); err != nil {
		return nil, "", err
	}

	token, err := crypto.NewToken()
	if err != nil {
		return nil, "", err
	}

	var out *User
	err = s.update(func(state *State) error {
		if !state.Settings.SelfRegistration {
			return ErrRegistrationClosed
		}
		existing := findUserByLogin(state, login)
		if existing != nil {
			if !existing.Unconfirmed || existing.Email != email {
				return ErrLoginTaken
			}
			if err := s.setCredentials(existing, password, state.Settings.Escrow); err != nil {
				return err
			}
			stampRegistration(existing, email, token, s.now().Add(DefaultEmailChangeTTL))
			appendAudit(state, s.now(), AuditEntry{
				Action:      ActionUserCreate,
				ActorLogin:  login,
				TargetID:    existing.ID,
				TargetLogin: existing.Login,
				IP:          ip,
				Detail:      "self-registration resent",
			})
			out = existing.Clone()
			return nil
		}

		u, err := s.newUser(state, login, email, RoleUser, password, "")
		if err != nil {
			return err
		}
		stampRegistration(u, email, token, s.now().Add(DefaultEmailChangeTTL))
		state.Users = append(state.Users, u)
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionUserCreate,
			ActorLogin:  login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          ip,
			Detail:      "self-registration",
		})
		out = u.Clone()
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return out, token, nil
}

func stampRegistration(u *User, email, token string, expires time.Time) {
	u.Unconfirmed = true
	u.Email = email
	u.PendingEmail = email
	u.EmailChangeTokenHash = crypto.HashToken(token)
	u.EmailChangeExpires = expires
}
