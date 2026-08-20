// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
)

// prepareWebviewProfile puts WebKitGTK's default context under the OS-user
// Carrel directory. Wails v2 has no Linux equivalent of WebviewUserDataPath;
// the default context follows XDG_DATA_HOME / XDG_CACHE_HOME.
func prepareWebviewProfile(p Paths) error {
	if err := os.MkdirAll(p.WebviewDataDir, 0o700); err != nil {
		return fmt.Errorf("desktop: webview profile: %w", err)
	}
	cache := filepath.Join(p.WebviewDataDir, "cache")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return fmt.Errorf("desktop: webview cache: %w", err)
	}
	if err := os.Setenv("XDG_DATA_HOME", p.WebviewDataDir); err != nil {
		return fmt.Errorf("desktop: webview profile: %w", err)
	}
	if err := os.Setenv("XDG_CACHE_HOME", cache); err != nil {
		return fmt.Errorf("desktop: webview profile: %w", err)
	}
	return nil
}
