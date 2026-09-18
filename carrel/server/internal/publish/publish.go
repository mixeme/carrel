// Package publish holds what a published link is made of (S-25): its
// lifetime, its public identity and the envelope of credentials it reads
// with. The link itself lives in the instance state; the page it answers is
// built by web from a live read — publishing makes no copy.
//
// The package does not import internal/web and fills no view page.
package publish

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"carrel.dev/carrel/server/internal/conf"
)

// DefaultTTL is how long a link lives unless the person chooses otherwise
// (S-25): conf.PublicLinkDefaultTTL, thirty days. "No expiry" is an explicit
// choice, never a default.
const DefaultTTL = conf.PublicLinkDefaultTTL

// Envelope is what a link needs to read its one connection without a
// session: the address, the login, the password and an accepted certificate.
// It is sealed under the server key and bound to the link (crypt.SealLink);
// only the connection of this link is in it, never the others.
type Envelope struct {
	Server          string `json:"server"`
	Login           string `json:"login"`
	Password        string `json:"password"`
	CertFingerprint string `json:"certFingerprint,omitempty"`
}

// Marshal is the envelope's plaintext.
func (e Envelope) Marshal() ([]byte, error) { return json.Marshal(e) }

// ParseEnvelope reads an opened envelope.
func ParseEnvelope(plain []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(plain, &e); err != nil {
		return Envelope{}, err
	}
	if e.Server == "" {
		return Envelope{}, errors.New("publish: envelope names no server")
	}
	return e, nil
}

// ID is how the owner's screens name a link: a prefix of its hash. It says
// nothing about the secret and cannot open anything.
func ID(hash []byte) string {
	if len(hash) < 8 {
		return hex.EncodeToString(hash)
	}
	return hex.EncodeToString(hash[:8])
}

// Expired reports whether a link with this expiry is dead at now; nil never
// expires.
func Expired(expires *time.Time, now time.Time) bool {
	return expires != nil && !now.Before(*expires)
}
