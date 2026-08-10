// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"crypto/sha256"
	"encoding/base64"
)

// NewToken returns a bearer secret for invite links and similar one-time URLs:
// TokenLen random bytes in unpadded base64url, safe to place in a path segment
// (§5.3).
func NewToken() (string, error) {
	b, err := Random(TokenLen)
	if err != nil {
		return "", err
	}
	defer Zero(b)
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns the digest kept in place of a bearer token. A token from
// NewToken already carries full entropy, so a plain SHA-256 is enough: there is
// nothing to brute-force, and stretching would only slow down lookups. Compare
// digests with Equal, never with ==.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
