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

	DefaultCacheCollectionTTL  = 60 * time.Second
	DefaultCacheMaxCollections = 256
	DefaultCacheMaxETagEntries = 4096
	DefaultCacheMaxThumbBytes  = 16 << 20 // 16 MiB
	DefaultCacheMaxThumbEntries = 512

	// Photo defaults from §11 — starting values, not acceptance criteria.
	DefaultPhotoMaxSide      = 512
	DefaultPhotoJPEGQuality  = 85
	DefaultPhotoMaxPixels    = 100_000_000
	DefaultPhotoThumbSide    = 96
	DefaultImportMaxBytes    = 16 << 20 // 16 MiB upload ceiling for .vcf / .zip
	DefaultImportMaxCards    = 5000

	// Fan-out progress defaults from §16.
	DefaultProgressPollMillis    = 700
	DefaultProgressSourceTimeout = 10 * time.Second
	DefaultProgressTotalTimeout  = 30 * time.Second
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
	MaxThumbBytes        int   `json:"max_thumb_bytes"`
	MaxThumbEntries      int   `json:"max_thumb_entries"`
}

// Photo holds contact photo processing limits (§11).
type Photo struct {
	MaxSide        int   `json:"max_side"`
	JPEGQuality    int   `json:"jpeg_quality"`
	MaxPixels      int64 `json:"max_pixels"`
	ThumbSide      int   `json:"thumb_side"`
	MaxUploadBytes int64 `json:"max_upload_bytes"` // 0 = no byte ceiling
}

// Import holds contact import limits (§23.7 standard .vcf).
type Import struct {
	MaxBytes int64 `json:"max_bytes"`
	MaxCards int   `json:"max_cards"`
}

// ProgressMode is how fan-out progress reaches the browser (§16).
type ProgressMode string

const (
	// ProgressSSE streams fragments over one event-source connection and
	// falls back to polling on its own when the stream will not open — which
	// is the normal case on a mobile network and behind some proxies (§13).
	ProgressSSE ProgressMode = "sse"
	// ProgressPoll never opens a stream. It exists for a reverse proxy that
	// buffers responses and cannot be reconfigured.
	ProgressPoll ProgressMode = "poll"
)

// Valid reports whether m is a known mode.
func (m ProgressMode) Valid() bool { return m == ProgressSSE || m == ProgressPoll }

// Progress holds the fan-out progress settings of §16.
type Progress struct {
	Mode ProgressMode `json:"mode"`
	// PollMillis is how often the fallback asks for the status fragment.
	PollMillis int `json:"poll_millis"`
	// SourceTimeout caps one source; TotalTimeout caps the whole task, after
	// which whatever has not answered is marked timed out.
	SourceTimeout Duration `json:"source_timeout"`
	TotalTimeout  Duration `json:"total_timeout"`
}

// SSE reports whether a stream should be offered.
func (p Progress) SSE() bool { return p.Mode != ProgressPoll }

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
	Import         Import   `json:"import"`
	Progress       Progress `json:"progress"`
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
	Photo          *Photo    `json:"photo,omitempty"`
	Import         *Import   `json:"import,omitempty"`
	Progress       *Progress `json:"progress,omitempty"`
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
			MaxThumbBytes:        DefaultCacheMaxThumbBytes,
			MaxThumbEntries:      DefaultCacheMaxThumbEntries,
		},
		Photo: Photo{
			MaxSide:     DefaultPhotoMaxSide,
			JPEGQuality: DefaultPhotoJPEGQuality,
			MaxPixels:   DefaultPhotoMaxPixels,
			ThumbSide:   DefaultPhotoThumbSide,
		},
		Import: Import{
			MaxBytes: DefaultImportMaxBytes,
			MaxCards: DefaultImportMaxCards,
		},
		Progress: Progress{
			Mode:          ProgressSSE,
			PollMillis:    DefaultProgressPollMillis,
			SourceTimeout: Duration(DefaultProgressSourceTimeout),
			TotalTimeout:  Duration(DefaultProgressTotalTimeout),
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
	if fc.Import != nil {
		cfg.Import = *fc.Import
	}
	if fc.Progress != nil {
		cfg.Progress = *fc.Progress
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
	if v := strings.TrimSpace(os.Getenv("CARREL_CACHE_MAX_THUMB_BYTES")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_CACHE_MAX_THUMB_BYTES: invalid integer %q", v)
		}
		cfg.Cache.MaxThumbBytes = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_CACHE_MAX_THUMB_ENTRIES")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_CACHE_MAX_THUMB_ENTRIES: invalid integer %q", v)
		}
		cfg.Cache.MaxThumbEntries = n
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
	if v := strings.TrimSpace(os.Getenv("CARREL_IMPORT_MAX_BYTES")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("CARREL_IMPORT_MAX_BYTES: invalid integer %q", v)
		}
		cfg.Import.MaxBytes = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_IMPORT_MAX_CARDS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_IMPORT_MAX_CARDS: invalid integer %q", v)
		}
		cfg.Import.MaxCards = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_PROGRESS_MODE")); v != "" {
		cfg.Progress.Mode = ProgressMode(strings.ToLower(v))
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_PROGRESS_POLL_MILLIS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CARREL_PROGRESS_POLL_MILLIS: invalid integer %q", v)
		}
		cfg.Progress.PollMillis = n
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_PROGRESS_SOURCE_TIMEOUT")); v != "" {
		d, err := parseDurationSeconds(v)
		if err != nil {
			return fmt.Errorf("CARREL_PROGRESS_SOURCE_TIMEOUT: %w", err)
		}
		cfg.Progress.SourceTimeout = Duration(d)
	}
	if v := strings.TrimSpace(os.Getenv("CARREL_PROGRESS_TOTAL_TIMEOUT")); v != "" {
		d, err := parseDurationSeconds(v)
		if err != nil {
			return fmt.Errorf("CARREL_PROGRESS_TOTAL_TIMEOUT: %w", err)
		}
		cfg.Progress.TotalTimeout = Duration(d)
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
	if err := c.Import.validate(); err != nil {
		return err
	}
	if err := c.Progress.validate(); err != nil {
		return err
	}
	return nil
}

func (p Progress) validate() error {
	if !p.Mode.Valid() {
		return fmt.Errorf("progress mode must be %q or %q, got %q", ProgressSSE, ProgressPoll, p.Mode)
	}
	if p.PollMillis < 100 {
		return errors.New("progress poll interval must be at least 100 ms")
	}
	if p.SourceTimeout.Duration() <= 0 {
		return errors.New("progress source timeout must be positive")
	}
	if p.TotalTimeout.Duration() < p.SourceTimeout.Duration() {
		return errors.New("progress total timeout must not be shorter than the source timeout")
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
	if c.MaxThumbBytes <= 0 {
		return errors.New("cache max thumb bytes must be positive")
	}
	if c.MaxThumbEntries <= 0 {
		return errors.New("cache max thumb entries must be positive")
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

func (i Import) validate() error {
	if i.MaxBytes <= 0 {
		return errors.New("import max bytes must be positive")
	}
	if i.MaxCards <= 0 {
		return errors.New("import max cards must be positive")
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
