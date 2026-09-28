package user

import (
	"context"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

const (
	tokenEmailConfirm = "email_confirm"
	tokenForgotWipe   = "forgot_wipe"
	tokenConsoleWipe  = "console_wipe"
)

func inviteLive(inv *store.Invite, now time.Time) bool {
	return !inv.Revoked && !inv.Used && now.Before(inv.ExpiresAt)
}

func hasLiveInvite(st *store.State, login string, now time.Time) bool {
	for i := range st.Invites {
		if st.Invites[i].Login == login && inviteLive(&st.Invites[i], now) {
			return true
		}
	}
	return false
}

func (s *Service) InviteValid(ctx context.Context, token string) bool {
	hash, err := hashToken(token)
	if err != nil {
		return false
	}
	snap := s.st.Snapshot()
	now := s.now()
	for i := range snap.Invites {
		inv := &snap.Invites[i]
		if !inviteLive(inv, now) {
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
			if inviteLive(&st.Invites[i], now) && verifyHash(st.Invites[i].SecretHash, hash) {
				inv = &st.Invites[i]
				break
			}
		}
		if inv == nil {
			return nil, ErrNotFound
		}
		u := st.UserByLogin(inv.Login)
		if u != nil && len(u.LoginHash) > 0 {
			return nil, ErrLoginTaken
		}
		if u == nil {
			id, err := NewUserID()
			if err != nil {
				return nil, err
			}
			st.Users = append(st.Users, store.User{
				ID: id, Login: inv.Login, Email: inv.Email, Role: store.RoleUser, CreatedAt: now,
			})
			u = st.UserByLogin(inv.Login)
		}
		applyEscrowCoverage(st, u)
		if err := store.ApplyPassword(ctx, u, u.Login, password, st, now); err != nil {
			return nil, err
		}
		// The address is proven only when the invitation went to it: a link
		// handed over proves nothing about the address typed beside it, and
		// with Reply-To (S-26.1) a wrong one sends people's answers to a
		// stranger.
		// An unproven address waits as pending — in Email it would lock the
		// person out: sign-in refuses an unconfirmed Email (registration).
		u.EmailConfirmed = u.Email != "" && inv.Mailed == "sent"
		if u.Email != "" && !u.EmailConfirmed {
			u.Email, u.PendingEmail = "", u.Email
		}
		inv.Used = true
		burnWipeTokens(st, u.ID)
		pruneTokens(st, now)
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "user.invite.complete", Whom: u.Login,
		}}, nil
	})
}

