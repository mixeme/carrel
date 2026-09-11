package user

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/store"
)

var (
	ErrMasterPasswordShort    = errors.New("master password must be at least 16 characters")
	ErrMasterPasswordMismatch = errors.New("master passwords do not match")
	ErrEscrowNotConfigured    = errors.New("key escrow is not configured")
	ErrEscrowConfigured       = errors.New("key escrow is already configured")
	ErrNotDeposited           = errors.New("no deposited key for this account")
	ErrEscrowDisabled         = errors.New("key escrow is disabled on this instance")
)

func validateMasterPassword(pw string) error {
	if len(pw) < conf.MinMasterPasswordLen {
		return ErrMasterPasswordShort
	}
	return nil
}

func escrowCoverNew(st *store.State) bool {
	return st.Settings.EscrowEnabled && st.Settings.EscrowCoverNewUsers
}

func applyEscrowCoverage(st *store.State, u *store.User) {
	if escrowCoverNew(st) {
		u.EscrowOptIn = true
		u.EscrowNoticePending = true
	}
}

// EnableEscrow turns on key escrow with a new master password (S-10.5).
func (s *Service) EnableEscrow(ctx context.Context, actorID, master, master2 string) error {
	if err := validateMasterPassword(master); err != nil {
		return err
	}
	if master != master2 {
		return ErrMasterPasswordMismatch
	}
	pub, priv, err := crypt.GenerateEscrowKeyPair()
	if err != nil {
		return err
	}
	wrapped, salt, kdf, err := crypt.WrapEscrowPrivateKey(ctx, master, priv)
	crypt.Wipe(priv[:])
	if err != nil {
		return err
	}
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if store.EscrowActive(st) {
			return nil, ErrEscrowConfigured
		}
		st.Settings.EscrowEnabled = true
		st.Settings.EscrowCoverNewUsers = true
		st.Settings.EscrowPublicKey = pub[:]
		st.Settings.EscrowPrivateWrap = wrapped
		st.Settings.EscrowMasterSalt = salt
		store.SetMasterKDF(&st.Settings, kdf)
		return []store.AuditEntry{{
			When: now, Who: actorLogin(st, actorID), What: "escrow.enable", Detail: "new accounts covered",
		}}, nil
	})
}

// SetEscrowCoverNew toggles whether new accounts are covered (S-10.5).
func (s *Service) SetEscrowCoverNew(actorID string, cover bool) error {
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if !store.EscrowActive(st) {
			return nil, ErrEscrowNotConfigured
		}
		st.Settings.EscrowCoverNewUsers = cover
		return []store.AuditEntry{{
			When: now, Who: actorLogin(st, actorID), What: "escrow.policy",
			Detail: fmt.Sprintf("cover new=%v", cover),
		}}, nil
	})
}

// ChangeEscrowMaster re-encrypts the recovery private key (S-10.5).
func (s *Service) ChangeEscrowMaster(ctx context.Context, actorID, current, newPW, newPW2 string) error {
	if err := validateMasterPassword(newPW); err != nil {
		return err
	}
	if newPW != newPW2 {
		return ErrMasterPasswordMismatch
	}
	snap := s.st.Snapshot()
	if !store.EscrowActive(&snap) {
		return ErrEscrowNotConfigured
	}
	privBytes, err := crypt.UnwrapEscrowPrivateKey(ctx, current, snap.Settings.EscrowMasterSalt,
		store.MasterKDFParams(snap.Settings), snap.Settings.EscrowPrivateWrap)
	if err != nil {
		return ErrNotFound
	}
	var priv [crypt.KeyLen]byte
	copy(priv[:], privBytes)
	crypt.Wipe(privBytes)
	wrapped, salt, kdf, err := crypt.WrapEscrowPrivateKey(ctx, newPW, priv)
	crypt.Wipe(priv[:])
	if err != nil {
		return err
	}
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if !store.EscrowActive(st) {
			return nil, ErrEscrowNotConfigured
		}
		st.Settings.EscrowPrivateWrap = wrapped
		st.Settings.EscrowMasterSalt = salt
		store.SetMasterKDF(&st.Settings, kdf)
		return []store.AuditEntry{{
			When: now, Who: actorLogin(st, actorID), What: "escrow.master.change",
		}}, nil
	})
}

