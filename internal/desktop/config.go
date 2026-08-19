// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Mode is how the desktop app reaches a Carrel instance.
type Mode string

const (
	ModeRemote Mode = "remote"
	ModeLocal  Mode = "local"
)

// Config is persisted per OS-user in desktop.json.
type Config struct {
	Mode         Mode   `json:"mode"`
	RemoteURL    string `json:"remote_url,omitempty"`
	Tray         bool   `json:"tray"`
	StartAtLogin bool   `json:"start_at_login,omitempty"`
}

var (
	ErrNotConfigured = errors.New("desktop: not configured")
	ErrInvalidMode   = errors.New("desktop: invalid mode")
	ErrRemoteURL     = errors.New("desktop: remote_url required for remote mode")
)

// LoadConfig reads desktop.json. A missing file yields ErrNotConfigured.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("desktop: read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("desktop: parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveConfig writes desktop.json after validation.
func SaveConfig(path string, cfg *Config) error {
	if cfg == nil {
		return errors.New("desktop: nil config")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("desktop: marshal config: %w", err)
	}
	data = append(data, '\n')
	if err := writeAtomic(path, data); err != nil {
		return err
	}
	return nil
}

// Validate checks mode and required fields.
func (c *Config) Validate() error {
	switch c.Mode {
	case ModeRemote:
		if strings.TrimSpace(c.RemoteURL) == "" {
			return ErrRemoteURL
		}
	case ModeLocal:
	case "":
		return ErrNotConfigured
	default:
		return ErrInvalidMode
	}
	return nil
}
