// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sidecar

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// InstallPaths are the desktop install locations needed to ensure a sidecar.
type InstallPaths struct {
	InstallDir  string
	SidecarPath string
	VersionPath string
}

// EnsureOptions configures lazy sidecar download before Local mode starts.
type EnsureOptions struct {
	Paths        InstallPaths
	Version      string
	OverridePath string
	SkipDownload bool
	Downloader   *Downloader
}

// Ensure installs the sidecar when it is missing or the recorded version differs.
// OverridePath skips download and uses an explicit binary (development).
func Ensure(ctx context.Context, opts EnsureOptions) error {
	if opts.OverridePath != "" {
		return nil
	}
	if opts.SkipDownload {
		if _, err := os.Stat(opts.Paths.SidecarPath); err != nil {
			return err
		}
		return nil
	}
	if opts.Version == "" {
		return errors.New("sidecar: version required")
	}

	installed, err := ReadVersion(opts.Paths.VersionPath)
	if err != nil {
		return err
	}
	if fileExists(opts.Paths.SidecarPath) && VersionsMatch(installed, opts.Version) {
		return nil
	}

	dl := opts.Downloader
	if dl == nil {
		dl = &Downloader{}
	}
	return dl.Download(ctx, opts.Version, opts.Paths.InstallDir)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// PathsFromDesktop maps desktop.Paths without importing a cycle from desktop
// into sidecar tests — callers pass InstallPaths.
func InstallPathsFrom(installDir, sidecarPath, versionPath string) InstallPaths {
	if sidecarPath == "" {
		sidecarPath = filepath.Join(installDir, BinaryName(runtime.GOOS))
	}
	if versionPath == "" {
		versionPath = filepath.Join(installDir, "version.json")
	}
	return InstallPaths{
		InstallDir:  installDir,
		SidecarPath: sidecarPath,
		VersionPath: versionPath,
	}
}
