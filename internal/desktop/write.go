// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// writeAtomic mirrors the store package pattern: temp file in the target
// directory, sync, rename.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("desktop: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("desktop: create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	cleanup := func(cause error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return cause
	}

	if err := tmp.Chmod(0o600); err != nil && !errors.Is(err, os.ErrInvalid) {
		if !isUnsupported(err) {
			return cleanup(fmt.Errorf("desktop: chmod %s: %w", tmpName, err))
		}
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(fmt.Errorf("desktop: write %s: %w", tmpName, err))
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(fmt.Errorf("desktop: sync %s: %w", tmpName, err))
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("desktop: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("desktop: replace %s: %w", path, err)
	}
	syncDir(dir)
	return nil
}

func syncDir(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Sync()
}

func isUnsupported(err error) bool {
	return errors.Is(err, os.ErrInvalid) ||
		errors.Is(err, os.ErrPermission) && os.PathSeparator == '\\'
}
