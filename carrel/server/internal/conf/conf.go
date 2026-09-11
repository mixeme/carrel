package conf

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds every C-12 variable plus CARREL_DAVLOOM_PORT from architecture.md.
// Values come from settings.env in the data directory, overridden by the
// process environment.
type Config struct {
	Port     int
	Bind     string
	DataDir  string
	BasePath string

	PublicURL      string
	TrustedProxies string
	LogLevel       string
	Timezone       string

	DAVSSRFAllowlist    string
	DAVConnectTimeout   time.Duration
	DAVRequestTimeout   time.Duration
	DAVMaxResponseBytes int64
	DAVMaxRedirects     int

	CacheCollectionTTL   time.Duration
	CacheMaxCollections  int
	CacheMaxETagEntries  int
	CacheMaxBodyBytes    int64
	CacheMaxProcessBytes int64
	CacheMaxThumbBytes   int64
	CacheMaxThumbEntries int

	PhotoMaxSide        int
	PhotoJPEGQuality    int
	PhotoMaxPixels      int64
	PhotoThumbSide      int
	PhotoMaxUploadBytes int64

	ImportMaxBytes   int64
	ImportMaxRecords int

	FilesMaxEntries int

	ProgressMode          string
	ProgressPollMillis    int
	ProgressSourceTimeout time.Duration
	ProgressTotalTimeout  time.Duration

	DuplicatesThreshold int

	UnlockTTL time.Duration

	MailEnabled         bool
	MailMaxAttachBytes  int64
	MailMaxMessageBytes int64
	MailMaxRecipients   int
	MailMaxPerDay       int

	// DAVloomPort is zero when davloom is disabled (empty CARREL_DAVLOOM_PORT).
	DAVloomPort int
}

const settingsFile = "settings.env"

