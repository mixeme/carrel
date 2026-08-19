// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
)

func profileDirs() (userProfileDirs, error) {
	root, err := localAppData()
	if err != nil {
		return userProfileDirs{}, err
	}
	carrelRoot := filepath.Join(root, "Carrel")
	return userProfileDirs{
		configDir: carrelRoot,
		lockDir:   carrelRoot,
		dataDir:   filepath.Join(carrelRoot, "data"),
	}, nil
}

func localAppData() (string, error) {
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("desktop: LOCALAPPDATA: %w", err)
	}
	return filepath.Join(home, "AppData", "Local"), nil
}

func sidecarBinaryName() string { return "carrel.exe" }
