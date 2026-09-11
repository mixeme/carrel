package crypt

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrServerKeyExists  = errors.New("crypt: server key already exists")
	ErrServerKeyShort   = errors.New("crypt: server key too short")
	ErrServerKeyCorrupt = errors.New("crypt: server key corrupt")
)

// LoadServerKey reads the 32-byte server key. It must already exist.
func LoadServerKey(dataDir string) ([KeyLen]byte, error) {
	var key [KeyLen]byte
	b, err := os.ReadFile(filepath.Join(dataDir, ServerKey))
	if err != nil {
		return key, err
	}
	if len(b) != KeyLen {
		return key, ErrServerKeyShort
	}
	copy(key[:], b)
	return key, nil
}

// CreateServerKey creates server.key with O_EXCL and mode 0600. Never overwrites.
func CreateServerKey(dataDir string) ([KeyLen]byte, error) {
	var key [KeyLen]byte
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return key, fmt.Errorf("mkdir data dir: %w", err)
	}
	path := filepath.Join(dataDir, ServerKey)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return key, ErrServerKeyExists
		}
		return key, err
	}
	defer f.Close()

	if _, err := rand.Read(key[:]); err != nil {
		_ = os.Remove(path)
		return key, err
	}
	if _, err := f.Write(key[:]); err != nil {
		_ = os.Remove(path)
		return key, err
	}
	// Without the sync a crash can leave a zero-length key: the volume then
	// refuses to start (ErrServerKeyCorrupt) with the data still sealed.
	if err := f.Sync(); err != nil {
		_ = os.Remove(path)
		return key, err
	}
	return key, nil
}

// LoadOrCreateServerKey loads an existing key or creates one on first run.
func LoadOrCreateServerKey(dataDir string) ([KeyLen]byte, error) {
	key, err := LoadServerKey(dataDir)
	if err == nil {
		return key, nil
	}
	if errors.Is(err, ErrServerKeyShort) {
		return key, ErrServerKeyCorrupt
	}
	if !errors.Is(err, os.ErrNotExist) {
		return key, err
	}
	return CreateServerKey(dataDir)
}
