// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ServerKeyFile is the name of the server key inside the data directory.
const ServerKeyFile = "server.key"

// ErrServerKeyCorrupt means the key file exists but does not hold a usable
// key. Overwriting it would make every encrypted record on the volume
// unreadable, so this is always fatal and never repaired automatically.
var ErrServerKeyCorrupt = errors.New("crypto: server key file is corrupt")

// LoadOrCreateServerKey returns the server key stored in dataDir, generating
// it on first run. The key protects service data that must be readable before
// any user logs in (§4).
func LoadOrCreateServerKey(dataDir string) (Key, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("crypto: create data dir %s: %w", dataDir, err)
	}
	path := filepath.Join(dataDir, ServerKeyFile)

	key, err := loadServerKey(path)
	switch {
	case err == nil:
		return key, nil
	case errors.Is(err, os.ErrNotExist):
		// First run: fall through and generate.
	default:
		return nil, err
	}

	key, err = createServerKey(path)
	if errors.Is(err, os.ErrExist) {
		// Another process won the race; use what it wrote.
		return loadServerKey(path)
	}
	return key, err
}

func loadServerKey(path string) (Key, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("crypto: read server key %s: %w", path, err)
	}
	if len(raw) != KeyLen {
		Zero(raw)
		return nil, fmt.Errorf("%w: %s holds %d bytes, want %d", ErrServerKeyCorrupt, path, len(raw), KeyLen)
	}
	return Key(raw), nil
}

// createServerKey writes a new key with O_EXCL, so an existing key is never
// overwritten. A crash mid-write leaves a short file, which loadServerKey
// reports as corrupt rather than silently replacing.
func createServerKey(path string) (Key, error) {
	key, err := Random(KeyLen)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		Zero(key)
		if errors.Is(err, os.ErrExist) {
			return nil, err
		}
		return nil, fmt.Errorf("crypto: create server key %s: %w", path, err)
	}

	if _, err := f.Write(key); err != nil {
		Zero(key)
		_ = f.Close()
		return nil, fmt.Errorf("crypto: write server key %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		Zero(key)
		_ = f.Close()
		return nil, fmt.Errorf("crypto: sync server key %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		Zero(key)
		return nil, fmt.Errorf("crypto: close server key %s: %w", path, err)
	}
	return Key(key), nil
}
