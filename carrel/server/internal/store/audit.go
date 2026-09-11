package store

import (
	"errors"
	"strings"
	"unicode"
)

const maxAuditEntries = 10_000

var (
	ErrAuditSecret    = errors.New("store: audit entry must not contain secrets")
	ErrAuditEmptyWho  = errors.New("store: audit entry needs who")
	ErrAuditEmptyWhat = errors.New("store: audit entry needs what")
)

// validateAudit rejects entries that would leak secrets into the log.
func validateAudit(e AuditEntry) error {
	if strings.TrimSpace(e.Who) == "" {
		return ErrAuditEmptyWho
	}
	if strings.TrimSpace(e.What) == "" {
		return ErrAuditEmptyWhat
	}
	for _, field := range []string{e.Who, e.What, e.Whom, e.Detail} {
		if containsSecret(field) {
			return ErrAuditSecret
		}
	}
	return nil
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
