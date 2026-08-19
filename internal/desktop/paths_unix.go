// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
)

func profileDirs() (userProfileDirs, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return userProfileDirs{}, fmt.Errorf("desktop: config dir: %w", err)
	}
	dataRoot, err := xdgDataHome()
	if err != nil {
		return userProfileDirs{}, err
	}
	configDir := filepath.Join(configRoot, "carrel")
	return userProfileDirs{
		configDir: configDir,
		lockDir:   configDir,
		dataDir:   filepath.Join(dataRoot, "carrel", "data"),
	}, nil
}

func xdgDataHome() (string, error) {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("desktop: data home: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}

func sidecarBinaryName() string { return "carrel" }
