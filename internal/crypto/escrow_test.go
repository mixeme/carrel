// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const (
	masterPassword    = "master password for escrow"
	newMasterPassword = "another sufficiently long master password"
)

func newTestEscrow(t *testing.T) *Escrow {
	t.Helper()
	e, err := NewEscrowWithParams(masterPassword, testParams())
	if err != nil {
		t.Fatalf("NewEscrowWithParams: %v", err)
	}
	return e
}

func TestEscrowRoundTrip(t *testing.T) {
	e := newTestEscrow(t)
	dek := mustDEK(t)

	sealed, err := e.SealDEK(dek)
	if err != nil {
		t.Fatalf("SealDEK: %v", err)
	}
	if bytes.Contains(sealed, dek) {
		t.Fatal("escrow copy contains the plaintext DEK")
	}

	recovered, err := e.RecoverDEK(masterPassword, sealed)
	if err != nil {
		t.Fatalf("RecoverDEK: %v", err)
	}
	if !bytes.Equal(recovered, dek) {
		t.Fatal("RecoverDEK returned a different DEK")
	}

	// Depositing the same DEK twice must not produce the same blob: the key
	// agreement is ephemeral per call.
	other, err := e.SealDEK(dek)
	if err != nil {
		t.Fatalf("SealDEK: %v", err)
	}
	if bytes.Equal(sealed, other) {
		t.Fatal("two SealDEK calls produced identical output")
	}
}

// Depositing a DEK needs no secret at all: it happens at user creation, while
// the DEK is in memory and the master password is nowhere near the process.
func TestEscrowSealNeedsNoSecret(t *testing.T) {
	e := newTestEscrow(t)
	pubOnly := &Escrow{PublicKey: append([]byte(nil), e.PublicKey...)}

	dek := mustDEK(t)
	sealed, err := pubOnly.SealDEK(dek)
	if err != nil {
		t.Fatalf("SealDEK with public key only: %v", err)
	}
	recovered, err := e.RecoverDEK(masterPassword, sealed)
	if err != nil {
		t.Fatalf("RecoverDEK: %v", err)
	}
	if !bytes.Equal(recovered, dek) {
		t.Fatal("RecoverDEK returned a different DEK")
	}
}

func TestEscrowWrongMasterPassword(t *testing.T) {
	e := newTestEscrow(t)
	sealed, err := e.SealDEK(mustDEK(t))
	if err != nil {
		t.Fatalf("SealDEK: %v", err)
	}

	for _, wrong := range []string{"", "master password for escro", strings.ToUpper(masterPassword), newMasterPassword} {
		if _, err := e.RecoverDEK(wrong, sealed); !errors.Is(err, ErrWrongMasterPassword) {
			t.Errorf("RecoverDEK(%q) returned %v, want ErrWrongMasterPassword", wrong, err)
		}
		if e.CheckMasterPassword(wrong) {
			t.Errorf("CheckMasterPassword accepted %q", wrong)
		}
	}
	if !e.CheckMasterPassword(masterPassword) {
		t.Error("CheckMasterPassword rejected the correct master password")
	}
}

func TestEscrowTamperedInput(t *testing.T) {
	e := newTestEscrow(t)
	sealed, err := e.SealDEK(mustDEK(t))
	if err != nil {
		t.Fatalf("SealDEK: %v", err)
	}

	flippedTag := append([]byte(nil), sealed...)
	flippedTag[len(flippedTag)-1] ^= 0x01
	flippedPub := append([]byte(nil), sealed...)
	flippedPub[0] ^= 0x01

	cases := map[string][]byte{
		"tampered tag":            flippedTag,
		"replaced ephemeral key":  flippedPub,
		"truncated":               sealed[:len(sealed)-1],
		"public key only":         sealed[:32],
		"shorter than public key": sealed[:16],
		"empty":                   nil,
	}
	for name, blob := range cases {
		if _, err := e.RecoverDEK(masterPassword, blob); !errors.Is(err, ErrDecrypt) {
			t.Errorf("%s: RecoverDEK returned %v, want ErrDecrypt", name, err)
		}
	}

	// A tampered sealed private key must not decrypt either.
	broken := *e
	broken.SealedKey = append([]byte(nil), e.SealedKey...)
	broken.SealedKey[len(broken.SealedKey)-1] ^= 0x01
	if _, err := broken.RecoverDEK(masterPassword, sealed); !errors.Is(err, ErrWrongMasterPassword) {
		t.Errorf("RecoverDEK with a tampered private key returned %v, want ErrWrongMasterPassword", err)
	}
}

