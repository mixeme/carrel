// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"fmt"
)

// ResolveRemoteURL returns the URL for Remote mode from config and an optional
// command-line override.
func ResolveRemoteURL(cfg *Config, override string) (string, error) {
	if override != "" {
		return NormalizeRemoteURL(override)
	}
	if cfg == nil {
		return "", ErrNotConfigured
	}
	if cfg.Mode != ModeRemote {
		return "", fmt.Errorf("desktop: mode %q is not remote", cfg.Mode)
	}
	return NormalizeRemoteURL(cfg.RemoteURL)
}
