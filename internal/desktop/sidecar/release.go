// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sidecar

import (
	"fmt"
	"runtime"
	"strings"
)

// DefaultReleaseRepo is the GitHub owner/name for sidecar release assets.
const DefaultReleaseRepo = "mixeme/carrel"

// ArchiveName returns the release archive filename for the current platform.
func ArchiveName(version string) string {
	return ArchiveNameFor(version, runtime.GOOS, runtime.GOARCH)
}

// ArchiveNameFor builds carrel_{version}_{goos}_{goarch}.tar.gz or .zip.
func ArchiveNameFor(version, goos, goarch string) string {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("carrel_%s_%s_%s%s", v, goos, goarch, ext)
}

// BinaryName is the sidecar executable inside the install directory.
func BinaryName(goos string) string {
	if goos == "windows" {
		return "carrel.exe"
	}
	return "carrel"
}

// ReleaseBaseURL is https://github.com/{repo}/releases/download/v{version}.
func ReleaseBaseURL(repo, version string) string {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s", strings.Trim(repo, "/"), v)
}

// AssetURL joins base URL and archive file name.
func AssetURL(baseURL, archive string) string {
	return strings.TrimRight(baseURL, "/") + "/" + archive
}