// CreateInvite adds an invitation and returns the raw secret for the link.
// A second invitation for the same login replaces the secret: the earlier
// link stops working (S-10.3, "отправить повторно с заменой секрета").
func (s *Service) CreateInvite(actorID, login, email string) (secret string, err error) {
	login = normalizeLogin(login)
	if login == "" {
		return "", ErrNotFound
	}
	raw, hash, err := newSecret()
	if err != nil {
		return "", err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if u := st.UserByLogin(login); u != nil && len(u.LoginHash) > 0 {
			return nil, ErrLoginTaken
		}
		for i := range st.Invites {
			if st.Invites[i].Login == login && inviteLive(&st.Invites[i], now) {
				st.Invites[i].Revoked = true
			}
		}
		inviteID, err := NewInviteID()
		if err != nil {
			return nil, err
		}
		st.Invites = append(st.Invites, store.Invite{
			ID: inviteID, Login: login, Email: email,
			SecretHash: hash, Secret: raw, CreatedAt: now, ExpiresAt: now.Add(inviteTTL(st)),
		})
		if st.UserByLogin(login) == nil {
			userID, err := NewUserID()
			if err != nil {
				return nil, err
			}
			st.Users = append(st.Users, store.User{
				ID: userID, Login: login, Email: email, Role: store.RoleUser, CreatedAt: now,
			})
		}
		if u := st.UserByLogin(login); u != nil {
			applyEscrowCoverage(st, u)
		}
		return []store.AuditEntry{{
			When: now, Who: actorLogin(st, actorID), What: "user.invite.create", Whom: login,
		}}, nil
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

// Register creates an unconfirmed account and returns the confirmation
// token for the mail layer (S-10.2). An existing record is reused only when
// it is this same self-registration still awaiting confirmation: same login,
// same address, a password already set, and no invitation pending for the
// login. Anything else is somebody's account and answers ErrLoginTaken —
// open registration must never wipe or take over an existing record.
func (s *Service) Register(ctx context.Context, login, email, password string) (token string, err error) {
	if !s.RegisterOpen() {
		return "", ErrNotFound
	}
	if err := validatePassword(password); err != nil {
		return "", err
	}
	login = normalizeLogin(login)
	email = strings.TrimSpace(email)
	if login == "" || email == "" {
		return "", ErrNotFound
	}
	raw, hash, err := newSecret()
	if err != nil {
		return "", err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		pruneTokens(st, now)
		u := st.UserByLogin(login)
		if u != nil {
			resubmit := u.Email != "" && !u.EmailConfirmed && len(u.LoginHash) > 0 &&
				strings.EqualFold(u.Email, email) && !hasLiveInvite(st, login, now)
			if !resubmit {
				return nil, ErrLoginTaken
			}
			store.WipeAccount(st, u)
			burnTokens(st, u.ID, tokenEmailConfirm)
		} else {
			if hasLiveInvite(st, login, now) {
				return nil, ErrLoginTaken
			}
			id, err := NewUserID()
			if err != nil {
				return nil, err
			}
			st.Users = append(st.Users, store.User{
				ID: id, Login: login, Email: email, Role: store.RoleUser, CreatedAt: now,
			})
			u = st.UserByLogin(login)
		}
		applyEscrowCoverage(st, u)
		if err := store.ApplyPassword(ctx, u, login, password, st, now); err != nil {
			return nil, err
		}
		u.EmailConfirmed = false
		burnWipeTokens(st, u.ID)
		st.Tokens = append(st.Tokens, store.SecretToken{
			Kind: tokenEmailConfirm, Hash: hash, UserID: u.ID,
			CreatedAt: now, ExpiresAt: now.Add(conf.EmailConfirmTTL),
		})
		return []store.AuditEntry{{
			When: now, Who: login, What: "user.register", Whom: login,
		}}, nil
	})
	if err != nil {
		return "", err
	}
	return raw, nil
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
			if t.Kind != tokenEmailConfirm || t.Used || !now.Before(t.ExpiresAt) {
				continue
			}
			if !verifyHash(t.Hash, hash) {
				continue
			}
			u := st.UserByID(t.UserID)
			if u == nil {
				continue
			}
			if t.Email != "" {
				if !strings.EqualFold(t.Email, u.PendingEmail) && !strings.EqualFold(t.Email, u.Email) {
					continue // an address asked for and then replaced
				}
				u.Email, u.PendingEmail = t.Email, ""
			}
			u.EmailConfirmed = true
			t.Used = true
			ok = true
			pruneTokens(st, now)
			return []store.AuditEntry{{
				When: now, Who: u.Login, What: "user.email.confirm", Whom: u.Login,
			}}, nil
		}
		return nil, ErrNotFound
	})
	return ok && err == nil
}

// ForgotRequest answers /forgot (S-10.4). It returns the wipe token for the
// mail layer when a link was issued. Unknown login, unconfirmed address, and
// a disabled account all come back with nothing: none of them can be helped
// from the sign-in screen, and none is confirmed to exist.
func (s *Service) ForgotRequest(ctx context.Context, loginOrEmail string) (token string, sent, asked, hasMail bool, err error) {
	loginOrEmail = strings.TrimSpace(loginOrEmail)
	snap := s.st.Snapshot()
	var u *store.User
	for i := range snap.Users {
		x := &snap.Users[i]
		if strings.EqualFold(x.Login, loginOrEmail) || (x.Email != "" && strings.EqualFold(x.Email, loginOrEmail)) {
			u = x
			break
		}
	}
	if u == nil || u.Disabled || u.Email == "" || !u.EmailConfirmed {
		return "", false, false, false, nil
	}
	if snap.Settings.EscrowEnabled && store.UserDeposited(u) {
		return "", false, true, true, nil
	}
	raw, hash, err := newSecret()
	if err != nil {
		return "", false, false, true, err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		pruneTokens(st, now)
		st.Tokens = append(st.Tokens, store.SecretToken{
			Kind: tokenForgotWipe, Hash: hash, UserID: u.ID, Step: string(view.WipeConfirm),
			CreatedAt: now, ExpiresAt: now.Add(conf.ForgotLinkTTL),
		})
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "user.forgot.request", Whom: u.Login,
		}}, nil
	})
	if err != nil {
		return "", false, false, true, err
	}
	return raw, true, false, true, nil
}

