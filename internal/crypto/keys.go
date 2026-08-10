// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"runtime"
)

const (
	// KeyLen is the length of every symmetric key Carrel uses (AES-256).
	KeyLen = 32
	// SaltLen is the length of Argon2id salts.
	SaltLen = 16
	// NonceLen is the AES-GCM nonce length.
	NonceLen = 12
	// TokenLen is the length of invite tokens and similar bearer secrets (§5.3).
	TokenLen = 32
)

var (
	// ErrDecrypt is returned for every authentication or decryption failure.
	// It is deliberately uniform: callers must not be able to tell a wrong key
	// from a corrupt ciphertext.
	ErrDecrypt = errors.New("crypto: decryption failed")
	// ErrKeySize is returned when key material has the wrong length.
	ErrKeySize = errors.New("crypto: invalid key size")
)

// Key is symmetric key material. Wipe it with Zero when it is no longer needed.
// A Key aliases its backing array, so zeroing one copy zeroes them all.
type Key []byte

// Zero overwrites the key material in place.
func (k Key) Zero() { Zero(k) }

// Clone returns an independent copy of the key.
func (k Key) Clone() Key {
	if k == nil {
		return nil
	}
	out := make(Key, len(k))
	copy(out, k)
	return out
}

// Zero overwrites b in place. Go gives no hard guarantee that the write
// survives the optimiser, but in practice it does; the KeepAlive keeps the
// backing array from being collected before the clear is issued (§24.6).
func Zero(b []byte) {
	if len(b) == 0 {
		return
	}
	clear(b)
	runtime.KeepAlive(b)
}

// Random returns n cryptographically random bytes.
func Random(n int) ([]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("crypto: random length must be positive, got %d", n)
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("crypto: read random: %w", err)
	}
	return b, nil
}

// NewSalt returns a fresh Argon2id salt.
func NewSalt() ([]byte, error) { return Random(SaltLen) }

// NewDEK returns a fresh data encryption key.
func NewDEK() (Key, error) {
	b, err := Random(KeyLen)
	if err != nil {
		return nil, err
	}
	return Key(b), nil
}
