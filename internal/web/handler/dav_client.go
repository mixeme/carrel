// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/config"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/provider/contacts"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// Photo holds contact photo processing limits (§11).
type PhotoConfig = config.Photo

// ImportConfig holds contact import upload limits.
type ImportConfig = config.Import

// EncodeCollectionPath encodes a DAV collection path for use in a URL segment.
func EncodeCollectionPath(path string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(path))
}

// DecodeCollectionPath reverses EncodeCollectionPath.
func DecodeCollectionPath(enc string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return "", fmt.Errorf("bad collection path")
	}
	path := string(raw)
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("bad collection path")
	}
	return path, nil
}

func (s *Server) contactsProvider(sess *session.Session, accountID string) (*contacts.Provider, *account.Account, error) {
	if s.Guard == nil {
		return nil, nil, fmt.Errorf("DAV connections are not configured")
	}
	acc, err := s.Store.GetDAVAccount(sess.UserID, accountID, sess.DEK())
	if err != nil {
		return nil, nil, fmt.Errorf("account not found")
	}
	if !acc.Enabled {
		return nil, nil, fmt.Errorf("account is disabled")
	}
	client, err := dav.NewClient(s.Guard, acc.BaseURL, acc.Username, acc.Password)
	if err != nil {
		return nil, nil, err
	}
	p, err := contacts.New(client, contacts.Options{
		AccountID: acc.ID,
		Cache:     sess.Cache(),
		Losses:    sess.Losses(),
	})
	if err != nil {
		return nil, nil, err
	}
	return p, acc, nil
}

func findAddressBook(acc *account.Account, collection string) (discovery.Collection, error) {
	collection = normalizeCollectionPath(collection)
	for _, col := range acc.Collections {
		if col.Kind != discovery.KindAddressBook {
			continue
		}
		if normalizeCollectionPath(col.Path) == collection {
			return col, nil
		}
	}
	return discovery.Collection{}, fmt.Errorf("address book not found")
}

func addressBooks(accounts []account.Account) []addressBookRef {
	var out []addressBookRef
	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}
		for _, col := range acc.Collections {
			if col.Kind != discovery.KindAddressBook {
				continue
			}
			out = append(out, addressBookRef{
				AccountID:   acc.ID,
				AccountLabel: accountLabel(acc),
				Collection:  col,
				ColEnc:      EncodeCollectionPath(col.Path),
			})
		}
	}
	return out
}

type addressBookRef struct {
	AccountID    string
	AccountLabel string
	Collection   discovery.Collection
	ColEnc       string
}

func accountLabel(acc account.Account) string {
	if acc.Label != "" {
		return acc.Label
	}
	return acc.Username
}

func normalizeCollectionPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func objectPathForUID(collection, uid string) string {
	collection = normalizeCollectionPath(collection)
	return collection + url.PathEscape(uid) + ".vcf"
}

func uidFromObjectPath(path string) string {
	base := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		base = path[i+1:]
	}
	base = strings.TrimSuffix(base, ".vcf")
	if dec, err := url.PathUnescape(base); err == nil {
		return dec
	}
	return base
}

func conflictKey(accountID, collection, uid string) string {
	return accountID + "|" + normalizeCollectionPath(collection) + "|" + uid
}

func photoDraftKey(accountID, collection, uid string) string {
	return conflictKey(accountID, collection, uid)
}

var errBadCollection = errors.New("bad collection")
