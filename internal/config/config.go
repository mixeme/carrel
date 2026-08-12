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
	"time"
)

const (
	DefaultPort    = 8080
	DefaultDataDir = "/var/lib/carrel"
	DefaultLogLevel = "info"

	DefaultDAVConnectTimeout  = 10 * time.Second
	DefaultDAVRequestTimeout  = 30 * time.Second
	DefaultDAVMaxResponseSize = 10 << 20 // 10 MiB
	DefaultDAVMaxRedirects    = 5

	DefaultCacheCollectionTTL = 60 * time.Second
	DefaultCacheMaxCollections = 256
	DefaultCacheMaxETagEntries = 4096

	// Photo defaults from §11 — starting values, not acceptance criteria.
	DefaultPhotoMaxSide      = 512
	DefaultPhotoJPEGQuality  = 85
	DefaultPhotoMaxPixels    = 100_000_000
	DefaultPhotoThumbSide    = 96
)

// LogLevel values accepted by CARREL_LOG_LEVEL and config file.
const (
	LogDebug = "debug"
	LogInfo  = "info"
	LogWarn  = "warn"
	LogError = "error"
)

// DAV holds outbound CalDAV/CardDAV client limits (§24.2).
type DAV struct {
	SSRFAllowlist    []string `json:"ssrf_allowlist"`
	ConnectTimeout   Duration `json:"connect_timeout"`
	RequestTimeout   Duration `json:"request_timeout"`
	MaxResponseBytes int64    `json:"max_response_bytes"`
	MaxRedirects     int      `json:"max_redirects"`
}

// Cache holds per-session cache limits consumed in stage 2 §12.
type Cache struct {
	CollectionTTLSeconds int64 `json:"collection_ttl_seconds"`
	MaxCollections       int   `json:"max_collections"`
	MaxETagEntries       int   `json:"max_etag_entries"`
}

// Photo holds contact photo processing limits (§11).
type Photo struct {
	MaxSide        int   `json:"max_side"`
	JPEGQuality    int   `json:"jpeg_quality"`
	MaxPixels      int64 `json:"max_pixels"`
	ThumbSide      int   `json:"thumb_side"`
	MaxUploadBytes int64 `json:"max_upload_bytes"` // 0 = no byte ceiling
}

// Duration is a time.Duration marshaled as a JSON number of seconds.
type Duration time.Duration

func (d Duration) Duration() time.Duration { return time.Duration(d) }

func (d Duration) MarshalJSON() ([]byte, error) {
	sec := time.Duration(d).Seconds()
	return json.Marshal(sec)
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var sec float64
	if err := json.Unmarshal(b, &sec); err != nil {
		return err
	}
	*d = Duration(time.Duration(sec * float64(time.Second)))
	return nil
}

// Config holds server runtime settings.
type Config struct {
	Port           int      `json:"port"`
	DataDir        string   `json:"data_dir"`
	TrustedProxies []string `json:"trusted_proxies"`
	BasePath       string   `json:"base_path"`
	LogLevel       string   `json:"log_level"`
	DAV            DAV      `json:"dav"`
	Cache          Cache    `json:"cache"`
	Photo          Photo    `json:"photo"`
}

