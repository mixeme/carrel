// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"fmt"
	"sort"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// Invites returns every invite, newest first.
func (s *Store) Invites() []*Invite {
	var out []*Invite
	s.read(func(state *State) {
		out = make([]*Invite, 0, len(state.Invites))
		for _, i := range state.Invites {
			out = append(out, i.Clone())
		}
	})
	sort.Slice(out, func(a, b int) bool { return out[a].CreatedAt.After(out[b].CreatedAt) })
	return out
}

// Invite returns one invite by ID.
func (s *Store) Invite(id string) (*Invite, error) {
	var out *Invite
	s.read(func(state *State) {
		if i := findInvite(state, id); i != nil {
			out = i.Clone()
		}
	})
	if out == nil {
		return nil, ErrNotFound
	}
	return out, nil
}

// CreateInvite issues a one-time link for a new account. The token is returned
// once and only its digest is kept (§5.3).
//
// For InviteDeliveryLink the administrator copies the link; login and optional
// email are chosen when the bearer accepts. For InviteDeliveryEmail the
// administrator supplies the recipient address and the link is delivered by
// mail only — the caller must queue delivery separately.
func (s *Store) CreateInvite(actor Actor, role Role, delivery InviteDelivery, email string, ttl time.Duration) (*Invite, string, error) {
	if !role.Valid() {
		return nil, "", fmt.Errorf("store: unknown role %q", role)
	}
	if !delivery.Valid() {
		return nil, "", fmt.Errorf("store: unknown invite delivery %q", delivery)
	}
	email = NormalizeEmail(email)
	switch delivery {
	case InviteDeliveryLink:
		if email != "" {
			return nil, "", fmt.Errorf("store: link invitations must not carry an email address")
		}
	case InviteDeliveryEmail:
		if err := ValidateEmail(email); err != nil {
			return nil, "", err
		}
	}

	token, err := crypto.NewToken()
	if err != nil {
		return nil, "", err
	}
	id, err := newID()
	if err != nil {
		return nil, "", err
	}

	var out *Invite
	err = s.update(func(state *State) error {
		if ttl <= 0 {
			ttl = state.Settings.InviteTTL()
		}
		now := s.now()
		status := SendNotConfigured
		if delivery == InviteDeliveryEmail {
			if state.Settings.SMTP.Configured() {
				status = SendPending
			}
		}
		inv := &Invite{
			ID:         id,
			Role:       role,
			Delivery:   delivery,
			Email:      email,
			TokenHash:  crypto.HashToken(token),
			CreatedAt:  now,
			CreatedBy:  actor.ID,
			ExpiresAt:  now.Add(ttl),
			SendStatus: status,
		}
		state.Invites = append(state.Invites, inv)
		entry := AuditEntry{
			Action:     ActionInviteCreate,
			ActorID:    actor.ID,
			ActorLogin: actor.Login,
			IP:         actor.IP,
		}
		if delivery == InviteDeliveryEmail {
			entry.Detail = email
		}
		appendAudit(state, now, entry)
		out = inv.Clone()
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return out, token, nil
}

// LookupInvite resolves a token from an invite link. Unknown, revoked, expired
// and already-accepted tokens all yield ErrInviteInvalid, and every candidate
// digest is compared in constant time, so probing the endpoint tells an
// attacker nothing (§24.3).
func (s *Store) LookupInvite(token string) (*Invite, error) {
	if token == "" {
		return nil, ErrInviteInvalid
	}
	digest := crypto.HashToken(token)

	var (
		found *Invite
		now   time.Time
	)
	s.read(func(state *State) {
		now = s.now()
		for _, i := range state.Invites {
			// No early exit: every invite is compared, so the response time
			// does not depend on where a match sits in the list.
			if crypto.Equal(i.TokenHash, digest) {
				found = i.Clone()
			}
		}
	})
	if found == nil || !found.Usable(now) {
		return nil, ErrInviteInvalid
	}
	return found, nil
}

// AcceptInvite consumes a token and creates the account. Login and password
// are always chosen here. Email comes from the invitation for email delivery,
// or from the bearer for link delivery (§5.2).
func (s *Store) AcceptInvite(token, login, email, password, ip string) (*User, error) {
	login = NormalizeLogin(login)
	if err := ValidateLogin(login); err != nil {
		return nil, err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	if token == "" {
		return nil, ErrInviteInvalid
	}
	digest := crypto.HashToken(token)

	var out *User
	err := s.update(func(state *State) error {
		now := s.now()
		var inv *Invite
		for _, i := range state.Invites {
			if crypto.Equal(i.TokenHash, digest) {
				inv = i
			}
		}
		if inv == nil || !inv.Usable(now) {
			return ErrInviteInvalid
		}
		if err := s.checkLoginFree(state, login); err != nil {
			return err
		}

		accountEmail := NormalizeEmail(email)
		switch inv.Delivery {
		case InviteDeliveryEmail:
			accountEmail = inv.Email
		default:
			if err := ValidateEmail(accountEmail); err != nil {
				return err
			}
		}

		inv.AcceptedAt = now
		inv.Login = login
		inv.Email = accountEmail

		u, err := s.newUser(state, login, accountEmail, inv.Role, password, inv.CreatedBy)
		if err != nil {
			return err
		}
		state.Users = append(state.Users, u)

		appendAudit(state, now, AuditEntry{
			Action:      ActionInviteAccept,
			ActorID:     u.ID,
			ActorLogin:  u.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
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

// RevokeInvite makes a link stop working. The record stays for the audit
// trail; only its usability changes.
func (s *Store) RevokeInvite(actor Actor, id string) error {
	return s.update(func(state *State) error {
		inv := findInvite(state, id)
		if inv == nil {
			return ErrNotFound
		}
		if !inv.Usable(s.now()) {
			return ErrInviteInvalid
		}
		inv.RevokedAt = s.now()
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionInviteRevoke,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetLogin: inv.Login,
			IP:          actor.IP,
		})
		return nil
	})
}

// ExtendInvite pushes the expiry out from now. The token itself is unchanged,
// so a link already handed over keeps working.
func (s *Store) ExtendInvite(actor Actor, id string, ttl time.Duration) error {
	return s.update(func(state *State) error {
		inv := findInvite(state, id)
		if inv == nil {
			return ErrNotFound
		}
		if !inv.AcceptedAt.IsZero() || !inv.RevokedAt.IsZero() {
			return ErrInviteInvalid
		}
		if ttl <= 0 {
			ttl = state.Settings.InviteTTL()
		}
		inv.ExpiresAt = s.now().Add(ttl)
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionInviteExtend,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetLogin: inv.Login,
			IP:          actor.IP,
		})
		return nil
	})
}

// ResendInvite rotates the token and returns the new one for an email
// invitation. The old link stops working. Link invitations cannot be resent
// through this path — create a new link instead (§5.3).
func (s *Store) ResendInvite(actor Actor, id string) (string, error) {
	token, err := crypto.NewToken()
	if err != nil {
		return "", err
	}

	err = s.update(func(state *State) error {
		inv := findInvite(state, id)
		if inv == nil {
			return ErrNotFound
		}
		if !inv.Usable(s.now()) {
			return ErrInviteInvalid
		}
		if inv.Delivery != InviteDeliveryEmail {
			return ErrInviteNotByEmail
		}
		inv.TokenHash = crypto.HashToken(token)
		status := SendNotConfigured
		if state.Settings.SMTP.Configured() {
			status = SendPending
		}
		inv.SendStatus = status
		inv.SendError = ""
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionInviteResend,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetLogin: inv.Login,
			Detail:      inv.Email,
			IP:          actor.IP,
		})
		return nil
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// RecordInviteSend stores the outcome of a delivery attempt. A failure is
// informational: the invite stays valid and the link stays usable (§5.3).
func (s *Store) RecordInviteSend(id string, status SendStatus, sendErr string) error {
	return s.update(func(state *State) error {
		inv := findInvite(state, id)
		if inv == nil {
			return ErrNotFound
		}
		inv.SendStatus = status
		inv.SendError = sendErr
		if status == SendOK {
			inv.LastSentAt = s.now()
			inv.SendError = ""
		}
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionInviteSend,
			TargetLogin: inv.Login,
			Detail:      string(status),
		})
		return nil
	})
}

func findInvite(state *State, id string) *Invite {
	for _, i := range state.Invites {
		if i.ID == id {
			return i
		}
	}
	return nil
}
