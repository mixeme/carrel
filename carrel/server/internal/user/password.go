package user

import (
	"context"
	"errors"

	"carrel.dev/carrel/server/internal/store"
)

var ErrWrongPassword = errors.New("wrong password")

// ChangePassword re-wraps the account data key under a new password, clears
// MustChangePassword, burns wipe tokens, and ends every other live session of
// the account (S-10). The sealed blob is untouched.
func (s *Service) ChangePassword(ctx context.Context, accountID, exceptSID, current, newPW, newPW2 string) error {
	if err := validatePassword(newPW); err != nil {
		return err
	}
	if newPW != newPW2 {
		return ErrPasswordMismatch
	}
	snap := s.st.Snapshot()
	u := snap.UserByID(accountID)
	if u == nil || u.Disabled || len(u.LoginHash) == 0 {
		s.verifyDummy(ctx, current)
		return ErrWrongPassword
	}
	if _, ok := store.DataKeyForLogin(ctx, u, current); !ok {
		s.verifyDummy(ctx, current)
		return ErrWrongPassword
	}
	now := s.now()
	err := s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		x := st.UserByID(accountID)
		if x == nil || x.Disabled {
			return nil, ErrNotFound
		}
		if err := store.ChangePassword(ctx, x, x.Login, current, newPW); err != nil {
			return nil, err
		}
		burnWipeTokens(st, x.ID)
		pruneTokens(st, now)
		return []store.AuditEntry{{
			When: now, Who: x.Login, What: "password.change", Whom: x.Login,
		}}, nil
	})
	if err != nil {
		return err
	}
	s.mgr.EndAccountExcept(accountID, exceptSID)
	return nil
}
