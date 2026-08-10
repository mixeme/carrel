// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"bytes"
	"errors"
	"testing"
)

// testParams keep the tests fast. Cost profiles themselves are checked in
// TestProfiles and exercised once in TestHashAuthRealProfile.
func testParams() Params {
	return Params{Time: 1, Memory: 8 * 1024, Threads: 1, KeyLen: KeyLen}
}

func mustSalt(t *testing.T) []byte {
	t.Helper()
	salt, err := NewSalt()
	if err != nil {
		t.Fatalf("NewSalt: %v", err)
	}
	return salt
}

func mustDEK(t *testing.T) Key {
	t.Helper()
	dek, err := NewDEK()
	if err != nil {
		t.Fatalf("NewDEK: %v", err)
	}
	return dek
}

func mustDerive(t *testing.T, password string, salt []byte) Key {
	t.Helper()
	key, err := DeriveKey(password, salt, testParams())
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	return key
}

func TestProfiles(t *testing.T) {
	for name, p := range map[string]Params{
		"auth":   AuthParams(),
		"kek":    KEKParams(),
		"master": MasterParams(),
	} {
		if err := p.Validate(); err != nil {
			t.Errorf("%s profile invalid: %v", name, err)
		}
		if p.KeyLen != KeyLen {
			t.Errorf("%s profile derives %d bytes, want %d", name, p.KeyLen, KeyLen)
		}
	}

	// The master password guards another person's data and is entered rarely;
	// it must cost strictly more than an interactive login (§5.4).
	auth, master := AuthParams(), MasterParams()
	if master.Memory <= auth.Memory || master.Time <= auth.Time {
		t.Errorf("master profile %+v is not stronger than auth profile %+v", master, auth)
	}
	if MinMasterPasswordLength <= MinPasswordLength {
		t.Errorf("master password minimum %d must exceed login minimum %d", MinMasterPasswordLength, MinPasswordLength)
	}
}

func TestParamsValidate(t *testing.T) {
	valid := testParams()
	cases := map[string]Params{
		"zero time":    {Time: 0, Memory: 8 * 1024, Threads: 1, KeyLen: KeyLen},
		"zero threads": {Time: 1, Memory: 8 * 1024, Threads: 0, KeyLen: KeyLen},
		"low memory":   {Time: 1, Memory: 4, Threads: 4, KeyLen: KeyLen},
		"short key":    {Time: 1, Memory: 8 * 1024, Threads: 1, KeyLen: 8},
	}
	for name, p := range cases {
		if err := p.Validate(); err == nil {
			t.Errorf("%s: Validate accepted %+v", name, p)
		}
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate rejected valid params: %v", err)
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	salt := mustSalt(t)
	a := mustDerive(t, "correct horse battery staple", salt)
	b := mustDerive(t, "correct horse battery staple", salt)
	if !bytes.Equal(a, b) {
		t.Fatal("same password and salt produced different keys")
	}
	if len(a) != KeyLen {
		t.Fatalf("derived %d bytes, want %d", len(a), KeyLen)
	}
}

// The login password is used for two different purposes; mixing them would
// mean the stored auth hash is the KEK (§4).
func TestSaltSeparation(t *testing.T) {
	const password = "correct horse battery staple"
	saltAuth, saltKEK := mustSalt(t), mustSalt(t)

	auth := mustDerive(t, password, saltAuth)
	kek := mustDerive(t, password, saltKEK)
	if bytes.Equal(auth, kek) {
		t.Fatal("auth hash and KEK collide despite different salts")
	}

	other := mustDerive(t, "different password", saltAuth)
	if bytes.Equal(auth, other) {
		t.Fatal("different passwords produced the same key")
	}
}

func TestDeriveKeyRejectsShortSalt(t *testing.T) {
	if _, err := DeriveKey("password", []byte("short"), testParams()); err == nil {
		t.Fatal("DeriveKey accepted a 5-byte salt")
	}
}

func TestPasswordHashVerify(t *testing.T) {
	const password = "correct horse battery staple"
	h, err := HashPassword(password, testParams())
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if bytes.Contains(h.Hash, []byte(password)) {
		t.Fatal("digest contains the password")
	}
	if !h.Verify(password) {
		t.Fatal("Verify rejected the correct password")
	}
	for _, wrong := range []string{"", "Correct horse battery staple", "correct horse battery stapl", password + " "} {
		if h.Verify(wrong) {
			t.Errorf("Verify accepted wrong password %q", wrong)
		}
	}

	tampered := *h
	tampered.Hash = append([]byte(nil), h.Hash...)
	tampered.Hash[0] ^= 0x01
	if tampered.Verify(password) {
		t.Error("Verify accepted a tampered digest")
	}

	reSalted := *h
	reSalted.Salt = mustSalt(t)
	if reSalted.Verify(password) {
		t.Error("Verify accepted a replaced salt")
	}

	var empty *PasswordHash
	if empty.Verify(password) {
		t.Error("Verify accepted a nil hash")
	}
	if (&PasswordHash{Params: testParams()}).Verify(password) {
		t.Error("Verify accepted an empty hash")
	}
}

func TestHashAuthRealProfile(t *testing.T) {
	h, err := HashAuth("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashAuth: %v", err)
	}
	if h.Params != AuthParams() {
		t.Fatalf("HashAuth used %+v, want %+v", h.Params, AuthParams())
	}
	if !VerifyAuth("correct horse battery staple", h.Salt, h.Hash) {
		t.Fatal("VerifyAuth rejected the correct password")
	}
	if VerifyAuth("wrong password", h.Salt, h.Hash) {
		t.Fatal("VerifyAuth accepted a wrong password")
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	key := mustDEK(t)
	aad := []byte("carrel:test")
	plaintext := []byte("dav password")

	ciphertext, err := Seal(key, plaintext, aad)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatal("ciphertext contains the plaintext")
	}

	got, err := Open(key, ciphertext, aad)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("Open returned %q, want %q", got, plaintext)
	}

	// Same plaintext twice must not produce the same ciphertext: the nonce is
	// fresh per call.
	again, err := Seal(key, plaintext, aad)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Equal(ciphertext, again) {
		t.Fatal("two Seal calls produced identical ciphertext")
	}
}

