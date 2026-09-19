package user

import (
	"context"

	"carrel.dev/carrel/server/internal/store"
)

func (s *Service) SetupFirstAdmin(ctx context.Context, login, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	login = normalizeLogin(login)
	if login == "" {
		return ErrNotFound
	}
	id, err := NewUserID()
	if err != nil {
		return err
	}
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if len(st.Users) > 0 {
			return nil, ErrNotFound
		}
		u := store.User{
			ID:        id,
			Login:     login,
			Role:      store.RoleAdmin,
			CreatedAt: now,
		}
		if err := store.ApplyPassword(ctx, &u, login, password, st, now); err != nil {
			return nil, err
		}
		st.Users = []store.User{u}
		return []store.AuditEntry{{
			When: now, Who: "system", What: "setup.first_admin", Whom: login,
		}}, nil
	})
}

// Authenticate checks a login and password and returns the account ID and
// data key. Unknown login, wrong password, disabled account, unconfirmed
// address, and an account without a password yet all answer the same way and
// cost the same: every miss pays one Argon2 derivation (S-29.1).
func (s *Service) Authenticate(ctx context.Context, login, password string) (string, [32]byte, bool, bool) {
	var zero [32]byte
	login = normalizeLogin(login)
	snap := s.st.Snapshot()
	u := snap.UserByLogin(login)
	if u == nil || u.Disabled || (u.Email != "" && !u.EmailConfirmed) || len(u.LoginHash) == 0 {
		s.verifyDummy(ctx, password)
		return "", zero, false, false
	}
	key, ok := store.DataKeyForLogin(ctx, u, password)
	now := s.now()
	if !ok {
		// A wrong password on an account that exists is journaled as the
		// account's; an unknown login is not — the journal keeps no typed
		// text — and a disabled account answers above without a line
		// (audit №3, Ф3-7). No address: the journal is kept for good.
		_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
			return []store.AuditEntry{{When: now, Who: u.Login, What: "login.fail", Whom: u.Login}}, nil
		})
		return "", zero, false, false
	}
	_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		x := st.UserByID(u.ID)
		if x != nil {
			x.LastLogin = &now
		}
		return []store.AuditEntry{{When: now, Who: u.Login, What: "login.ok", Whom: u.Login}}, nil
	})
	return u.ID, key, u.MustChangePassword, true
}
