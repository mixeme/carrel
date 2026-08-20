// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux

package desktop

import (
	"fmt"
	"os"
)

func prepareWebviewProfile(p Paths) error {
	if err := os.MkdirAll(p.WebviewDataDir, 0o700); err != nil {
		return fmt.Errorf("desktop: webview profile: %w", err)
	}
	return nil
}
