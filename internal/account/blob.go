// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package account

import (
	"encoding/hex"
	"fmt"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
)

const blobVersion = 1

var aadSecrets = []byte("carrel:secrets:v1")

// Blob is the plaintext secrets payload stored sealed under the user's DEK.
type Blob struct {
	Version  int       `json:"version"`
	Accounts []Account `json:"accounts"`
}

// Account is one connected DAV server (§6).
type Account struct {
	ID          string                 `json:"id"`
	Label       string                 `json:"label,omitempty"`
	BaseURL     string                 `json:"base_url"`
	Username    string                 `json:"username"`
	Password    string                 `json:"password"`
	Principal   string                 `json:"principal"`
	Enabled     bool                   `json:"enabled"`
	Collections []discovery.Collection `json:"collections"`
	DiscoveredAt time.Time             `json:"discovered_at"`
}

// Open decrypts and unmarshals a secrets blob.
func Open(dek crypto.Key, sealed []byte) (*Blob, error) {
	if len(sealed) == 0 {
		return &Blob{Version: blobVersion}, nil
	}
	raw, err := crypto.Open(dek, sealed, aadSecrets)
	if err != nil {
		return nil, err
	}
	defer crypto.Zero(raw)

	var blob Blob
	if err := decodeJSON(raw, &blob); err != nil {
		return nil, err
	}
	if blob.Version == 0 {
		blob.Version = blobVersion
	}
	return &blob, nil
}

// Seal marshals and encrypts a secrets blob.
func Seal(dek crypto.Key, blob *Blob) ([]byte, error) {
	if blob == nil {
		blob = &Blob{Version: blobVersion}
	}
	if blob.Version == 0 {
		blob.Version = blobVersion
	}
	raw, err := encodeJSON(blob)
	if err != nil {
		return nil, err
	}
	defer crypto.Zero(raw)
	return crypto.Seal(dek, raw, aadSecrets)
}

// NewID returns a stable random account identifier.
func NewID() (string, error) {
	b, err := crypto.Random(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CountEnabled returns how many accounts are enabled.
func (b *Blob) CountEnabled() int {
	if b == nil {
		return 0
	}
	n := 0
	for _, a := range b.Accounts {
		if a.Enabled {
			n++
		}
	}
	return n
}

// Find returns one account by ID.
func (b *Blob) Find(id string) (*Account, error) {
	if b == nil {
		return nil, fmt.Errorf("account: not found")
	}
	for i := range b.Accounts {
		if b.Accounts[i].ID == id {
			return &b.Accounts[i], nil
		}
	}
	return nil, fmt.Errorf("account: not found")
}

// Upsert inserts or replaces an account by ID.
func (b *Blob) Upsert(acc Account) {
	if b == nil {
		return
	}
	for i := range b.Accounts {
		if b.Accounts[i].ID == acc.ID {
			b.Accounts[i] = acc
			return
		}
	}
	b.Accounts = append(b.Accounts, acc)
}

// Remove deletes an account by ID. It reports whether anything was removed.
func (b *Blob) Remove(id string) bool {
	if b == nil {
		return false
	}
	for i, a := range b.Accounts {
		if a.ID == id {
			b.Accounts = append(b.Accounts[:i], b.Accounts[i+1:]...)
			return true
		}
	}
	return false
}
