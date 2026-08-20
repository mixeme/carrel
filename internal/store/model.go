// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// StateVersion is the on-disk format version. A file written by a newer
// version is refused rather than guessed at; older versions are migrated
// forward on load.
const StateVersion = 1

// Role is what a user may do. There are only two (§5.1).
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return r == RoleUser || r == RoleAdmin }

// TLSMode is the transport security of the SMTP relay (§5.3).
type TLSMode string

const (
	TLSStartTLS TLSMode = "starttls"
	TLSImplicit TLSMode = "implicit"
	// TLSNone is for a relay on localhost only.
	TLSNone TLSMode = "none"
)

// Valid reports whether m is a known TLS mode.
func (m TLSMode) Valid() bool {
	return m == TLSStartTLS || m == TLSImplicit || m == TLSNone
}

// User is one account. Everything that can decrypt the user's own data hangs
// off their password: KEKSalt derives the KEK, the KEK unwraps WrappedDEK, and
// the DEK opens Secrets. The server can do none of that on its own (§4, §5.1).
type User struct {
	ID        string    `json:"id"`
	Login     string    `json:"login"`
	Email     string    `json:"email,omitempty"`
	Role      Role      `json:"role"`
	Disabled  bool      `json:"disabled,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by,omitempty"`

	// LastLoginAt is zero until the first successful login.
	LastLoginAt time.Time `json:"last_login_at,omitempty"`
	// MustChangePassword is set for accounts created with a temporary
	// password and for accounts restored from escrow (§5.2, §5.4).
	MustChangePassword bool `json:"must_change_password,omitempty"`
	// Unconfirmed is set until a self-registered account follows the email
	// link. They cannot sign in until then (§5.2).
	Unconfirmed bool `json:"unconfirmed,omitempty"`

	// Auth verifies the login password. Its salt is never the KEK salt (§4).
	Auth *crypto.PasswordHash `json:"auth"`
	// KEKSalt and KEKParams derive the key that unwraps WrappedDEK. The
	// parameters travel with the record so costs can be raised later without
	// locking anyone out.
	KEKSalt    []byte        `json:"kek_salt"`
	KEKParams  crypto.Params `json:"kek_params"`
	WrappedDEK []byte        `json:"wrapped_dek"`

	// EscrowDEK is a copy of the DEK encrypted to the escrow public key. It
	// exists only for users covered by escrow, and its absence is what makes
	// recovery impossible for everyone else (§5.4).
	EscrowDEK []byte `json:"escrow_dek,omitempty"`
	// EscrowNoticeSeen records that the user has been told escrow is active.
	EscrowNoticeSeen bool `json:"escrow_notice_seen,omitempty"`
	// EscrowRecoveredAt is the last time an administrator recovered this
	// account. Shown in the profile, never cleared (§5.4).
	EscrowRecoveredAt time.Time `json:"escrow_recovered_at,omitempty"`

	// Secrets is the per-user blob sealed under the DEK: DAV credentials from
	// stage 2 onward. Empty in stage 1, but the field fixes where they go.
	Secrets []byte `json:"secrets,omitempty"`
	// DAVAccountCount is the number of enabled DAV accounts, maintained without
	// decrypting Secrets so the admin list can show it (§5.5).
	DAVAccountCount int `json:"dav_account_count,omitempty"`

	// PendingEmail holds an address awaiting confirmation. The token digest
	// and expiry live here too; only the plaintext token is ever mailed (§5.3).
	PendingEmail         string    `json:"pending_email,omitempty"`
	EmailChangeTokenHash []byte    `json:"email_change_token_hash,omitempty"`
	EmailChangeExpires   time.Time `json:"email_change_expires,omitempty"`

	// WeekStart is "monday" or "sunday"; empty defaults to Monday, the week
	// the agenda's Week preset used before this preference existed (2.6.C7).
	// Unlike theme and row density it cannot live in the browser alone: the
	// week's boundary is computed on the server before the page is sent, so
	// there is nothing client-side to apply a local-only preference to.
	WeekStart string `json:"week_start,omitempty"`

	// ContactSort and NoteSort remember the order chosen on a per-collection
	// list (2.6.B2, closed properly by 2.6.G11): the parameter was applied
	// on the request it arrived on and forgotten the moment it did not, which
	// is not what "remembered in the profile" promised.
	ContactSort string `json:"contact_sort,omitempty"`
	NoteSort    string `json:"note_sort,omitempty"`
}

