// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sidecar

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// VersionInfo is written beside the sidecar binary as version.json.
type VersionInfo struct {
	Version string `json:"version"`
}

// ReadVersion reads version.json when present.
func ReadVersion(path string) (*VersionInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("sidecar: read version: %w", err)
	}
	var info VersionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("sidecar: parse version: %w", err)
	}
	return &info, nil
}

// WriteVersion writes version.json after a successful install.
func WriteVersion(path string, version string) error {
	data, err := json.MarshalIndent(VersionInfo{Version: normalizeVersion(version)}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWrite(path, data)
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// VersionsMatch reports whether installed and wanted versions are the same.
func VersionsMatch(installed *VersionInfo, wanted string) bool {
	if installed == nil {
		return false
	}
	return normalizeVersion(installed.Version) == normalizeVersion(wanted)
}
