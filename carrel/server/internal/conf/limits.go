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

	MaxLiveFanouts       = 4
	FanoutParallelism    = 4
	MinPasswordLen       = 8
	MinMasterPasswordLen = 16
)
