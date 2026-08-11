// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

// An invite token travels in a URL path segment, so it may not need escaping,
// and it must carry the full TokenLen of entropy (§5.3).
func TestNewToken(t *testing.T) {
	token, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("token is not unpadded base64url: %v", err)
	}
	if len(raw) != TokenLen {
		t.Errorf("token carries %d bytes of entropy, want %d", len(raw), TokenLen)
	}
	if strings.ContainsAny(token, "+/=") {
		t.Errorf("token %q contains characters that would need escaping in a path", token)
	}
	if bytes.Equal(raw, make([]byte, TokenLen)) {
		t.Error("token is all zeroes")
	}
}

func TestNewTokenIsUnpredictable(t *testing.T) {
	const n = 64
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		token, err := NewToken()
		if err != nil {
			t.Fatalf("NewToken: %v", err)
		}
		if seen[token] {
			t.Fatalf("NewToken repeated %q within %d draws", token, n)
		}
		seen[token] = true
	}
}

func TestHashToken(t *testing.T) {
	token, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}

	digest := HashToken(token)
	if len(digest) != sha256.Size {
		t.Fatalf("digest is %d bytes, want %d", len(digest), sha256.Size)
	}
	if !Equal(digest, HashToken(token)) {
		t.Error("HashToken is not deterministic")
	}
	// The digest is what the store keeps, so it must not be reversible to the
	// token by anyone reading the volume.
	if strings.Contains(string(digest), token) {
		t.Error("the digest contains the token")
	}

	// A token that differs anywhere hashes elsewhere: this is what makes the
	// stored digest a usable lookup key.
	other, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if Equal(digest, HashToken(other)) {
		t.Error("two different tokens share a digest")
	}
	flipped := "A" + token[1:]
	if flipped != token && Equal(digest, HashToken(flipped)) {
		t.Error("changing the first character left the digest unchanged")
	}
	if Equal(digest, HashToken("")) {
		t.Error("the empty token hashes to a real token's digest")
	}
}

// Digests are compared with Equal, never with bytes.Equal or ==, so that a
// near-miss token cannot be walked towards by timing the comparison (§24.3).
func TestEqualRejectsNearMissDigests(t *testing.T) {
	token, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	digest := HashToken(token)

	near := append([]byte(nil), digest...)
	near[len(near)-1] ^= 0x01
	if Equal(digest, near) {
		t.Error("digests differing in one bit compared equal")
	}
	if Equal(digest, digest[:len(digest)-1]) {
		t.Error("a truncated digest compared equal")
	}
	if Equal(digest, nil) {
		t.Error("a nil digest compared equal to a real one")
	}
}
