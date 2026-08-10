// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"crypto/subtle"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// Minimum password lengths. The escrow master password is held to a higher bar
// than a login password: losing it makes the whole scheme useless, and it
// guards a key that can open another person's data (§5.4).
const (
	MinPasswordLength       = 10
	MinMasterPasswordLength = 16
)

// ErrMasterPasswordTooShort is returned when an escrow master password is
// below MinMasterPasswordLength.
var ErrMasterPasswordTooShort = fmt.Errorf("crypto: escrow master password must be at least %d characters", MinMasterPasswordLength)

// Params are Argon2id cost parameters. They are stored next to the salt so
// that costs can be raised later without invalidating existing records.
type Params struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory_kib"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"key_len"`
}

// AuthParams are the costs for verifying a login password.
func AuthParams() Params {
	return Params{Time: 3, Memory: 64 * 1024, Threads: 4, KeyLen: KeyLen}
}

// KEKParams are the costs for deriving a key encryption key. They match
// AuthParams; what keeps the two derivations apart is the salt, not the cost.
func KEKParams() Params {
	return Params{Time: 3, Memory: 64 * 1024, Threads: 4, KeyLen: KeyLen}
}

// MasterParams are the strengthened costs for the escrow master password (§5.4).
func MasterParams() Params {
	return Params{Time: 6, Memory: 256 * 1024, Threads: 4, KeyLen: KeyLen}
}

// Validate rejects parameters that Argon2id cannot use or that are too weak to
// be worth storing.
func (p Params) Validate() error {
	if p.Time < 1 {
		return errors.New("crypto: argon2 time must be at least 1")
	}
	if p.Threads < 1 {
		return errors.New("crypto: argon2 threads must be at least 1")
	}
	if p.Memory < 8*uint32(p.Threads) {
		return fmt.Errorf("crypto: argon2 memory must be at least %d KiB for %d threads", 8*uint32(p.Threads), p.Threads)
	}
	if p.KeyLen < 16 {
		return fmt.Errorf("crypto: argon2 key length must be at least 16, got %d", p.KeyLen)
	}
	return nil
}

// DeriveKey runs Argon2id over password and salt with the given parameters.
func DeriveKey(password string, salt []byte, p Params) (Key, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if len(salt) < 8 {
		return nil, fmt.Errorf("crypto: salt must be at least 8 bytes, got %d", len(salt))
	}
	pw := []byte(password)
	defer Zero(pw)
	return Key(argon2.IDKey(pw, salt, p.Time, p.Memory, p.Threads, p.KeyLen)), nil
}

// DeriveKEK derives the key encryption key that unwraps a user's DEK. The salt
// must be the user's KEK salt, never the auth salt (§4).
func DeriveKEK(password string, salt []byte) (Key, error) {
	return DeriveKey(password, salt, KEKParams())
}

// PasswordHash is a stored Argon2id verifier: parameters, salt and digest.
type PasswordHash struct {
	Params Params `json:"params"`
	Salt   []byte `json:"salt"`
	Hash   []byte `json:"hash"`
}

// HashPassword hashes password with a fresh salt and the given parameters.
func HashPassword(password string, p Params) (*PasswordHash, error) {
	salt, err := NewSalt()
	if err != nil {
		return nil, err
	}
	key, err := DeriveKey(password, salt, p)
	if err != nil {
		return nil, err
	}
	return &PasswordHash{Params: p, Salt: salt, Hash: key}, nil
}

// HashAuth hashes a login password with the auth profile.
func HashAuth(password string) (*PasswordHash, error) {
	return HashPassword(password, AuthParams())
}

// Verify reports whether password reproduces the stored digest. The comparison
// is constant time.
func (h *PasswordHash) Verify(password string) bool {
	if h == nil || len(h.Salt) == 0 || len(h.Hash) == 0 {
		return false
	}
	got, err := DeriveKey(password, h.Salt, h.Params)
	if err != nil {
		return false
	}
	defer got.Zero()
	return subtle.ConstantTimeCompare(got, h.Hash) == 1
}

// VerifyAuth reports whether password matches hash for salt under the auth
// profile. Use it when parameters were not stored alongside the digest;
// otherwise prefer PasswordHash.Verify.
func VerifyAuth(password string, salt, hash []byte) bool {
	h := &PasswordHash{Params: AuthParams(), Salt: salt, Hash: hash}
	return h.Verify(password)
}

// Equal reports, in constant time, whether a and b are the same. It is meant
// for digests of bearer secrets such as invite tokens (§5.3).
func Equal(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
