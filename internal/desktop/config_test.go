// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop.json")

	cfg := &Config{
		Mode:         ModeRemote,
		RemoteURL:    "https://carrel.example",
		Tray:         true,
		StartAtLogin: true,
	}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if got.Mode != cfg.Mode || got.RemoteURL != cfg.RemoteURL || got.Tray != cfg.Tray || got.StartAtLogin != cfg.StartAtLogin {
		t.Errorf("got %+v, want %+v", got, cfg)
	}
}

func TestConfigMissing(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("LoadConfig() = %v, want ErrNotConfigured", err)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want error
	}{
		{name: "remote needs url", cfg: Config{Mode: ModeRemote}, want: ErrRemoteURL},
		{name: "local ok", cfg: Config{Mode: ModeLocal}, want: nil},
		{name: "bad mode", cfg: Config{Mode: "hybrid"}, want: ErrInvalidMode},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if !errors.Is(err, tc.want) && (err != nil || tc.want != nil) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestConfigAtomicReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop.json")
	first := &Config{Mode: ModeLocal}
	if err := SaveConfig(path, first); err != nil {
		t.Fatal(err)
	}
	second := &Config{Mode: ModeRemote, RemoteURL: "https://a.example"}
	if err := SaveConfig(path, second); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("config file empty")
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != ModeRemote {
		t.Fatalf("got mode %q", got.Mode)
	}
}