// fileConfig mirrors Config for JSON unmarshaling with optional fields.
type fileConfig struct {
	Port           *int     `json:"port,omitempty"`
	DataDir        *string  `json:"data_dir,omitempty"`
	TrustedProxies []string `json:"trusted_proxies,omitempty"`
	BasePath       *string  `json:"base_path,omitempty"`
	LogLevel       *string  `json:"log_level,omitempty"`
	DAV            *DAV     `json:"dav,omitempty"`
	Cache          *Cache   `json:"cache,omitempty"`
	Photo          *Photo   `json:"photo,omitempty"`
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
		DAV: DAV{
			ConnectTimeout:   Duration(DefaultDAVConnectTimeout),
			RequestTimeout:   Duration(DefaultDAVRequestTimeout),
			MaxResponseBytes: DefaultDAVMaxResponseSize,
			MaxRedirects:     DefaultDAVMaxRedirects,
		},
		Cache: Cache{
			CollectionTTLSeconds: int64(DefaultCacheCollectionTTL / time.Second),
			MaxCollections:       DefaultCacheMaxCollections,
			MaxETagEntries:       DefaultCacheMaxETagEntries,
		},
		Photo: Photo{
			MaxSide:     DefaultPhotoMaxSide,
			JPEGQuality: DefaultPhotoJPEGQuality,
			MaxPixels:   DefaultPhotoMaxPixels,
			ThumbSide:   DefaultPhotoThumbSide,
		},
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
	if fc.DAV != nil {
		cfg.DAV = *fc.DAV
	}
	if fc.Cache != nil {
		cfg.Cache = *fc.Cache
	}
	if fc.Photo != nil {
		cfg.Photo = *fc.Photo
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
	if v, ok := os.LookupEnv("CARREL_DAV_SSRF_ALLOWLIST"); ok && strings.TrimSpace(v) != "" {
		cfg.DAV.SSRFAllowlist = parseList(v)
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_DAV_CONNECT_TIMEOUT")); v != "" {
		d, err := parseDurationSeconds(v)
		if err != nil {
			return fmt.Errorf("CARREL_DAV_CONNECT_TIMEOUT: %w", err)
		}
		cfg.DAV.ConnectTimeout = Duration(d)
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_DAV_REQUEST_TIMEOUT")); v != "" {
		d, err := parseDurationSeconds(v)
		if err != nil {
			return fmt.Errorf("CARREL_DAV_REQUEST_TIMEOUT: %w", err)
		}
		cfg.DAV.RequestTimeout = Duration(d)
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_DAV_MAX_RESPONSE_BYTES")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("CARREL_DAV_MAX_RESPONSE_BYTES: invalid integer %q", v)
		}
		cfg.DAV.MaxResponseBytes = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_DAV_MAX_REDIRECTS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_DAV_MAX_REDIRECTS: invalid integer %q", v)
		}
		cfg.DAV.MaxRedirects = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_CACHE_COLLECTION_TTL")); v != "" {
		d, err := parseDurationSeconds(v)
		if err != nil {
			return fmt.Errorf("CARREL_CACHE_COLLECTION_TTL: %w", err)
		}
		cfg.Cache.CollectionTTLSeconds = int64(d / time.Second)
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_CACHE_MAX_COLLECTIONS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_CACHE_MAX_COLLECTIONS: invalid integer %q", v)
		}
		cfg.Cache.MaxCollections = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_CACHE_MAX_ETAG_ENTRIES")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_CACHE_MAX_ETAG_ENTRIES: invalid integer %q", v)
		}
		cfg.Cache.MaxETagEntries = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_PHOTO_MAX_SIDE")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_PHOTO_MAX_SIDE: invalid integer %q", v)
		}
		cfg.Photo.MaxSide = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_PHOTO_JPEG_QUALITY")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_PHOTO_JPEG_QUALITY: invalid integer %q", v)
		}
		cfg.Photo.JPEGQuality = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_PHOTO_MAX_PIXELS")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("CARREL_PHOTO_MAX_PIXELS: invalid integer %q", v)
		}
		cfg.Photo.MaxPixels = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_PHOTO_THUMB_SIDE")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_PHOTO_THUMB_SIDE: invalid integer %q", v)
		}
		cfg.Photo.ThumbSide = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_PHOTO_MAX_UPLOAD_BYTES")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("CARREL_PHOTO_MAX_UPLOAD_BYTES: invalid integer %q", v)
		}
		cfg.Photo.MaxUploadBytes = n
	}
	return nil
}

func parseDurationSeconds(s string) (time.Duration, error) {
	sec, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	if sec <= 0 {
		return 0, fmt.Errorf("duration must be positive, got %q", s)
	}
	return time.Duration(sec * float64(time.Second)), nil
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
	if err := c.DAV.validate(); err != nil {
		return err
	}
	if err := c.Cache.validate(); err != nil {
		return err
	}
	if err := c.Photo.validate(); err != nil {
		return err
	}
	return nil
}

func (d DAV) validate() error {
	if d.ConnectTimeout.Duration() <= 0 {
		return errors.New("dav connect timeout must be positive")
	}
	if d.RequestTimeout.Duration() <= 0 {
		return errors.New("dav request timeout must be positive")
	}
	if d.MaxResponseBytes <= 0 {
		return errors.New("dav max response bytes must be positive")
	}
	if d.MaxRedirects < 0 {
		return errors.New("dav max redirects must not be negative")
	}
	return nil
}

func (c Cache) validate() error {
	if c.CollectionTTLSeconds <= 0 {
		return errors.New("cache collection TTL must be positive")
	}
	if c.MaxCollections <= 0 {
		return errors.New("cache max collections must be positive")
	}
	if c.MaxETagEntries <= 0 {
		return errors.New("cache max etag entries must be positive")
	}
	return nil
}

// CollectionTTL returns the per-session collection metadata TTL.
func (c Cache) CollectionTTL() time.Duration {
	return time.Duration(c.CollectionTTLSeconds) * time.Second
}

func (p Photo) validate() error {
	if p.MaxSide <= 0 {
		return errors.New("photo max side must be positive")
	}
	if p.JPEGQuality < 1 || p.JPEGQuality > 100 {
		return errors.New("photo JPEG quality must be between 1 and 100")
	}
	if p.MaxPixels <= 0 {
		return errors.New("photo max pixels must be positive")
	}
	if p.ThumbSide <= 0 {
		return errors.New("photo thumb side must be positive")
	}
	if p.MaxUploadBytes < 0 {
		return errors.New("photo max upload bytes must not be negative")
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