// Activated reports whether the user has set a password. An invited user has
// no credentials at all until they accept (§5.2).
func (u *User) Activated() bool { return u != nil && u.Auth != nil }

// IsAdmin reports whether the user may manage accounts.
func (u *User) IsAdmin() bool { return u != nil && u.Role == RoleAdmin }

// Clone returns a deep copy, so callers cannot reach into stored state.
func (u *User) Clone() *User {
	if u == nil {
		return nil
	}
	out := *u
	out.Auth = cloneHash(u.Auth)
	out.KEKSalt = cloneBytes(u.KEKSalt)
	out.WrappedDEK = cloneBytes(u.WrappedDEK)
	out.EscrowDEK = cloneBytes(u.EscrowDEK)
	out.Secrets = cloneBytes(u.Secrets)
	out.EmailChangeTokenHash = cloneBytes(u.EmailChangeTokenHash)
	return &out
}

// InviteStatus is what the administrator sees next to an invite. It is a
// derived value: the store keeps timestamps, not a status field.
type InviteStatus string

const (
	InvitePending  InviteStatus = "pending"
	InviteAccepted InviteStatus = "accepted"
	InviteRevoked  InviteStatus = "revoked"
	InviteExpired  InviteStatus = "expired"
)

// SendStatus is the outcome of the last delivery attempt. It is informational:
// an invite is usable whether or not the mail ever went out (§5.3).
type SendStatus string

const (
	// SendNotConfigured means SMTP is unset, so the link must be handed over
	// by other means.
	SendNotConfigured SendStatus = "not_configured"
	SendPending       SendStatus = "pending"
	SendOK            SendStatus = "sent"
	SendFailed        SendStatus = "failed"
)

// InviteDelivery distinguishes how an invitation reaches the new user.
type InviteDelivery string

const (
	// InviteDeliveryLink: the administrator copies the link and shares it.
	InviteDeliveryLink InviteDelivery = "link"
	// InviteDeliveryEmail: the link is emailed to the address the administrator
	// provides; the administrator never sees it.
	InviteDeliveryEmail InviteDelivery = "email"
)

func (d InviteDelivery) Valid() bool {
	return d == InviteDeliveryLink || d == InviteDeliveryEmail
}

