// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"fmt"
	"net/mail"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// MaxLoginLength bounds a login so it stays usable in the UI and in log lines.
const MaxLoginLength = 64

// NormalizeLogin folds a login to its canonical form. Logins are compared
// case-insensitively so that two accounts cannot differ only in case.
func NormalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

// ValidateLogin accepts the normalized form of a login: lowercase letters,
// digits, and `.`, `_`, `-` inside. The character set is deliberately narrow —
// logins end up in file names, mail headers and audit lines.
func ValidateLogin(login string) error {
	if login == "" {
		return fmt.Errorf("login must not be empty")
	}
	if len(login) > MaxLoginLength {
		return fmt.Errorf("login must be at most %d characters, got %d", MaxLoginLength, len(login))
	}
	for i, r := range login {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case (r == '.' || r == '_' || r == '-') && i > 0 && i < len(login)-1:
		default:
			return fmt.Errorf("login may contain lowercase letters, digits and . _ - inside, got %q", login)
		}
	}
	return nil
}

// NormalizeEmail trims an address; an empty address is allowed, since mail is
// optional throughout (§5.3).
func NormalizeEmail(email string) string { return strings.TrimSpace(email) }

// ValidateEmail accepts an empty address or a parseable one.
func ValidateEmail(email string) error {
	if email == "" {
		return nil
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("invalid email address %q", email)
	}
	return nil
}

// ValidatePassword enforces the minimum length. Counting runes rather than
// bytes keeps a non-Latin passphrase from being judged longer than it is.
func ValidatePassword(password string) error {
	if n := len([]rune(password)); n < crypto.MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters, got %d", crypto.MinPasswordLength, n)
	}
	return nil
}
