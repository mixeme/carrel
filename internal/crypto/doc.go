// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package crypto implements Carrel's key schedule: Argon2id password hashing,
// DEK/KEK wrapping, the server key that protects service data, and the
// optional escrow (key deposit) key pair.
//
// The login password is used twice, and the two uses must never be mixed
// (spec §4):
//
//	auth: Argon2id(password, salt_auth) -> hash, compared in constant time
//	kek:  Argon2id(password, salt_kek)  -> KEK, unwraps the user's DEK
//
// The DEK is 32 random bytes that encrypt a user's DAV credentials and
// duplicate decisions. It exists only in session memory; changing a password
// re-wraps the DEK and leaves every ciphertext encrypted under it untouched.
//
// Key material is held in [Key] values and must be wiped with [Key.Zero] once
// it is no longer needed (spec §24.6). Passwords are taken as strings because
// that is what HTTP handlers have; Go strings cannot be wiped, so callers
// should keep them short-lived and never log them.
package crypto
