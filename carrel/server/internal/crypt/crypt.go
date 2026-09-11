// Package crypt implements S-2: Argon2id key derivation, XChaCha20-Poly1305
// AEAD with per-purpose AAD, and server-key management.
package crypt

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	KeyLen    = 32
	NonceLen  = chacha20poly1305.NonceSizeX // 24 bytes for XChaCha20-Poly1305
	SaltLen   = 16
	ServerKey = "server.key"
)

// KDFParams are stored alongside each record so cost can rise later.
type KDFParams struct {
	Time    uint32
	Memory  uint32 // KiB
	Threads uint8
	KeyLen  uint32
}

// Login and wrap KDF defaults from architecture.md.
var (
	LoginKDF  = KDFParams{Time: 3, Memory: 64 * 1024, Threads: 1, KeyLen: KeyLen}
	WrapKDF   = KDFParams{Time: 3, Memory: 64 * 1024, Threads: 1, KeyLen: KeyLen}
	MasterKDF = KDFParams{Time: 4, Memory: 256 * 1024, Threads: 1, KeyLen: KeyLen}
)

var (
	ErrCiphertextTooShort = errors.New("crypt: ciphertext too short")
	ErrDecrypt            = errors.New("crypt: decrypt failed")
	ErrKDFParams          = errors.New("crypt: invalid KDF parameters")
)

const (
	minKDFMemoryKiB = 8 * 1024    // 8 MiB
	maxKDFMemoryKiB = 1024 * 1024 // 1 GiB
)

func NewSalt() ([]byte, error) {
	s := make([]byte, SaltLen)
	if _, err := rand.Read(s); err != nil {
		return nil, err
	}
	return s, nil
}

func NewDataKey() ([KeyLen]byte, error) {
	var k [KeyLen]byte
	if _, err := rand.Read(k[:]); err != nil {
		return k, err
	}
	return k, nil
}

func DeriveKey(password string, salt []byte, p KDFParams) []byte {
	return argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
}

// HashLogin derives a login verifier. Salt must be unique per account.
func HashLogin(password string, salt []byte) ([]byte, KDFParams) {
	return DeriveKey(password, salt, LoginKDF), LoginKDF
}

// VerifyLogin compares a password to a stored login hash in constant time.
func VerifyLogin(password string, salt, hash []byte, p KDFParams) bool {
	got := DeriveKey(password, salt, p)
	return subtle.ConstantTimeCompare(got, hash) == 1
}

// DeriveWrapKey derives the wrap key from a different salt than login (S-2).
// The returned slice holds key material; wipe it after UnwrapDataKey or copy
// into a fixed-size buffer.
func DeriveWrapKey(password string, salt []byte) ([]byte, KDFParams) {
	return DeriveKey(password, salt, WrapKDF), WrapKDF
}

// ValidateKDFParams checks stored cost parameters. All-zero params are allowed
// only when there is nothing to unwrap yet (no login hash / wrapped key).
func ValidateKDFParams(p KDFParams, unwrap bool) error {
	if !unwrap {
		if p.Time == 0 && p.Memory == 0 && p.Threads == 0 && p.KeyLen == 0 {
			return nil
		}
	}
	if p.Time < 1 || p.Threads < 1 || p.KeyLen != KeyLen {
		return ErrKDFParams
	}
	if p.Memory < minKDFMemoryKiB || p.Memory > maxKDFMemoryKiB {
		return ErrKDFParams
	}
	return nil
}

func aadWrap(login string) []byte { return []byte("carrel.wrap.v1|" + login) }
func aadBlob(login string) []byte { return []byte("carrel.blob.v1|" + login) }
func aadState(version uint32) []byte {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], version)
	return append([]byte("carrel.state.v1|"), b[:]...)
}

func seal(key, aad, plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, NonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, NonceLen, NonceLen+len(plaintext)+aead.Overhead())
	out = append(out, aead.Seal(nil, nonce, plaintext, aad)...)
	copy(out[:NonceLen], nonce)
	return out, nil
}

func open(key, aad, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < NonceLen {
		return nil, ErrCiphertextTooShort
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := ciphertext[:NonceLen]
	box := ciphertext[NonceLen:]
	plain, err := aead.Open(nil, nonce, box, aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plain, nil
}

// WrapDataKey encrypts a data key under the wrap key (AAD binds login).
func WrapDataKey(wrapKey, dataKey [KeyLen]byte, login string) ([]byte, error) {
	return seal(wrapKey[:], aadWrap(login), dataKey[:])
}

// UnwrapDataKey decrypts a wrapped data key.
func UnwrapDataKey(wrapKey [KeyLen]byte, wrapped []byte, login string) ([KeyLen]byte, error) {
	var out [KeyLen]byte
	plain, err := open(wrapKey[:], aadWrap(login), wrapped)
	if err != nil {
		return out, err
	}
	if len(plain) != KeyLen {
		Wipe(plain)
		return out, fmt.Errorf("crypt: wrapped key length %d", len(plain))
	}
	copy(out[:], plain)
	Wipe(plain)
	return out, nil
}

// RewrapDataKey changes the wrap password without touching the data key (S-2).
func RewrapDataKey(oldWrap, newWrap [KeyLen]byte, wrapped []byte, login string) ([]byte, error) {
	dataKey, err := UnwrapDataKey(oldWrap, wrapped, login)
	if err != nil {
		return nil, err
	}
	return WrapDataKey(newWrap, dataKey, login)
}

// EncryptBlob seals a user's sealed blob under their data key.
func EncryptBlob(dataKey [KeyLen]byte, plaintext []byte, login string) ([]byte, error) {
	return seal(dataKey[:], aadBlob(login), plaintext)
}

// DecryptBlob opens a user's sealed blob.
func DecryptBlob(dataKey [KeyLen]byte, ciphertext []byte, login string) ([]byte, error) {
	return open(dataKey[:], aadBlob(login), ciphertext)
}

// EncryptState seals the instance state file under the server key.
func EncryptState(serverKey [KeyLen]byte, plaintext []byte, version uint32) ([]byte, error) {
	return seal(serverKey[:], aadState(version), plaintext)
}

// DecryptState opens the instance state file.
func DecryptState(serverKey [KeyLen]byte, ciphertext []byte, version uint32) ([]byte, error) {
	return open(serverKey[:], aadState(version), ciphertext)
}

// Wipe overwrites wrapped key material so the data key cannot be recovered.
func Wipe(wrapped []byte) {
	for i := range wrapped {
		wrapped[i] = 0
	}
	if len(wrapped) > 0 {
		if _, err := rand.Read(wrapped); err != nil {
			for i := range wrapped {
				wrapped[i] = 0xff
			}
		}
	}
}
