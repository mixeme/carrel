// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARREL_DATA_DIR", dir)
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.DataDir != dir {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, dir)
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, DefaultLogLevel)
	}
}

func TestLoadFileThenEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARREL_DATA_DIR", dir)
	clearEnv(t)

	file := filepath.Join(dir, "config.json")
	content := `{"port": 9000, "log_level": "warn", "base_path": "/carrel"}`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CARREL_PORT", "3000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Port != 3000 {
		t.Errorf("Port = %d, want 3000 (env override)", cfg.Port)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn (from file)", cfg.LogLevel)
	}
	if cfg.BasePath != "/carrel" {
		t.Errorf("BasePath = %q, want /carrel", cfg.BasePath)
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"bad port", Config{Port: 0, DataDir: "/tmp", LogLevel: "info"}},
		{"empty data dir", Config{Port: 8080, DataDir: "  ", LogLevel: "info"}},
		{"bad base path", Config{Port: 8080, DataDir: "/tmp", BasePath: "carrel", LogLevel: "info"}},
		{"trailing slash", Config{Port: 8080, DataDir: "/tmp", BasePath: "/carrel/", LogLevel: "info"}},
		{"bad log level", Config{Port: 8080, DataDir: "/tmp", LogLevel: "trace"}},
		{"bad proxy", Config{Port: 8080, DataDir: "/tmp", LogLevel: "info", TrustedProxies: []string{"not-an-ip"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestTrustedProxiesEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARREL_DATA_DIR", dir)
	t.Setenv("CARREL_TRUSTED_PROXIES", "127.0.0.1, 10.0.0.0/8")
	clearEnvExcept(t, "CARREL_DATA_DIR", "CARREL_TRUSTED_PROXIES")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.TrustedProxies) != 2 {
		t.Fatalf("TrustedProxies = %v, want 2 entries", cfg.TrustedProxies)
	}
	if cfg.TrustedProxies[0] != "127.0.0.1" || cfg.TrustedProxies[1] != "10.0.0.0/8" {
		t.Errorf("TrustedProxies = %v", cfg.TrustedProxies)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CARREL_PORT", "CARREL_TRUSTED_PROXIES", "CARREL_BASE_PATH", "CARREL_LOG_LEVEL"} {
		t.Setenv(k, "")
	}
}

func clearEnvExcept(t *testing.T, keep ...string) {
	t.Helper()
	keepSet := make(map[string]bool, len(keep))
	for _, k := range keep {
		keepSet[k] = true
	}
	for _, k := range []string{"CARREL_PORT", "CARREL_TRUSTED_PROXIES", "CARREL_BASE_PATH", "CARREL_LOG_LEVEL"} {
		if !keepSet[k] {
			t.Setenv(k, "")
		}
	}
}