// DepositEscrow stores the session data key copy (S-10.5). Caller must have
// confirmed the password via the unlock sheet.
func (s *Service) DepositEscrow(accountID string, dataKey [crypt.KeyLen]byte) error {
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if !store.EscrowActive(st) {
			return nil, ErrEscrowDisabled
		}
		u := st.UserByID(accountID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		u.EscrowOptIn = true
		if err := store.TryDepositEscrow(st, u, dataKey, now); err != nil {
			return nil, err
		}
		if !store.UserDeposited(u) {
			return nil, ErrNotFound
		}
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "escrow.deposit", Whom: u.Login,
		}}, nil
	})
}

// WithdrawEscrow removes the deposited copy and opts out (S-10.5).
func (s *Service) WithdrawEscrow(accountID string) error {
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(accountID)
		if u == nil {
			return nil, ErrNotFound
		}
		store.ClearEscrowDeposit(u)
		u.EscrowOptIn = false
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "escrow.withdraw", Whom: u.Login,
		}}, nil
	})
}

// RecoverEscrow opens a deposited key and re-wraps it under a temporary
// password. Master password is derived and wiped in this call only (S-10.5).
func (s *Service) RecoverEscrow(ctx context.Context, actorID, login, masterPassword string) (tempPassword string, err error) {
	login = normalizeLogin(login)
	snap := s.st.Snapshot()
	if !store.EscrowActive(&snap) {
		return "", ErrEscrowNotConfigured
	}
	u := snap.UserByLogin(login)
	if u == nil || !store.UserDeposited(u) {
		return "", ErrNotDeposited
	}
	privBytes, err := crypt.UnwrapEscrowPrivateKey(ctx, masterPassword, snap.Settings.EscrowMasterSalt,
		store.MasterKDFParams(snap.Settings), snap.Settings.EscrowPrivateWrap)
	if err != nil {
		return "", ErrNotFound
	}
	var priv [crypt.KeyLen]byte
	copy(priv[:], privBytes)
	crypt.Wipe(privBytes)
	dataKey, err := crypt.OpenDepositedKey(priv, u.EscrowDeposited, login)
	crypt.Wipe(priv[:])
	if err != nil {
		return "", ErrNotFound
	}
	temp, err := newTempPassword()
	if err != nil {
		crypt.Wipe(dataKey[:])
		return "", err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		x := st.UserByLogin(login)
		if x == nil || !store.UserDeposited(x) {
			return nil, ErrNotDeposited
		}
		if err := store.RewrapPasswordWithKey(ctx, x, login, temp, dataKey); err != nil {
			return nil, err
		}
		burnWipeTokens(st, x.ID)
		pruneTokens(st, now)
		x.EscrowLastRecovery = &now
		s.mgr.EndAccount(x.ID)
		return []store.AuditEntry{{
			When: now, Who: actorLogin(st, actorID), What: "escrow.recover", Whom: login,
		}}, nil
	})
	crypt.Wipe(dataKey[:])
	if err != nil {
		return "", err
	}
	return temp, nil
}

// ConsumeEscrowNotice clears the first-login flag and returns whether a notice
// was pending (S-10.5).
func (s *Service) ConsumeEscrowNotice(accountID string) bool {
	snap := s.st.Snapshot()
	u := snap.UserByID(accountID)
	if u == nil || !u.EscrowNoticePending {
		return false
	}
	pending := false
	_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		x := st.UserByID(accountID)
		if x == nil || !x.EscrowNoticePending {
			return nil, nil
		}
		x.EscrowNoticePending = false
		pending = true
		return nil, nil
	})
	return pending
}

// EscrowEnabled reports whether instance escrow is active.
func (s *Service) EscrowEnabled() bool {
	snap := s.st.Snapshot()
	return store.EscrowActive(&snap)
}

func newTempPassword() (string, error) {
	var b [15]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	s := enc.EncodeToString(b[:])
	if len(s) < conf.MinPasswordLen {
		return s + "-extra", nil
	}
	return s[:20], nil
}
