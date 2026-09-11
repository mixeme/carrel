package web

import (
	"context"
	"errors"
	"time"

	"carrel.dev/carrel/server/internal/store"
)

// stubUsers is a minimal Users implementation for wiring tests before
// internal/user lands. Replaced by user.Service in production.
type stubUsers struct {
	st *store.Store
}

func NewStubUsers(st *store.Store) Users {
	return &stubUsers{st: st}
}

func (u *stubUsers) FirstRun() bool { return u.st.FirstRun() }

func (u *stubUsers) SetupFirstAdmin(ctx context.Context, login, password string) error {
	return errNotImplemented
}

func (u *stubUsers) Authenticate(ctx context.Context, login, password string) (string, [32]byte, bool, bool) {
	return "", [32]byte{}, false, false
}

func (u *stubUsers) LoginExists(login string) bool { return false }

func (u *stubUsers) RegisterOpen() bool { return false }

func (u *stubUsers) InviteValid(ctx context.Context, token string) bool { return false }

func (u *stubUsers) CompleteInvite(ctx context.Context, token, password string) error {
	return errNotImplemented
}

func (u *stubUsers) ConfirmEmail(ctx context.Context, token string) bool { return false }

func (u *stubUsers) Register(ctx context.Context, login, email, password string) error {
	return errNotImplemented
}

func (u *stubUsers) ForgotRequest(ctx context.Context, loginOrEmail string) (bool, bool, bool, error) {
	return false, false, false, nil
}

func (u *stubUsers) ForgotWipeState(ctx context.Context, token string) (string, string, bool) {
	return "", "", false
}

func (u *stubUsers) ForgotConfirmWipe(ctx context.Context, token string) error {
	return errNotImplemented
}

func (u *stubUsers) ForgotSetPassword(ctx context.Context, token, password string) error {
	return errNotImplemented
}

func (u *stubUsers) IsAdmin(accountID string) bool { return false }

func (u *stubUsers) AccountRole(accountID string) store.Role { return store.RoleUser }

func (u *stubUsers) OwnsAccount(viewerID, accountParam string) bool {
	return viewerID == accountParam
}

func (u *stubUsers) OwnsCollection(viewerID, accountParam, col string) bool {
	return u.OwnsAccount(viewerID, accountParam) && col != ""
}

func (u *stubUsers) OwnsRecord(viewerID, accountParam, col, uid string) bool {
	return u.OwnsCollection(viewerID, accountParam, col) && uid != ""
}

func (u *stubUsers) PublicLinkValid(ctx context.Context, token string) bool { return false }

func (u *stubUsers) UnlockUntil(accountID string) *time.Time { return nil }

func (u *stubUsers) SetUnlockUntil(accountID string, until *time.Time) {}

var errNotImplemented = errors.New("web: stub users")
