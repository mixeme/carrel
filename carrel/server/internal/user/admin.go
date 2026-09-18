package user

import (
	"context"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

func inviteByID(st *store.State, id string) *store.Invite {
	for i := range st.Invites {
		if st.Invites[i].ID == id {
			return &st.Invites[i]
		}
	}
	return nil
}

func wouldRemoveLastAdmin(st *store.State, u *store.User, newRole store.Role) bool {
	if u == nil || u.Role != store.RoleAdmin || u.Disabled {
		return false
	}
	if newRole == store.RoleAdmin {
		return false
	}
	return store.AdminCount(st) <= 1
}

// RevokeInvite withdraws a live invitation (S-10.3).
func (s *Service) RevokeInvite(actorID, inviteID string) error {
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		inv := inviteByID(st, inviteID)
		if inv == nil || !inviteLive(inv, now) {
			return nil, ErrNotFound
		}
		inv.Revoked = true
		return []store.AuditEntry{{
			When: now, Who: actorLogin(st, actorID), What: "user.invite.revoke", Whom: inv.Login,
		}}, nil
	})
}

// ExtendInvite pushes expiry to now + InviteTTL without changing the secret
// (architecture.md).
func (s *Service) ExtendInvite(actorID, inviteID string) error {
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		inv := inviteByID(st, inviteID)
		if inv == nil || !inviteLive(inv, now) {
			return nil, ErrNotFound
		}
		inv.ExpiresAt = now.Add(inviteTTL(st))
		return []store.AuditEntry{{
			When: now, Who: actorLogin(st, actorID), What: "user.invite.extend", Whom: inv.Login,
		}}, nil
	})
}

// DeleteUser removes an account after the administrator types its login
// (S-10.6). Data and sessions are destroyed; the last administrator cannot
// be removed.
func (s *Service) DeleteUser(actorID, targetID, loginConfirm string) error {
	loginConfirm = normalizeLogin(loginConfirm)
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(targetID)
		if u == nil {
			return nil, ErrNotFound
		}
		if normalizeLogin(u.Login) != loginConfirm {
			return nil, ErrLoginConfirm
		}
		if u.Role == store.RoleAdmin && store.AdminCount(st) <= 1 {
			return nil, ErrLastAdmin
		}
		login := u.Login
		s.mgr.EndAccount(targetID)
		removeUser(st, targetID, login)
		return []store.AuditEntry{{
			When: s.now(), Who: actorLogin(st, actorID), What: "user.delete", Whom: login,
		}}, nil
	})
}

// SetUserRole changes an account role; demoting the last administrator is
// refused (S-10.6).
func (s *Service) SetUserRole(actorID, targetID string, role store.Role) error {
	if role != store.RoleAdmin && role != store.RoleUser {
		return ErrNotFound
	}
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(targetID)
		if u == nil {
			return nil, ErrNotFound
		}
		if wouldRemoveLastAdmin(st, u, role) {
			return nil, ErrLastAdmin
		}
		u.Role = role
		return []store.AuditEntry{{
			When: s.now(), Who: actorLogin(st, actorID), What: "user.role.set", Whom: u.Login,
		}}, nil
	})
}

// EndUserSessions terminates every live session of one account (S-10.6).
func (s *Service) EndUserSessions(actorID, targetID string) error {
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(targetID)
		if u == nil {
			return nil, ErrNotFound
		}
		s.mgr.EndAccount(targetID)
		return []store.AuditEntry{{
			When: s.now(), Who: actorLogin(st, actorID), What: "user.sessions.end", Whom: u.Login,
		}}, nil
	})
}

// AdminResetPassword issues the same wipe token as /forgot (S-10.4, S-10.6).
// Confirmed email: token goes to the mail layer. No email: the raw token is
// returned for the administrator to pass on. Deposited escrow: no wipe token.
func (s *Service) AdminResetPassword(_ context.Context, actorID, targetID string) (token string, mailed, escrowAsk, handLink bool, err error) {
	snap := s.st.Snapshot()
	u := snap.UserByID(targetID)
	if u == nil || u.Disabled {
		return "", false, false, false, ErrNotFound
	}
	if snap.Settings.EscrowEnabled && store.UserDeposited(u) {
		return "", false, true, false, nil
	}
	hasMail := u.Email != "" && u.EmailConfirmed
	raw, hash, err := newSecret()
	if err != nil {
		return "", false, false, false, err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		pruneTokens(st, now)
		st.Tokens = append(st.Tokens, store.SecretToken{
			Kind: tokenForgotWipe, Hash: hash, UserID: u.ID, Step: string(view.WipeConfirm),
			CreatedAt: now, ExpiresAt: now.Add(conf.ForgotLinkTTL),
		})
		return []store.AuditEntry{{
			When: now, Who: actorLogin(st, actorID), What: "user.reset.request", Whom: u.Login,
		}}, nil
	})
	if err != nil {
		return "", false, false, false, err
	}
	if hasMail {
		return raw, false, false, false, nil
	}
	return raw, false, false, true, nil
}

// SetSelfRegEnabled toggles open registration on the instance.
func (s *Service) SetSelfRegEnabled(actorID string, enabled bool) error {
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Settings.SelfRegEnabled = enabled
		detail := "off"
		if enabled {
			detail = "on"
		}
		return []store.AuditEntry{{
			When: s.now(), Who: actorLogin(st, actorID), What: "settings.selfreg", Detail: detail,
		}}, nil
	})
}

func removeUser(st *store.State, userID, login string) {
	kept := st.Users[:0]
	for _, u := range st.Users {
		if u.ID != userID {
			kept = append(kept, u)
		}
	}
	st.Users = kept
	if login != "" {
		inv := st.Invites[:0]
		for _, x := range st.Invites {
			if normalizeLogin(x.Login) != normalizeLogin(login) {
				inv = append(inv, x)
			}
		}
		st.Invites = inv
	}
	tok := st.Tokens[:0]
	for _, t := range st.Tokens {
		if t.UserID != userID {
			tok = append(tok, t)
		}
	}
	st.Tokens = tok
}
