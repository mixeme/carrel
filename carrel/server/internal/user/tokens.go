package user

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
)

const secretBytes = 32

func newSecret() (raw string, hash []byte, err error) {
	b := make([]byte, secretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	hash = hashSecret(b)
	return raw, hash, nil
}

func hashSecret(raw []byte) []byte {
	h := sha256.Sum256(raw)
	return h[:]
}

func hashToken(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	if len(raw) < secretBytes {
		return nil, errors.New("token too short")
	}
	return hashSecret(raw), nil
}

func verifyHash(stored, got []byte) bool {
	if len(stored) != len(got) {
		return false
	}
	return subtle.ConstantTimeCompare(stored, got) == 1
}
