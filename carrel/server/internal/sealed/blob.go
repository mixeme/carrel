// Package sealed holds JSON structures stored inside a user's sealed blob.
package sealed

import (
	"encoding/json"
	"fmt"

	"carrel.dev/carrel/server/internal/crypt"
)

// Blob is the plaintext inside a user's SealedBlob ciphertext.
type Blob struct {
	Connections []Connection `json:"connections,omitempty"`
}

// Connection stores DAV credentials and an optional accepted leaf certificate
// fingerprint (F-13).
type Connection struct {
	ID              string `json:"id"`
	Server          string `json:"server"`
	Login           string `json:"login"`
	Password        string `json:"password"`
	CertFingerprint string `json:"certFingerprint,omitempty"`
}

// Open decrypts a user's sealed blob.
func Open(dataKey [crypt.KeyLen]byte, ciphertext []byte, login string) (Blob, error) {
	var out Blob
	if len(ciphertext) == 0 {
		return out, nil
	}
	plain, err := crypt.DecryptBlob(dataKey, ciphertext, login)
	if err != nil {
		return out, err
	}
	if len(plain) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(plain, &out); err != nil {
		return out, fmt.Errorf("sealed: decode blob: %w", err)
	}
	return out, nil
}

// Seal encrypts a user's sealed blob.
func Seal(dataKey [crypt.KeyLen]byte, b Blob, login string) ([]byte, error) {
	plain, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	if len(plain) == 0 {
		plain = []byte("{}")
	}
	return crypt.EncryptBlob(dataKey, plain, login)
}