func isWipeToken(t *store.SecretToken) bool {
	return t.Kind == tokenForgotWipe || t.Kind == tokenConsoleWipe
}

func (s *Service) ForgotWipeState(ctx context.Context, token string) (login string, step string, valid bool) {
	hash, err := hashToken(token)
	if err != nil {
		return "", "", false
	}
	snap := s.st.Snapshot()
	now := s.now()
	for i := range snap.Tokens {
		t := &snap.Tokens[i]
		if !isWipeToken(t) || t.Used || !now.Before(t.ExpiresAt) {
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
	var wiped string
	defer func() {
		if wiped != "" {
			s.devicesChanged(wiped)
		}
	}()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Tokens {
			t := &st.Tokens[i]
			if !isWipeToken(t) || t.Used || !now.Before(t.ExpiresAt) || !verifyHash(t.Hash, hash) {
				continue
			}
			u := st.UserByID(t.UserID)
			if u == nil {
				continue
			}
			store.WipeAccount(st, u)
			wiped = u.ID
			t.Step = string(view.WipePassword)
			s.mgr.EndAccount(u.ID)
			return []store.AuditEntry{{
				When: now, Who: u.Login, What: "user.wipe", Whom: u.Login,
			}}, nil
		}
		return nil, ErrNotFound
	})
}

// ForgotSetPassword is step two. Setting the password burns this token and
// every other outstanding wipe token of the person: an older link must not
// wipe the account a second time (S-10.4, "сгорает вместе с заданным
// паролем").
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
			if !isWipeToken(t) || t.Used || !now.Before(t.ExpiresAt) || !verifyHash(t.Hash, hash) {
				continue
			}
			if t.Step != string(view.WipePassword) {
				continue
			}
			u := st.UserByID(t.UserID)
			if u == nil {
				continue
			}
			if err := store.ApplyPassword(ctx, u, u.Login, password, st, now); err != nil {
				return nil, err
			}
			burnWipeTokens(st, u.ID)
			pruneTokens(st, now)
			return []store.AuditEntry{{
				When: now, Who: u.Login, What: "user.password.set", Whom: u.Login,
			}}, nil
		}
		return nil, ErrNotFound
	})
}

// IssueConsoleWipeLink creates a one-time console wipe link (S-10.4).
// Repeating the command issues a new secret; the previous one dies.
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
		pruneTokens(st, now)
		burnTokens(st, u.ID, tokenConsoleWipe)
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
	defer s.devicesChanged(targetID) // the proxy forgets the keys it opened
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
		return []store.AuditEntry{{
			When: s.now(), Who: actorLogin(st, actorID), What: "user.disable", Whom: u.Login,
		}}, nil
	})
}

// EnableUser lets a disabled account sign in again (S-10.6, s-admin-12):
// its data was kept, so nothing else changes.
func (s *Service) EnableUser(actorID, targetID string) error {
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(targetID)
		if u == nil {
			return nil, ErrNotFound
		}
		u.Disabled = false
		return []store.AuditEntry{{
			When: s.now(), Who: actorLogin(st, actorID), What: "user.enable", Whom: u.Login,
		}}, nil
	})
}

// actorLogin names the acting account in the journal. An unknown actor is
// recorded as "admin" so the entry still says who could have done it.
func actorLogin(st *store.State, actorID string) string {
	if a := st.UserByID(actorID); a != nil {
		return a.Login
	}
	return "admin"
}

// burnTokens marks every live token of one kind for one account as used.
func burnTokens(st *store.State, userID, kind string) {
	for i := range st.Tokens {
		t := &st.Tokens[i]
		if t.Kind == kind && t.UserID == userID {
			t.Used = true
		}
	}
}

// burnWipeTokens kills every outstanding forgot and console link of one
// account. Called whenever the account gets a password.
func burnWipeTokens(st *store.State, userID string) {
	burnTokens(st, userID, tokenForgotWipe)
	burnTokens(st, userID, tokenConsoleWipe)
}

// pruneTokens drops used and expired tokens. A dead token can never match
// again, and keeping it only grows the state file that every Mutate
// re-encodes and rewrites.
func pruneTokens(st *store.State, now time.Time) {
	kept := st.Tokens[:0]
	for _, t := range st.Tokens {
		if t.Used || !now.Before(t.ExpiresAt) {
			continue
		}
		kept = append(kept, t)
	}
	st.Tokens = kept
}
