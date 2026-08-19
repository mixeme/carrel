// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds install, profile, and sidecar locations for the desktop app.
// See docs/plans/desktop-wrapper.md §4.
type Paths struct {
	InstallDir  string
	ConfigPath  string
	LockPath    string
	DataDir     string
	SidecarPath string
	VersionPath string
}

// DefaultPaths resolves paths from the running executable and the current
// OS-user profile.
func DefaultPaths() (Paths, error) {
	exe, err := os.Executable()
	if err != nil {
		return Paths{}, fmt.Errorf("desktop: executable path: %w", err)
	}
	installDir, err := filepath.Abs(filepath.Dir(exe))
	if err != nil {
		return Paths{}, fmt.Errorf("desktop: install dir: %w", err)
	}
	return pathsForInstall(installDir)
}

// PathsFromInstall builds profile paths for a fixed install directory. Tests
// and callers that relocate the binary use this instead of DefaultPaths.
func PathsFromInstall(installDir string) (Paths, error) {
	installDir, err := filepath.Abs(installDir)
	if err != nil {
		return Paths{}, fmt.Errorf("desktop: install dir: %w", err)
	}
	return pathsForInstall(installDir)
}

func pathsForInstall(installDir string) (Paths, error) {
	profile, err := profileDirs()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		InstallDir:  installDir,
		ConfigPath:  filepath.Join(profile.configDir, "desktop.json"),
		LockPath:    filepath.Join(profile.lockDir, "instance.lock"),
		DataDir:     profile.dataDir,
		SidecarPath: filepath.Join(installDir, sidecarBinaryName()),
		VersionPath: filepath.Join(installDir, "version.json"),
	}, nil
}

type userProfileDirs struct {
	configDir string
	lockDir   string
	dataDir   string
}
