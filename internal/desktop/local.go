// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import "errors"

// ResolveLocalMode reports whether Local mode should run.
func ResolveLocalMode(cfg *Config, forceLocal bool, remoteOverride string) bool {
	if remoteOverride != "" {
		return false
	}
	if forceLocal {
		return true
	}
	return cfg != nil && cfg.Mode == ModeLocal
}

// ErrLocalConfig is returned when Local mode is requested without configuration.
var ErrLocalConfig = errors.New("desktop: local mode requires desktop.json or -local with sidecar")
