// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

// ErrWrongMasterPassword is returned when the escrow master password does not
// unwrap the recovery private key.
var ErrWrongMasterPassword = errors.New("crypto: wrong escrow master password")

// escrowHKDFInfo separates the escrow key agreement from any other use of
// X25519 in the service.
const escrowHKDFInfo = "carrel:escrow:x25519:v1"

// Escrow is the optional key deposit scheme (§5.4). The public key is stored
// in the clear and encrypts a copy of each participating user's DEK; the
// private key is sealed under the administrator's master password, which is
// distinct from their login password and is entered only during a recovery.
//
// Escrow is off by default and applies only to users created after it was
// enabled: an existing DEK cannot be deposited without its owner's password.
type Escrow struct {
	PublicKey    []byte `json:"public_key"`
	SealedKey    []byte `json:"sealed_private_key"`
	MasterSalt   []byte `json:"master_salt"`
	MasterParams Params `json:"master_params"`
}

// NewEscrow generates a recovery key pair and seals the private key under
// masterPassword using the strengthened Argon2id profile.
func NewEscrow(masterPassword string) (*Escrow, error) {
	return NewEscrowWithParams(masterPassword, MasterParams())
}

// NewEscrowWithParams is NewEscrow with explicit Argon2id costs. Production
// code should call NewEscrow; this exists for tests and for future cost
// migrations.
func NewEscrowWithParams(masterPassword string, p Params) (*Escrow, error) {
	if len([]rune(masterPassword)) < MinMasterPasswordLength {
		return nil, ErrMasterPasswordTooShort
	}
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("crypto: generate escrow key: %w", err)
	}
	salt, err := NewSalt()
	if err != nil {
		return nil, err
	}
	sealed, err := sealPrivate(priv, masterPassword, salt, p)
	if err != nil {
		return nil, err
	}
	return &Escrow{
		PublicKey:    priv.PublicKey().Bytes(),
		SealedKey:    sealed,
		MasterSalt:   salt,
		MasterParams: p,
	}, nil
}

func sealPrivate(priv *ecdh.PrivateKey, masterPassword string, salt []byte, p Params) ([]byte, error) {
	kek, err := DeriveKey(masterPassword, salt, p)
	if err != nil {
		return nil, err
	}
	defer kek.Zero()

	raw := priv.Bytes()
	defer Zero(raw)

	return Seal(kek, raw, aadEscrowP)
}

// unwrapPrivate recovers the recovery private key. The master password is
// never cached: it is derived, used and wiped within this call (§5.4).
func (e *Escrow) unwrapPrivate(masterPassword string) (*ecdh.PrivateKey, error) {
	if e == nil || len(e.SealedKey) == 0 || len(e.MasterSalt) == 0 {
		return nil, errors.New("crypto: escrow is not configured")
	}
	kek, err := DeriveKey(masterPassword, e.MasterSalt, e.MasterParams)
	if err != nil {
		return nil, err
	}
	defer kek.Zero()

	raw, err := Open(kek, e.SealedKey, aadEscrowP)
	if err != nil {
		return nil, ErrWrongMasterPassword
	}
	defer Zero(raw)

	priv, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return nil, ErrWrongMasterPassword
	}
	return priv, nil
}

// CheckMasterPassword reports whether masterPassword unwraps the private key.
func (e *Escrow) CheckMasterPassword(masterPassword string) bool {
	_, err := e.unwrapPrivate(masterPassword)
	return err == nil
}

// ChangeMasterPassword re-seals the private key under a new master password.
// Deposited DEK copies are encrypted to the public key and are not touched
// (§5.4).
func (e *Escrow) ChangeMasterPassword(oldPassword, newPassword string) error {
	if len([]rune(newPassword)) < MinMasterPasswordLength {
		return ErrMasterPasswordTooShort
	}
	priv, err := e.unwrapPrivate(oldPassword)
	if err != nil {
		return err
	}
	salt, err := NewSalt()
	if err != nil {
		return err
	}
	sealed, err := sealPrivate(priv, newPassword, salt, e.MasterParams)
	if err != nil {
		return err
	}
	e.SealedKey = sealed
	e.MasterSalt = salt
	return nil
}

// SealDEK encrypts a copy of dek to the recovery public key. It needs no
// secret, so it can run at user creation time while the DEK is in memory.
func (e *Escrow) SealDEK(dek Key) ([]byte, error) {
	if e == nil || len(e.PublicKey) == 0 {
		return nil, errors.New("crypto: escrow is not configured")
	}
	if len(dek) != KeyLen {
		return nil, ErrKeySize
	}
	pub, err := ecdh.X25519().NewPublicKey(e.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: escrow public key: %w", err)
	}
	return sealToPublic(pub, dek)
}

// RecoverDEK decrypts a deposited DEK copy. Callers must record the recovery
// in the audit log and notify the user by email (§5.4).
func (e *Escrow) RecoverDEK(masterPassword string, sealedDEK []byte) (Key, error) {
	priv, err := e.unwrapPrivate(masterPassword)
	if err != nil {
		return nil, err
	}
	plaintext, err := openWithPrivate(priv, sealedDEK)
	if err != nil {
		return nil, err
	}
	if len(plaintext) != KeyLen {
		Zero(plaintext)
		return nil, ErrDecrypt
	}
	return Key(plaintext), nil
}

// sealToPublic is X25519 + HKDF-SHA256 + AES-256-GCM: an ephemeral key
// agreement with the recipient's public key. The output is
// ephemeral_public||nonce||ciphertext.
func sealToPublic(pub *ecdh.PublicKey, plaintext []byte) ([]byte, error) {
	eph, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("crypto: generate ephemeral key: %w", err)
	}
	ephPub := eph.PublicKey().Bytes()

	key, err := agree(eph, pub, ephPub)
	if err != nil {
		return nil, err
	}
	defer key.Zero()

	ciphertext, err := Seal(key, plaintext, escrowAAD(ephPub))
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(ephPub)+len(ciphertext))
	out = append(out, ephPub...)
	out = append(out, ciphertext...)
	return out, nil
}

func openWithPrivate(priv *ecdh.PrivateKey, blob []byte) ([]byte, error) {
	const pubLen = 32
	if len(blob) < pubLen {
		return nil, ErrDecrypt
	}
	ephPub := blob[:pubLen]
	pub, err := ecdh.X25519().NewPublicKey(ephPub)
	if err != nil {
		return nil, ErrDecrypt
	}

	key, err := agree(priv, pub, ephPub)
	if err != nil {
		return nil, ErrDecrypt
	}
	defer key.Zero()

	return Open(key, blob[pubLen:], escrowAAD(ephPub))
}

// agree derives the content key from an X25519 shared secret. The ephemeral
// public key goes into the derivation on both sides.
func agree(priv *ecdh.PrivateKey, pub *ecdh.PublicKey, ephPub []byte) (Key, error) {
	shared, err := priv.ECDH(pub)
	if err != nil {
		return nil, fmt.Errorf("crypto: escrow key agreement: %w", err)
	}
	defer Zero(shared)

	key, err := hkdf.Key(sha256.New, shared, ephPub, escrowHKDFInfo, KeyLen)
	if err != nil {
		return nil, fmt.Errorf("crypto: escrow hkdf: %w", err)
	}
	return Key(key), nil
}

func escrowAAD(ephPub []byte) []byte {
	aad := make([]byte, 0, len(aadEscrowD)+len(ephPub))
	aad = append(aad, aadEscrowD...)
	aad = append(aad, ephPub...)
	return aad
}