// A copy deposited under one escrow must be useless to another (§5.4: enabling
// escrow again does not reach back to older users' keys).
func TestEscrowKeysAreIndependent(t *testing.T) {
	a, b := newTestEscrow(t), newTestEscrow(t)
	if bytes.Equal(a.PublicKey, b.PublicKey) {
		t.Fatal("two escrows share a public key")
	}
	sealed, err := a.SealDEK(mustDEK(t))
	if err != nil {
		t.Fatalf("SealDEK: %v", err)
	}
	if _, err := b.RecoverDEK(masterPassword, sealed); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("another escrow recovered the DEK: %v", err)
	}
}

// Changing the master password re-encrypts the private key only; deposited DEK
// copies are encrypted to the public key and must keep working (§5.4).
func TestEscrowChangeMasterPassword(t *testing.T) {
	e := newTestEscrow(t)
	dek := mustDEK(t)
	sealed, err := e.SealDEK(dek)
	if err != nil {
		t.Fatalf("SealDEK: %v", err)
	}
	publicBefore := append([]byte(nil), e.PublicKey...)

	if err := e.ChangeMasterPassword(masterPassword, newMasterPassword); err != nil {
		t.Fatalf("ChangeMasterPassword: %v", err)
	}
	if !bytes.Equal(publicBefore, e.PublicKey) {
		t.Fatal("ChangeMasterPassword rotated the public key")
	}

	recovered, err := e.RecoverDEK(newMasterPassword, sealed)
	if err != nil {
		t.Fatalf("RecoverDEK after master password change: %v", err)
	}
	if !bytes.Equal(recovered, dek) {
		t.Fatal("RecoverDEK returned a different DEK after master password change")
	}
	if _, err := e.RecoverDEK(masterPassword, sealed); !errors.Is(err, ErrWrongMasterPassword) {
		t.Fatalf("the old master password still works: %v", err)
	}
}

func TestEscrowChangeMasterPasswordRejections(t *testing.T) {
	e := newTestEscrow(t)
	if err := e.ChangeMasterPassword("wrong password entirely", newMasterPassword); !errors.Is(err, ErrWrongMasterPassword) {
		t.Errorf("ChangeMasterPassword with a wrong old password returned %v", err)
	}
	if err := e.ChangeMasterPassword(masterPassword, "short"); !errors.Is(err, ErrMasterPasswordTooShort) {
		t.Errorf("ChangeMasterPassword accepted a short new password: %v", err)
	}
	if !e.CheckMasterPassword(masterPassword) {
		t.Error("a rejected change altered the stored key")
	}
}

func TestEscrowMasterPasswordLength(t *testing.T) {
	short := strings.Repeat("a", MinMasterPasswordLength-1)
	if _, err := NewEscrowWithParams(short, testParams()); !errors.Is(err, ErrMasterPasswordTooShort) {
		t.Errorf("NewEscrowWithParams accepted a %d-character password: %v", len(short), err)
	}
	exact := strings.Repeat("a", MinMasterPasswordLength)
	if _, err := NewEscrowWithParams(exact, testParams()); err != nil {
		t.Errorf("NewEscrowWithParams rejected a %d-character password: %v", len(exact), err)
	}
}

func TestEscrowNotConfigured(t *testing.T) {
	var nilEscrow *Escrow
	if _, err := nilEscrow.SealDEK(mustDEK(t)); err == nil {
		t.Error("SealDEK on a nil escrow did not fail")
	}
	if _, err := nilEscrow.RecoverDEK(masterPassword, nil); err == nil {
		t.Error("RecoverDEK on a nil escrow did not fail")
	}
	if nilEscrow.CheckMasterPassword(masterPassword) {
		t.Error("CheckMasterPassword on a nil escrow returned true")
	}
	if _, err := (&Escrow{}).SealDEK(mustDEK(t)); err == nil {
		t.Error("SealDEK on an empty escrow did not fail")
	}
}

func TestEscrowRejectsShortDEK(t *testing.T) {
	e := newTestEscrow(t)
	if _, err := e.SealDEK(Key("short")); !errors.Is(err, ErrKeySize) {
		t.Errorf("SealDEK accepted a short DEK: %v", err)
	}
}

// NewEscrow must use the strengthened profile without being told to.
func TestNewEscrowUsesMasterParams(t *testing.T) {
	if testing.Short() {
		t.Skip("Argon2id with the master profile is slow")
	}
	e, err := NewEscrow(masterPassword)
	if err != nil {
		t.Fatalf("NewEscrow: %v", err)
	}
	if e.MasterParams != MasterParams() {
		t.Fatalf("NewEscrow used %+v, want %+v", e.MasterParams, MasterParams())
	}
	dek := mustDEK(t)
	sealed, err := e.SealDEK(dek)
	if err != nil {
		t.Fatalf("SealDEK: %v", err)
	}
	recovered, err := e.RecoverDEK(masterPassword, sealed)
	if err != nil {
		t.Fatalf("RecoverDEK: %v", err)
	}
	if !bytes.Equal(recovered, dek) {
		t.Fatal("RecoverDEK returned a different DEK")
	}
}
