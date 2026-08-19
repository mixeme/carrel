// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sidecar

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("sidecar: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("sidecar: create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func(cause error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return cause
	}
	if err := tmp.Chmod(0o600); err != nil && !isUnsupportedChmod(err) {
		return cleanup(fmt.Errorf("sidecar: chmod: %w", err))
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(err)
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("sidecar: replace %s: %w", path, err)
	}
	return nil
}

func atomicReplaceFile(dst string, src string, mode os.FileMode) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func(cause error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return cause
	}
	in, err := os.Open(src)
	if err != nil {
		return cleanup(err)
	}
	defer in.Close()
	if _, err := io.Copy(tmp, in); err != nil {
		return cleanup(err)
	}
	if err := tmp.Chmod(mode); err != nil && !isUnsupportedChmod(err) {
		return cleanup(err)
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func isUnsupportedChmod(err error) bool {
	return errors.Is(err, os.ErrInvalid) ||
		(errors.Is(err, os.ErrPermission) && os.PathSeparator == '\\')
}
