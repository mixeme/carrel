// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrCreateServerKey(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")

	key, err := LoadOrCreateServerKey(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateServerKey: %v", err)
	}
	if len(key) != KeyLen {
		t.Fatalf("server key is %d bytes, want %d", len(key), KeyLen)
	}
	if bytes.Equal(key, make([]byte, KeyLen)) {
		t.Fatal("server key is all zeroes")
	}

	// A second call on a populated volume must return the same key, otherwise
	// every record written before the restart becomes unreadable.
	again, err := LoadOrCreateServerKey(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateServerKey (second call): %v", err)
	}
	if !bytes.Equal(key, again) {
		t.Fatal("server key changed between calls")
	}

	path := filepath.Join(dir, ServerKeyFile)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat server key: %v", err)
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("server key mode is %o, want 600", perm)
		}
	}
}

func TestLoadOrCreateServerKeyIndependentDirs(t *testing.T) {
	a, err := LoadOrCreateServerKey(t.TempDir())
	if err != nil {
		t.Fatalf("LoadOrCreateServerKey: %v", err)
	}
	b, err := LoadOrCreateServerKey(t.TempDir())
	if err != nil {
		t.Fatalf("LoadOrCreateServerKey: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("two volumes got the same server key")
	}
}

// A truncated key file means a crash during first-run generation. Silently
// generating a replacement would destroy the whole volume, so it is an error.
func TestLoadOrCreateServerKeyCorrupt(t *testing.T) {
	for name, content := range map[string][]byte{
		"empty":     {},
		"short":     bytes.Repeat([]byte{0x41}, KeyLen-1),
		"too long":  bytes.Repeat([]byte{0x41}, KeyLen+1),
		"text file": []byte("not a key\n"),
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, ServerKeyFile)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("%s: write key file: %v", name, err)
		}
		if _, err := LoadOrCreateServerKey(dir); !errors.Is(err, ErrServerKeyCorrupt) {
			t.Errorf("%s: LoadOrCreateServerKey returned %v, want ErrServerKeyCorrupt", name, err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: read back key file: %v", name, err)
		}
		if !bytes.Equal(got, content) {
			t.Errorf("%s: corrupt key file was overwritten", name)
		}
	}
}

func TestServerKeyProtectsState(t *testing.T) {
	dir := t.TempDir()
	key, err := LoadOrCreateServerKey(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateServerKey: %v", err)
	}

	state := []byte(`{"users":[{"login":"admin","role":"admin"}]}`)
	sealed, err := SealState(key, state)
	if err != nil {
		t.Fatalf("SealState: %v", err)
	}
	if bytes.Contains(sealed, []byte("admin")) {
		t.Fatal("sealed state leaks plaintext")
	}

	got, err := OpenState(key, sealed)
	if err != nil {
		t.Fatalf("OpenState: %v", err)
	}
	if !bytes.Equal(got, state) {
		t.Fatalf("OpenState returned %q, want %q", got, state)
	}

	otherKey, err := LoadOrCreateServerKey(t.TempDir())
	if err != nil {
		t.Fatalf("LoadOrCreateServerKey: %v", err)
	}
	if _, err := OpenState(otherKey, sealed); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("state opened with another volume's key: %v", err)
	}
}