func TestOpenFailures(t *testing.T) {
	key := mustDEK(t)
	other := mustDEK(t)
	aad := []byte("carrel:test")
	ciphertext, err := Seal(key, []byte("dav password"), aad)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 0x01
	nonceFlipped := append([]byte(nil), ciphertext...)
	nonceFlipped[0] ^= 0x01

	cases := map[string]struct {
		key        Key
		ciphertext []byte
		aad        []byte
	}{
		"wrong key":         {other, ciphertext, aad},
		"tampered tag":      {key, tampered, aad},
		"tampered nonce":    {key, nonceFlipped, aad},
		"wrong aad":         {key, ciphertext, []byte("carrel:other")},
		"truncated":         {key, ciphertext[:NonceLen+1], aad},
		"empty":             {key, nil, aad},
		"nonce length only": {key, ciphertext[:NonceLen], aad},
	}
	for name, c := range cases {
		got, err := Open(c.key, c.ciphertext, c.aad)
		if !errors.Is(err, ErrDecrypt) {
			t.Errorf("%s: Open returned err %v, want ErrDecrypt", name, err)
		}
		if got != nil {
			t.Errorf("%s: Open returned plaintext %q on failure", name, got)
		}
	}

	if _, err := Seal(Key("too short"), []byte("x"), aad); !errors.Is(err, ErrKeySize) {
		t.Errorf("Seal with short key returned %v, want ErrKeySize", err)
	}
	if _, err := Open(Key("too short"), ciphertext, aad); !errors.Is(err, ErrKeySize) {
		t.Errorf("Open with short key returned %v, want ErrKeySize", err)
	}
}

// Domain separation: a wrapped DEK must not open as state, and vice versa.
func TestAADDomainSeparation(t *testing.T) {
	key := mustDEK(t)
	dek := mustDEK(t)

	wrapped, err := WrapDEK(key, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}
	if _, err := OpenState(key, wrapped); !errors.Is(err, ErrDecrypt) {
		t.Errorf("OpenState accepted a wrapped DEK: %v", err)
	}

	state, err := SealState(key, []byte(`{"users":[]}`))
	if err != nil {
		t.Fatalf("SealState: %v", err)
	}
	if _, err := UnwrapDEK(key, state); !errors.Is(err, ErrDecrypt) {
		t.Errorf("UnwrapDEK accepted sealed state: %v", err)
	}
}

func TestWrapUnwrapDEK(t *testing.T) {
	dek := mustDEK(t)
	salt := mustSalt(t)
	kek := mustDerive(t, "correct horse battery staple", salt)

	wrapped, err := WrapDEK(kek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}
	if bytes.Contains(wrapped, dek) {
		t.Fatal("wrapped DEK contains the plaintext DEK")
	}

	got, err := UnwrapDEK(kek, wrapped)
	if err != nil {
		t.Fatalf("UnwrapDEK: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("UnwrapDEK returned a different DEK")
	}

	wrongKEK := mustDerive(t, "wrong password", salt)
	if _, err := UnwrapDEK(wrongKEK, wrapped); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("UnwrapDEK with wrong password returned %v, want ErrDecrypt", err)
	}

	// A KEK derived from the right password but the auth salt must fail too.
	wrongSalt := mustDerive(t, "correct horse battery staple", mustSalt(t))
	if _, err := UnwrapDEK(wrongSalt, wrapped); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("UnwrapDEK with the wrong salt returned %v, want ErrDecrypt", err)
	}

	if _, err := WrapDEK(kek, Key("short")); !errors.Is(err, ErrKeySize) {
		t.Fatalf("WrapDEK accepted a short DEK: %v", err)
	}
}

