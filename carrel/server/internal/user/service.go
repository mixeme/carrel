package user

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
)

// Service implements account management (S-10).
type Service struct {
	st  *store.Store
	mgr *session.Manager
	cfg *conf.Config
	now func() time.Time

	// dummy is a login record with a random password. Authenticate verifies
	// against it when the login is unknown, disabled, unconfirmed, or has no
	// password yet, so a miss costs one Argon2 like a wrong password does
	// (S-29.1: unknown login and wrong password answer alike). Built on first
	// use; startup does not pay for it.
	// seen is the last request of each davloom device, in memory (S-27).
	seen deviceSeen

	dummyOnce sync.Once
	dummySalt []byte
	dummyHash []byte
	dummyKDF  crypt.KDFParams
}

// New creates a user service.
func New(st *store.Store, mgr *session.Manager, cfg *conf.Config) *Service {
	s := &Service{
		st:  st,
		mgr: mgr,
		cfg: cfg,
		now: time.Now,
	}
	s.applyLifetimes()
	return s
}

var (
	ErrPasswordShort    = errors.New("password must be at least 8 characters")
	ErrPasswordMismatch = errors.New("passwords do not match")
	ErrLoginTaken       = errors.New("login is already taken")
	ErrLastAdmin        = errors.New("cannot remove the last administrator")
	ErrLoginConfirm     = errors.New("type the login to confirm deletion")
	ErrNotFound         = errors.New("not found")
)

func (s *Service) FirstRun() bool { return s.st.FirstRun() }

func (s *Service) RegisterOpen() bool {
	snap := s.st.Snapshot()
	// The confirmation letter needs a relay (S-26); without one nobody who
	// registers could ever sign in.
	return snap.Settings.SelfRegEnabled && snap.Settings.SMTP.Configured()
}

func (s *Service) LoginExists(login string) bool {
	snap := s.st.Snapshot()
	return snap.UserByLogin(login) != nil
}

func (s *Service) IsAdmin(accountID string) bool {
	snap := s.st.Snapshot()
	u := snap.UserByID(accountID)
	return u != nil && u.Role == store.RoleAdmin && !u.Disabled
}

func (s *Service) AccountRole(accountID string) store.Role {
	snap := s.st.Snapshot()
	u := snap.UserByID(accountID)
	if u == nil {
		return store.RoleUser
	}
	return u.Role
}

// OwnsAccount reports whether {account} is one of the viewer's connections.
// An account is a DAV connection (C-4), and the only list of a person's
// connections lives in their sealed blob, so the check opens the blob with
// the session's data key. A foreign ID, a stale key, and the viewer's own
// user ID all come back false: the user ID is not a connection.
func (s *Service) OwnsAccount(viewerID string, dataKey [crypt.KeyLen]byte, accountParam string) bool {
	if accountParam == "" {
		return false
	}
	snap := s.st.Snapshot()
	u := snap.UserByID(viewerID)
	if u == nil || u.Disabled {
		return false
	}
	blob, err := sealed.Open(dataKey, u.SealedBlob, u.Login)
	if err != nil {
		return false
	}
	if _, ok := blob.Subscription(accountParam); ok {
		return true // a subscription is the person's too (S-25.2)
	}
	for _, c := range blob.Connections {
		if c.ID == accountParam {
			return true
		}
	}
	return false
}

// Unlock verifies the password of the session's own account for the S-10.7
// confirmation sheet and journals the unlock. It never returns key material:
// the session already holds the data key.
func (s *Service) Unlock(ctx context.Context, accountID, password string) bool {
	snap := s.st.Snapshot()
	u := snap.UserByID(accountID)
	if u == nil || u.Disabled || len(u.LoginHash) == 0 {
		s.verifyDummy(ctx, password)
		return false
	}
	if !store.VerifyPassword(ctx, u, password) {
		return false
	}
	now := s.now()
	_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "secret.unlock", Whom: u.Login,
		}}, nil
	})
	return true
}

// verifyDummy burns one login derivation so a miss is not faster than a
// wrong password. The result is discarded.
func (s *Service) verifyDummy(ctx context.Context, password string) {
	s.dummyOnce.Do(func() {
		salt, err := crypt.NewSalt()
		if err != nil {
			return
		}
		pw, err := crypt.NewDataKey()
		if err != nil {
			return
		}
		hash, p, err := crypt.HashLogin(context.Background(), string(pw[:]), salt)
		if err != nil {
			return
		}
		s.dummySalt, s.dummyHash, s.dummyKDF = salt, hash, p
	})
	if s.dummyHash == nil {
		return
	}
	_, _ = crypt.VerifyLogin(ctx, password, s.dummySalt, s.dummyHash, s.dummyKDF)
}

func normalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

func validatePassword(pw string) error {
	if len(pw) < conf.MinPasswordLen {
		return ErrPasswordShort
	}
	return nil
}
