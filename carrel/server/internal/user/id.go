package user

import (
	"crypto/rand"
	"encoding/base32"
)

// crockford alphabet without I, L, O, U
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var b32 = base32.NewEncoding(crockford).WithPadding(base32.NoPadding)

// NewAccountID returns a- + 10 Crockford chars from 6 random bytes.
func NewAccountID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "a-" + b32.EncodeToString(b[:])[:10], nil
}
