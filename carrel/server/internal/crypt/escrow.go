package crypt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

const escrowPrivLen = curve25519.ScalarSize

func aadEscrowMaster() []byte { return []byte("carrel.escrow.master.v1") }
func aadEscrowDeposit(login string) []byte {
	return []byte("carrel.escrow.deposit.v1|" + login)
}

// GenerateEscrowKeyPair creates an X25519 recovery key pair (S-2).
func GenerateEscrowKeyPair() (pub, priv [KeyLen]byte, err error) {
	var scalar [escrowPrivLen]byte
	if _, err = rand.Read(scalar[:]); err != nil {
		return pub, priv, err
	}
	scalar[0] &= 248
	scalar[31] &= 127
	scalar[31] |= 64
	pubBytes, err := curve25519.X25519(scalar[:], curve25519.Basepoint)
	if err != nil {
		return pub, priv, err
	}
	copy(pub[:], pubBytes)
	copy(priv[:], scalar[:])
	return pub, priv, nil
}

// WrapEscrowPrivateKey seals the recovery private key under a master password.
func WrapEscrowPrivateKey(ctx context.Context, masterPassword string, priv [KeyLen]byte) (wrapped []byte, salt []byte, p KDFParams, err error) {
	salt, err = NewSalt()
	if err != nil {
		return nil, nil, p, err
	}
	key, err := DeriveKey(ctx, masterPassword, salt, MasterKDF)
	if err != nil {
		return nil, nil, p, err
	}
	wrapped, err = seal(key32Bytes(key), aadEscrowMaster(), priv[:])
	Wipe(key)
	if err != nil {
		return nil, nil, p, err
	}
	return wrapped, salt, MasterKDF, nil
}

// UnwrapEscrowPrivateKey opens the recovery private key. The returned slice
// holds key material; wipe it after use.
func UnwrapEscrowPrivateKey(ctx context.Context, masterPassword string, salt []byte, p KDFParams, wrapped []byte) ([]byte, error) {
	key, err := DeriveKey(ctx, masterPassword, salt, p)
	if err != nil {
		return nil, err
	}
	plain, err := open(key32Bytes(key), aadEscrowMaster(), wrapped)
	Wipe(key)
	if err != nil {
		return nil, err
	}
	if len(plain) != KeyLen {
		Wipe(plain)
		return nil, fmt.Errorf("crypt: escrow private key length %d", len(plain))
	}
	return plain, nil
}

// DepositDataKey encrypts a data key with an ephemeral X25519 key and the
// escrow public key. Output: ephemeral public || sealed ciphertext.
func DepositDataKey(escrowPub [KeyLen]byte, dataKey [KeyLen]byte, login string) ([]byte, error) {
	ephPub, ephPriv, err := GenerateEscrowKeyPair()
	if err != nil {
		return nil, err
	}
	shared, err := curve25519.X25519(ephPriv[:], escrowPub[:])
	Wipe(ephPriv[:])
	if err != nil {
		return nil, err
	}
	derived, err := hkdfKey(shared, ephPub[:], aadEscrowDeposit(login))
	Wipe(shared)
	if err != nil {
		return nil, err
	}
	box, err := seal(derived, aadEscrowDeposit(login), dataKey[:])
	Wipe(derived)
	if err != nil {
		return nil, err
	}
	out := make([]byte, KeyLen+len(box))
	copy(out[:KeyLen], ephPub[:])
	copy(out[KeyLen:], box)
	return out, nil
}

// OpenDepositedKey decrypts a deposited data key with the escrow private key.
func OpenDepositedKey(escrowPriv [KeyLen]byte, deposited []byte, login string) ([KeyLen]byte, error) {
	var zero [KeyLen]byte
	if len(deposited) < KeyLen+NonceLen {
		return zero, ErrCiphertextTooShort
	}
	ephPub := deposited[:KeyLen]
	box := deposited[KeyLen:]
	shared, err := curve25519.X25519(escrowPriv[:], ephPub)
	if err != nil {
		return zero, err
	}
	derived, err := hkdfKey(shared, ephPub, aadEscrowDeposit(login))
	Wipe(shared)
	if err != nil {
		return zero, err
	}
	plain, err := open(derived, aadEscrowDeposit(login), box)
	Wipe(derived)
	if err != nil {
		return zero, err
	}
	if len(plain) != KeyLen {
		Wipe(plain)
		return zero, fmt.Errorf("crypt: deposited key length %d", len(plain))
	}
	var out [KeyLen]byte
	copy(out[:], plain)
	Wipe(plain)
	return out, nil
}

func hkdfKey(secret, salt, info []byte) ([]byte, error) {
	r := hkdf.New(sha256.New, secret, salt, info)
	out := make([]byte, KeyLen)
	if _, err := r.Read(out); err != nil {
		return nil, err
	}
	return out, nil
}

func key32Bytes(b []byte) []byte {
	var k [KeyLen]byte
	copy(k[:], b)
	return k[:]
}
