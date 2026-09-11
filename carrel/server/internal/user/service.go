package user

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
)

// Service implements account management (S-10).
type Service struct {
	st  *store.Store
	mgr *session.Manager
	cfg *conf.Config
	now func() time.Time

	mu     sync.Mutex
	unlock map[string]time.Time
}

// New creates a user service.
func New(st *store.Store, mgr *session.Manager, cfg *conf.Config) *Service {
	return &Service{
		st:     st,
		mgr:    mgr,
		cfg:    cfg,
		now:    time.Now,
		unlock: make(map[string]time.Time),
	}
}

var (
	ErrPasswordShort    = errors.New("password must be at least 8 characters")
	ErrPasswordMismatch = errors.New("passwords do not match")
	ErrLoginTaken       = errors.New("login is already taken")
	ErrLastAdmin        = errors.New("cannot remove the last administrator")
	ErrNotFound         = errors.New("not found")
)

func (s *Service) FirstRun() bool { return s.st.FirstRun() }

func (s *Service) RegisterOpen() bool {
	snap := s.st.Snapshot()
	return snap.Settings.SelfRegEnabled && s.cfg.MailEnabled
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

func (s *Service) OwnsAccount(viewerID, accountParam string) bool {
	return viewerID == accountParam
}

func (s *Service) OwnsCollection(viewerID, accountParam, col string) bool {
	return s.OwnsAccount(viewerID, accountParam) && col != "" && !strings.Contains(col, "/")
}

func (s *Service) OwnsRecord(viewerID, accountParam, col, uid string) bool {
	return s.OwnsCollection(viewerID, accountParam, col) && uid != ""
}

func (s *Service) UnlockUntil(accountID string) *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.unlock[accountID]
	if !ok || !s.now().Before(t) {
		delete(s.unlock, accountID)
		return nil
	}
	return &t
}

func (s *Service) SetUnlockUntil(accountID string, until *time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if until == nil {
		delete(s.unlock, accountID)
		return
	}
	s.unlock[accountID] = *until
}

func (s *Service) PublicLinkValid(_ context.Context, token string) bool {
	return false
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
