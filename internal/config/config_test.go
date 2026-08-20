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
		{"bad bind", Config{Port: 8080, DataDir: "/tmp", LogLevel: "info", Bind: "not-an-ip"}},
		{"bind with port", Config{Port: 8080, DataDir: "/tmp", LogLevel: "info", Bind: "127.0.0.1:8080"}},
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

// The duplicate threshold of §15 is a setting: it has a conservative default, can
// be lowered in the file, and can be overridden per process.
func TestDuplicateThreshold(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARREL_DATA_DIR", dir)
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Duplicates.Threshold != DefaultDuplicateThreshold {
		t.Errorf("threshold = %d, want %d", cfg.Duplicates.Threshold, DefaultDuplicateThreshold)
	}

	file := filepath.Join(dir, "config.json")
	if err := os.WriteFile(file, []byte(`{"duplicates": {"threshold": 45}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Duplicates.Threshold != 45 {
		t.Errorf("threshold from file = %d, want 45", cfg.Duplicates.Threshold)
	}

	t.Setenv("CARREL_DUPLICATES_THRESHOLD", "80")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Duplicates.Threshold != 80 {
		t.Errorf("threshold from env = %d, want 80", cfg.Duplicates.Threshold)
	}

	t.Setenv("CARREL_DUPLICATES_THRESHOLD", "not-a-number")
	if _, err := Load(); err == nil {
		t.Error("a threshold that is not a number was accepted")
	}
	t.Setenv("CARREL_DUPLICATES_THRESHOLD", "0")
	if _, err := Load(); err == nil {
		t.Error("a threshold of zero was accepted: nothing would be a duplicate of nothing")
	}
}

// The file section of §7 has its own ceilings: one on an upload, one on how many
// members of a folder are listed. A large upload is bandwidth rather than memory,
// because nothing buffers it, so the default is generous; a listing is a page and
// a PROPFIND response, so that one is not.
func TestFilesLimits(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARREL_DATA_DIR", dir)
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Files.MaxUploadBytes != DefaultFilesMaxUploadBytes {
		t.Errorf("max upload = %d, want %d", cfg.Files.MaxUploadBytes, DefaultFilesMaxUploadBytes)
	}
	if cfg.Files.MaxEntries != DefaultFilesMaxEntries {
		t.Errorf("max entries = %d, want %d", cfg.Files.MaxEntries, DefaultFilesMaxEntries)
	}

	file := filepath.Join(dir, "config.json")
	if err := os.WriteFile(file, []byte(`{"files": {"max_upload_bytes": 1048576, "max_entries": 50}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Files.MaxUploadBytes != 1<<20 || cfg.Files.MaxEntries != 50 {
		t.Errorf("from file = %d / %d", cfg.Files.MaxUploadBytes, cfg.Files.MaxEntries)
	}

	t.Setenv("CARREL_FILES_MAX_UPLOAD_BYTES", "2097152")
	t.Setenv("CARREL_FILES_MAX_ENTRIES", "10")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Files.MaxUploadBytes != 2<<20 || cfg.Files.MaxEntries != 10 {
		t.Errorf("from env = %d / %d", cfg.Files.MaxUploadBytes, cfg.Files.MaxEntries)
	}

	t.Setenv("CARREL_FILES_MAX_ENTRIES", "0")
	if _, err := Load(); err == nil {
		t.Error("a listing ceiling of zero was accepted: no folder would ever show anything")
	}
	t.Setenv("CARREL_FILES_MAX_ENTRIES", "10")
	t.Setenv("CARREL_FILES_MAX_UPLOAD_BYTES", "not-a-number")
	if _, err := Load(); err == nil {
		t.Error("an upload ceiling that is not a number was accepted")
	}
}

func TestCacheCeilings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARREL_DATA_DIR", dir)
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cache.MaxBodyBytes != DefaultCacheMaxBodyBytes {
		t.Errorf("body = %d, want %d", cfg.Cache.MaxBodyBytes, DefaultCacheMaxBodyBytes)
	}
	if cfg.Cache.MaxProcessBytes != DefaultCacheMaxProcessBytes {
		t.Errorf("process = %d, want %d", cfg.Cache.MaxProcessBytes, DefaultCacheMaxProcessBytes)
	}

	file := filepath.Join(dir, "config.json")
	// An older cache block that never named the new ceilings still loads.
	if err := os.WriteFile(file, []byte(`{"cache": {"collection_ttl_seconds": 60, "max_collections": 10, "max_etag_entries": 20, "max_thumb_bytes": 100, "max_thumb_entries": 4}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cache.MaxCollections != 10 || cfg.Cache.MaxBodyBytes != DefaultCacheMaxBodyBytes || cfg.Cache.MaxProcessBytes != DefaultCacheMaxProcessBytes {
		t.Errorf("partial file = %+v", cfg.Cache)
	}

	t.Setenv("CARREL_CACHE_MAX_BODY_BYTES", "1048576")
	t.Setenv("CARREL_CACHE_MAX_PROCESS_BYTES", "2097152")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cache.MaxBodyBytes != 1<<20 || cfg.Cache.MaxProcessBytes != 2<<20 {
		t.Errorf("from env = %d / %d", cfg.Cache.MaxBodyBytes, cfg.Cache.MaxProcessBytes)
	}

	t.Setenv("CARREL_CACHE_MAX_PROCESS_BYTES", "0")
	if _, err := Load(); err == nil {
		t.Error("a process ceiling of zero was accepted")
	}
	t.Setenv("CARREL_CACHE_MAX_PROCESS_BYTES", "2097152")
	t.Setenv("CARREL_CACHE_MAX_BODY_BYTES", "not-a-number")
	if _, err := Load(); err == nil {
		t.Error("a body ceiling that is not a number was accepted")
	}
}

func TestBindEnvAndAddr(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARREL_DATA_DIR", dir)
	t.Setenv("CARREL_BIND", "127.0.0.1")
	t.Setenv("CARREL_PORT", "9090")
	clearEnvExcept(t, "CARREL_DATA_DIR", "CARREL_BIND", "CARREL_PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Bind != "127.0.0.1" {
		t.Errorf("Bind = %q, want 127.0.0.1", cfg.Bind)
	}
	if got := cfg.Addr(); got != "127.0.0.1:9090" {
		t.Errorf("Addr() = %q, want 127.0.0.1:9090", got)
	}
}

func TestAddrDefaultBind(t *testing.T) {
	cfg := defaults()
	if got := cfg.Addr(); got != ":8080" {
		t.Errorf("Addr() = %q, want :8080", got)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CARREL_PORT", "CARREL_BIND", "CARREL_TRUSTED_PROXIES", "CARREL_BASE_PATH", "CARREL_LOG_LEVEL"} {
		t.Setenv(k, "")
	}
}

func clearEnvExcept(t *testing.T, keep ...string) {
	t.Helper()
	keepSet := make(map[string]bool, len(keep))
	for _, k := range keep {
		keepSet[k] = true
	}
	for _, k := range []string{"CARREL_PORT", "CARREL_BIND", "CARREL_TRUSTED_PROXIES", "CARREL_BASE_PATH", "CARREL_LOG_LEVEL"} {
		if !keepSet[k] {
			t.Setenv(k, "")
		}
	}
}
