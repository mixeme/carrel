package model

import (
	"crypto/rand"
	"encoding/hex"
)

// NewUID returns the UID of a record Carrel creates: 32 lowercase hex digits
// from 16 random bytes (S-11). Not Crockford — that alphabet names Carrel's own
// entities (C-4), and a record UID belongs to the far server's data.
func NewUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
