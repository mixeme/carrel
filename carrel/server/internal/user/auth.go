package user

import (
	"context"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/store"
)

// SetupFirstAdmin makes the first administrator without an address.
func (s *Service) SetupFirstAdmin(ctx context.Context, login, password string) error {
	return s.SetupFirstAdminWith(ctx, login, password, "")
}

// SetupFirstAdminWith makes the first administrator, with an address when
// one was given (s-auth-4: Email is optional). The address is not confirmed
// here — nothing was sent to it — and until it is confirmed it recovers
// nothing (S-10.5); the account page sends the letter.
func (s *Service) SetupFirstAdminWith(ctx context.Context, login, password, email string) error {
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
			Email:     strings.TrimSpace(email),
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
		// A wrong password on an account that exists is journaled; an
		// unknown login is not — the journal keeps no typed text — and a
		// disabled account answers above without a line (audit №3, Ф3-7).
		s.loginRefused(u.Login, clientIPOf(ctx), false)
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

// clientIPKey carries the address a sign-in came from (WithClientIP).
type clientIPKey struct{}

// WithClientIP is ctx carrying the address a sign-in comes from: a refused
// one is journaled with it (the owner's decision of 16.10.2026, Ад38).
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

func clientIPOf(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}

// LoginLimited journals a sign-in to an account refused before its password
// was even read: too many attempts (Ад38). An unknown login leaves no line.
func (s *Service) LoginLimited(login, ip string) {
	login = normalizeLogin(login)
	snap := s.st.Snapshot()
	if u := snap.UserByLogin(login); u == nil || u.Disabled {
		return
	}
	s.loginRefused(login, ip, true)
}

// loginSeries is how long after its last attempt a series of refused
// sign-ins stays one line of the journal.
const loginSeries = time.Hour

// loginRefused journals a refused sign-in as the owner decided on
// 16.10.2026 (Ад38; s-admin-7): nobody acts before a sign-in, so its actor
// is «—»; the attempts in a row on one account are one line — «3rd attempt
// · ip 203.0.113.9 · rate-limited» — moved to the end at each attempt, so a
// stream of guesses cannot flush the journal; a successful sign-in or an
// hour without attempts starts a new one. The address is written of a
// refused sign-in only: where a person usually is is nobody's business.
func (s *Service) loginRefused(login, ip string, limited bool) {
	now := s.now()
	_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		n := 1
		for i := len(st.Audit) - 1; i >= 0; i-- {
			e := st.Audit[i]
			if e.Whom != login {
				continue
			}
			if e.What == "login.ok" {
				break
			}
			if e.What == "login.fail" {
				if now.Sub(e.When) <= loginSeries {
					n = attemptOf(e.Detail) + 1
					limited = limited || strings.Contains(e.Detail, "rate-limited")
					st.Audit = append(st.Audit[:i:i], st.Audit[i+1:]...)
				}
				break
			}
		}
		parts := []string{ordinal(n) + " attempt"}
		if ip != "" {
			parts = append(parts, "ip "+ip)
		}
		if limited {
			parts = append(parts, "rate-limited")
		}
		return []store.AuditEntry{{When: now, Who: "—", What: "login.fail", Whom: login, Detail: strings.Join(parts, " · ")}}, nil
	})
}

// attemptOf reads the count a line of refused sign-ins begins with.
func attemptOf(detail string) int {
	n := 0
	for _, r := range detail {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return 1
	}
	return n
}

// ordinal writes a count as English does: 1st, 2nd, 3rd, 4th, 11th, 22nd.
func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}
