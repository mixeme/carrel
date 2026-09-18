package user

import (
	"errors"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
)

// tokenDeviceProfile is the one-time link to an Apple profile (S-23). The
// state holds its hash, who it is for and which connections it carries —
// never the profile: that is built in memory when the link is followed, from
// the blob of the session that follows it.
const tokenDeviceProfile = "device-profile"

// ErrNothingToIssue is a profile link with no connection to carry.
var ErrNothingToIssue = errors.New("user: no connection goes into the profile")

// IssueDeviceProfile makes the one-time link to a profile carrying accounts,
// for conf.DeviceProfileLinkTTL. The caller checks the unlock window. The
// journal names the connections, not the servers or the logins.
func (s *Service) IssueDeviceProfile(viewerID string, accounts []string) (token string, expires time.Time, err error) {
	if len(accounts) == 0 {
		return "", time.Time{}, ErrNothingToIssue
	}
	raw, hash, err := newSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	now := s.now()
	expires = now.Add(conf.DeviceProfileLinkTTL)
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		pruneTokens(st, now)
		st.Tokens = append(st.Tokens, store.SecretToken{
			Kind: tokenDeviceProfile, Hash: hash, UserID: u.ID, Step: strings.Join(accounts, ","),
			CreatedAt: now, ExpiresAt: expires,
		})
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "device.profile.link", Whom: u.Login, Detail: strings.Join(accounts, " "),
		}}, nil
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return raw, expires, nil
}

// TakeDeviceProfile burns the link and returns the connections it carries.
// Unknown, used, expired and someone else's are one answer: false. The
// download is journaled with the connections that went in.
func (s *Service) TakeDeviceProfile(viewerID, token string) ([]string, bool) {
	hash, err := hashToken(token)
	if err != nil {
		return nil, false
	}
	var accounts []string
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Tokens {
			t := &st.Tokens[i]
			if t.Kind != tokenDeviceProfile || t.Used || !now.Before(t.ExpiresAt) || !verifyHash(t.Hash, hash) {
				continue
			}
			u := st.UserByID(t.UserID)
			if u == nil || u.Disabled || t.UserID != viewerID {
				return nil, ErrNotFound
			}
			t.Used = true
			accounts = strings.Split(t.Step, ",")
			return []store.AuditEntry{{
				When: now, Who: u.Login, What: "device.profile.download", Whom: u.Login, Detail: strings.Join(accounts, " "),
			}}, nil
		}
		return nil, ErrNotFound
	})
	return accounts, err == nil
}

// RevokeDeviceProfiles burns every link of the viewer not yet followed.
func (s *Service) RevokeDeviceProfiles(viewerID string) error {
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil {
			return nil, ErrNotFound
		}
		n := 0
		for i := range st.Tokens {
			t := &st.Tokens[i]
			if t.Kind == tokenDeviceProfile && t.UserID == viewerID && !t.Used && now.Before(t.ExpiresAt) {
				t.Used = true
				n++
			}
		}
		if n == 0 {
			return nil, nil
		}
		return []store.AuditEntry{{When: now, Who: u.Login, What: "device.profile.revoke", Whom: u.Login}}, nil
	})
}

// SetDeviceExcluded marks a connection left out of every device set-up, or
// takes the mark off (S-23). The journal names the connection.
func (s *Service) SetDeviceExcluded(viewerID string, dataKey [crypt.KeyLen]byte, accountID string, excluded bool) error {
	what := "device.include"
	if excluded {
		what = "device.exclude"
	}
	return s.editCatalog(viewerID, dataKey, what, accountID, func(blob *sealed.Blob) error {
		if _, ok := blob.Connection(accountID); !ok {
			return ErrNotFound
		}
		var kept []string
		for _, id := range blob.DeviceExcluded {
			if id != accountID {
				kept = append(kept, id)
			}
		}
		if excluded {
			kept = append(kept, accountID)
		}
		blob.DeviceExcluded = kept
		return nil
	})
}
