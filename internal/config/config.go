// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultPort    = 8080
	DefaultDataDir = "/var/lib/carrel"
	DefaultLogLevel = "info"
)

// LogLevel values accepted by CARREL_LOG_LEVEL and config file.
const (
	LogDebug = "debug"
	LogInfo  = "info"
	LogWarn  = "warn"
	LogError = "error"
)

// Config holds server runtime settings.
type Config struct {
	Port           int      `json:"port"`
	DataDir        string   `json:"data_dir"`
	TrustedProxies []string `json:"trusted_proxies"`
	BasePath       string   `json:"base_path"`
	LogLevel       string   `json:"log_level"`
}

// fileConfig mirrors Config for JSON unmarshaling with optional fields.
type fileConfig struct {
	Port           *int     `json:"port,omitempty"`
	DataDir        *string  `json:"data_dir,omitempty"`
	TrustedProxies []string `json:"trusted_proxies,omitempty"`
	BasePath       *string  `json:"base_path,omitempty"`
	LogLevel       *string  `json:"log_level,omitempty"`
}

// Load reads configuration from an optional file in dataDir, then applies
// environment variables (env wins over file).
func Load() (*Config, error) {
	cfg := defaults()

	dataDir := cfg.DataDir
	if v := strings.TrimSpace(os.Getenv("CARREL_DATA_DIR")); v != "" {
		dataDir = v
	}

	filePath := filepath.Join(dataDir, "config.json")
	if raw, err := os.ReadFile(filePath); err == nil {
		if err := applyFile(cfg, raw); err != nil {
			return nil, fmt.Errorf("parse config file %s: %w", filePath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read config file %s: %w", filePath, err)
	}

	if err := applyEnv(cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Port:           DefaultPort,
		DataDir:        DefaultDataDir,
		TrustedProxies: nil,
		BasePath:       "",
		LogLevel:       DefaultLogLevel,
	}
}

func applyFile(cfg *Config, raw []byte) error {
	var fc fileConfig
	if err := json.Unmarshal(raw, &fc); err != nil {
		return err
	}
	if fc.Port != nil {
		cfg.Port = *fc.Port
	}
	if fc.DataDir != nil {
		cfg.DataDir = *fc.DataDir
	}
	if fc.TrustedProxies != nil {
		cfg.TrustedProxies = append([]string(nil), fc.TrustedProxies...)
	}
	if fc.BasePath != nil {
		cfg.BasePath = *fc.BasePath
	}
	if fc.LogLevel != nil {
		cfg.LogLevel = *fc.LogLevel
	}
	return nil
}

func applyEnv(cfg *Config) error {
	if v := strings.TrimSpace(os.Getenv("CARREL_PORT")); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_PORT: invalid integer %q", v)
		}
		cfg.Port = port
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_DATA_DIR")); v != "" {
		cfg.DataDir = v
	}
	if v, ok := os.LookupEnv("CARREL_TRUSTED_PROXIES"); ok && strings.TrimSpace(v) != "" {
		cfg.TrustedProxies = parseList(v)
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_BASE_PATH")); v != "" {
		cfg.BasePath = v
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_LOG_LEVEL")); v != "" {
		cfg.LogLevel = strings.ToLower(v)
	}
	return nil
}

func parseList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Validate checks configuration values and returns a descriptive error.
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return errors.New("data directory must not be empty")
	}
	if err := validateBasePath(c.BasePath); err != nil {
		return err
	}
	if err := validateLogLevel(c.LogLevel); err != nil {
		return err
	}
	for _, proxy := range c.TrustedProxies {
		if err := validateTrustedProxy(proxy); err != nil {
			return fmt.Errorf("trusted proxy %q: %w", proxy, err)
		}
	}
	return nil
}

func validateBasePath(p string) error {
	if p == "" {
		return nil
	}
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("base path must start with /, got %q", p)
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		return fmt.Errorf("base path must not end with /, got %q", p)
	}
	return nil
}

func validateLogLevel(level string) error {
	switch level {
	case LogDebug, LogInfo, LogWarn, LogError:
		return nil
	default:
		return fmt.Errorf("log level must be one of debug, info, warn, error, got %q", level)
	}
}

func validateTrustedProxy(s string) error {
	if strings.Contains(s, "/") {
		if _, _, err := net.ParseCIDR(s); err != nil {
			return fmt.Errorf("invalid CIDR: %w", err)
		}
		return nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return errors.New("must be an IP address or CIDR")
	}
	return nil
}

// Addr returns the listen address for the configured port.
func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}
