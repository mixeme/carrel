package user

import (
	"errors"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	cmail "carrel.dev/carrel/server/internal/mail"
	"carrel.dev/carrel/server/internal/store"
)

// ErrBadAddress is an address that is not one address.
var ErrBadAddress = errors.New("Enter one address, like name@example.org.")

// ErrNameLong is a display name over the limit.
var ErrNameLong = errors.New("A name of up to 100 characters, on one line.")

// SetDisplayName is the name letters are sent and signed with (S-26.1);
// empty means the login.
func (s *Service) SetDisplayName(userID, name string) error {
	name = strings.TrimSpace(name)
	if len([]rune(name)) > 100 || strings.ContainsAny(name, "\r\n") {
		return ErrNameLong
	}
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(userID)
		if u == nil {
			return nil, ErrNotFound
		}
		if u.DisplayName == name {
			return nil, nil
		}
		u.DisplayName = name
		return []store.AuditEntry{{When: s.now(), Who: u.Login, What: "user.name", Whom: u.Login}}, nil
	})
}

// RequestEmail starts confirming an address: the new one waits as pending
// until its link is opened, and the address in use stays until then, so a
// typo does not take away the reply address. The
// token goes to the mail layer. Asking again replaces the link before.
// The same address as the confirmed one is nothing to do: token is empty.
func (s *Service) RequestEmail(userID, email string) (token string, err error) {
	addrs, err := cmail.Addresses([]string{email})
	if err != nil || len(addrs) != 1 {
		return "", ErrBadAddress
	}
	email = addrs[0]
	raw, hash, err := newSecret()
	if err != nil {
		return "", err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(userID)
		if u == nil {
			return nil, ErrNotFound
		}
		if u.EmailConfirmed && strings.EqualFold(u.Email, email) {
			u.PendingEmail = ""
			burnTokens(st, u.ID, tokenEmailConfirm)
			raw = ""
			return nil, nil
		}
		pruneTokens(st, now)
		burnTokens(st, u.ID, tokenEmailConfirm)
		// Only a registration keeps an unconfirmed address in Email (sign-in
		// waits for it); any other new address waits as pending, or it would
		// lock the person out.
		if !u.EmailConfirmed && strings.EqualFold(u.Email, email) {
			u.PendingEmail = ""
		} else {
			u.PendingEmail = email
		}
		st.Tokens = append(st.Tokens, store.SecretToken{
			Kind: tokenEmailConfirm, Hash: hash, UserID: u.ID, Email: email,
			CreatedAt: now, ExpiresAt: now.Add(conf.EmailConfirmTTL),
		})
		return []store.AuditEntry{{When: now, Who: u.Login, What: "user.email.request", Whom: u.Login}}, nil
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

// EmailToConfirm is the address a confirmation link for userID would go to:
// the pending one, else the unconfirmed one in use, else "".
func (s *Service) EmailToConfirm(userID string) string {
	snap := s.st.Snapshot()
	u := snap.UserByID(userID)
	switch {
	case u == nil:
		return ""
	case u.PendingEmail != "":
		return u.PendingEmail
	case u.Email != "" && !u.EmailConfirmed:
		return u.Email
	}
	return ""
}
