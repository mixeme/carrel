package user

import (
	"crypto/rand"
	"encoding/base32"
)

// crockford alphabet without I, L, O, U
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var b32 = base32.NewEncoding(crockford).WithPadding(base32.NoPadding)

func newEntityID(prefix string) (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + b32.EncodeToString(b[:])[:10], nil
}

// NewUserID returns u- + 10 Crockford chars from 6 random bytes.
func NewUserID() (string, error) {
	return newEntityID("u-")
}

// NewInviteID returns i- + 10 Crockford chars from 6 random bytes.
func NewInviteID() (string, error) {
	return newEntityID("i-")
}

// NewTaskID returns a fanout task ID (S-15): t- + 10 Crockford chars from 6
// random bytes. A task answers in its own session only.
func NewTaskID() (string, error) {
	return newEntityID("t-")
}

// NewTransferID returns the ID of a row of the transfer queue (S-17.3): q- +
// 10 Crockford chars from 6 random bytes. A row answers in its own session
// only.
func NewTransferID() (string, error) {
	return newEntityID("q-")
}

// NewAccountID returns a connection ID: a- + 10 Crockford chars from 6 random bytes.
func NewAccountID() (string, error) {
	return newEntityID("a-")
}

// NewFeedID is the {account} of a subscription (S-25.2): issued here, like a
// connection's, never derived from its address.
func NewFeedID() (string, error) {
	return newEntityID("f-")
}