// A DEK of the wrong length must be rejected rather than handed to AES.
func TestUnwrapDEKRejectsWrongLength(t *testing.T) {
	kek := mustDerive(t, "correct horse battery staple", mustSalt(t))
	wrapped, err := Seal(kek, []byte("sixteen bytes.xx"), aadDEK)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := UnwrapDEK(kek, wrapped); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("UnwrapDEK accepted a 16-byte payload: %v", err)
	}
}

// Changing a password re-wraps the DEK and nothing else: data encrypted under
// the DEK stays readable (§4, §5.5).
func TestPasswordChangeRewrapsDEKOnly(t *testing.T) {
	const oldPassword = "correct horse battery staple"
	const newPassword = "a whole new passphrase"

	dek := mustDEK(t)
	secret := []byte("https://dav.example/ + login + password")
	encrypted, err := Seal(dek, secret, []byte("carrel:account"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	oldSalt := mustSalt(t)
	oldKEK := mustDerive(t, oldPassword, oldSalt)
	wrapped, err := WrapDEK(oldKEK, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	// Password change: unwrap with the old KEK, re-wrap under a new salt.
	unwrapped, err := UnwrapDEK(oldKEK, wrapped)
	if err != nil {
		t.Fatalf("UnwrapDEK: %v", err)
	}
	newSalt := mustSalt(t)
	newKEK := mustDerive(t, newPassword, newSalt)
	rewrapped, err := WrapDEK(newKEK, unwrapped)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	final, err := UnwrapDEK(newKEK, rewrapped)
	if err != nil {
		t.Fatalf("UnwrapDEK after password change: %v", err)
	}
	if !bytes.Equal(final, dek) {
		t.Fatal("password change produced a different DEK")
	}

	// The account ciphertext was never touched and still opens.
	got, err := Open(final, encrypted, []byte("carrel:account"))
	if err != nil {
		t.Fatalf("account ciphertext unreadable after password change: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatal("account ciphertext decrypted to the wrong plaintext")
	}

	// The old wrapping must no longer be usable with the new password.
	if _, err := UnwrapDEK(newKEK, wrapped); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("old wrapped DEK still opens with the new KEK: %v", err)
	}
}

func TestZero(t *testing.T) {
	key := mustDEK(t)
	alias := key
	key.Zero()
	for i, b := range alias {
		if b != 0 {
			t.Fatalf("byte %d not wiped: %#x", i, b)
		}
	}

	// Zero must tolerate nil and empty input.
	Zero(nil)
	Zero([]byte{})
	Key(nil).Zero()
}

func TestClone(t *testing.T) {
	key := mustDEK(t)
	clone := key.Clone()
	if !bytes.Equal(key, clone) {
		t.Fatal("Clone returned different bytes")
	}
	clone.Zero()
	if bytes.Equal(key, clone) {
		t.Fatal("zeroing the clone also wiped the original")
	}
	if Key(nil).Clone() != nil {
		t.Fatal("Clone of nil is not nil")
	}
}

func TestRandom(t *testing.T) {
	a, err := Random(TokenLen)
	if err != nil {
		t.Fatalf("Random: %v", err)
	}
	b, err := Random(TokenLen)
	if err != nil {
		t.Fatalf("Random: %v", err)
	}
	if len(a) != TokenLen {
		t.Fatalf("Random returned %d bytes, want %d", len(a), TokenLen)
	}
	if bytes.Equal(a, b) {
		t.Fatal("two Random calls returned the same bytes")
	}
	if bytes.Equal(a, make([]byte, TokenLen)) {
		t.Fatal("Random returned all zeroes")
	}
	if _, err := Random(0); err == nil {
		t.Fatal("Random(0) did not fail")
	}
}

func TestEqual(t *testing.T) {
	if !Equal([]byte("token-hash"), []byte("token-hash")) {
		t.Error("Equal rejected identical values")
	}
	if Equal([]byte("token-hash"), []byte("token-hasi")) {
		t.Error("Equal accepted different values")
	}
	if Equal([]byte("token-hash"), []byte("token")) {
		t.Error("Equal accepted values of different length")
	}
	if Equal(nil, []byte("token")) {
		t.Error("Equal accepted nil against a value")
	}
}
