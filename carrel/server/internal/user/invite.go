package user

import (
	"context"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

func (s *Service) InviteValid(ctx context.Context, token string) bool {
	hash, err := hashToken(token)
	if err != nil {
		return false
	}
	snap := s.st.Snapshot()
	for _, inv := range snap.Invites {
		if inv.Revoked {
			continue
		}
		if !s.now().Before(inv.ExpiresAt) {
			continue
		}
		if verifyHash(inv.SecretHash, hash) {
			return true
		}
	}
	return false
}

func (s *Service) CompleteInvite(ctx context.Context, token, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := hashToken(token)
	if err != nil {
		return ErrNotFound
	}
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		var inv *store.Invite
		for i := range st.Invites {
			if verifyHash(st.Invites[i].SecretHash, hash) && !st.Invites[i].Revoked && now.Before(st.Invites[i].ExpiresAt) {
				inv = &st.Invites[i]
				break
			}
		}
		if inv == nil {
			return nil, ErrNotFound
		}
		if st.UserByLogin(inv.Login) != nil && len(st.UserByLogin(inv.Login).LoginHash) > 0 {
			return nil, ErrLoginTaken
		}
		u := st.UserByLogin(inv.Login)
		if u == nil {
			id, err := NewAccountID()
			if err != nil {
				return nil, err
			}
			st.Users = append(st.Users, store.User{
				ID: id, Login: inv.Login, Email: inv.Email, Role: store.RoleUser, CreatedAt: now,
			})
			u = st.UserByLogin(inv.Login)
		}
		if err := store.ApplyPassword(u, u.Login, password); err != nil {
			return nil, err
		}
		if u.Email != "" {
			u.EmailConfirmed = true
		}
		inv.Revoked = true
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "user.invite.complete", Whom: u.Login,
		}}, nil
	})
}

// CreateInvite adds an invitation and returns the raw secret for the link.
func (s *Service) CreateInvite(login, email string) (secret string, err error) {
	login = normalizeLogin(login)
	raw, hash, err := newSecret()
	if err != nil {
		return "", err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if st.UserByLogin(login) != nil && len(st.UserByLogin(login).LoginHash) > 0 {
			return nil, ErrLoginTaken
		}
		id, err := NewAccountID()
		if err != nil {
			return nil, err
		}
		st.Invites = append(st.Invites, store.Invite{
			ID: id, Login: login, Email: email,
			SecretHash: hash, CreatedAt: now, ExpiresAt: now.Add(conf.InviteTTL),
		})
		if st.UserByLogin(login) == nil {
			st.Users = append(st.Users, store.User{
				ID: id, Login: login, Email: email, Role: store.RoleUser, CreatedAt: now,
			})
		}
		return []store.AuditEntry{{
			When: now, Who: "admin", What: "user.invite.create", Whom: login,
		}}, nil
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Service) Register(ctx context.Context, login, email, password string) error {
	if !s.RegisterOpen() {
		return ErrNotFound
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	login = normalizeLogin(login)
	raw, hash, err := newSecret()
	if err != nil {
		return err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByLogin(login)
		if u != nil && u.EmailConfirmed {
			return nil, ErrLoginTaken
		}
		id, err := NewAccountID()
		if err != nil {
			return nil, err
		}
		if u == nil {
			st.Users = append(st.Users, store.User{
				ID: id, Login: login, Email: email, Role: store.RoleUser, CreatedAt: now,
			})
			u = st.UserByLogin(login)
		} else {
			u.Email = email
			store.WipeCredentials(u)
		}
		if err := store.ApplyPassword(u, login, password); err != nil {
			return nil, err
		}
		u.EmailConfirmed = false
		st.Tokens = append(st.Tokens, store.SecretToken{
			Kind: "email_confirm", Hash: hash, UserID: u.ID,
			CreatedAt: now, ExpiresAt: now.Add(conf.EmailConfirmTTL),
		})
		return []store.AuditEntry{{
			When: now, Who: login, What: "user.register", Whom: login,
		}}, nil
	})
	_ = raw // mail layer sends link later
	return err
}

func (s *Service) ConfirmEmail(ctx context.Context, token string) bool {
	hash, err := hashToken(token)
	if err != nil {
		return false
	}
	now := s.now()
	ok := false
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Tokens {
			t := &st.Tokens[i]
			if t.Kind != "email_confirm" || t.Used || !now.Before(t.ExpiresAt) {
				continue
			}
			if !verifyHash(t.Hash, hash) {
				continue
			}
			u := st.UserByID(t.UserID)
			if u == nil {
				continue
			}
			u.EmailConfirmed = true
			t.Used = true
			ok = true
			return []store.AuditEntry{{
				When: now, Who: u.Login, What: "user.email.confirm", Whom: u.Login,
			}}, nil
		}
		return nil, nil
	})
	return ok && err == nil
}

const (
	tokenForgotWipe  = "forgot_wipe"
	tokenConsoleWipe = "console_wipe"
)

