// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPathsFromInstall(t *testing.T) {
	install := filepath.Join(t.TempDir(), "Carrel")
	if err := os.MkdirAll(install, 0o700); err != nil {
		t.Fatal(err)
	}

	p, err := PathsFromInstall(install)
	if err != nil {
		t.Fatalf("PathsFromInstall() error: %v", err)
	}
	if p.InstallDir != install {
		t.Errorf("InstallDir = %q, want %q", p.InstallDir, install)
	}
	if p.SidecarPath != filepath.Join(install, sidecarBinaryName()) {
		t.Errorf("SidecarPath = %q, want %q", p.SidecarPath, filepath.Join(install, sidecarBinaryName()))
	}
	if p.VersionPath != filepath.Join(install, "version.json") {
		t.Errorf("VersionPath = %q", p.VersionPath)
	}

	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(p.ConfigPath, filepath.Join("Carrel", "desktop.json")) {
			t.Errorf("ConfigPath = %q", p.ConfigPath)
		}
		if p.DataDir != filepath.Join(filepath.Dir(p.ConfigPath), "data") {
			t.Errorf("DataDir = %q, want sibling of config", p.DataDir)
		}
		if p.LockPath != filepath.Join(filepath.Dir(p.ConfigPath), "instance.lock") {
			t.Errorf("LockPath = %q", p.LockPath)
		}
	default:
		if !strings.HasSuffix(p.ConfigPath, filepath.Join("carrel", "desktop.json")) {
			t.Errorf("ConfigPath = %q", p.ConfigPath)
		}
		if !strings.HasSuffix(p.DataDir, filepath.Join("carrel", "data")) {
			t.Errorf("DataDir = %q", p.DataDir)
		}
		if p.LockPath != filepath.Join(filepath.Dir(p.ConfigPath), "instance.lock") {
			t.Errorf("LockPath = %q", p.LockPath)
		}
	}
}

func TestDefaultPaths(t *testing.T) {
	p, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths() error: %v", err)
	}
	if p.InstallDir == "" || p.ConfigPath == "" {
		t.Fatalf("empty paths: %+v", p)
	}
}
