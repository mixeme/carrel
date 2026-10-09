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

	// FileThumbMaxBytes is the largest picture a thumbnail is made for: the
	// server reads the file whole to make one, so a larger picture gets the
	// icon of its type and says «too large» (S-17.1; the owner's decision).
	FileThumbMaxBytes = 16 << 20
	// FileThumbSide is the square a file's thumbnail fits, in pixels: a tile
	// and the panel's preview on a dense screen (s-files-5, s-files-1).
	FileThumbSide = 480
	// FileThumbBuilds is how many thumbnails are made at once in the whole
	// process: each holds a picture decoded, tens of megabytes of it.
	FileThumbBuilds = 2
	// ImageHeadBytes is how much of an image is read for its width and
	// height when it is the one file chosen (S-17): one GET with Range:
	// bytes=0-65535, and no more of an answer that came whole. A header
	// further in leaves the size unsaid.
	ImageHeadBytes = 64 << 10
	// AttachInlineMax is the largest record an attachment is written into
	// (S-20): up to it the file travels inside the event, the task or the note;
	// past it the file goes to the attachment folder and the record keeps a
	// link. A collection's max-resource-size can only lower it.
	AttachInlineMax = 2 << 20
	// TransferQueueLen is how many rows the transfer queue of a session keeps
	// (S-17.3). Past it the oldest row that asks nothing goes, then the oldest
	// question with the upload it holds.
	TransferQueueLen = 50
	// ColumnsChoiceAge is how long a browser keeps the columns a person
	// chose for a list (S-9.6): a standing preference of this browser,
	// renewed by every choice.
	ColumnsChoiceAge = 365 * 24 * time.Hour
)
