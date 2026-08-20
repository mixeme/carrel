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
	ActionCollectionCreate = "collection_create"
	ActionCollectionRename = "collection_rename"
	ActionCollectionDelete = "collection_delete"
)

// UpsertCollection adds or replaces one collection on an account.
func (s *Store) UpsertCollection(actor Actor, userID string, dek crypto.Key, accountID string, col discovery.Collection, auditAction string) error {
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
		copy := *acc
		path := normalizeStorePath(col.Path)
		replaced := false
		for i := range copy.Collections {
			if normalizeStorePath(copy.Collections[i].Path) == path {
				copy.Collections[i] = col
				replaced = true
				break
			}
		}
		if !replaced {
			copy.Collections = append(copy.Collections, col)
		}
		blob.Upsert(copy)
		sealed, err := account.Seal(dek, blob)
		if err != nil {
			return err
		}
		u.Secrets = sealed
		if auditAction != "" {
			name := col.DisplayName
			if name == "" {
				name = col.Path
			}
			appendAudit(state, s.now(), AuditEntry{
				Action:      auditAction,
				ActorID:     actor.ID,
				ActorLogin:  actor.Login,
				TargetID:    u.ID,
				TargetLogin: u.Login,
				IP:          actor.IP,
				Detail:      fmt.Sprintf("%s:%s", accountID, name),
			})
		}
		return nil
	})
}

// RemoveCollection drops one collection from the stored account and purges views.
func (s *Store) RemoveCollection(actor Actor, userID string, dek crypto.Key, accountID, colPath string) error {
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
		want := normalizeStorePath(colPath)
		out := acc.Collections[:0]
		var removed discovery.Collection
		found := false
		for _, c := range acc.Collections {
			if normalizeStorePath(c.Path) == want {
				removed = c
				found = true
				continue
			}
			out = append(out, c)
		}
		if !found {
			return ErrNotFound
		}
		acc.Collections = out
		ref := account.SourceRef{AccountID: accountID, Collection: colPath}
		blob.Views.PurgeCollection(ref)
		blob.Duplicates.PurgeCollection(ref)
		blob.Upsert(*acc)
		sealed, err := account.Seal(dek, blob)
		if err != nil {
			return err
		}
		u.Secrets = sealed
		name := removed.DisplayName
		if name == "" {
			name = removed.Path
		}
		appendAudit(state, s.now(), AuditEntry{
			Action:      ActionCollectionDelete,
			ActorID:     actor.ID,
			ActorLogin:  actor.Login,
			TargetID:    u.ID,
			TargetLogin: u.Login,
			IP:          actor.IP,
			Detail:      fmt.Sprintf("%s:%s", accountID, name),
		})
		return nil
	})
}

func normalizeStorePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}
