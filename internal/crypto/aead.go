// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

// Additional authenticated data, one constant per use of a key. Domain
// separation keeps a ciphertext from one context from being replayed into
// another.
var (
	aadDEK     = []byte("carrel:dek:v1")
	aadState   = []byte("carrel:state:v1")
	aadEscrowP = []byte("carrel:escrow-private:v1")
	aadEscrowD = []byte("carrel:escrow-dek:v1")
)

func newAEAD(key Key) (cipher.AEAD, error) {
	if len(key) != KeyLen {
		return nil, ErrKeySize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	return aead, nil
}

// Seal encrypts plaintext with AES-256-GCM under key and returns
// nonce||ciphertext. aad is authenticated but not encrypted.
func Seal(key Key, plaintext, aad []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, NonceLen, NonceLen+len(plaintext)+aead.Overhead())
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("crypto: read random: %w", err)
	}
	return aead.Seal(buf, buf[:NonceLen], plaintext, aad), nil
}

// Open reverses Seal. Every failure returns ErrDecrypt.
func Open(key Key, ciphertext, aad []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < NonceLen+aead.Overhead() {
		return nil, ErrDecrypt
	}
	plaintext, err := aead.Open(nil, ciphertext[:NonceLen], ciphertext[NonceLen:], aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}

// WrapDEK encrypts a DEK under a KEK. The result is stored next to the user
// record; changing a password re-wraps only this blob (§4).
func WrapDEK(kek, dek Key) ([]byte, error) {
	if len(dek) != KeyLen {
		return nil, ErrKeySize
	}
	return Seal(kek, dek, aadDEK)
}

// UnwrapDEK recovers a DEK wrapped by WrapDEK. A wrong password surfaces here
// as ErrDecrypt.
func UnwrapDEK(kek Key, wrapped []byte) (Key, error) {
	plaintext, err := Open(kek, wrapped, aadDEK)
	if err != nil {
		return nil, err
	}
	if len(plaintext) != KeyLen {
		Zero(plaintext)
		return nil, ErrDecrypt
	}
	return Key(plaintext), nil
}

// SealState encrypts service data (users, roles, SMTP settings) under the
// server key. It has to be readable before anyone logs in, so it cannot be
// protected by a user's DEK (§4).
func SealState(serverKey Key, plaintext []byte) ([]byte, error) {
	return Seal(serverKey, plaintext, aadState)
}

// OpenState reverses SealState.
func OpenState(serverKey Key, ciphertext []byte) ([]byte, error) {
	return Open(serverKey, ciphertext, aadState)
}
