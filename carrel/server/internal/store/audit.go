package store

import (
	"errors"
	"strings"
	"unicode"
)

const (
	maxAuditEntries = 10_000
	maxAuditIdent   = 320 // a login is at most an address
)

var (
	ErrAuditSecret    = errors.New("store: audit entry must not contain secrets")
	ErrAuditEmptyWho  = errors.New("store: audit entry needs who")
	ErrAuditEmptyWhat = errors.New("store: audit entry needs what")
	ErrAuditWhat      = errors.New("store: audit what must be an event code")
	ErrAuditIdent     = errors.New("store: audit identity too long")
)

// validateAudit rejects entries that would leak secrets into the log (S-3).
//
// Who and Whom are identities the instance already stores in the clear, and
// they are not scanned for keywords: a person may legitimately be called
// "secretary" or "tokarev", and a name must never make an account
// unauditable — and so unmanageable, since a failed audit fails the whole
// Mutate. What is a programmer-chosen event code, checked for shape so prose
// (and anything a person typed) cannot arrive through it. Detail is the only
// free-form field, and it is the one scanned.
func validateAudit(e AuditEntry) error {
	if strings.TrimSpace(e.Who) == "" {
		return ErrAuditEmptyWho
	}
	if strings.TrimSpace(e.What) == "" {
		return ErrAuditEmptyWhat
	}
	if len(e.Who) > maxAuditIdent || len(e.Whom) > maxAuditIdent {
		return ErrAuditIdent
	}
	if !isEventCode(e.What) {
		return ErrAuditWhat
	}
	if containsSecret(e.Detail) {
		return ErrAuditSecret
	}
	return nil
}

// isEventCode accepts lowercase dotted codes such as "user.create" or
// "password.change": letters, digits, dot, underscore and hyphen, no spaces.
func isEventCode(s string) bool {
	if len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

func containsSecret(s string) bool {
	lower := strings.ToLower(s)
	for _, needle := range []string{
		"password", "passwd", "token", "secret", "bearer ",
		"authorization:", "wrappedkey", "loginhash",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	// Reject long hex/base64 blobs that look like ciphertext or hashes.
	if len(s) >= 64 && mostlyOpaque(s) {
		return true
	}
	return false
}

func mostlyOpaque(s string) bool {
	var opaque int
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '/' || r == '=' || r == '-' || r == '_' {
			opaque++
		}
	}
	return opaque >= len(s)*9/10
}

func trimAudit(entries *[]AuditEntry) {
	if len(*entries) <= maxAuditEntries {
		return
	}
	*entries = (*entries)[len(*entries)-maxAuditEntries:]
}
