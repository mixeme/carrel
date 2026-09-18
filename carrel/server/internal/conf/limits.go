package conf

import "time"

// Product limits from architecture.md — not C-12, not CARREL_*. The first
// three are the defaults of settings the administrator changes on
// /admin/settings (owner's decision 19.09.2026).
const (
	SessionIdleTimeout     = 12 * time.Hour
	SessionAbsoluteTimeout = 30 * 24 * time.Hour
	InviteTTL              = 72 * time.Hour
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

	// FileThumbMaxBytes is the largest image a folder shows as its own
	// thumbnail: the original is what is served, so a larger one gets the icon
	// of its type (S-17).
	FileThumbMaxBytes = 512 << 10
	// AttachInlineMax is the largest record an attachment is written into
	// (S-20): up to it the file travels inside the event, the task or the note;
	// past it the file goes to the attachment folder and the record keeps a
	// link. A collection's max-resource-size can only lower it.
	AttachInlineMax = 2 << 20
	// TransferQueueLen is how many rows the transfer queue of a session keeps
	// (S-17.3). Past it the oldest row that asks nothing goes, then the oldest
	// question with the upload it holds.
	TransferQueueLen = 50
)
