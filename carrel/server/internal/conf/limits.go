package conf

import "time"

// Product limits from architecture.md — not C-12, not CARREL_*, not settings UI.
const (
	SessionIdleTimeout     = 12 * time.Hour
	SessionAbsoluteTimeout = 7 * 24 * time.Hour
	InviteTTL              = 7 * 24 * time.Hour
	EmailConfirmTTL        = 48 * time.Hour
	ForgotLinkTTL          = 24 * time.Hour
	ConsoleWipeLinkTTL     = 15 * time.Minute
	DeviceProfileLinkTTL   = 5 * time.Minute
	PublicLinkDefaultTTL   = 30 * 24 * time.Hour
	MaxFormBodyBytes       = 1 << 20 // 1 MiB

	LoginRatePerAccount = 5   // per minute
	LoginRatePerIP      = 20  // per minute
	AnonTokenRatePerIP  = 30  // per minute
	SessionRateLimit    = 120 // per minute, post-login

	MaxLiveFanouts    = 4
	FanoutParallelism = 4
	SearchHistoryLen  = 10 // S-15.3
	// MaxKeptFanouts is how many tasks a session keeps, running or finished:
	// as many as the history has rows, so a query picked from it can open
	// the result it names (S-15.3).
	MaxKeptFanouts       = SearchHistoryLen
	MinPasswordLen       = 8
	MinMasterPasswordLen = 16

	// PhotoMaxZoom is the closest frame of a contact photo, in percent of the
	// largest square that fits: 400 is a quarter of its side (S-13).
	PhotoMaxZoom = 400
)