// Invite is a pending account. Only the token digest is stored; the token
// itself is returned once, at creation, and then exists solely in the link
// (§5.3).
type Invite struct {
	ID        string         `json:"id"`
	Login     string         `json:"login"`
	Email     string         `json:"email,omitempty"`
	Role      Role           `json:"role"`
	Delivery  InviteDelivery `json:"delivery,omitempty"`
	TokenHash []byte         `json:"token_hash"`

	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`

	AcceptedAt time.Time `json:"accepted_at,omitempty"`
	RevokedAt  time.Time `json:"revoked_at,omitempty"`

	LastSentAt time.Time  `json:"last_sent_at,omitempty"`
	SendStatus SendStatus `json:"send_status,omitempty"`
	SendError  string     `json:"send_error,omitempty"`
}

// Status derives the invite state at time now.
func (i *Invite) Status(now time.Time) InviteStatus {
	switch {
	case !i.AcceptedAt.IsZero():
		return InviteAccepted
	case !i.RevokedAt.IsZero():
		return InviteRevoked
	case !now.Before(i.ExpiresAt):
		return InviteExpired
	default:
		return InvitePending
	}
}

// Usable reports whether the invite can still be accepted.
func (i *Invite) Usable(now time.Time) bool { return i.Status(now) == InvitePending }

// Clone returns a deep copy.
func (i *Invite) Clone() *Invite {
	if i == nil {
		return nil
	}
	out := *i
	out.TokenHash = cloneBytes(i.TokenHash)
	return &out
}

// SMTP is the relay configuration. The password is sealed with the server key,
// so it is readable by the process but not by anyone reading the volume (§5.3).
type SMTP struct {
	Host        string  `json:"host,omitempty"`
	Port        int     `json:"port,omitempty"`
	TLS         TLSMode `json:"tls,omitempty"`
	Username    string  `json:"username,omitempty"`
	Password    []byte  `json:"password,omitempty"`
	FromAddress string  `json:"from_address,omitempty"`
	FromName    string  `json:"from_name,omitempty"`
}

// Configured reports whether mail can be attempted at all. Everything that
// depends on mail has to keep working when this is false (§21).
func (s SMTP) Configured() bool { return s.Host != "" && s.Port > 0 && s.FromAddress != "" }

// Clone returns a deep copy.
func (s SMTP) Clone() SMTP {
	s.Password = cloneBytes(s.Password)
	return s
}

// EscrowSettings governs the optional key deposit scheme. Enabled is false on
// a fresh volume and turning it on affects only accounts created afterwards:
// an existing DEK cannot be deposited without its owner's password (§5.4).
type EscrowSettings struct {
	Enabled bool `json:"enabled,omitempty"`
	// ForbidOptOut is read from older state files and ignored. Withdrawal is
	// always the key owner's decision (§5.4).
	ForbidOptOut bool `json:"forbid_opt_out,omitempty"`
	// Config holds the recovery key pair. It survives Enabled going false so
	// that already-deposited copies stay recoverable.
	Config *crypto.Escrow `json:"config,omitempty"`
}

// Active reports whether new users should get a deposited DEK copy.
func (e EscrowSettings) Active() bool { return e.Enabled && e.Config != nil }

// Clone returns a deep copy.
func (e EscrowSettings) Clone() EscrowSettings {
	if e.Config != nil {
		c := *e.Config
		c.PublicKey = cloneBytes(e.Config.PublicKey)
		c.SealedKey = cloneBytes(e.Config.SealedKey)
		c.MasterSalt = cloneBytes(e.Config.MasterSalt)
		e.Config = &c
	}
	return e
}

// Default durations for settings that later stages let the administrator
// change. They are stored as seconds so the state file stays readable when
// decrypted for debugging.
const (
	DefaultInviteTTL       = 72 * time.Hour
	DefaultSessionIdle     = 12 * time.Hour
	DefaultSessionAbsolute = 7 * 24 * time.Hour
)

// Settings are the global options from §5.5. Fields that later stages consume
// are present with their defaults so the format does not change again.
type Settings struct {
	// SelfRegistration publishes the public sign-up form. Off by default so
	// a private instance is not open to visitors (§5.2).
	SelfRegistration bool `json:"self_registration,omitempty"`

	InviteTTLSeconds       int64 `json:"invite_ttl_seconds"`
	SessionIdleSeconds     int64 `json:"session_idle_seconds"`
	SessionAbsoluteSeconds int64 `json:"session_absolute_seconds"`

	SMTP   SMTP           `json:"smtp"`
	Escrow EscrowSettings `json:"escrow"`
}

// InviteTTL is how long a new invite stays usable.
func (s Settings) InviteTTL() time.Duration { return secondsOr(s.InviteTTLSeconds, DefaultInviteTTL) }

// SessionIdle is how long a session survives without a request.
func (s Settings) SessionIdle() time.Duration {
	return secondsOr(s.SessionIdleSeconds, DefaultSessionIdle)
}

// SessionAbsolute is the hard lifetime of a session regardless of activity.
func (s Settings) SessionAbsolute() time.Duration {
	return secondsOr(s.SessionAbsoluteSeconds, DefaultSessionAbsolute)
}

// Clone returns a deep copy.
func (s Settings) Clone() Settings {
	s.SMTP = s.SMTP.Clone()
	s.Escrow = s.Escrow.Clone()
	return s
}

func defaultSettings() Settings {
	return Settings{
		InviteTTLSeconds:       int64(DefaultInviteTTL / time.Second),
		SessionIdleSeconds:     int64(DefaultSessionIdle / time.Second),
		SessionAbsoluteSeconds: int64(DefaultSessionAbsolute / time.Second),
	}
}

func secondsOr(v int64, fallback time.Duration) time.Duration {
	if v <= 0 {
		return fallback
	}
	return time.Duration(v) * time.Second
}

// Audit actions. The log records that something happened and to whom, never
// what was in it: no passwords, no tokens, no object contents (§5.5).
const (
	ActionBootstrap         = "bootstrap"
	ActionLogin             = "login"
	ActionLoginFailed       = "login_failed"
	ActionLogout            = "logout"
	ActionUserCreate        = "user_create"
	ActionUserDisable       = "user_disable"
	ActionUserEnable        = "user_enable"
	ActionUserDelete        = "user_delete"
	ActionUserRole          = "user_role_change"
	ActionUserEmail         = "user_email_change"
	ActionPasswordChange    = "password_change"
	ActionPasswordReset     = "password_reset"
	ActionSessionsKilled    = "sessions_killed"
	ActionSessionEnd        = "session_end"
	ActionSessionsEndOthers = "sessions_end_others"
	ActionAuditExport       = "audit_export"
	ActionInviteCreate      = "invite_create"
	ActionInviteAccept      = "invite_accept"
	ActionInviteRevoke      = "invite_revoke"
	ActionInviteExtend      = "invite_extend"
	ActionInviteSend        = "invite_send"
	ActionInviteResend      = "invite_resend"
	ActionSettings          = "settings_update"
	ActionSMTPTest          = "smtp_test"
	ActionDAVTest           = "dav_test"
	ActionDAVExercise       = "dav_exercise"
	ActionInstallCheck      = "install_check"
	ActionEmailConfirm      = "email_confirm"
	ActionEscrowEnable      = "escrow_enable"
	ActionEscrowDisable     = "escrow_disable"
	ActionEscrowOptIn       = "escrow_opt_in"
	ActionEscrowOptOut      = "escrow_opt_out"
	ActionEscrowRecover     = "escrow_recover"
)

// Actor identifies who caused an audited change.
type Actor struct {
	ID    string
	Login string
	IP    string
}

// System is the actor for changes with no human behind them.
var System = Actor{Login: "system"}

// AuditEntry is one append-only record. Entries are written in the same atomic
// commit as the change they describe, so the log cannot drift from the state.
type AuditEntry struct {
	Seq         int64     `json:"seq"`
	At          time.Time `json:"at"`
	Action      string    `json:"action"`
	ActorID     string    `json:"actor_id,omitempty"`
	ActorLogin  string    `json:"actor_login,omitempty"`
	TargetID    string    `json:"target_id,omitempty"`
	TargetLogin string    `json:"target_login,omitempty"`
	IP          string    `json:"ip,omitempty"`
	// Detail is a short human-readable note. Callers must keep secrets out of
	// it; the store does not sanitise.
	Detail string `json:"detail,omitempty"`
}

// AuditFilter narrows a log query. Zero fields mean "no constraint".
type AuditFilter struct {
	Action   string
	ActorID  string
	TargetID string
	Since    time.Time
	// Categories, when non-empty, keeps an entry only if its Action is one of
	// these. It narrows alongside Action rather than replacing it, so the
	// admin audit log's exact-action dropdown and its All/Security/Failures
	// segment (2.6.C6) can be applied together.
	Categories []string
	// Limit caps the result, counting from the newest entry.
	Limit int
}

// MaxAuditEntries bounds the log so a long-lived instance cannot grow the
// state file without limit. Trimming drops the oldest entries; nothing is ever
// rewritten in place.
const MaxAuditEntries = 10000

// State is the whole encrypted store: everything the service knows before
// anyone logs in.
type State struct {
	Version   int          `json:"version"`
	CreatedAt time.Time    `json:"created_at"`
	Users     []*User      `json:"users"`
	Invites   []*Invite    `json:"invites"`
	Settings  Settings     `json:"settings"`
	Audit     []AuditEntry `json:"audit"`
	AuditSeq  int64        `json:"audit_seq"`
}

func newState(now time.Time) *State {
	return &State{
		Version:   StateVersion,
		CreatedAt: now,
		Settings:  defaultSettings(),
	}
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func cloneHash(h *crypto.PasswordHash) *crypto.PasswordHash {
	if h == nil {
		return nil
	}
	out := *h
	out.Salt = cloneBytes(h.Salt)
	out.Hash = cloneBytes(h.Hash)
	return &out
}