func (s *Service) ForgotRequest(ctx context.Context, loginOrEmail string) (sent, asked, hasMail bool, err error) {
	loginOrEmail = strings.TrimSpace(loginOrEmail)
	snap := s.st.Snapshot()
	var u *store.User
	for _, x := range snap.Users {
		if strings.EqualFold(x.Login, loginOrEmail) || strings.EqualFold(x.Email, loginOrEmail) {
			u = &x
			break
		}
	}
	if u == nil || !u.EmailConfirmed {
		return false, false, false, nil
	}
	if snap.Settings.EscrowEnabled && u.EscrowOptIn {
		return false, false, true, nil
	}
	raw, hash, err := newSecret()
	if err != nil {
		return false, false, true, err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Tokens = append(st.Tokens, store.SecretToken{
			Kind: tokenForgotWipe, Hash: hash, UserID: u.ID, Step: string(view.WipeConfirm),
			CreatedAt: now, ExpiresAt: now.Add(conf.ForgotLinkTTL),
		})
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "user.forgot.request", Whom: u.Login,
		}}, nil
	})
	if err != nil {
		return false, false, true, err
	}
	_ = raw
	return true, false, true, nil
}

func (s *Service) ForgotWipeState(ctx context.Context, token string) (login string, step string, valid bool) {
	hash, err := hashToken(token)
	if err != nil {
		return "", "", false
	}
	snap := s.st.Snapshot()
	now := s.now()
	for _, t := range snap.Tokens {
		if t.Kind != tokenForgotWipe && t.Kind != tokenConsoleWipe {
			continue
		}
		if t.Used || !now.Before(t.ExpiresAt) {
			continue
		}
		if !verifyHash(t.Hash, hash) {
			continue
		}
		u := snap.UserByID(t.UserID)
		if u == nil {
			continue
		}
		step := t.Step
		if step == "" {
			step = string(view.WipeConfirm)
		}
		return u.Login, step, true
	}
	return "", "", false
}

func (s *Service) ForgotConfirmWipe(ctx context.Context, token string) error {
	hash, err := hashToken(token)
	if err != nil {
		return ErrNotFound
	}
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Tokens {
			t := &st.Tokens[i]
			if t.Kind != tokenForgotWipe && t.Kind != tokenConsoleWipe {
				continue
			}
			if t.Used || !now.Before(t.ExpiresAt) || !verifyHash(t.Hash, hash) {
				continue
			}
			u := st.UserByID(t.UserID)
			if u == nil {
				continue
			}
			store.WipeCredentials(u)
			t.Step = string(view.WipePassword)
			s.mgr.EndAccount(u.ID)
			return []store.AuditEntry{{
				When: now, Who: u.Login, What: "user.wipe", Whom: u.Login,
			}}, nil
		}
		return nil, ErrNotFound
	})
}

func (s *Service) ForgotSetPassword(ctx context.Context, token, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := hashToken(token)
	if err != nil {
		return ErrNotFound
	}
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Tokens {
			t := &st.Tokens[i]
			if t.Kind != tokenForgotWipe && t.Kind != tokenConsoleWipe {
				continue
			}
			if t.Used || !now.Before(t.ExpiresAt) || !verifyHash(t.Hash, hash) {
				continue
			}
			if t.Step != string(view.WipePassword) {
				continue
			}
			u := st.UserByID(t.UserID)
			if u == nil {
				continue
			}
			if err := store.ApplyPassword(u, u.Login, password); err != nil {
				return nil, err
			}
			t.Used = true
			return []store.AuditEntry{{
				When: now, Who: u.Login, What: "user.password.set", Whom: u.Login,
			}}, nil
		}
		return nil, ErrNotFound
	})
}

// IssueConsoleWipeLink creates a one-time console wipe link (S-10.4).
func (s *Service) IssueConsoleWipeLink(login string) (url string, err error) {
	login = normalizeLogin(login)
	snap := s.st.Snapshot()
	u := snap.UserByLogin(login)
	if u == nil {
		return "", ErrNotFound
	}
	raw, hash, err := newSecret()
	if err != nil {
		return "", err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Tokens {
			if st.Tokens[i].Kind == tokenConsoleWipe && st.Tokens[i].UserID == u.ID && !st.Tokens[i].Used {
				st.Tokens[i].Used = true
			}
		}
		st.Tokens = append(st.Tokens, store.SecretToken{
			Kind: tokenConsoleWipe, Hash: hash, UserID: u.ID, Step: string(view.WipeConfirm),
			CreatedAt: now, ExpiresAt: now.Add(conf.ConsoleWipeLinkTTL),
		})
		return []store.AuditEntry{{
			When: now, Who: "console", What: "user.console.wipe", Whom: login,
		}}, nil
	})
	if err != nil {
		return "", err
	}
	return "/forgot/" + raw, nil
}

// DisableUser disables an account; refuses last admin (S-10.6).
func (s *Service) DisableUser(actorID, targetID string) error {
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(targetID)
		if u == nil {
			return nil, ErrNotFound
		}
		if u.Role == store.RoleAdmin && store.AdminCount(st) <= 1 {
			return nil, ErrLastAdmin
		}
		u.Disabled = true
		s.mgr.EndAccount(targetID)
		actor := st.UserByID(actorID)
		who := "admin"
		if actor != nil {
			who = actor.Login
		}
		return []store.AuditEntry{{
			When: s.now(), Who: who, What: "user.disable", Whom: u.Login,
		}}, nil
	})
}