// Load reads configuration: defaults, then settings.env in the data directory,
// then the process environment (which wins).
func Load() (*Config, error) {
	c := defaults()
	if v := os.Getenv("CARREL_DATA_DIR"); v != "" {
		c.DataDir = v
	}
	if err := mergeFile(c, filepath.Join(c.DataDir, settingsFile)); err != nil {
		return nil, err
	}
	if err := mergeEnv(c); err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func defaults() *Config {
	return &Config{
		Port:    8080,
		DataDir: "/var/lib/carrel",

		LogLevel: "info",

		DAVConnectTimeout:   10 * time.Second,
		DAVRequestTimeout:   30 * time.Second,
		DAVMaxResponseBytes: 10 << 20,
		DAVMaxRedirects:     5,

		CacheCollectionTTL:   60 * time.Second,
		CacheMaxCollections:  256,
		CacheMaxETagEntries:  4096,
		CacheMaxBodyBytes:    64 << 20,
		CacheMaxProcessBytes: 256 << 20,
		CacheMaxThumbBytes:   16 << 20,
		CacheMaxThumbEntries: 512,

		PhotoMaxSide:        512,
		PhotoJPEGQuality:    85,
		PhotoMaxPixels:      100_000_000,
		PhotoThumbSide:      96,
		PhotoMaxUploadBytes: 64 << 20,

		ImportMaxBytes:   16 << 20,
		ImportMaxRecords: 5000,

		FilesMaxEntries: 2000,

		ProgressMode:          "sse",
		ProgressPollMillis:    700,
		ProgressSourceTimeout: 10 * time.Second,
		ProgressTotalTimeout:  30 * time.Second,

		DuplicatesThreshold: 60,

		UnlockTTL: 300 * time.Second,

		MailMaxAttachBytes:  10 << 20,
		MailMaxMessageBytes: 25 << 20,
		MailMaxRecipients:   20,
		MailMaxPerDay:       50,
	}
}

func mergeFile(c *Config, path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if err := applyKV(c, strings.TrimSpace(key), strings.TrimSpace(val)); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return sc.Err()
}

func mergeEnv(c *Config) error {
	for _, kv := range os.Environ() {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if err := applyKV(c, key, val); err != nil {
			return err
		}
	}
	return nil
}

func applyKV(c *Config, key, val string) error {
	var err error
	switch key {
	case "CARREL_PORT":
		c.Port, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_BIND":
		c.Bind = val
	case "CARREL_DATA_DIR":
		c.DataDir = val
	case "CARREL_BASE_PATH":
		c.BasePath = val
	case "CARREL_PUBLIC_URL":
		c.PublicURL = val
	case "CARREL_TRUSTED_PROXIES":
		c.TrustedProxies = val
	case "CARREL_LOG_LEVEL":
		c.LogLevel = val
	case "CARREL_TIMEZONE":
		c.Timezone = val
	case "CARREL_DAV_SSRF_ALLOWLIST":
		c.DAVSSRFAllowlist = val
	case "CARREL_DAV_CONNECT_TIMEOUT":
		c.DAVConnectTimeout, err = seconds(key, val)
		if err != nil {
			return err
		}
	case "CARREL_DAV_REQUEST_TIMEOUT":
		c.DAVRequestTimeout, err = seconds(key, val)
		if err != nil {
			return err
		}
	case "CARREL_DAV_MAX_RESPONSE_BYTES":
		c.DAVMaxResponseBytes, err = atoi64(key, val)
		if err != nil {
			return err
		}
	case "CARREL_DAV_MAX_REDIRECTS":
		c.DAVMaxRedirects, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_CACHE_COLLECTION_TTL":
		c.CacheCollectionTTL, err = seconds(key, val)
		if err != nil {
			return err
		}
	case "CARREL_CACHE_MAX_COLLECTIONS":
		c.CacheMaxCollections, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_CACHE_MAX_ETAG_ENTRIES":
		c.CacheMaxETagEntries, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_CACHE_MAX_BODY_BYTES":
		c.CacheMaxBodyBytes, err = atoi64(key, val)
		if err != nil {
			return err
		}
	case "CARREL_CACHE_MAX_PROCESS_BYTES":
		c.CacheMaxProcessBytes, err = atoi64(key, val)
		if err != nil {
			return err
		}
	case "CARREL_CACHE_MAX_THUMB_BYTES":
		c.CacheMaxThumbBytes, err = atoi64(key, val)
		if err != nil {
			return err
		}
	case "CARREL_CACHE_MAX_THUMB_ENTRIES":
		c.CacheMaxThumbEntries, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_PHOTO_MAX_SIDE":
		c.PhotoMaxSide, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_PHOTO_JPEG_QUALITY":
		c.PhotoJPEGQuality, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_PHOTO_MAX_PIXELS":
		c.PhotoMaxPixels, err = atoi64(key, val)
		if err != nil {
			return err
		}
	case "CARREL_PHOTO_THUMB_SIDE":
		c.PhotoThumbSide, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_PHOTO_MAX_UPLOAD_BYTES":
		c.PhotoMaxUploadBytes, err = atoi64(key, val)
		if err != nil {
			return err
		}
	case "CARREL_IMPORT_MAX_BYTES":
		c.ImportMaxBytes, err = atoi64(key, val)
		if err != nil {
			return err
		}
	case "CARREL_IMPORT_MAX_RECORDS":
		c.ImportMaxRecords, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_FILES_MAX_ENTRIES":
		c.FilesMaxEntries, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_PROGRESS_MODE":
		c.ProgressMode = val
	case "CARREL_PROGRESS_POLL_MILLIS":
		c.ProgressPollMillis, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_PROGRESS_SOURCE_TIMEOUT":
		c.ProgressSourceTimeout, err = seconds(key, val)
		if err != nil {
			return err
		}
	case "CARREL_PROGRESS_TOTAL_TIMEOUT":
		c.ProgressTotalTimeout, err = seconds(key, val)
		if err != nil {
			return err
		}
	case "CARREL_DUPLICATES_THRESHOLD":
		c.DuplicatesThreshold, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_UNLOCK_TTL":
		c.UnlockTTL, err = seconds(key, val)
		if err != nil {
			return err
		}
	case "CARREL_MAIL_ENABLED":
		c.MailEnabled, err = boolVal(key, val)
		if err != nil {
			return err
		}
	case "CARREL_MAIL_MAX_ATTACH_BYTES":
		c.MailMaxAttachBytes, err = atoi64(key, val)
		if err != nil {
			return err
		}
	case "CARREL_MAIL_MAX_MESSAGE_BYTES":
		c.MailMaxMessageBytes, err = atoi64(key, val)
		if err != nil {
			return err
		}
	case "CARREL_MAIL_MAX_RECIPIENTS":
		c.MailMaxRecipients, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_MAIL_MAX_PER_DAY":
		c.MailMaxPerDay, err = atoi(key, val)
		if err != nil {
			return err
		}
	case "CARREL_DAVLOOM_PORT":
		if val == "" {
			c.DAVloomPort = 0
		} else {
			c.DAVloomPort, err = atoi(key, val)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("CARREL_PORT: out of range %d", c.Port)
	}
	if c.DAVloomPort < 0 || c.DAVloomPort > 65535 {
		return fmt.Errorf("CARREL_DAVLOOM_PORT: out of range %d", c.DAVloomPort)
	}
	// davloom gets its own listener and its own buckets; it does not share the
	// web port (architecture.md).
	if c.DAVloomPort != 0 && c.DAVloomPort == c.Port {
		return fmt.Errorf("CARREL_DAVLOOM_PORT: must differ from CARREL_PORT (%d)", c.Port)
	}
	if c.DataDir == "" {
		return fmt.Errorf("CARREL_DATA_DIR: empty")
	}
	if c.CacheCollectionTTL < 0 {
		return fmt.Errorf("CARREL_CACHE_COLLECTION_TTL: negative")
	}
	if c.CacheMaxCollections < 0 {
		return fmt.Errorf("CARREL_CACHE_MAX_COLLECTIONS: negative")
	}
	if c.CacheMaxETagEntries < 0 {
		return fmt.Errorf("CARREL_CACHE_MAX_ETAG_ENTRIES: negative")
	}
	if c.CacheMaxBodyBytes < 0 {
		return fmt.Errorf("CARREL_CACHE_MAX_BODY_BYTES: negative")
	}
	if c.CacheMaxProcessBytes < 0 {
		return fmt.Errorf("CARREL_CACHE_MAX_PROCESS_BYTES: negative")
	}
	if c.CacheMaxThumbBytes < 0 {
		return fmt.Errorf("CARREL_CACHE_MAX_THUMB_BYTES: negative")
	}
	if c.CacheMaxThumbEntries < 0 {
		return fmt.Errorf("CARREL_CACHE_MAX_THUMB_ENTRIES: negative")
	}
	if _, err := c.ParseLogLevel(); err != nil {
		return err
	}
	if c.DAVConnectTimeout <= 0 {
		return fmt.Errorf("CARREL_DAV_CONNECT_TIMEOUT: must be positive")
	}
	if c.DAVRequestTimeout <= 0 {
		return fmt.Errorf("CARREL_DAV_REQUEST_TIMEOUT: must be positive")
	}
	if c.DAVMaxResponseBytes <= 0 {
		return fmt.Errorf("CARREL_DAV_MAX_RESPONSE_BYTES: must be positive")
	}
	if c.DAVMaxRedirects < 0 {
		return fmt.Errorf("CARREL_DAV_MAX_REDIRECTS: negative")
	}
	if c.ProgressSourceTimeout <= 0 {
		return fmt.Errorf("CARREL_PROGRESS_SOURCE_TIMEOUT: must be positive")
	}
	if c.ProgressTotalTimeout <= 0 {
		return fmt.Errorf("CARREL_PROGRESS_TOTAL_TIMEOUT: must be positive")
	}
	return nil
}

// ParseLogLevel maps CARREL_LOG_LEVEL to a slog level (C-12).
func (c *Config) ParseLogLevel() (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("CARREL_LOG_LEVEL: unknown level %q", c.LogLevel)
	}
}

// A malformed value is a configuration error the operator can read and fix,
// not a panic in the middle of startup.
func atoi(key, val string) (int, error) {
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("%s: not a number: %q", key, val)
	}
	return n, nil
}

func atoi64(key, val string) (int64, error) {
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: not a number: %q", key, val)
	}
	return n, nil
}

func seconds(key, val string) (time.Duration, error) {
	n, err := atoi(key, val)
	if err != nil {
		return 0, err
	}
	return time.Duration(n) * time.Second, nil
}

func boolVal(key, val string) (bool, error) {
	switch strings.ToLower(val) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%s: not a bool: %q", key, val)
	}
}
