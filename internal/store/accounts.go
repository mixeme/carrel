// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"fmt"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/crypto"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
)

const (
	ActionDAVAccountAdd    = "dav_account_add"
	ActionDAVAccountUpdate = "dav_account_update"
	ActionDAVAccountDelete = "dav_account_delete"
)

// DAVAccountCount returns how many enabled DAV accounts a user has.
func (s *Store) DAVAccountCount(userID string, dek crypto.Key) (int, error) {
	blob, err := s.openSecrets(userID, dek)
	if err != nil {
		return 0, err
	}
	return blob.CountEnabled(), nil
}

// ListDAVAccounts returns every stored DAV account for a user.
func (s *Store) ListDAVAccounts(userID string, dek crypto.Key) ([]account.Account, error) {
	blob, err := s.openSecrets(userID, dek)
	if err != nil {
		return nil, err
	}
	out := make([]account.Account, len(blob.Accounts))
	copy(out, blob.Accounts)
	return out, nil
}

// GetDAVAccount returns one DAV account by ID.
func (s *Store) GetDAVAccount(userID, accountID string, dek crypto.Key) (*account.Account, error) {
	blob, err := s.openSecrets(userID, dek)
	if err != nil {
		return nil, err
	}
	acc, err := blob.Find(accountID)
	if err != nil {
		return nil, ErrNotFound
	}
	copy := *acc
	return &copy, nil
}

// PutDAVAccount stores or replaces a DAV account in the user's secrets blob.
func (s *Store) PutDAVAccount(actor Actor, userID string, dek crypto.Key, acc account.Account) error {
	if strings.TrimSpace(acc.BaseURL) == "" {
		return fmt.Errorf("store: DAV base URL is required")
	}
	if strings.TrimSpace(acc.Username) == "" {
		return fmt.Errorf("store: DAV username is required")
	}
	if acc.ID == "" {
		id, err := account.NewID()
		if err != nil {
			return err
		}
		acc.ID = id
	}
	if acc.DiscoveredAt.IsZero() {
		acc.DiscoveredAt = s.now()
	}
	if !acc.Enabled {
		acc.Enabled = true
	}

	return s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		blob, err := account.Open(dek, u.Secrets)
		if err != nil {
			return err
		}
		_, findErr := blob.Find(acc.ID)
		blob.Upsert(acc)
		sealed, err := account.Seal(dek, blob)
		if err != nil {
			return err
		}
		u.Secrets = sealed
		u.DAVAccountCount = blob.CountEnabled()
		action := ActionDAVAccountUpdate
		if findErr != nil {
			action = ActionDAVAccountAdd
		}
		appendAudit(state, s.now(), AuditEntry{
			Action:      action,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
			Detail:      acc.ID,
		})
		return nil
	})
}

// PutDAVAccountFromDiscovery builds an account from a discovery result.
func (s *Store) PutDAVAccountFromDiscovery(actor Actor, userID string, dek crypto.Key, label string, creds discovery.Credentials, result *discovery.Result) (*account.Account, error) {
	if result == nil {
		return nil, fmt.Errorf("store: discovery result is required")
	}
	id, err := account.NewID()
	if err != nil {
		return nil, err
	}
	acc := account.Account{
		ID:           id,
		Label:        label,
		BaseURL:      result.BaseURL,
		Username:     creds.Username,
		Password:     creds.Password,
		Principal:    result.Principal,
		Enabled:      true,
		Collections:  result.Collections,
		DiscoveredAt: s.now(),
	}
	if err := s.PutDAVAccount(actor, userID, dek, acc); err != nil {
		return nil, err
	}
	return &acc, nil
}

// DeleteDAVAccount removes one DAV account.
func (s *Store) DeleteDAVAccount(actor Actor, userID, accountID string, dek crypto.Key) error {
	return s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		blob, err := account.Open(dek, u.Secrets)
		if err != nil {
			return err
		}
		if !blob.Remove(accountID) {
			return ErrNotFound
		}
		sealed, err := account.Seal(dek, blob)
		if err != nil {
			return err
		}
		u.Secrets = sealed
		u.DAVAccountCount = blob.CountEnabled()
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionDAVAccountDelete,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
			Detail:      accountID,
		})
		return nil
	})
}

// SetDAVAccountEnabled toggles one account without dropping credentials.
func (s *Store) SetDAVAccountEnabled(actor Actor, userID, accountID string, dek crypto.Key, enabled bool) error {
	return s.update(func(state *State) error {
		u := findUser(state, userID)
		if u == nil {
			return ErrNotFound
		}
		blob, err := account.Open(dek, u.Secrets)
		if err != nil {
			return err
		}
		acc, err := blob.Find(accountID)
		if err != nil {
			return ErrNotFound
		}
		acc.Enabled = enabled
		blob.Upsert(*acc)
		sealed, err := account.Seal(dek, blob)
		if err != nil {
			return err
		}
		u.Secrets = sealed
		u.DAVAccountCount = blob.CountEnabled()
		return nil
	})
}

func (s *Store) openSecrets(userID string, dek crypto.Key) (*account.Blob, error) {
	var sealed []byte
	found := false
	s.read(func(state *State) {
		if u := findUser(state, userID); u != nil {
			found = true
			sealed = cloneBytes(u.Secrets)
		}
	})
	if !found {
		return nil, ErrNotFound
	}
	return account.Open(dek, sealed)
}
