// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// Duplicates returns the user's decisions about duplicate groups (§15).
func (s *Store) Duplicates(userID string, dek crypto.Key) (account.Duplicates, error) {
	blob, err := s.openSecrets(userID, dek)
	if err != nil {
		return account.Duplicates{}, err
	}
	return blob.Duplicates.Clone(), nil
}

// UpdateDuplicates applies fn to the stored decisions and commits the result.
//
// §15 puts these on the volume rather than in the session on purpose: a group
// marked "not duplicates" has to stay marked after a restart (§21). They travel
// in the same sealed blob as the credentials, so a decision costs one decryption
// of what the session already has the key for.
func (s *Store) UpdateDuplicates(userID string, dek crypto.Key, fn func(*account.Duplicates) error) error {
	if fn == nil {
		return nil
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
		decisions := blob.Duplicates.Clone()
		if err := fn(&decisions); err != nil {
			return err
		}
		blob.Duplicates = decisions
		sealed, err := account.Seal(dek, blob)
		if err != nil {
			return err
		}
		u.Secrets = sealed
		return nil
	})
}
