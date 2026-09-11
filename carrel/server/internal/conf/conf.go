package conf

import (
	"bufio"
	"fmt"
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
	mergeEnv(c)
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
		applyKV(c, strings.TrimSpace(key), strings.TrimSpace(val))
	}
	return sc.Err()
}

func mergeEnv(c *Config) {
	for _, kv := range os.Environ() {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		applyKV(c, key, val)
	}
}

func applyKV(c *Config, key, val string) {
	switch key {
	case "CARREL_PORT":
		c.Port = atoi(key, val)
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
		c.DAVConnectTimeout = seconds(key, val)
	case "CARREL_DAV_REQUEST_TIMEOUT":
		c.DAVRequestTimeout = seconds(key, val)
	case "CARREL_DAV_MAX_RESPONSE_BYTES":
		c.DAVMaxResponseBytes = atoi64(key, val)
	case "CARREL_DAV_MAX_REDIRECTS":
		c.DAVMaxRedirects = atoi(key, val)
	case "CARREL_CACHE_COLLECTION_TTL":
		c.CacheCollectionTTL = seconds(key, val)
	case "CARREL_CACHE_MAX_COLLECTIONS":
		c.CacheMaxCollections = atoi(key, val)
	case "CARREL_CACHE_MAX_ETAG_ENTRIES":
		c.CacheMaxETagEntries = atoi(key, val)
	case "CARREL_CACHE_MAX_BODY_BYTES":
		c.CacheMaxBodyBytes = atoi64(key, val)
	case "CARREL_CACHE_MAX_PROCESS_BYTES":
		c.CacheMaxProcessBytes = atoi64(key, val)
	case "CARREL_CACHE_MAX_THUMB_BYTES":
		c.CacheMaxThumbBytes = atoi64(key, val)
	case "CARREL_CACHE_MAX_THUMB_ENTRIES":
		c.CacheMaxThumbEntries = atoi(key, val)
	case "CARREL_PHOTO_MAX_SIDE":
		c.PhotoMaxSide = atoi(key, val)
	case "CARREL_PHOTO_JPEG_QUALITY":
		c.PhotoJPEGQuality = atoi(key, val)
	case "CARREL_PHOTO_MAX_PIXELS":
		c.PhotoMaxPixels = atoi64(key, val)
	case "CARREL_PHOTO_THUMB_SIDE":
		c.PhotoThumbSide = atoi(key, val)
	case "CARREL_PHOTO_MAX_UPLOAD_BYTES":
		c.PhotoMaxUploadBytes = atoi64(key, val)
	case "CARREL_IMPORT_MAX_BYTES":
		c.ImportMaxBytes = atoi64(key, val)
	case "CARREL_IMPORT_MAX_RECORDS":
		c.ImportMaxRecords = atoi(key, val)
	case "CARREL_FILES_MAX_ENTRIES":
		c.FilesMaxEntries = atoi(key, val)
	case "CARREL_PROGRESS_MODE":
		c.ProgressMode = val
	case "CARREL_PROGRESS_POLL_MILLIS":
		c.ProgressPollMillis = atoi(key, val)
	case "CARREL_PROGRESS_SOURCE_TIMEOUT":
		c.ProgressSourceTimeout = seconds(key, val)
	case "CARREL_PROGRESS_TOTAL_TIMEOUT":
		c.ProgressTotalTimeout = seconds(key, val)
	case "CARREL_DUPLICATES_THRESHOLD":
		c.DuplicatesThreshold = atoi(key, val)
	case "CARREL_UNLOCK_TTL":
		c.UnlockTTL = seconds(key, val)
	case "CARREL_MAIL_ENABLED":
		c.MailEnabled = boolVal(key, val)
	case "CARREL_MAIL_MAX_ATTACH_BYTES":
		c.MailMaxAttachBytes = atoi64(key, val)
	case "CARREL_MAIL_MAX_MESSAGE_BYTES":
		c.MailMaxMessageBytes = atoi64(key, val)
	case "CARREL_MAIL_MAX_RECIPIENTS":
		c.MailMaxRecipients = atoi(key, val)
	case "CARREL_MAIL_MAX_PER_DAY":
		c.MailMaxPerDay = atoi(key, val)
	case "CARREL_DAVLOOM_PORT":
		if val == "" {
			c.DAVloomPort = 0
		} else {
			c.DAVloomPort = atoi(key, val)
		}
	}
}

func (c *Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("CARREL_PORT: out of range %d", c.Port)
	}
	if c.DAVloomPort < 0 || c.DAVloomPort > 65535 {
		return fmt.Errorf("CARREL_DAVLOOM_PORT: out of range %d", c.DAVloomPort)
	}
	if c.DataDir == "" {
		return fmt.Errorf("CARREL_DATA_DIR: empty")
	}
	return nil
}

func atoi(key, val string) int {
	n, err := strconv.Atoi(val)
	if err != nil {
		panic(fmt.Sprintf("%s: %v", key, err))
	}
	return n
}

func atoi64(key, val string) int64 {
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("%s: %v", key, err))
	}
	return n
}

func seconds(key, val string) time.Duration {
	return time.Duration(atoi(key, val)) * time.Second
}

func boolVal(key, val string) bool {
	switch strings.ToLower(val) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		panic(fmt.Sprintf("%s: not a bool: %q", key, val))
	}
}
